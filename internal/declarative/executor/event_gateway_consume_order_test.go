package executor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/require"
)

type consumeOrderingAPI struct {
	helpers.EventGatewayConsumePolicyAPI
	policies []kkComps.EventGatewayPolicy
	chain    []string
	moves    []string
	fail     bool
}

func (a *consumeOrderingAPI) CreateEventGatewayVirtualClusterConsumePolicy(_ context.Context,
	r kkOps.CreateEventGatewayVirtualClusterConsumePolicyRequest, _ ...kkOps.Option,
) (*kkOps.CreateEventGatewayVirtualClusterConsumePolicyResponse, error) {
	data, err := json.Marshal(r.EventGatewayConsumePolicyCreate)
	if err != nil {
		return nil, err
	}
	var policy kkComps.EventGatewayPolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, err
	}
	policy.ID = "parent-new-id"
	if policy.Name != nil {
		policy.ID = "created-" + *policy.Name
	}
	a.policies = append(a.policies, policy)
	a.chain = append(a.chain, policy.ID)
	return &kkOps.CreateEventGatewayVirtualClusterConsumePolicyResponse{EventGatewayPolicy: &policy}, nil
}

func (a *consumeOrderingAPI) ListEventGatewayVirtualClusterConsumePolicies(context.Context,
	kkOps.ListEventGatewayVirtualClusterConsumePoliciesRequest, ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClusterConsumePoliciesResponse, error) {
	return &kkOps.ListEventGatewayVirtualClusterConsumePoliciesResponse{ListConsumePoliciesResponse: a.policies}, nil
}

func (a *consumeOrderingAPI) GetEventGatewayVirtualClusterConsumePolicyChain(context.Context,
	kkOps.GetEventGatewayVirtualClusterConsumePolicyChainRequest, ...kkOps.Option,
) (*kkOps.GetEventGatewayVirtualClusterConsumePolicyChainResponse, error) {
	return &kkOps.GetEventGatewayVirtualClusterConsumePolicyChainResponse{
		EventGatewayPolicyChainResponse: &kkComps.EventGatewayPolicyChainResponse{Policies: a.chain},
	}, nil
}

func (a *consumeOrderingAPI) MoveEventGatewayVirtualClusterConsumePolicy(_ context.Context,
	r kkOps.MoveEventGatewayVirtualClusterConsumePolicyRequest, _ ...kkOps.Option,
) (*kkOps.MoveEventGatewayVirtualClusterConsumePolicyResponse, error) {
	if a.fail {
		return nil, errors.New("ordering failed")
	}
	a.moves = append(a.moves, r.PolicyID)
	parents := make(map[string]string)
	for _, policy := range a.policies {
		parent := ""
		if policy.ParentPolicyID != nil {
			parent = *policy.ParentPolicyID
		}
		parents[policy.ID] = parent
	}
	groups := make(map[string][]string)
	for _, id := range a.chain {
		groups[parents[id]] = append(groups[parents[id]], id)
	}
	parent := parents[r.PolicyID]
	siblings := groups[parent]
	pos := slices.Index(siblings, r.PolicyID)
	index := int(r.MoveEventGatewayPolicy.Index)
	if pos < 0 || index < 0 || index >= len(siblings) {
		return nil, errors.New("invalid sibling move index")
	}
	siblings = slices.Delete(siblings, pos, pos+1)
	groups[parent] = slices.Insert(siblings, index, r.PolicyID)
	a.chain = nil
	var appendChildren func(string)
	appendChildren = func(parent string) {
		for _, id := range groups[parent] {
			a.chain = append(a.chain, id)
			appendChildren(id)
		}
	}
	appendChildren("")
	return &kkOps.MoveEventGatewayVirtualClusterConsumePolicyResponse{}, nil
}

func TestConsumeOrderSavedPlanExecution(t *testing.T) {
	for _, parent := range []string{"", "parent-new-id"} {
		for _, fail := range []bool{false, true} {
			t.Run(parent+"/fail="+map[bool]string{false: "false", true: "true"}[fail], func(t *testing.T) {
				api := &consumeOrderingAPI{chain: []string{"extra-a", "second-new-id", "extra-b", "first-new-id"}, fail: fail}
				for _, item := range []struct{ id, name string }{
					{"extra-a", "extra-a"}, {"second-new-id", "second"}, {"extra-b", "extra-b"}, {"first-new-id", "first"},
				} {
					policy := kkComps.EventGatewayPolicy{ID: item.id, Name: new(item.name)}
					if parent != "" {
						policy.ParentPolicyID = new(parent)
					}
					api.policies = append(api.policies, policy)
				}
				intent := &planner.ConsumePolicyOrder{Names: []string{"first", "second"}}
				if parent != "" {
					api.policies = append(api.policies, kkComps.EventGatewayPolicy{ID: parent, Name: new("schema")})
					api.chain = append([]string{parent}, api.chain...)
					intent.ParentName = "schema"
				}
				plan := planner.NewPlan(planner.CurrentPlanVersion, "test", planner.PlanModeApply)
				plan.AddChange(planner.PlannedChange{
					ID: "order", ResourceType: planner.ResourceTypeEventGatewayConsumePolicy,
					ResourceID: "vc", ResourceRef: "order", Action: planner.ActionUpdate, ConsumePolicyOrder: intent,
					References: map[string]planner.ReferenceInfo{
						planner.FieldEventGatewayID: {ID: "gw"}, planner.FieldEventGatewayVirtualClusterID: {ID: "vc"},
					},
				})
				plan.ExecutionOrder = []string{"order"}
				data, err := json.Marshal(plan)
				require.NoError(t, err)
				var saved planner.Plan
				require.NoError(t, json.Unmarshal(data, &saved))
				e := New(state.NewClient(state.ClientConfig{EventGatewayConsumePolicyAPI: api}), nil, false)
				result := e.Execute(t.Context(), &saved)
				if fail {
					require.Equal(t, 1, result.FailureCount)
					require.Zero(t, result.SuccessCount)
					require.Contains(t, result.Errors[0].Error, "ordering failed")
					return
				}
				require.Zero(t, result.FailureCount, "%+v", result.Errors)
				expected := []string{"first-new-id", "second-new-id", "extra-a", "extra-b"}
				if parent != "" {
					expected = append([]string{parent}, expected...)
				}
				require.Equal(t, expected, api.chain)
				require.Equal(t, []string{"first-new-id", "second-new-id"}, api.moves)
				api.moves = nil
				result = e.Execute(t.Context(), &saved)
				require.Zero(t, result.FailureCount)
				require.Empty(t, api.moves)
			})
		}
	}
}

func TestConsumeOrderSavedPlanResolvesCreatedUnnamedParent(t *testing.T) {
	api := &consumeOrderingAPI{
		policies: []kkComps.EventGatewayPolicy{{ID: "existing-id", Name: new("existing")}},
		chain:    []string{"existing-id"},
	}
	plan := planner.NewPlan(planner.CurrentPlanVersion, "test", planner.PlanModeApply)
	references := map[string]planner.ReferenceInfo{
		planner.FieldEventGatewayID: {ID: "gw"}, planner.FieldEventGatewayVirtualClusterID: {ID: "vc"},
	}
	plan.AddChange(planner.PlannedChange{
		ID: "create-parent", ResourceType: planner.ResourceTypeEventGatewayConsumePolicy,
		ResourceRef: "schema-ref", Action: planner.ActionCreate, References: references,
		Fields: map[string]any{
			planner.FieldType: "schema_validation", planner.FieldConfig: map[string]any{planner.FieldType: "json"},
		},
	})
	for _, name := range []string{"second", "first"} {
		plan.AddChange(planner.PlannedChange{
			ID: "create-" + name, ResourceType: planner.ResourceTypeEventGatewayConsumePolicy,
			ResourceRef: name + "-ref", Action: planner.ActionCreate, References: references,
			Fields: map[string]any{
				planner.FieldType: "skip_record", planner.FieldName: name,
				planner.FieldParentPolicyID: "parent-new-id",
			},
			DependsOn: []string{"create-parent"},
		})
	}
	for i, intent := range []*planner.ConsumePolicyOrder{
		{Names: []string{"schema-ref", "existing"}, Refs: []string{"schema-ref", "existing-ref"}},
		{
			ParentRef: "schema-ref", ParentName: "schema-ref", Names: []string{"first", "second"},
			Refs: []string{"first-ref", "second-ref"},
		},
	} {
		plan.AddChange(planner.PlannedChange{
			ID:           []string{"order-root", "order-children"}[i],
			ResourceType: planner.ResourceTypeEventGatewayConsumePolicy, ResourceID: "vc",
			ResourceRef: "order", Action: planner.ActionUpdate, References: references,
			ConsumePolicyOrder: intent, DependsOn: []string{"create-first", "create-second"},
		})
	}
	for _, change := range plan.Changes {
		plan.ExecutionOrder = append(plan.ExecutionOrder, change.ID)
	}
	data, err := json.Marshal(plan)
	require.NoError(t, err)
	var saved planner.Plan
	require.NoError(t, json.Unmarshal(data, &saved))
	e := New(state.NewClient(state.ClientConfig{EventGatewayConsumePolicyAPI: api}), nil, false)
	ctx := context.WithValue(t.Context(), log.LoggerKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
	result := e.Execute(ctx, &saved)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Equal(t, 5, result.SuccessCount)
	require.Nil(t, api.policies[1].Name)
	require.Equal(t, []string{"parent-new-id", "created-first", "created-second", "existing-id"}, api.chain)
	require.Equal(t, []string{"parent-new-id", "created-first"}, api.moves)
}

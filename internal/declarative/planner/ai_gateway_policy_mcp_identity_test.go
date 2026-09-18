package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayPolicyAndMCPServerPlannerIdentity(t *testing.T) {
	const (
		cachedID  = "11111111-1111-4111-8111-111111111111"
		refID     = "22222222-2222-4222-8222-222222222222"
		nameID    = "33333333-3333-4333-8333-333333333333"
		missingID = "44444444-4444-4444-8444-444444444444"
	)
	for _, kind := range []string{ResourceTypeAIGatewayPolicy, ResourceTypeAIGatewayMCPServer} {
		t.Run(kind, func(t *testing.T) {
			for _, tt := range []struct {
				name        string
				ref         string
				cachedID    string
				desiredName string
				wantAction  ActionType
			}{
				{"name wins over UUID ref", refID, "", "desired-name", ActionUpdate},
				{"name wins over cached ID", "local-ref", cachedID, "desired-name", ActionUpdate},
				{"missing IDs do not hide name match", missingID, missingID, "desired-name", ActionUpdate},
				{"UUID ref does not rename or retain", refID, "", "new-name", ActionCreate},
				{"cached ID does not rename or retain", "local-ref", cachedID, "new-name", ActionCreate},
			} {
				t.Run(tt.name, func(t *testing.T) {
					var policies []kkComps.AIGatewayPolicy
					var servers []kkComps.AIGatewayMCPServer
					for _, identity := range []struct{ id, name string }{
						{cachedID, "cached-name"}, {refID, "ref-name"}, {nameID, "desired-name"},
					} {
						policy := testAIGatewayPolicy()
						policy.ID, policy.Name = identity.id, identity.name
						policies = append(policies, policy)
						servers = append(servers, testAIGatewayMCPServer(t, identity.id, identity.name))
					}
					policyAPI := &policyIdentityAPI{testAIGatewayPolicyAPI: &testAIGatewayPolicyAPI{policies: policies}}
					serverAPI := &mcpServerIdentityAPI{
						testAIGatewayMCPServerAPI: &testAIGatewayMCPServerAPI{servers: servers},
					}
					p := NewPlanner(state.NewClient(state.ClientConfig{
						AIGatewayPoliciesAPI:   policyAPI,
						AIGatewayMCPServersAPI: serverAPI,
					}), slog.Default())
					p.resources = &resources.ResourceSet{}
					plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
					var reads [][2]string
					switch kind {
					case ResourceTypeAIGatewayPolicy:
						desired := testAIGatewayPolicyResource(t)
						desired.Ref, desired.Name = tt.ref, tt.desiredName
						desired.DisplayName = "Updated child"
						desired.SetKonnectID(tt.cachedID)
						p.resources.AIGatewayPolicies = []resources.AIGatewayPolicyResource{desired}
						err := p.planAIGatewayPolicyChanges(
							t.Context(),
							"default",
							"support-gateway",
							"Support Gateway",
							"gateway-id",
							"",
							p.resources.AIGatewayPolicies,
							plan,
						)
						require.NoError(t, err)
						reads = policyAPI.reads
						if tt.wantAction == ActionUpdate {
							require.Equal(t, nameID, p.resources.AIGatewayPolicies[0].GetKonnectID())
						}
					case ResourceTypeAIGatewayMCPServer:
						desired := testAIGatewayMCPServerResource(t)
						desired.Ref = tt.ref
						desired.AIGatewayMCPServerConversionOnly.Name = tt.desiredName
						desired.AIGatewayMCPServerConversionOnly.DisplayName = "Updated child"
						desired.SetKonnectID(tt.cachedID)
						p.resources.AIGatewayMCPServers = []resources.AIGatewayMCPServerResource{desired}
						err := p.planAIGatewayMCPServerChanges(
							t.Context(),
							"default",
							"support-gateway",
							"Support Gateway",
							"gateway-id",
							"",
							nil,
							p.resources.AIGatewayMCPServers,
							plan,
						)
						require.NoError(t, err)
						reads = serverAPI.reads
					}

					wantDeleteIDs := []string{cachedID, refID}
					wantID := nameID
					if tt.wantAction == ActionCreate {
						wantDeleteIDs = []string{cachedID, refID, nameID}
						wantID = ""
						require.Empty(t, reads)
					} else {
						require.Equal(t, [][2]string{{"gateway-id", nameID}}, reads)
					}
					require.Len(t, plan.Changes, 1+len(wantDeleteIDs))
					change := plan.Changes[0]
					require.Equal(t, tt.wantAction, change.Action)
					require.Equal(t, wantID, change.ResourceID)
					require.Equal(t, tt.ref, change.ResourceRef)
					require.Equal(t, tt.desiredName, change.Fields[FieldName])
					require.Equal(t, "Updated child", change.Fields[FieldDisplayName])
					if tt.wantAction == ActionUpdate {
						require.Contains(t, change.ChangedFields, FieldDisplayName)
					}
					for i, id := range wantDeleteIDs {
						require.Equal(t, ActionDelete, plan.Changes[i+1].Action)
						require.Equal(t, id, plan.Changes[i+1].ResourceID)
					}
					for _, change := range plan.Changes {
						require.Equal(t, kind, change.ResourceType)
						require.Equal(t, "default", change.Namespace)
						require.Equal(t, &ParentInfo{Ref: "support-gateway", ID: "gateway-id"}, change.Parent)
					}
				})
			}
		})
	}
}

func TestAIGatewayPolicyAndMCPServerResourceIdentity(t *testing.T) {
	const (
		id      = "11111111-1111-4111-8111-111111111111"
		otherID = "22222222-2222-4222-8222-222222222222"
	)
	for _, tt := range []struct {
		name     string
		ref      string
		cachedID string
		sameName bool
	}{
		{"UUID ref cannot override name", id, "", false},
		{"cached ID cannot override name", "local-ref", id, false},
		{"unrelated IDs do not hide name match", otherID, otherID, true},
		{"matching IDs allow name match", id, id, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			policy := testAIGatewayPolicyResource(t)
			server := testAIGatewayMCPServerResource(t)
			policy.Ref, server.Ref = tt.ref, tt.ref
			policy.SetKonnectID(tt.cachedID)
			server.SetKonnectID(tt.cachedID)
			currentPolicy := testAIGatewayPolicy()
			currentPolicy.ID = id
			serverName := server.Name()
			if !tt.sameName {
				currentPolicy.Name, serverName = "different-name", "different-name"
			}
			for _, pair := range []struct {
				desired resources.Resource
				current any
			}{
				{&policy, currentPolicy},
				{&server, testAIGatewayMCPServer(t, id, serverName)},
			} {
				t.Run(string(pair.desired.GetType()), func(t *testing.T) {
					require.Equal(t, tt.sameName, pair.desired.TryMatchKonnectResource(pair.current))
					wantID := tt.cachedID
					if tt.sameName {
						wantID = id
					}
					require.Equal(t, wantID, pair.desired.GetKonnectID())
				})
			}
		})
	}
}

type policyIdentityAPI struct {
	*testAIGatewayPolicyAPI
	reads [][2]string
}

func (a *policyIdentityAPI) GetAiGatewayPolicy(
	ctx context.Context, gatewayID, policyID string, opts ...kkOps.Option,
) (*kkOps.GetAiGatewayPolicyResponse, error) {
	a.reads = append(a.reads, [2]string{gatewayID, policyID})
	return a.testAIGatewayPolicyAPI.GetAiGatewayPolicy(ctx, gatewayID, policyID, opts...)
}

type mcpServerIdentityAPI struct {
	*testAIGatewayMCPServerAPI
	reads [][2]string
}

func (a *mcpServerIdentityAPI) GetAiGatewayMcpServer(
	_ context.Context, gatewayID, serverID string, _ ...kkOps.Option,
) (*kkOps.GetAiGatewayMcpServerResponse, error) {
	a.reads = append(a.reads, [2]string{gatewayID, serverID})
	for _, server := range a.servers {
		if resources.AIGatewayMCPServerID(server) == serverID {
			return &kkOps.GetAiGatewayMcpServerResponse{AIGatewayMCPServer: &server}, nil
		}
	}
	return &kkOps.GetAiGatewayMcpServerResponse{}, nil
}

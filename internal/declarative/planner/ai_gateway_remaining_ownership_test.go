package planner

import (
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAdditionalDeleteRequiresProvenConfigurationDetachment(t *testing.T) {
	for _, target := range []string{
		ResourceTypeAIGatewayCACertificate,
		ResourceTypeAIGatewaySNI,
		ResourceTypeAIGatewayVault,
	} {
		for _, mode := range []string{"retained", "metadata", "detach", "delete", "attach"} {
			t.Run(target+"/"+mode, func(t *testing.T) {
				config := map[string]any{}
				switch target {
				case ResourceTypeAIGatewayCACertificate:
					config["ca_certificates"] = []any{"target"}
				case ResourceTypeAIGatewaySNI:
					config["snis"] = []any{"target"}
				case ResourceTypeAIGatewayVault:
					config["header"] = "{vault://target/key}"
				}
				policy := testAIGatewayPolicy()
				policy.ID = "user-id"
				policy.Name = "foreign-user"
				policy.Config = config
				cfg := emptyAIGatewayCollectionsConfig()
				cfg.AIGatewayPoliciesAPI = &testAIGatewayPolicyAPI{
					policies: []kkComps.AIGatewayPolicy{policy},
				}
				p := NewPlanner(state.NewClient(cfg), slog.Default())
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
				plan.AddChange(
					PlannedChange{
						ID:           "delete-target",
						Action:       ActionDelete,
						ResourceType: target,
						ResourceID:   "target-id",
						Parent:       &ParentInfo{ID: "gateway-id"},
						Fields:       map[string]any{FieldName: "target"},
					},
				)
				if mode != "retained" {
					change := PlannedChange{
						ID:           "change-user",
						Action:       ActionUpdate,
						ResourceType: ResourceTypeAIGatewayPolicy,
						ResourceID:   "user-id",
						Namespace:    "other-team",
						Parent:       &ParentInfo{Ref: "other-alias", ID: "gateway-id"},
						Fields:       map[string]any{FieldConfig: map[string]any{}},
						ChangedFields: map[string]FieldChange{
							FieldConfig: {Old: config, New: map[string]any{}},
						},
					}
					if mode == "metadata" {
						change.ChangedFields = map[string]FieldChange{
							FieldManagedBy: {New: map[string]string{}},
						}
					}
					if mode == "delete" {
						change.Action = ActionDelete
					}
					if mode == "attach" {
						change.Action = ActionCreate
						change.ResourceID = ""
						change.Fields[FieldConfig] = config
					}
					plan.AddChange(change)
				}
				err := p.resolveAdditionalAIGatewayReferenceDeletes(
					t.Context(),
					"team-a",
					"shared",
					"gateway-id",
					target,
					plan,
				)
				if mode == "detach" || mode == "delete" {
					require.NoError(t, err)
					require.Contains(t, plan.Changes[0].DependsOn, "change-user")
				} else {
					require.ErrorContains(t, err, "still references it")
				}
			})
		}
	}
}

func TestOpaqueVaultCredentialCannotBeDetachedByOmittedSecret(t *testing.T) {
	_, provider := ownedProvider(t, "provider", "foreign-team")
	cfg := emptyAIGatewayCollectionsConfig()
	cfg.AIGatewayProvidersAPI = &nameIdentityProviderAPI{
		children: []kkComps.AIGatewayModelProvider{provider},
	}
	p := NewPlanner(state.NewClient(cfg), slog.Default())
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	plan.AddChange(
		PlannedChange{
			ID:           "delete-vault",
			Action:       ActionDelete,
			ResourceType: ResourceTypeAIGatewayVault,
			ResourceID:   "vault-id",
			Parent:       &ParentInfo{ID: "gateway-id"},
			Fields:       map[string]any{FieldName: "vault"},
		},
	)
	plan.AddChange(
		PlannedChange{
			ID:            "update-provider",
			Action:        ActionUpdate,
			ResourceType:  ResourceTypeAIGatewayProvider,
			ResourceID:    "provider-id",
			Parent:        &ParentInfo{ID: "gateway-id"},
			Fields:        map[string]any{FieldConfig: map[string]any{}},
			ChangedFields: map[string]FieldChange{FieldConfig: {New: map[string]any{}}},
		},
	)
	require.ErrorContains(
		t,
		p.resolveAIGatewayVaultDeletes(t.Context(), "team-a", "shared", "gateway-id", plan),
		"redacted credentials",
	)
	plan.Changes[1].Action = ActionDelete
	require.NoError(
		t,
		p.resolveAIGatewayVaultDeletes(t.Context(), "team-a", "shared", "gateway-id", plan),
	)
	require.Contains(t, plan.Changes[0].DependsOn, "update-provider")
}

type remainingOwnershipCase struct {
	kind, path string
	payload    map[string]any
}

func remainingOwnershipCases() []remainingOwnershipCase {
	return []remainingOwnershipCase{
		{
			ResourceTypeAIGatewayMCPServer,
			"mcp-servers",
			map[string]any{
				FieldType:   "conversion-only",
				FieldConfig: map[string]any{"url": "https://example.test"},
				FieldTools:  []any{},
			},
		},
		{
			ResourceTypeAIGatewayVault,
			"vaults",
			map[string]any{FieldType: "env", FieldConfig: map[string]any{"prefix": "TEST_"}},
		},
		{ResourceTypeAIGatewayConsumer, "consumers", map[string]any{FieldType: "api-key"}},
		{ResourceTypeAIGatewayConsumerGroup, "consumer-groups", map[string]any{}},
		{
			ResourceTypeAIGatewayCACertificate,
			"ca-certificates",
			map[string]any{FieldCert: "public-ca"},
		},
		{
			ResourceTypeAIGatewaySNI,
			"snis",
			map[string]any{FieldHostname: "example.test", FieldCertificate: "platform-cert"},
		},
	}
}

func remainingOwnedDeclaration(
	t *testing.T,
	rs *resources.ResourceSet,
	tc remainingOwnershipCase,
	name, ns, parent string,
) {
	t.Helper()
	payload := maps.Clone(tc.payload)
	payload["ref"], payload["ai_gateway"] = name, parent
	payload[FieldName], payload[FieldDisplayName] = name, name
	payload["kongctl"] = map[string]any{"namespace": ns}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	switch tc.kind {
	case ResourceTypeAIGatewayMCPServer:
		var r resources.AIGatewayMCPServerResource
		require.NoError(t, json.Unmarshal(data, &r))
		rs.AIGatewayMCPServers = append(rs.AIGatewayMCPServers, r)
	case ResourceTypeAIGatewayVault:
		var r resources.AIGatewayVaultResource
		require.NoError(t, json.Unmarshal(data, &r))
		rs.AIGatewayVaults = append(rs.AIGatewayVaults, r)
	case ResourceTypeAIGatewayConsumer:
		var r resources.AIGatewayConsumerResource
		require.NoError(t, json.Unmarshal(data, &r))
		rs.AIGatewayConsumers = append(rs.AIGatewayConsumers, r)
	case ResourceTypeAIGatewayConsumerGroup:
		var r resources.AIGatewayConsumerGroupResource
		require.NoError(t, json.Unmarshal(data, &r))
		rs.AIGatewayConsumerGroups = append(rs.AIGatewayConsumerGroups, r)
	case ResourceTypeAIGatewayCACertificate:
		var r resources.AIGatewayCACertificateResource
		require.NoError(t, json.Unmarshal(data, &r))
		rs.AIGatewayCACertificates = append(rs.AIGatewayCACertificates, r)
	case ResourceTypeAIGatewaySNI:
		var r resources.AIGatewaySNIResource
		require.NoError(t, json.Unmarshal(data, &r))
		rs.AIGatewaySNIs = append(rs.AIGatewaySNIs, r)
	}
	rs.SyncScope.AddChild(resources.ResourceTypeAIGateway, parent, resources.ResourceType(tc.kind))
	rs.SyncScope.AddChildNamespace(resources.ResourceType(tc.kind), parent, ns)
}

type remainingOwnershipTransport struct {
	path   string
	values []map[string]any
	lists  int
}

func (a *remainingOwnershipTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return emptyAIGatewayCollectionsTransport{}.RoundTrip(req)
	}
	var payload any = map[string]any{"data": []any{}}
	if strings.HasSuffix(req.URL.Path, "/"+a.path) {
		a.lists++
		payload = map[string]any{"data": a.values}
	} else {
		for _, value := range a.values {
			if strings.HasSuffix(req.URL.Path, "/"+value[FieldID].(string)) {
				payload = value
			}
		}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(data))),
		Request:    req,
	}, nil
}

func TestRemainingOwnedChildrenAliasesScopesAndCollisions(t *testing.T) {
	for _, tc := range remainingOwnershipCases() {
		t.Run(tc.kind, func(t *testing.T) {
			if tc.kind == ResourceTypeAIGatewayCACertificate {
				tc.payload[FieldCert] = "public-ca\n"
			}
			api := &remainingOwnershipTransport{path: tc.path}
			for _, item := range []struct{ name, ns string }{{"a", "team-a"}, {"b", "team-b"}, {"legacy", ""}} {
				value := maps.Clone(tc.payload)
				if tc.kind == ResourceTypeAIGatewayCACertificate {
					value[FieldCert] = "public-ca"
				}
				value[FieldID] = item.name + "-id"
				value[FieldName] = item.name
				value[FieldDisplayName] = item.name
				value[FieldManagedBy] = resources.BuildChildManagedBy(
					item.ns,
					false,
					"dev",
				)
				value["created_at"], value["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
				api.values = append(api.values, value)
			}
			client := state.NewClient(aigatewayCollectionsConfig(api))
			rs := federatedResourceSet()
			alias := rs.AIGateways[0]
			alias.Ref = "alias-b"
			rs.AIGateways = append(rs.AIGateways, alias)
			remainingOwnedDeclaration(t, rs, tc, "a", "team-a", "shared")
			remainingOwnedDeclaration(t, rs, tc, "b", "team-b", "alias-b")
			plan, err := NewPlanner(
				client,
				slog.Default(),
			).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
			require.Equal(t, 1, api.lists)
			empty := federatedResourceSet()
			empty.SyncScope.AddChild(
				resources.ResourceTypeAIGateway,
				"shared",
				resources.ResourceType(tc.kind),
			)
			empty.SyncScope.AddChildNamespace(resources.ResourceType(tc.kind), "shared", "team-a")
			plan, err = NewPlanner(
				client,
				slog.Default(),
			).GeneratePlan(t.Context(), empty, Options{Mode: PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes)
			plan, err = NewPlanner(
				client,
				slog.Default(),
			).GeneratePlan(t.Context(), empty, Options{Mode: PlanModeSync})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			require.Equal(t, "a-id", plan.Changes[0].ResourceID)
			require.Equal(t, "team-a", plan.Changes[0].Namespace)
			for _, name := range []string{"b", "legacy"} {
				collision := federatedResourceSet()
				remainingOwnedDeclaration(t, collision, tc, name, "team-a", "shared")
				_, err = NewPlanner(
					client,
					slog.Default(),
				).GeneratePlan(t.Context(), collision, Options{Mode: PlanModeApply})
				require.Error(t, err)
			}
			api.values[0][FieldManagedBy] = resources.BuildChildManagedBy(
				"team-a",
				true,
				"dev",
			)
			_, err = NewPlanner(
				client,
				slog.Default(),
			).GeneratePlan(t.Context(), empty, Options{Mode: PlanModeSync})
			require.ErrorContains(t, err, "protected")
		})
	}
}

func TestAdditionalReferencesIgnoreProvenanceAndInspectPublicConfig(t *testing.T) {
	payload := map[string]any{
		FieldManagedBy: map[string]any{"inventory": "{vault://ignored/key}"},
		FieldLabels:    map[string]any{"sni": "ignored"},
		FieldConfig: map[string]any{
			"tls": map[string]any{
				"ca_certificates": []any{"root"},
				"snis":            []any{"api.example.test"},
			},
			"header": "{vault://env/key}",
		},
	}
	require.Equal(t, []string{"env"}, publicChildReferences(ResourceTypeAIGatewayVault, payload))
	require.Equal(
		t,
		[]string{"root"},
		publicChildReferences(ResourceTypeAIGatewayCACertificate, payload),
	)
	require.Equal(
		t,
		[]string{"api.example.test"},
		publicChildReferences(ResourceTypeAIGatewaySNI, payload),
	)
}

package executor

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestRemainingChildSavedWritesRecheckOwnershipProtectionAndCollisions(t *testing.T) {
	for _, tc := range []struct {
		kind, path string
		fields     map[string]any
	}{
		{
			planner.ResourceTypeAIGatewayMCPServer,
			"mcp-servers",
			map[string]any{
				planner.FieldType:   "conversion-only",
				planner.FieldTools:  []any{},
				planner.FieldConfig: map[string]any{"url": "https://example.test"},
			},
		},

		{
			planner.ResourceTypeAIGatewayVault,
			"vaults",
			map[string]any{
				planner.FieldType:   "env",
				planner.FieldConfig: map[string]any{"prefix": "TEST_"},
			},
		},

		{planner.ResourceTypeAIGatewayVault, "vaults", map[string]any{
			planner.FieldType: "hcv", planner.FieldConfig: map[string]any{
				"auth_method": "approle", "host": "vault.example.test", "port": 8200, "role_id": "role",
			},
		}},

		{planner.ResourceTypeAIGatewayConsumer, "consumers", map[string]any{planner.FieldType: "api-key"}},
		{planner.ResourceTypeAIGatewayConsumerGroup, "consumer-groups", map[string]any{}},
		{planner.ResourceTypeAIGatewayCACertificate, "ca-certificates", map[string]any{planner.FieldCert: "public-ca"}},
		{
			planner.ResourceTypeAIGatewaySNI,
			"snis",
			map[string]any{
				planner.FieldCertificate: "platform-cert",
				planner.FieldHostname:    "example.test",
			},
		},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			for _, owner := range []string{"team-a", "team-b", ""} {
				for _, protected := range []bool{false, true} {
					for _, action := range []planner.ActionType{planner.ActionCreate, planner.ActionUpdate, planner.ActionDelete} {
						t.Run(
							owner+"/"+map[bool]string{false: "unprotected", true: "protected"}[protected]+"/"+string(
								action,
							),
							func(t *testing.T) {
								observed := maps.Clone(tc.fields)
								observed[planner.FieldID],
									observed[planner.FieldName],
									observed[planner.FieldDisplayName] = "child-id",
									"child",
									"original"
								observed[planner.FieldManagedBy] = resources.BuildChildManagedBy(
									owner,
									protected,
									"old",
								)
								observed["created_at"], observed["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
								writes := 0
								lateUser := false
								transport := customPolicyHTTPClient(
									func(req *http.Request) (*http.Response, error) {
										var payload any = map[string]any{"data": []any{}}
										if lateUser &&
											strings.HasSuffix(req.URL.Path, "/policies") {
											payload = map[string]any{
												"data": []any{
													map[string]any{
														"id":           "foreign-id",
														"name":         "foreign",
														"display_name": "Foreign",
														"type":         "http-log",
														"config": map[string]any{
															"ca_certificates": []string{"child"},
														},
														"created_at": "2026-01-01T00:00:00Z",
														"updated_at": "2026-01-01T00:00:00Z",
													},
												},
											}
										}
										if strings.HasSuffix(req.URL.Path, "/"+tc.path) {
											payload = map[string]any{"data": []any{observed}}
										}
										if strings.HasSuffix(req.URL.Path, "/child-id") {
											payload = observed
										}
										status := 200
										if req.Method != http.MethodGet {
											writes++
											if req.Method == http.MethodPut {
												var body map[string]any
												require.NoError(
													t,
													json.NewDecoder(req.Body).Decode(&body),
												)
												metadata := body[planner.FieldManagedBy].(map[string]any)
												require.NotContains(t, metadata, "inventory")
												require.Equal(t, "kongctl", metadata["tool"])
											}
											payload = observed
											if req.Method == http.MethodDelete {
												status = 204
											}
										}
										data, err := json.Marshal(payload)
										require.NoError(t, err)
										if status == 204 {
											data = nil
										}
										return &http.Response{
											StatusCode: status,
											Header: http.Header{
												"Content-Type": {"application/json"},
											},
											Body:    io.NopCloser(strings.NewReader(string(data))),
											Request: req,
										}, nil
									},
								)
								api := sdk.New(
									sdk.WithServerURL("https://example.test"),
									sdk.WithClient(transport),
								)
								client := state.NewClient(state.ClientConfig{
									AIGatewayProvidersAPI: &helpers.AIGatewayProvidersAPIImpl{
										SDK: api,
									}, AIGatewayAuthStrategiesAPI: api.AIGatewayAuthStrategies, AIGatewayModelAPI: api.AIGatewayModels,
									AIGatewayPoliciesAPI:       api.AIGatewayPolicies,
									AIGatewayCustomPoliciesAPI: api.AIGatewayCustomPolicies,
									AIGatewayAgentsAPI:         api.AIGatewayAgents,

									AIGatewayMCPServersAPI:     api.AIGatewayMCPServers,
									AIGatewayVaultsAPI:         api.AIGatewayVaults,
									AIGatewayConsumersAPI:      api.AIGatewayConsumers,
									AIGatewayConsumerGroupsAPI: api.AIGatewayConsumerGroups,

									AIGatewayCertificatesAPI:   api.AIGatewayCertificates,
									AIGatewayCACertificatesAPI: api.AIGatewayCACertificates,
									AIGatewaySNIsAPI:           api.AIGatewaySNIs,
								})
								fields := maps.Clone(tc.fields)
								fields[planner.FieldName], fields[planner.FieldDisplayName] = "child", "updated"
								fields[planner.FieldManagedBy] = resources.BuildChildManagedBy(
									"team-a",
									false,
									"old",
								)
								change := planner.PlannedChange{
									Action:       action,
									ResourceType: tc.kind,
									ResourceRef:  "child",
									ResourceID:   "child-id",
									Namespace:    "team-a",
									Parent:       &planner.ParentInfo{ID: "gateway-id"},
									Fields:       fields,
									ChangedFields: map[string]planner.FieldChange{
										planner.FieldDisplayName: {Old: "original", New: "updated"},
									},
								}
								if tc.kind == planner.ResourceTypeAIGatewayVault ||
									tc.kind == planner.ResourceTypeAIGatewayCACertificate {
									delete(fields, planner.FieldDisplayName)
									fields[planner.FieldLabels] = map[string]string{
										"revision": "new",
									}
									change.ChangedFields = map[string]planner.FieldChange{
										planner.FieldLabels: {New: fields[planner.FieldLabels]},
									}
								}
								data, err := json.Marshal(change)
								require.NoError(t, err)
								var saved planner.PlannedChange
								require.NoError(t, json.Unmarshal(data, &saved))
								exec := New(client, nil, false)
								if tc.kind == planner.ResourceTypeAIGatewayCACertificate &&
									action == planner.ActionDelete &&
									owner == "team-a" &&
									!protected {
									lateUser = true
									require.ErrorContains(
										t,
										exec.deleteResource(testContextWithLogger(), &saved),
										"still references it",
									)
									require.Zero(t, writes)
									lateUser = false
								}
								switch action {
								case planner.ActionCreate:
									_, err = exec.createResource(testContextWithLogger(), &saved)
								case planner.ActionUpdate:
									_, err = exec.updateResource(testContextWithLogger(), &saved)
								case planner.ActionDelete:
									err = exec.deleteResource(testContextWithLogger(), &saved)
								case planner.ActionExternalTool:
									t.Fatal("unexpected action")
								}
								config, _ := tc.fields[planner.FieldConfig].(map[string]any)
								blockedAppRole := action == planner.ActionUpdate && config["auth_method"] == "approle"
								if action == planner.ActionCreate || owner != "team-a" || protected || blockedAppRole {
									if blockedAppRole && owner == "team-a" && !protected {
										require.ErrorContains(t, err, "cannot preserve redacted secret_id")
									}
									require.Error(t, err)
									require.Zero(t, writes)
								} else {
									require.NoError(t, err)
									require.Equal(t, 1, writes)
								}
							},
						)
					}
				}
			}
		})
	}
}

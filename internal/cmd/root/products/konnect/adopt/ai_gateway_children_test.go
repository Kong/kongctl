package adopt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

const (
	adoptionGatewayID = "4078f559-063b-4f10-9098-9e6553b21d9b"
	adoptionChildID   = "5078f559-063b-4f10-9098-9e6553b21d9b"
)

var aiAdoptionPayloads = map[string]string{
	"models": `{
  "type": "model",
  "enabled": true,
  "config": {
    "route": {},
    "model": {}
  },
  "formats": [
    {
      "type": "openai"
    }
  ],
  "targets": [
    {
      "name": "gpt-4o",
      "provider": "openai",
      "config": {
        "type": "openai"
      }
    }
  ],
  "policies": [],
  "capabilities": [
    "generate"
  ]
}`,
	"model-providers": `{"type":"openai","config":{"auth":{"type":"basic","headers":[{"name":"Authorization"}]}}}`,
	"auth-strategies": `{
  "type": "openid-connect",
  "config": {
    "cache_tokens_salt": "keep-salt",
    "issuer": "https://issuer.example.com",
    "client_id": [
      "public-client"
    ],
    "consumer_groups_claim": [
      "groups"
    ]
  }
}`,
	"policies":        `{"type":"ai-sanitizer","enabled":true,"global":false,"config":{"anonymize":["email"]}}`,
	"custom-policies": `{"type":"streaming","schema":"return { name = 'keep-policy' }","handler":"return {}"}`,
	"agents":          `{"type":"a2a","config":{"url":"https://agent.example.com"},"policies":["keep-policy"]}`,
	"mcp-servers": `{
  "type": "passthrough-listener",
  "enabled": true,
  "config": {
    "url": "https://mcp.example.com",
    "route": {
      "paths": [
        "/mcp"
      ]
    }
  }
}`,
	"vaults":          `{"type":"env","config":{"prefix":"KEEP_"}}`,
	"consumers":       `{"type":"apikey"}`,
	"consumer-groups": `{}`,
	"ca-certificates": `{"cert":"public-certificate"}`,
	"snis":            `{"hostname":"*.example.com","certificate":"keep-certificate"}`,
}

func aiAdoptionPayload(t *testing.T, family string) map[string]any {
	t.Helper()
	var value map[string]any
	require.NoError(t, json.Unmarshal([]byte(aiAdoptionPayloads[family]), &value))
	value[planner.FieldID] = adoptionChildID
	value[planner.FieldName] = "team-child"
	value[planner.FieldDisplayName] = "Keep display name"
	if family == "ca-certificates" {
		delete(value, planner.FieldDisplayName)
	}
	value[planner.FieldLabels] = map[string]any{"public": "keep"}
	value[planner.FieldManagedBy] = map[string]any{resources.ChildProtectedKey: "true", "outside": "replace"}
	value["created_at"], value["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
	// Real GET responses include API defaults. Round-trip the typed response so
	// the fixture represents the complete observation consumed by adoption.
	responses := map[string]any{
		"models": &kkComps.AIGatewayModel{}, "model-providers": &kkComps.AIGatewayModelProvider{},
		"auth-strategies": &kkComps.AIGatewayAuthStrategy{}, "policies": &kkComps.AIGatewayPolicy{},
		"custom-policies": &kkComps.AIGatewayCustomPolicy{}, "agents": &kkComps.AIGatewayAgent{},
		"mcp-servers": &kkComps.AIGatewayMCPServer{}, "vaults": &kkComps.AIGatewayVault{},
		"consumers": &kkComps.AIGatewayConsumer{}, "consumer-groups": &kkComps.AIGatewayConsumerGroup{},
		"ca-certificates": &kkComps.AIGatewayCACertificate{}, "snis": &kkComps.AIGatewaySNI{},
	}
	data, err := json.Marshal(value)
	require.NoError(t, err)
	response := responses[family]
	require.NoError(t, json.Unmarshal(data, response))
	data, err = json.Marshal(response)
	require.NoError(t, err)
	value = nil
	require.NoError(t, json.Unmarshal(data, &value))
	return value
}

func aiAdoptionTestClient(t *testing.T, handler http.HandlerFunc) *state.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	sdk := &helpers.KonnectSDK{
		SDK: kkSDK.New(kkSDK.WithServerURL(server.URL),
			kkSDK.WithClient(server.Client())),
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}
	return state.NewClient(state.ClientConfig{
		AIGatewayAPI:               sdk.GetAIGatewayAPI(),
		AIGatewayProvidersAPI:      sdk.GetAIGatewayProvidersAPI(),
		AIGatewayAuthStrategiesAPI: sdk.GetAIGatewayAuthStrategiesAPI(),
		AIGatewayCustomPoliciesAPI: sdk.GetAIGatewayCustomPoliciesAPI(), AIGatewayPoliciesAPI: sdk.GetAIGatewayPoliciesAPI(),
		AIGatewayAgentsAPI: sdk.GetAIGatewayAgentsAPI(), AIGatewayConsumersAPI: sdk.GetAIGatewayConsumersAPI(),
		AIGatewayConsumerGroupsAPI: sdk.GetAIGatewayConsumerGroupsAPI(), AIGatewayModelAPI: sdk.GetAIGatewayModelAPI(),
		AIGatewayMCPServersAPI: sdk.GetAIGatewayMCPServersAPI(), AIGatewayVaultsAPI: sdk.GetAIGatewayVaultsAPI(),
		AIGatewayCACertificatesAPI: sdk.GetAIGatewayCACertificatesAPI(), AIGatewaySNIsAPI: sdk.GetAIGatewaySNIsAPI(),
	})
}

func TestAIChildAdoptionPreservesPublicPayloadAcrossFamilies(t *testing.T) {
	for index, metadata := range aiChildFamilies(state.NewClient(state.ClientConfig{})) {
		t.Run(metadata.name, func(t *testing.T) {
			current := aiAdoptionPayload(t, metadata.name)
			updates := 0
			client := aiAdoptionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				require.Contains(t, r.URL.Path, "/"+adoptionGatewayID+"/")
				switch r.Method {
				case http.MethodGet:
					if strings.HasSuffix(r.URL.Path, "/"+adoptionChildID) {
						require.NoError(t, json.NewEncoder(w).Encode(current))
					} else {
						require.NoError(t,
							json.NewEncoder(w).Encode(map[string]any{
								"data": []any{current},
								"meta": map[string]any{"page": map[string]any{
									"next": nil,
									"size": 100,
								}},
							}))
					}
				case http.MethodPut:
					updates++
					var request map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
					require.NotContains(t, request, "credentials")
					require.NotContains(t, request, "consumers", "group adoption must not rewrite membership")
					maps.Copy(current, request)
					require.NoError(t, json.NewEncoder(w).Encode(current))
				default:
					t.Errorf("unexpected mutation: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			})
			family := aiChildFamilies(client)[index]
			ctx := context.WithValue(t.Context(), build.InfoKey, &build.Info{Version: "adoption-test"})
			candidates, err := family.list(ctx, adoptionGatewayID)
			require.NoError(t, err)
			require.Equal(t, []childCandidate{{adoptionChildID, "team-child"}}, candidates)
			before, err := json.Marshal(current)
			require.NoError(t, err)
			write, err := family.prepare(ctx, adoptionGatewayID, candidates[0], "team-a", false)
			require.NoError(t, err)
			result, err := write()
			require.NoError(t, err)
			require.Equal(t, adoptionChildID, result.ID)
			require.Equal(t, 1, updates)
			var original map[string]any
			require.NoError(t, json.Unmarshal(before, &original))
			delete(original, planner.FieldManagedBy)
			after := maps.Clone(current)
			delete(after, planner.FieldManagedBy)
			require.Equal(t, original, after, "adoption changes only ownership metadata")
			managedBy, err := childPayloadMetadata(current)
			require.NoError(t, err)
			require.Equal(t, resources.BuildChildManagedBy("team-a", true, "adoption-test"), managedBy)
			_, err = family.prepare(ctx, adoptionGatewayID, candidates[0], "team-b", false)
			require.ErrorContains(t, err, "overwrite-namespace")
			require.Equal(t, 1, updates)
			transfer, err := family.prepare(ctx, adoptionGatewayID, candidates[0], "team-b", true)
			require.NoError(t, err)
			_, err = transfer()
			require.NoError(t, err)
			require.Equal(t, 2, updates)
			transferred, err := childPayloadMetadata(current)
			require.NoError(t, err)
			require.Equal(t, resources.BuildChildManagedBy("team-b", true, "adoption-test"), transferred)
		})
	}
}

func childTestSetup(t *testing.T,
	dryRun bool,
	format cmdCommon.OutputFormat) (adoptCommon.AdoptRunSetup,
	*strings.Builder,
) {
	t.Helper()
	command := &cobra.Command{}
	require.NoError(t, adoptCommon.AddAdoptFlags(command))
	require.NoError(t, command.ParseFlags([]string{"--namespace", "team-a", fmt.Sprintf("--dry-run=%t", dryRun)}))
	output := new(strings.Builder)
	command.SetContext(context.WithValue(t.Context(),
		iostreams.StreamsKey,
		&iostreams.IOStreams{
			Out:    output,
			ErrOut: output,
		}))
	flags, err := adoptCommon.ReadAdoptFlags(command)
	require.NoError(t, err)
	return adoptCommon.AdoptRunSetup{Helper: cmd.BuildHelper(command, nil), AdoptFlags: flags, OutType: format}, output
}

func TestAIChildAdoptionDryRunAndUnsafeVault(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		t.Run(fmt.Sprint(unsafe), func(t *testing.T) {
			current := aiAdoptionPayload(t, "vaults")
			if unsafe {
				current[planner.FieldType] = "hcv"
				current[planner.FieldConfig] = map[string]any{
					"auth_method": "approle",
					"host":        "vault.example.com",
					"port":        8200,
					"role_id":     "public-role",
				}
			}
			client := aiAdoptionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method, "dry-run and unsafe adoption must never mutate")
				w.Header().Set("Content-Type", "application/json")
				require.NoError(t, json.NewEncoder(w).Encode(current))
			})
			setup, out := childTestSetup(t, true, cmdCommon.JSON)
			family := aiChildFamilies(client)[7]
			candidate := childCandidate{adoptionChildID, "team-child"}
			err := runChildAdoptions(setup,
				[]childCandidate{candidate},
				func(value childCandidate) (preparedChildAdoption, error) {
					return family.prepare(setup.Helper.GetContext(), adoptionGatewayID, value, "team-a", false)
				})
			if unsafe {
				require.ErrorContains(t, err, "cannot preserve redacted secret_id")
				require.Empty(t, out.String())
				return
			}
			require.NoError(t, err)
			var preview adoptCommon.AdoptionPreview
			require.NoError(t, json.Unmarshal([]byte(out.String()), &preview))
			require.True(t, preview.DryRun)
			require.Len(t, preview.Changes, 1)
			require.Equal(t, adoptionGatewayID, preview.Changes[0].Parent["gateway_id"])
			require.Equal(t, "true", preview.Changes[0].After[resources.ChildProtectedKey])
			require.NotContains(t, out.String(), "config")
		})
	}
}

func TestAIChildAdoptionIsRecognizedByFreshPlan(t *testing.T) {
	current := aiAdoptionPayload(t, "models")
	delete(current, planner.FieldManagedBy)
	client := aiAdoptionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			var request map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			maps.Copy(current, request)
		} else {
			require.Equal(t, http.MethodGet, r.Method)
		}
		if strings.Contains(r.URL.Path, "/models") {
			if strings.HasSuffix(r.URL.Path, "/"+adoptionChildID) || r.Method == http.MethodPut {
				require.NoError(t, json.NewEncoder(w).Encode(current))
			} else {
				require.NoError(t,
					json.NewEncoder(w).Encode(map[string]any{
						"data": []any{current},
						"meta": map[string]any{"page": map[string]any{
							"next": nil,
							"size": 100,
						}},
					}))
			}
		} else {
			require.NoError(t,
				json.NewEncoder(w).Encode(map[string]any{
					"id":           adoptionGatewayID,
					"name":         "shared",
					"display_name": "Shared",
					"created_at":   "2026-01-01T00:00:00Z",
					"updated_at":   "2026-01-01T00:00:00Z",
				}))
		}
	})
	var model resources.AIGatewayModelResource
	payload, err := json.Marshal(current)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(payload, &model))
	model.Ref, model.AIGateway = "team-child", "shared"
	model.Kongctl = &resources.KongctlMeta{Namespace: new("team-a"), Protected: new(false)}
	write,
		err := aiChildFamilies(client)[0].prepare(t.Context(),
		adoptionGatewayID,
		childCandidate{
			adoptionChildID,
			"team-child",
		},
		"team-a",
		false)
	require.NoError(t, err)
	_, err = write()
	require.NoError(t, err)
	gateway := resources.AIGatewayResource{
		BaseResource: resources.BaseResource{Ref: "shared"},
		External:     &resources.ExternalBlock{ID: adoptionGatewayID},
	}
	gateway.SetKonnectID(adoptionGatewayID)
	plan,
		err := planner.NewPlanner(client,
		slog.Default()).GeneratePlan(t.Context(),
		&resources.ResourceSet{
			AIGateways:      []resources.AIGatewayResource{gateway},
			AIGatewayModels: []resources.AIGatewayModelResource{model},
		},
		planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes, "fresh plan must match the adopted child without recreation or payload reconciliation")
}

func TestAIChildAdoptionCommandSelectors(t *testing.T) {
	for _, args := range [][]string{
		{},
		{
			"--all",
			"exact",
		},
		{
			"--all",
			"--name-pattern",
			"team-*",
		},
		{
			"--name-pattern",
			"[",
		},
		{
			"--model-id",
			adoptionChildID,
			"exact",
		},
	} {
		cmd, err := NewAIGatewayCmd(verbs.Adopt, nil, nil, nil)
		require.NoError(t, err)
		child, _, err := cmd.Find([]string{"models"})
		require.NoError(t, err)
		require.NoError(t, child.ParseFlags(args))
		require.Error(t, child.Args(child, child.Flags().Args()))
	}
	root, err := NewAIGatewayCmd(verbs.Adopt, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, root.Commands(), 12)
	for _, unsupported := range []string{"certificates", "credentials", "config-stores", "data-plane-certificates"} {
		_, remaining, err := root.Find([]string{unsupported})
		require.NoError(t, err)
		require.NotEmpty(t, remaining)
	}
}

func TestAIChildAdoptionRejectsChangesAfterPreflight(t *testing.T) {
	current := aiAdoptionPayload(t, "consumer-groups")
	client := aiAdoptionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(current))
	})
	family := aiChildFamilies(client)[9]
	write,
		err := family.prepare(t.Context(),
		adoptionGatewayID,
		childCandidate{
			adoptionChildID,
			"team-child",
		},
		"team-a",
		false)
	require.NoError(t, err)
	current[planner.FieldDisplayName] = "changed concurrently"
	_, err = write()
	require.ErrorContains(t, err, "changed after adoption validation")
	current[planner.FieldManagedBy] = map[string]any{resources.ChildProtectedKey: "invalid"}
	_, err = family.prepare(t.Context(), adoptionGatewayID, childCandidate{adoptionChildID, "team-child"}, "team-a", false)
	require.ErrorContains(t, err, "invalid protection")
}

func TestAIFamilyBindingUsesIdentityAcrossReorderingAndFiltering(t *testing.T) {
	families := aiChildFamilies(state.NewClient(state.ClientConfig{}))
	slices.Reverse(families)
	family, err := findAIChildFamily(families, "models")
	require.NoError(t, err)
	require.Equal(t, "model", family.selector)
	_, err = findAIChildFamily([]aiChildFamily{family}, "model-providers")
	require.ErrorContains(t, err, "unavailable")
}

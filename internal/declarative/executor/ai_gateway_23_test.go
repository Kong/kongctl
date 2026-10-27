package executor

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kong/kongctl/internal/log"

	kkSDK "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type aiGateway23HTTPClient func(*http.Request) (*http.Response, error)

func (f aiGateway23HTTPClient) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestAIGateway23DatastorePlanAndExecution(t *testing.T) {
	t.Setenv("TOKEN_VAULT_ENCRYPTION_KEY", "encryption-key-sentinel")
	filename := filepath.Join("..", "..", "..", "docs", "examples", "declarative", "ai-gateway", "token-vault.yaml")
	rs, err := loader.New().LoadFile(filename)
	require.NoError(t, err)
	var mu sync.Mutex
	var writes []string
	var datastorePayload, providerPayload, mcpPayload map[string]any
	observed := make(map[string]map[string]any)
	httpClient := aiGateway23HTTPClient(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		status, body := http.StatusOK, `{"data":[],"meta":{"page":{"size":100}}}`
		if req.Method == http.MethodPost {
			writes = append(writes, req.URL.Path)
			var value map[string]any
			require.NoError(t, json.NewDecoder(req.Body).Decode(&value))
			switch req.URL.Path {
			case "/v1/ai-gateways":
				value["id"] = "gateway-id"
			case "/v1/ai-gateways/gateway-id/datastores":
				datastorePayload = value
				value["id"] = "datastore-id"
			case "/v1/ai-gateways/gateway-id/model-providers":
				providerPayload = value
				value["id"] = "provider-id"
			case "/v1/ai-gateways/gateway-id/mcp-servers":
				mcpPayload = value
				value["id"] = "mcp-id"
			default:
				t.Errorf("unexpected write %s", req.URL.Path)
			}
			value["created_at"], value["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
			data, err := json.Marshal(value)
			require.NoError(t, err)
			status, body = http.StatusCreated, string(data)
			// Responses omit write-only encryption keys and include resolved storage.
			var stored map[string]any
			require.NoError(t, json.Unmarshal(data, &stored))
			if req.URL.Path == "/v1/ai-gateways/gateway-id/datastores" {
				config := stored[planner.FieldConfig].(map[string]any)
				for _, key := range config["keys"].([]any) {
					delete(key.(map[string]any), planner.FieldKey)
				}
			}
			if vault, ok := stored[planner.FieldTokenVault].(map[string]any); ok {
				vault["storage"] = map[string]any{
					"directory":       "support-credentials",
					"current_version": "v1",
					"keys":            []any{map[string]any{"version": "v1"}},
				}
			}
			observed[req.URL.Path] = stored
			observed[req.URL.Path+"/"+stored["id"].(string)] = stored
		} else {
			require.Equal(t, http.MethodGet, req.Method)
			var response any
			if stored := observed[req.URL.Path]; stored != nil {
				if strings.HasSuffix(req.URL.Path, "/"+stored["id"].(string)) {
					response = stored
				} else {
					response = map[string]any{"data": []any{stored}, "meta": map[string]any{"page": map[string]any{"size": 100}}}
				}
			}
			if response != nil {
				data, err := json.Marshal(response)
				require.NoError(t, err)
				body = string(data)
			}
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})
	sdk := &helpers.KonnectSDK{
		SDK: kkSDK.New(kkSDK.WithServerURL("https://konnect.test"), kkSDK.WithClient(httpClient)),
	}
	client := state.NewClient(
		state.ClientConfig{
			AIGatewayAPI:           sdk.GetAIGatewayAPI(),
			AIGatewayDatastoresAPI: sdk.GetAIGatewayDatastoresAPI(),
			AIGatewayProvidersAPI:  sdk.GetAIGatewayProvidersAPI(),
			AIGatewayMCPServersAPI: sdk.GetAIGatewayMCPServersAPI(),
		},
	)
	plan, err := planner.NewPlanner(client, slog.Default()).
		GeneratePlan(t.Context(), rs, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 4)
	saved, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NotContains(t, string(saved), "encryption-key-sentinel")
	result := New(
		client,
		nil,
		false,
	).Execute(context.WithValue(t.Context(), log.LoggerKey, slog.Default()), plan)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, writes, 4)
	require.Equal(t, []string{"/v1/ai-gateways", "/v1/ai-gateways/gateway-id/datastores"}, writes[:2])
	require.ElementsMatch(t, []string{
		"/v1/ai-gateways/gateway-id/model-providers",
		"/v1/ai-gateways/gateway-id/mcp-servers",
	}, writes[2:])
	config := datastorePayload[planner.FieldConfig].(map[string]any)
	keys := config["keys"].([]any)
	require.Equal(t, "encryption-key-sentinel", keys[0].(map[string]any)[planner.FieldKey])
	require.Equal(
		t,
		[]any{map[string]any{"name": "credential-storage"}},
		providerPayload[planner.FieldDatastores],
	)
	require.Equal(
		t,
		"upstream-credentials",
		providerPayload[planner.FieldTokenVault].(map[string]any)[planner.FieldProvider],
	)
	require.Equal(t, providerPayload[planner.FieldDatastores], mcpPayload[planner.FieldDatastores])
	require.Equal(t, providerPayload[planner.FieldTokenVault], mcpPayload[planner.FieldTokenVault])
	require.Equal(t, "https://mcp.example.com/mcp", mcpPayload[planner.FieldConfig].(map[string]any)["url"])
	after, err := json.Marshal(plan)
	require.NoError(t, err)
	require.Equal(t, saved, after, "executing must preserve the saved plan")
	// Use a fresh client so the second plan observes API responses, not cached state.
	client = state.NewClient(state.ClientConfig{
		AIGatewayAPI:           sdk.GetAIGatewayAPI(),
		AIGatewayDatastoresAPI: sdk.GetAIGatewayDatastoresAPI(),
		AIGatewayProvidersAPI:  sdk.GetAIGatewayProvidersAPI(),
		AIGatewayMCPServersAPI: sdk.GetAIGatewayMCPServersAPI(),
	})
	repeated, err := planner.NewPlanner(client, slog.Default()).
		GeneratePlan(t.Context(), rs, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, repeated.Changes, "%+v", repeated.Changes)
}

func TestAICostManagementTagsPlanAndExecution(t *testing.T) {
	for _, desired := range []string{`headers: [{name: my-tenant-id, tag_key: tenant}]
  prefixes: [{name: my-co}]`, `headers: []
  prefixes: []`} {
		t.Run(desired, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "cost.yaml")
			require.NoError(
				t,
				os.WriteFile(
					filename,
					[]byte("ai_cost_management_tags:\n- ref: capture\n  "+desired+"\n"),
					0o600,
				),
			)
			rs, err := loader.New().LoadFile(filename)
			require.NoError(t, err)
			var written map[string]any
			httpClient := aiGateway23HTTPClient(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "/v1/ai-cost-management/tag-capture", req.URL.Path)
				body := `{"headers":[{"name":"previous-header","tag_key":"previous"}],"prefixes":[]}`
				if req.Method == http.MethodPut {
					require.NoError(t, json.NewDecoder(req.Body).Decode(&written))
					data, err := json.Marshal(written)
					require.NoError(t, err)
					body = string(data)
				} else {
					require.Equal(t, http.MethodGet, req.Method)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": {"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    req,
				}, nil
			})
			sdk := &helpers.KonnectSDK{
				SDK: kkSDK.New(
					kkSDK.WithServerURL("https://konnect.test"),
					kkSDK.WithClient(httpClient),
				),
			}
			client := state.NewClient(
				state.ClientConfig{AICostManagementTagsAPI: sdk.GetAICostManagementTagsAPI()},
			)
			plan, err := planner.NewPlanner(client, slog.Default()).
				GeneratePlan(t.Context(), rs, planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			require.Equal(t, planner.ActionUpdate, plan.Changes[0].Action)
			result := New(
				client,
				nil,
				false,
			).Execute(context.WithValue(t.Context(), log.LoggerKey, slog.Default()), plan)
			require.Zero(t, result.FailureCount, "%+v", result.Errors)
			require.Equal(t, plan.Changes[0].Fields["headers"], written["headers"])
			require.Equal(t, plan.Changes[0].Fields["prefixes"], written["prefixes"])
			_, err = planner.NewPlanner(client, slog.Default()).
				GeneratePlan(t.Context(), rs, planner.Options{Mode: planner.PlanModeDelete})
			require.ErrorContains(t, err, "cannot be deleted")
		})
	}
}

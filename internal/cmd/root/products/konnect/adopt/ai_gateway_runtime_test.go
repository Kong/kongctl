package adopt

import (
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayAdoptionExportPreservesRuntimeSettings(t *testing.T) {
	for _, autoUpgrade := range []bool{false, true} {
		name := "disabled"
		if autoUpgrade {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			setup, export := childExportTestSetup(t, false, "")
			current := map[string]any{
				"id": adoptionGatewayID, "name": "existing-gateway", "display_name": "Existing gateway",
				"min_runtime_version": "2.2", "runtime_auto_upgrade": autoUpgrade, "deployment_type": "hybrid",
				"labels":     map[string]any{"scenario": "adoption"},
				"managed_by": map[string]any{"tool": "existing-system", "version": "1"},
				"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
			}
			var mu sync.Mutex
			var update map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodPut:
					require.Equal(t, "/v1/ai-gateways/"+adoptionGatewayID, r.URL.Path)
					require.NoError(t, json.NewDecoder(r.Body).Decode(&update))
					maps.Copy(current, update)
					require.NoError(t, json.NewEncoder(w).Encode(current))
				case r.URL.Path == "/v1/ai-gateways":
					require.Equal(t, http.MethodGet, r.Method)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"data": []any{current}, "meta": map[string]any{"page": map[string]any{"total": 1}},
					}))
				default:
					require.Equal(t, http.MethodGet, r.Method)
					require.Equal(t, "/v1/ai-gateways/"+adoptionGatewayID, r.URL.Path)
					require.NoError(t, json.NewEncoder(w).Encode(current))
				}
			}))
			defer server.Close()
			sdk := &helpers.KonnectSDK{SDK: kkSDK.New(kkSDK.WithServerURL(server.URL), kkSDK.WithClient(server.Client()))}
			result, err := adoptAIGateway(setup.Helper, sdk.GetAIGatewayAPI(), stubConfig{pageSize: 100},
				"team-a", false, adoptionGatewayID)
			require.NoError(t, err)
			require.NoError(t, adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "AI Gateway"))
			resources, err := loader.New().LoadFile(export.Path)
			require.NoError(t, err)
			client := state.NewClient(state.ClientConfig{AIGatewayAPI: sdk.GetAIGatewayAPI()})
			plan, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), resources,
				planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Empty(t, plan.Changes, "adoption must not change runtime settings behind its exported configuration")
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, autoUpgrade, current["runtime_auto_upgrade"])
			require.Equal(t, autoUpgrade, update["runtime_auto_upgrade"])
			require.Equal(t, "2.2", current["min_runtime_version"])
			require.Equal(t, "2.2", update["min_runtime_version"])
			require.Equal(t, map[string]any{"tool": "existing-system", "version": "1"}, update["managed_by"])
		})
	}
}

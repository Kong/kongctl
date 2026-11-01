package helpers_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

const (
	registryAgentOne    = "9aac6dbf-43b4-47cd-9bf6-d6ce23db0711"
	registryAgentTwo    = "9aac6dbf-43b4-47cd-9bf6-d6ce23db0712"
	registryAssociation = "9aac6dbf-43b4-47cd-9bf6-d6ce23db0713"
)

func registryCollection(data []any, number, total int) map[string]any {
	return map[string]any{"data": data, "meta": map[string]any{
		"page": map[string]any{"number": number, "size": 1, "total": total},
	}}
}

func TestRegistryReferencesReadAllAgentAndAssociationPagesThroughTheConfiguredSDK(t *testing.T) {
	calls := make([]string, 0)
	sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, request.Method)
		require.Equal(t, "eu.api.konghq.tech", request.URL.Host)
		require.Equal(t, "1", request.URL.Query().Get("page[size]"))
		require.Len(t, request.URL.Query(), 2, "no implicit ownership/name filter or extra parameter")
		number, err := strconv.Atoi(request.URL.Query().Get("page[number]"))
		require.NoError(t, err)
		calls = append(calls, request.URL.Path+"/"+strconv.Itoa(number))
		var body map[string]any
		switch request.URL.Path {
		case "/v1/agents":
			data := []any{}
			if number <= 2 {
				data = append(data, map[string]any{"id": []string{registryAgentOne, registryAgentTwo}[number-1]})
			}
			body = registryCollection(data, number, 2)
		case "/v1/agents/" + registryAgentOne + "/principals":
			body = registryCollection([]any{}, number, 0)
		case "/v1/agents/" + registryAgentTwo + "/principals":
			data := []any{}
			if number == 1 {
				data = append(data, map[string]any{
					"id": registryAssociation, "directory_id": identityDirectory, "principal_id": identityPrincipal,
				})
			}
			body = registryCollection(data, number, 1)
		default:
			t.Fatalf("unexpected Registry route")
		}
		return groupJSONResponse(t, request, body), nil
	})
	sdk.BaseURL = "https://eu.api.konghq.tech"
	links, err := helpers.ListRegistryAgentPrincipalReferences(t.Context(), sdk.GetRegistryAgentReferencesReader(), 1)
	require.NoError(t, err)
	require.Equal(t, []helpers.RegistryAgentPrincipalReference{{
		ID: registryAssociation, DirectoryID: identityDirectory, PrincipalID: identityPrincipal, AgentID: registryAgentTwo,
	}}, links)
	require.Len(t, calls, 6, "later Agents and terminal empty pages must be observed")
}

func TestRegistryReferencesRejectIncompleteInventoriesWithoutReturningPartialResults(t *testing.T) {
	for _, failure := range []string{
		"missing-data", "null-data", "missing-meta", "missing-number", "wrong-number",
		"fractional-size", "negative-total", "incomplete", "changed-total", "duplicate", "invalid-agent",
		"invalid-link", "late-agent-error", "association-error", "bad-json",
	} {
		t.Run(failure, func(t *testing.T) {
			calls := 0
			sdk := identityGroupTestSDK(func(request *http.Request) (*http.Response, error) {
				calls++
				number, err := strconv.Atoi(request.URL.Query().Get("page[number]"))
				require.NoError(t, err)
				if calls > 5 {
					t.Fatal("pagination did not terminate")
				}
				if failure == "late-agent-error" && number == 2 ||
					failure == "association-error" && strings.HasSuffix(request.URL.Path, "/principals") {
					response := groupJSONResponse(t, request, map[string]any{"detail": "private-error-marker"})
					response.StatusCode = http.StatusForbidden
					return response, nil
				}
				data := []any{}
				if number == 1 || failure == "duplicate" {
					data = append(data, map[string]any{"id": registryAgentOne})
				}
				body := registryCollection(data, number, 1)
				meta := body["meta"].(map[string]any)["page"].(map[string]any)
				switch failure {
				case "missing-data":
					delete(body, "data")
				case "null-data":
					body["data"] = nil
				case "missing-meta":
					delete(body, "meta")
				case "missing-number":
					delete(meta, "number")
				case "wrong-number":
					meta["number"] = 0
				case "fractional-size":
					meta["size"] = 1.5
				case "negative-total":
					meta["total"] = -1
				case "incomplete":
					meta["total"] = 2
				case "changed-total":
					if number == 2 {
						meta["total"] = 2
					}
				case "invalid-agent":
					body["data"] = []any{map[string]any{"id": "bad"}}
				case "invalid-link":
					if strings.HasSuffix(request.URL.Path, "/principals") {
						body["data"] = []any{map[string]any{"id": registryAssociation, "directory_id": identityDirectory}}
					}
				case "bad-json":
					return groupJSONResponse(t, request, "not-a-collection"), nil
				}
				return groupJSONResponse(t, request, body), nil
			})
			links, err := helpers.ListRegistryAgentPrincipalReferences(t.Context(), sdk.GetRegistryAgentReferencesReader(), 1)
			require.Error(t, err)
			require.Nil(t, links)
			require.NotContains(t, err.Error(), "private-error-marker")
		})
	}
	_, err := helpers.ListRegistryAgentPrincipalReferences(t.Context(), nil, 1)
	require.ErrorContains(t, err, "not configured")
	// Confirm the inventory projection never exports the observed Agent locator
	// as an association field in API-shaped JSON.
	data, err := json.Marshal(helpers.RegistryAgentPrincipalReference{AgentID: registryAgentOne})
	require.NoError(t, err)
	require.NotContains(t, string(data), registryAgentOne)
}

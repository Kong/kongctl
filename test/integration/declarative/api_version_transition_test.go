//go:build integration

package declarative_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/executor"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/stretchr/testify/require"
)

type catalogTransitionHTTPClient func(*http.Request) (*http.Response, error)

func (c catalogTransitionHTTPClient) Do(r *http.Request) (*http.Response, error) { return c(r) }

// Models the ACE restrictions verified against the live Catalog API: current
// operation removal requires force; the current child cannot be deleted.
func TestSavedAPIVersionTransition(t *testing.T) {
	for _, test := range []struct {
		name           string
		allowRemoval   bool
		inPlace        bool
		targetExists   bool
		rejectCreation bool
		apply          bool
		separate       bool
	}{
		{name: "switch and delete", allowRemoval: true},
		{name: "unapproved switch preserves old version"},
		{name: "failed creation blocks switch and deletion", allowRemoval: true, rejectCreation: true},
		{name: "in-place operation removal", allowRemoval: true, inPlace: true},
		{name: "unapproved in-place update", inPlace: true},
		{name: "update target before switching", allowRemoval: true, targetExists: true},
		{name: "apply retains old version", allowRemoval: true, apply: true},
		{name: "root-level version inherits policy", allowRemoval: true, inPlace: true, separate: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			const apiID = "0632abb3-1716-42bf-8d7a-f64793fd1c0f"
			const oldID = "cd3df9e9-5b86-47a5-b074-d311894ed4df"
			const newID = "8bf944d4-79f2-455b-98e6-d15e7f1186b4"
			original := `{"openapi":"3.0.3","info":{"title":"Bookings","version":"beta.1"},` +
				`"paths":{"/bookings":{"get":{"responses":{"200":{"description":"OK"}}},` +
				`"post":{"responses":{"201":{"description":"Created"}}}}}}`
			desired := `{"openapi":"3.0.3","info":{"title":"Bookings","version":"beta.2"},` +
				`"paths":{"/bookings":{"get":{"responses":{"200":{"description":"OK"}}}}}}`
			current := kkComps.APIResponseSchema{
				ID: apiID, Name: "Bookings", Version: new("beta.1"),
				Labels: map[string]string{labels.NamespaceKey: "default"},
			}
			versions := map[string]kkComps.APIVersionResponse{
				oldID: {ID: oldID, Version: "beta.1", Spec: &kkComps.APIVersionResponseSpec{Content: &original}},
			}
			if test.targetExists {
				versions[newID] = kkComps.APIVersionResponse{
					ID: newID, Version: "beta.2",
					Spec: &kkComps.APIVersionResponseSpec{Content: &original},
				}
			}
			if test.inPlace {
				desired = strings.ReplaceAll(desired, "beta.2", "beta.1")
			}
			desired, err := normalizers.SpecToJSON(desired)
			require.NoError(t, err)
			var mutations []string
			var transportErrors []error
			var mu sync.Mutex
			clientHTTP := catalogTransitionHTTPClient(func(r *http.Request) (httpResponse *http.Response, requestErr error) {
				mu.Lock()
				defer mu.Unlock()
				defer func() {
					if requestErr != nil {
						transportErrors = append(transportErrors, requestErr)
					}
				}()
				status := http.StatusOK
				var response any
				path := strings.TrimPrefix(r.URL.Path, "/v3/apis")
				force := r.URL.Query().Get("force") == "true"
				if r.Method != http.MethodGet {
					mutations = append(mutations, r.Method+" "+path)
				}
				if r.Body != nil {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						return nil, fmt.Errorf("read request body: %w", err)
					}
					if strings.Contains(string(body), "force_delete") {
						return nil, fmt.Errorf("operation removal policy leaked into request body")
					}
					r.Body = io.NopCloser(strings.NewReader(string(body)))
				}
				switch {
				case r.Method == http.MethodGet && path == "":
					response = map[string]any{
						"data": []kkComps.APIResponseSchema{current},
						"meta": map[string]any{"page": map[string]any{"total": 1}},
					}
				case r.Method == http.MethodGet && path == "/"+apiID:
					response = current
				case r.Method == http.MethodGet && path == "/"+apiID+"/versions":
					items := make([]kkComps.APIVersionResponse, 0, len(versions))
					for _, v := range versions {
						items = append(items, v)
					}
					response = map[string]any{"data": items, "meta": map[string]any{"page": map[string]any{"total": len(items)}}}
				case r.Method == http.MethodGet && strings.HasPrefix(path, "/"+apiID+"/versions/"):
					response = versions[strings.TrimPrefix(path, "/"+apiID+"/versions/")]
				case r.Method == http.MethodPost && path == "/"+apiID+"/versions":
					var request kkComps.CreateAPIVersionRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						return nil, fmt.Errorf("decode version create request: %w", err)
					}
					if test.rejectCreation {
						status, response = 400, map[string]any{"detail": "new version rejected"}
					} else {
						versions[newID] = kkComps.APIVersionResponse{
							ID: newID, Version: *request.Version,
							Spec: &kkComps.APIVersionResponseSpec{Content: request.Spec.Content},
						}
						status, response = 201, versions[newID]
					}
				case r.Method == http.MethodPatch && path == "/"+apiID:
					var request kkComps.UpdateAPIRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						return nil, fmt.Errorf("decode API update request: %w", err)
					}
					target := versions[newID]
					if target.Spec == nil || *target.Spec.Content != desired {
						status, response = 409, map[string]any{"detail": "target spec is not ready"}
					} else if !force {
						status, response = 409, map[string]any{"detail": "operation removal requires force"}
					} else {
						current.Version = request.Version
						response = current
					}
				case r.Method == http.MethodPatch && strings.HasPrefix(path, "/"+apiID+"/versions/"):
					id := strings.TrimPrefix(path, "/"+apiID+"/versions/")
					version := versions[id]
					var request kkComps.APIVersionRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						return nil, fmt.Errorf("decode version update request: %w", err)
					}
					if version.Version == *current.Version && !force {
						status, response = 409, map[string]any{"detail": "operation removal requires force"}
					} else {
						version.Spec = &kkComps.APIVersionResponseSpec{Content: request.Spec.Content}
						versions[id] = version
						response = version
					}
				case r.Method == http.MethodDelete && path == "/"+apiID+"/versions/"+oldID:
					if *current.Version == "beta.1" {
						status, response = 409, map[string]any{"detail": "cannot delete current version"}
					} else {
						delete(versions, oldID)
						status = 204
					}
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				body, err := json.Marshal(response)
				if err != nil {
					return nil, fmt.Errorf("encode response: %w", err)
				}
				return &http.Response{
					StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(string(body))), Request: r,
				}, nil
			})
			sdk := kkSDK.New(kkSDK.WithServerURL("https://catalog.example.test"), kkSDK.WithClient(clientHTTP))
			client := state.NewClient(state.ClientConfig{
				APIAPI:        &helpers.APIAPIImpl{SDK: sdk},
				APIVersionAPI: &helpers.APIVersionAPIImpl{SDK: sdk},
			})
			version := "beta.2"
			if test.inPlace {
				version = "beta.1"
			}
			policy := ""
			if test.allowRemoval {
				policy = "    force_delete: true\n"
			}
			manifest := fmt.Sprintf(`apis:
  - ref: bookings
    name: Bookings
    version: %s
%s`, version, policy)
			if test.separate {
				manifest += fmt.Sprintf(`api_versions:
  - ref: current-spec
    api: bookings
    version: %s
    spec: %s
`, version, desired)
			} else {
				manifest += fmt.Sprintf(`    versions:
      - ref: current-spec
        version: %s
        spec: %s
`, version, desired)
			}
			file := filepath.Join(t.TempDir(), "api.yaml")
			require.NoError(t, os.WriteFile(file, []byte(manifest), 0o600))
			rs, err := loader.New().LoadFile(file)
			require.NoError(t, err)
			mode := planner.PlanModeSync
			if test.apply {
				mode = planner.PlanModeApply
			}
			ctx := context.WithValue(t.Context(), log.LoggerKey, slog.Default())
			plan, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(ctx, rs, planner.Options{Mode: mode})
			require.NoError(t, err)
			data, err := json.Marshal(plan)
			require.NoError(t, err)
			var saved planner.Plan
			require.NoError(t, json.Unmarshal(data, &saved))
			result := executor.New(client, nil, false).Execute(ctx, &saved)
			require.Empty(t, transportErrors, "transport failures must not satisfy expected ACE rejections")
			if test.allowRemoval && !test.rejectCreation {
				require.Empty(t, result.Errors)
				require.Equal(t, version, *current.Version)
				if test.inPlace {
					require.Equal(t, desired, *versions[oldID].Spec.Content)
				} else if test.apply {
					require.Contains(t, versions, oldID)
				} else {
					require.NotContains(t, versions, oldID)
				}
				repeated, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(ctx, rs, planner.Options{Mode: mode})
				require.NoError(t, err)
				require.Empty(t, repeated.Changes, "authorization metadata must not cause perpetual updates")
			} else {
				require.Len(t, result.Errors, 1)
				expectedError := "operation removal requires force"
				if test.rejectCreation {
					expectedError = "new version rejected"
				}
				require.Contains(t, result.Errors[0].Error, expectedError)
				require.Equal(t, "beta.1", *current.Version)
				require.Equal(t, original, *versions[oldID].Spec.Content)
				for _, mutation := range mutations {
					require.False(t, strings.HasPrefix(mutation, http.MethodDelete))
					if test.rejectCreation {
						require.True(t, strings.HasPrefix(mutation, http.MethodPost))
					}
				}
			}
		})
	}
}

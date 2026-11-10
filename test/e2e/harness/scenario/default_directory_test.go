//go:build e2e

package scenario

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestSharedDirectoryCleanupPreservesIDAfterInterruptedScenario(t *testing.T) {
	const id = "00000000-0000-4000-8000-000000000001"
	var mu sync.Mutex
	directory := map[string]any{"id": id, "name": "default", "vault_enabled": false}
	posts, deletes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPut:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body["id"] = id
			directory = body
			_ = json.NewEncoder(w).Encode(directory)
		case http.MethodGet:
			data := []any{}
			if r.URL.Path == "/v2/directories" {
				data = append(data, directory)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
		case http.MethodPost:
			posts++
			w.WriteHeader(http.StatusForbidden)
		case http.MethodDelete:
			deletes++
			w.WriteHeader(http.StatusConflict)
		}
	}))
	defer server.Close()
	t.Setenv("KONGCTL_E2E_KONNECT_BASE_URL", server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "test-token")
	t.Setenv("KONGCTL_E2E_IDENTITY_MULTI_DIRECTORY", "0")
	cli, _ := cleanupFixtureCLI(t)
	t.Run("interrupted", func(t *testing.T) {
		vars := map[string]any{}
		require.NoError(t, registerResourceCleanup(t, cli, []CleanupResource{{
			Resource: string(resources.ResourceTypeIdentityDirectory), UseDefaultDirectory: true,
			NameVar: "directory_name", NameEnv: "TEST_DEFAULT_DIRECTORY_NAME",
		}}, vars))
		require.Equal(t, "default", vars["directory_name"])
		require.Equal(t, id, vars["identity_default_directory_id"])
		mu.Lock()
		directory["labels"] = map[string]any{"scenario": "interrupted"}
		directory["vault_enabled"] = true
		mu.Unlock()
	})
	mu.Lock()
	defer mu.Unlock()
	require.Zero(t, posts)
	require.Zero(t, deletes)
	require.Equal(t, id, directory["id"])
	require.Empty(t, directory["labels"])
	require.Equal(t, false, directory["vault_enabled"])
}

func TestSharedDirectoryOptInRetainsGeneratedFixtureMode(t *testing.T) {
	t.Setenv("KONGCTL_E2E_IDENTITY_MULTI_DIRECTORY", "1")
	cli, stateDir := cleanupFixtureCLI(t)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "resources.json"), []byte("[]"), 0o600))
	vars := map[string]any{}
	require.NoError(t, registerResourceCleanup(t, cli, []CleanupResource{{
		Resource: string(resources.ResourceTypeIdentityDirectory), UseDefaultDirectory: true,
		NameVar: "directory_name", NameEnv: "TEST_DEFAULT_DIRECTORY_NAME",
	}}, vars))
	require.True(t, strings.HasPrefix(vars["directory_name"].(string), "kongctl-e2e-identity-directory-"))
	require.NotContains(t, vars, "identity_default_directory_id")
}

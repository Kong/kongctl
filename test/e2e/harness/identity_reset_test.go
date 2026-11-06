//go:build e2e

package harness

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const resetDefaultID = "00000000-0000-4000-8000-000000000001"

type identityResetFixture struct {
	mu          sync.Mutex
	directory   map[string]any
	collections map[string][]map[string]any
	calls       []string
	failPath    string
	legacyKeys  bool
	posts       int
	paged       bool
}

func newIdentityResetFixture() *identityResetFixture {
	root := "/v2/directories/" + resetDefaultID
	f := &identityResetFixture{
		directory:   map[string]any{"id": resetDefaultID, "name": "default", "vault_enabled": false},
		collections: map[string][]map[string]any{},
	}
	sequence := 2
	add := func(path string) string {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", sequence)
		sequence++
		f.collections[path] = append(f.collections[path], map[string]any{"id": id})
		return path + "/" + id
	}
	group := add(root + "/groups")
	add(group + "/membership")
	add(group + "/api-keys")
	for range 2 {
		principal := add(root + "/principals")
		add(principal + "/identities")
		add(principal + "/api-keys")
		add(principal + "/vault-credentials")
		basic := add(principal + "/basic-auths")
		add(basic + "/passwords")
	}
	f.collections[root+"/principals"][0]["provisioned_by"] = "imported"
	f.collections[root+"/principals"][0]["active"] = true
	provider := add(root + "/vault/providers")
	add(provider + "/credentials")
	add(root + "/trusted-idps")
	add(root + "/vault/keys")
	return f
}

func (f *identityResetFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := r.URL.Path
	f.calls = append(f.calls, r.Method+" "+path)
	w.Header().Set("Content-Type", "application/json")
	if path == f.failPath {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"private employee diagnostics"}`))
		return
	}
	if r.Method == http.MethodPost && path == "/v2/directories" {
		f.posts++
		if f.directory != nil {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"type":"quota-exceeded",` +
				`"detail":"Maximum number of [directories] exceeded. Max allowed: [1]"}`))
			return
		}
		f.directory = map[string]any{"id": resetDefaultID}
		if err := json.NewDecoder(r.Body).Decode(&f.directory); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.directory["id"] = resetDefaultID
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(f.directory)
		return
	}
	if path == "/v2/directories/"+resetDefaultID && r.Method == http.MethodPut {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body["id"] = resetDefaultID
		f.directory = body
		_ = json.NewEncoder(w).Encode(body)
		return
	}
	if r.Method == http.MethodGet {
		data := f.collections[path]
		if path == "/v2/directories" {
			data = nil
			if f.directory != nil {
				data = append(data, f.directory)
			}
		}
		if f.legacyKeys && strings.Contains(path, "/groups/") && strings.HasSuffix(path, "/api-keys") {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		next := ""
		if strings.HasSuffix(path, "/principals") && len(data) == 2 {
			if r.URL.Query().Get("page[after]") == "second" {
				data = data[1:]
				f.paged = true
			} else {
				data, next = data[:1], "second"
			}
		}
		if data == nil {
			data = []map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": data, "meta": map[string]any{"page": map[string]any{"next": next}},
		})
		return
	}
	if r.Method == http.MethodPatch {
		for collection, values := range f.collections {
			for _, value := range values {
				if path == collection+"/"+value["id"].(string) && value["provisioned_by"] == "imported" {
					value["active"], value["provisioned_by"] = false, "konnect_user"
					_ = json.NewEncoder(w).Encode(value)
					return
				}
			}
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodDelete {
		// Every dependency must be retired before deleting its parent. Reset
		// must never ask the API to delete the default directory itself.
		if path == "/v2/directories/"+resetDefaultID {
			w.WriteHeader(http.StatusConflict)
			return
		}
		for collection, values := range f.collections {
			if strings.HasPrefix(collection, path+"/") && len(values) != 0 {
				// Legacy group keys are removed by the API with the group.
				if !f.legacyKeys || !strings.HasSuffix(collection, "/api-keys") {
					w.WriteHeader(http.StatusConflict)
					return
				}
				f.collections[collection] = nil
			}
		}
		index := strings.LastIndex(path, "/")
		collection, id := path[:index], path[index+1:]
		for i, value := range f.collections[collection] {
			if value["id"] == id {
				if value["provisioned_by"] == "imported" {
					w.WriteHeader(http.StatusConflict)
					return
				}
				f.collections[collection] = append(f.collections[collection][:i], f.collections[collection][i+1:]...)
				break
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func TestIdentityResetPreservesDefaultWithinOneDirectoryQuota(t *testing.T) {
	for _, legacyKeys := range []bool{false, true} {
		t.Run(fmt.Sprint(legacyKeys), func(t *testing.T) {
			fixture := newIdentityResetFixture()
			fixture.legacyKeys = legacyKeys
			server := httptest.NewServer(http.HandlerFunc(fixture.serve))
			defer server.Close()
			session := newResetHTTPSession(time.Second, HTTPTransportOptions{})
			defer session.Close()
			client := identityResetClient{session: session, baseURL: server.URL, token: "test-token"}
			for range 2 {
				directory, err := resetIdentityDirectories(t.Context(), client, true)
				require.NoError(t, err)
				require.Equal(t, resetDefaultID, directory["id"])
				require.Equal(t, "default", directory["name"])
				require.Equal(t, false, directory["vault_enabled"])
				require.EqualValues(t, 600, directory["ttl_secs"])
				require.Empty(t, directory["managed_by"])
				require.Empty(t, directory["labels"])
			}
			fixture.mu.Lock()
			defer fixture.mu.Unlock()
			require.Zero(t, fixture.posts)
			require.True(t, fixture.paged)
			for path, values := range fixture.collections {
				require.Empty(t, values, path)
			}
			require.NotContains(t, fixture.calls, "DELETE /v2/directories/"+resetDefaultID)
		})
	}
}

func TestIdentityResetBootstrapAndInventoryFailure(t *testing.T) {
	fixture := newIdentityResetFixture()
	fixture.directory = nil
	fixture.collections = map[string][]map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(fixture.serve))
	defer server.Close()
	session := newResetHTTPSession(time.Second, HTTPTransportOptions{})
	defer session.Close()
	client := identityResetClient{session: session, baseURL: server.URL, token: "test-token"}
	directory, err := resetIdentityDirectories(t.Context(), client, false)
	require.NoError(t, err)
	require.Equal(t, resetDefaultID, directory["id"])
	fixture.mu.Lock()
	require.Equal(t, 1, fixture.posts)
	fixture.failPath = "/v2/directories/" + resetDefaultID + "/principals"
	fixture.mu.Unlock()
	_, err = resetIdentityDirectories(t.Context(), client, false)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private employee")
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	require.NotContains(t, fixture.calls, "DELETE /v2/directories/"+resetDefaultID)
}

func TestIdentityResetDoesNotReplaceLostDefault(t *testing.T) {
	fixture := newIdentityResetFixture()
	fixture.directory = nil
	server := httptest.NewServer(http.HandlerFunc(fixture.serve))
	defer server.Close()
	session := newResetHTTPSession(time.Second, HTTPTransportOptions{})
	defer session.Close()
	_, err := resetIdentityDirectories(t.Context(), identityResetClient{
		session: session, baseURL: server.URL, token: "test-token", expectedDefaultID: resetDefaultID,
	}, true)
	require.ErrorContains(t, err, "changed or disappeared")
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	require.Zero(t, fixture.posts)
}

func TestDefaultFixtureReusesDirectoryAfterQuotaFailure(t *testing.T) {
	fixture := newIdentityResetFixture()
	server := httptest.NewServer(http.HandlerFunc(fixture.serve))
	defer server.Close()
	t.Setenv("KONGCTL_E2E_KONNECT_BASE_URL", server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "test-token")
	cli := &CLI{TestDir: t.TempDir(), Timeout: time.Second}
	step, err := NewStep(t, cli, "one-directory-quota")
	require.NoError(t, err)
	_, err = step.CreateResource("identity_directory", []byte(`{"name":"extra"}`), CreateResourceOptions{})
	require.ErrorContains(t, err, "Maximum number of [directories] exceeded")
	result, err := step.CreateResource("identity_directory", []byte(`{"name":"default","vault_enabled":true}`),
		CreateResourceOptions{DefaultDirectoryID: resetDefaultID})
	require.NoError(t, err)
	require.Equal(t, http.MethodPut, result.Method)
	require.Equal(t, http.StatusOK, result.Status)
	require.Contains(t, string(result.Body), resetDefaultID)
}

package root

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// These drive the whole root command against a fake control plane, so the
// pre-run, flag binding, target resolution, credential choice, HTTP request,
// pagination and error rendering all run.
//
// A self managed target resolves no Konnect credential, so pointing
// --control-plane-url at an httptest server exercises the real request path
// with no authentication set up at all. Before this, no mesh test issued an
// HTTP request of any kind.

// meshDescriptors is the /_resources payload: the control plane reports the
// types it serves, and kongctl renders whatever comes back.
func meshDescriptors() map[string]any {
	return map[string]any{
		"resources": []map[string]any{
			{"name": "Mesh", "path": "meshes", "scope": "Global", "shortName": "m", "readOnly": false},
			{"name": "MeshTimeout", "path": "meshtimeouts", "scope": "Mesh", "shortName": "mt", "readOnly": false},
			{"name": "Dataplane", "path": "dataplanes", "scope": "Mesh", "shortName": "dp", "readOnly": true},
		},
	}
}

// meshInputFile puts a resource document on disk and returns its path.
func meshInputFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "resource.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	return path
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

// A listing is returned across two pages, so a client that reads only the
// first page silently drops half the results.
func TestMeshGetPaginatesOverHTTP(t *testing.T) {
	const total = 150

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_resources":
			writeJSON(t, w, http.StatusOK, meshDescriptors())
		case "/meshes":
			requests.Add(1)
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			items := []map[string]any{}
			for i := offset; i < total && len(items) < 100; i++ {
				items = append(items, map[string]any{"name": fmt.Sprintf("mesh-%03d", i)})
			}
			writeJSON(t, w, http.StatusOK, map[string]any{"total": total, "items": items})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result := executeRootForTest(t,
		"get", "mesh", "meshes", "--control-plane-url", server.URL, "--output", "json")
	if result.exitCode != 0 {
		t.Fatalf("expected success\nstderr:\n%s", result.stderr)
	}

	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &payload); err != nil {
		t.Fatalf("decode output: %v\nstdout:\n%s", err, result.stdout)
	}
	if len(payload.Items) != total {
		t.Fatalf("collected %d of %d items; a single-page read would stop at 100", len(payload.Items), total)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("expected 2 page requests, got %d", got)
	}
}

// A self managed control plane authenticates its own callers. No Konnect
// credential is resolved, and none is sent; the control plane token is sent
// only when one was given.
func TestMeshSelfManagedAuthorization(t *testing.T) {
	cases := []struct {
		name           string
		extraArgs      []string
		wantAuthHeader string
	}{
		{name: "no token sends no authorization header", wantAuthHeader: ""},
		{
			name:           "the control plane token is sent when given",
			extraArgs:      []string{"--control-plane-token", "cp-secret"},
			wantAuthHeader: "Bearer cp-secret",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seen string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/meshes" {
					seen = r.Header.Get("Authorization")
				}
				if r.URL.Path == "/_resources" {
					writeJSON(t, w, http.StatusOK, meshDescriptors())
					return
				}
				writeJSON(t, w, http.StatusOK, map[string]any{"total": 0, "items": []any{}})
			}))
			defer server.Close()

			args := append([]string{"get", "mesh", "meshes", "--control-plane-url", server.URL}, tc.extraArgs...)
			result := executeRootForTest(t, args...)
			if result.exitCode != 0 {
				t.Fatalf("expected success\nstderr:\n%s", result.stderr)
			}
			if seen != tc.wantAuthHeader {
				t.Fatalf("Authorization = %q, want %q", seen, tc.wantAuthHeader)
			}
		})
	}
}

// A write that the control plane rejects must fail the command and surface
// what the control plane said, rather than reporting a successful apply.
func TestMeshApplyReportsWriteFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_resources" {
			writeJSON(t, w, http.StatusOK, meshDescriptors())
			return
		}
		writeJSON(t, w, http.StatusBadRequest, map[string]any{
			"title": "Resource is not valid",
			"invalid_parameters": []map[string]any{
				{"field": "spec.targetRef.name", "reason": "unknown field"},
			},
		})
	}))
	defer server.Close()

	// JSON output, so the reason is not truncated to a table column.
	result := executeRootForTest(t,
		"apply", "mesh", "-f", meshInputFile(t, "type: Mesh\nname: rejected\n"),
		"--control-plane-url", server.URL, "--output", "json")

	if result.exitCode == 0 {
		t.Fatalf("expected a rejected write to fail\nstdout:\n%s", result.stdout)
	}

	var rows []struct {
		Name   string `json:"name"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &rows); err != nil {
		t.Fatalf("decode output: %v\nstdout:\n%s", err, result.stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one reported resource, got %d", len(rows))
	}
	// The row must say it failed, and carry what the control plane objected
	// to, rather than reporting the write as applied.
	if !strings.HasPrefix(rows[0].Result, "failed:") {
		t.Fatalf("result = %q, want a failure", rows[0].Result)
	}
	for _, want := range []string{"Resource is not valid", "spec.targetRef.name", "unknown field"} {
		if !strings.Contains(rows[0].Result, want) {
			t.Fatalf("result %q does not surface %q", rows[0].Result, want)
		}
	}
}

// A read-only type is refused before a request is sent, naming the type.
func TestMeshApplyRefusesReadOnlyType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_resources" {
			writeJSON(t, w, http.StatusOK, meshDescriptors())
			return
		}
		t.Errorf("no request should reach %s for a read only type", r.URL.Path)
	}))
	defer server.Close()

	result := executeRootForTest(t,
		"apply", "mesh", "-f", meshInputFile(t, "type: Dataplane\nname: dp-1\nmesh: default\n"),
		"--control-plane-url", server.URL)

	if result.exitCode == 0 {
		t.Fatal("expected a read only type to be refused")
	}
	if !strings.Contains(result.stdout+result.stderr, "read only") {
		t.Fatalf("expected the refusal to say the type is read only\nstderr:\n%s", result.stderr)
	}
}

// A remote input named by `-f <url>` is not the control plane, so the control
// plane's trust policy must not reach it.
//
// One client builder installed the control plane's client certificate, private
// CA and tls-skip-verify on every destination, so `apply mesh -f https://...`
// presented the operator's control plane certificate to whatever server held
// the file and accepted that server's certificate through a skip-verify meant
// for the control plane. Omitting the bearer token did not isolate TLS.
//
// The input server here is self signed. The control plane is configured with
// --tls-skip-verify, so if that setting leaked the download would succeed.
func TestMeshInputDownloadDoesNotInheritControlPlaneTrust(t *testing.T) {
	inputServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		if _, err := w.Write([]byte("type: Mesh\nname: from-the-input-server\n")); err != nil {
			t.Errorf("write input: %v", err)
		}
	}))
	defer inputServer.Close()

	var reachedControlPlane bool
	controlPlane := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_resources" {
			writeJSON(t, w, http.StatusOK, meshDescriptors())
			return
		}
		reachedControlPlane = true
		writeJSON(t, w, http.StatusOK, map[string]any{})
	}))
	defer controlPlane.Close()

	result := executeRootForTest(t,
		"apply", "mesh", "-f", inputServer.URL+"/resource.yaml",
		"--control-plane-url", controlPlane.URL,
		"--tls-skip-verify")

	if result.exitCode == 0 {
		t.Fatalf("the input download inherited the control plane's skip-verify\nstdout:\n%s", result.stdout)
	}
	if reachedControlPlane {
		t.Fatal("a resource fetched over an unverified connection reached the control plane")
	}
	// The failure must be the input download's certificate, not something else.
	combined := result.stdout + result.stderr
	if !strings.Contains(combined, "certificate") && !strings.Contains(combined, "x509") {
		t.Fatalf("expected a certificate verification failure\nstdout:\n%s\nstderr:\n%s",
			result.stdout, result.stderr)
	}
}

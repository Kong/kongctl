//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kong/kongctl/test/e2e/harness"
	"github.com/kong/kongctl/test/e2e/harness/scenario"
)

// Run the live scenario definitions against a local API fixture as well, so
// their commands and assertions remain exercised without a provisioned Mesh CP.
// This validates the CLI contract; live scenarios validate server compatibility.
func TestMeshCLI(t *testing.T) {
	const cpID = "mesh-e2e-fixture"
	const prefix = "/v3/mesh/control-planes/" + cpID
	var mu sync.Mutex
	resources := map[string]map[string]any{
		"/meshes/default": {"type": "Mesh", "name": "default"},
		// Exercise recovery from an interrupted previous lifecycle run.
		"/meshes/kongctl-e2e-mesh-lifecycle": {
			"type": "Mesh", "name": "kongctl-e2e-mesh-lifecycle",
		},
		"/meshes/kongctl-e2e-mesh-lifecycle/meshtimeouts/kongctl-e2e-timeouts": {
			"type": "MeshTimeout", "name": "kongctl-e2e-timeouts", "mesh": "kongctl-e2e-mesh-lifecycle",
		},
	}
	descriptors := []map[string]any{
		{"name": "Mesh", "path": "meshes", "scope": "Global", "includeInFederation": true},
		{
			"name": "MeshTimeout", "path": "meshtimeouts", "scope": "Mesh", "includeInFederation": true,
			"policy": map[string]any{"isTargetRef": true},
		},
		{"name": "Dataplane", "path": "dataplanes", "scope": "Mesh", "readOnly": true},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		respond := func(value any) {
			if err := json.NewEncoder(w).Encode(value); err != nil {
				t.Errorf("encode fixture response: %v", err)
			}
		}
		if r.Header.Get("Authorization") != "Bearer mesh-fixture-pat" {
			t.Errorf("hosted request missing fixture credential")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/v3/mesh/control-planes" {
			respond(map[string]any{"data": []any{map[string]any{"id": cpID, "name": "fixture", "version": "v3"}}})
			return
		}
		path := strings.TrimPrefix(r.URL.Path, prefix)
		switch {
		case path == "/_resources":
			respond(map[string]any{"resources": descriptors})
		case strings.HasPrefix(path, "/tokens/"):
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "eyJ"+strings.Repeat("a", 60)+"."+strings.Repeat("b", 60)+".signature")
		case path == "/meshes/default/dataplanes/dp1/_layout":
			respond(map[string]any{"inbounds": []any{map[string]any{"kri": "kri_dp_default___dp1_http"}}})
		case strings.HasSuffix(path, "/_policies"):
			respond(map[string]any{"policies": []any{map[string]any{
				"kind": "MeshTimeout", "origins": []any{map[string]any{"name": "slow"}},
			}}})
		case strings.HasSuffix(path, "/stats"):
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "upstream_rq_total: 7\n")
		case strings.HasSuffix(path, "/_resources/dataplanes") || strings.HasSuffix(path, "/_overview"):
			// Return short pages despite size=100 to exercise pagination.
			name := "dp1"
			if r.URL.Query().Get("offset") == "1" {
				name = "dp2"
			}
			respond(map[string]any{"total": 2, "items": []any{map[string]any{"name": name, "mesh": "default"}}})
		case r.Method == http.MethodPut:
			var resource map[string]any
			if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
				t.Errorf("decode write: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, exists := resources[path]
			resource["labels"] = map[string]any{"kuma.io/origin": "global"}
			resources[path] = resource
			if !exists {
				w.WriteHeader(http.StatusCreated)
			}
			respond(map[string]any{})
		case r.Method == http.MethodDelete:
			if _, exists := resources[path]; !exists {
				w.WriteHeader(http.StatusNotFound)
				respond(map[string]any{"title": "Not Found", "detail": "resource not found"})
				return
			}
			delete(resources, path)
			w.WriteHeader(http.StatusNoContent)
		case resources[path] != nil:
			respond(resources[path])
		default:
			if path != "/meshes" && !strings.HasSuffix(path, "/meshtimeouts") {
				t.Errorf("unexpected fixture request: %s %s", r.Method, r.URL.String())
				w.WriteHeader(http.StatusNotFound)
				return
			}
			items := []any{}
			for key, resource := range resources {
				if strings.HasPrefix(key, path+"/") && !strings.Contains(strings.TrimPrefix(key, path+"/"), "/") {
					items = append(items, resource)
				}
			}
			respond(map[string]any{"items": items, "total": len(items)})
		}
	}))
	defer server.Close()
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "mesh-fixture-pat")
	t.Setenv("KONGCTL_E2E_KONNECT_BASE_URL", server.URL)
	t.Setenv("KONGCTL_E2E_RUN_MESH", "1")
	t.Setenv("KONGCTL_E2E_MESH_CONTROL_PLANE_ID", cpID)
	t.Setenv("KONGCTL_E2E_STOP_AFTER", "")
	t.Setenv("KONGCTL_E2E_SKIP_STEPS", "")
	for _, name := range []string{"discovery", "apply-get-delete", "tokens"} {
		t.Run(name, func(t *testing.T) {
			_, err := scenario.Run(t, filepath.Join("scenarios", "mesh", name, "scenario.yaml"), scenario.BetaModeFail)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("export-and-inspect", func(t *testing.T) {
		cli, err := harness.NewCLIT(t)
		if err != nil {
			t.Fatal(err)
		}
		run := func(args ...string) string {
			t.Helper()
			args = append(args, "--control-plane-id", cpID)
			result, err := cli.Run(t.Context(), args...)
			if err != nil {
				t.Fatalf("%v: %v\nstderr: %s", args, err, result.Stderr)
			}
			return result.Stdout
		}
		cli.OverrideNextStdin("type: MeshTimeout\nname: slow\nmesh: default\nspec:\n  targetRef:\n    kind: Mesh\n")
		run("apply", "mesh", "-f", "-")
		cli.DisableNextOutput()
		exported := run("dump", "mesh", "--export-profile", "federation-with-policies")
		if !strings.Contains(exported, "name: slow") || strings.Contains(exported, "kuma.io/origin") {
			t.Fatalf("unexpected migration export: %s", exported)
		}
		run("delete", "mesh", "meshtimeouts", "slow")
		cli.OverrideNextStdin(exported)
		run("apply", "mesh", "-f", "-")
		if !strings.Contains(run("get", "mesh", "meshtimeouts", "slow"), "slow") {
			t.Fatal("export failed to recreate the policy")
		}
		for _, args := range [][]string{
			{"get", "mesh", "inspect", "dataplanes"},
			{"get", "konnect", "mesh", "inspect", "meshtimeout", "slow"},
		} {
			output := run(args...)
			var envelope struct{ Total int }
			if err := json.Unmarshal([]byte(output), &envelope); err != nil || envelope.Total != 2 {
				t.Fatalf("inspection lost a page: %s (decode: %v)", output, err)
			}
		}
		if output := run("get", "mesh", "inspect", "dataplane", "dp1"); !strings.Contains(output, "MeshTimeout") {
			t.Fatalf("inspection lost port policies: %s", output)
		}
		if output := run("get", "mesh", "inspect", "dataplane", "dp1", "--type", "stats"); output != "upstream_rq_total: 7\n" {
			t.Fatalf("proxy statistics changed: %q", output)
		}
	})
}

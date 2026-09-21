//go:build e2e

package harness

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTeamCreateRecoversWithoutRepeatingPost(t *testing.T) {
	for _, mode := range []string{"recovered", "delayed", "duplicate", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			var posts atomic.Int32
			var gets atomic.Int32
			var created map[string]any
			ready := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts.Add(1)
					if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
						t.Error(err)
					}
					created["id"] = "created-id"
					close(ready)
					// Commit the create before the client deadline; never deliver its response in time.
					<-r.Context().Done()
					return
				}
				<-ready
				if mode == "delayed" {
					switch gets.Add(1) {
					case 1:
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					case 2:
						_, _ = fmt.Fprint(w, `{"data":[]}`)
						return
					}
				}
				item := map[string]any{}
				for k, v := range created {
					item[k] = v
				}
				if mode == "conflict" {
					item["name"] = "wrong"
				}
				data := []map[string]any{item}
				if mode == "duplicate" {
					data = append(data, item)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
			}))
			defer server.Close()
			t.Setenv(KonnectBaseAuthURLEnvName, server.URL)
			t.Setenv("KONGCTL_E2E_KONNECT_PAT", "secret")
			t.Setenv("KONGCTL_E2E_HTTP_TIMEOUT", "30ms")
			t.Setenv("KONGCTL_E2E_LOG_LEVEL", "trace")
			dir := t.TempDir()
			s := &Step{cli: &CLI{TestDir: dir}}
			result, err := s.CreateResource(
				"organization_team", []byte(`{"name":"team","labels":{"owner":"test"}}`),
				CreateResourceOptions{RecoverTeam: true},
			)
			if posts.Load() != 1 {
				t.Fatalf("POST count=%d", posts.Load())
			}
			commands, _ := os.ReadDir(filepath.Join(dir, "commands"))
			if len(commands) != 2 {
				t.Fatalf("expected one POST and one recovery directory, got %d", len(commands))
			}
			inventories, _ := filepath.Glob(filepath.Join(dir, "commands", "*", "response.json"))
			if len(inventories) != 0 {
				t.Fatal("recovery saved full inventory payloads")
			}
			if mode == "recovered" || mode == "delayed" {
				if err != nil {
					t.Fatal(err)
				}
				if result.Parsed.(map[string]any)["id"] != "created-id" {
					t.Fatal(result.Parsed)
				}
			} else if !IsCreateOutcomeUnknown(err) {
				t.Fatalf("expected unknown outcome: %v", err)
			}
			files, _ := filepath.Glob(filepath.Join(dir, "commands", "*", "http-phases.jsonl"))
			if len(files) == 0 {
				t.Fatal("missing synthetic request traces")
			}
			for _, path := range files {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), "secret") || strings.Contains(string(data), server.URL) {
					t.Fatal("trace leaked request data")
				}
			}
		})
	}
}

func TestCreateUnexpectedSuccessStatusDoesNotRecover(t *testing.T) {
	for _, recoverTeam := range []bool{false, true} {
		t.Run(fmt.Sprint(recoverTeam), func(t *testing.T) {
			var gets atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					gets.Add(1)
				}
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprint(w, `{}`)
			}))
			defer server.Close()
			t.Setenv(KonnectBaseAuthURLEnvName, server.URL)
			t.Setenv("KONGCTL_E2E_KONNECT_PAT", "secret")
			s := &Step{cli: &CLI{TestDir: t.TempDir()}}
			result, err := s.CreateResource("organization_team", []byte(`{"name":"team"}`),
				CreateResourceOptions{RecoverTeam: recoverTeam, ExpectStatus: http.StatusOK})
			if err == nil || IsCreateOutcomeUnknown(err) || !strings.Contains(err.Error(), "unexpected status 201") ||
				result.Status != http.StatusCreated || gets.Load() != 0 {
				t.Fatalf("status mismatch was masked: %+v %v, GETs=%d", result, err, gets.Load())
			}
		})
	}
}

func TestCreateBodyReadFailureIsUnknownWithoutCapture(t *testing.T) {
	t.Chdir(t.TempDir())
	previous := captureEnabled
	captureEnabled = false
	t.Cleanup(func() { captureEnabled = previous })
	t.Setenv("KONGCTL_E2E_LOG_LEVEL", "trace")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer server.Close()
	t.Setenv(KonnectBaseAuthURLEnvName, server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "secret")
	s := &Step{cli: &CLI{TestDir: t.TempDir()}}
	result, err := s.CreateResource("organization_team", []byte(`{"name":"team"}`), CreateResourceOptions{})
	if !IsCreateOutcomeUnknown(err) || result.Status != http.StatusCreated {
		t.Fatalf("body read failure was not unknown: %+v %v", result, err)
	}
	files, err := os.ReadDir(".")
	if err != nil || len(files) != 0 {
		t.Fatalf("capture disabled wrote artifacts: %v %v", files, err)
	}
}

func TestTeamRecoveryWithoutCapture(t *testing.T) {
	t.Chdir(t.TempDir())
	previous := captureEnabled
	captureEnabled = false
	t.Cleanup(func() { captureEnabled = previous })
	t.Setenv("KONGCTL_E2E_LOG_LEVEL", "trace")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":[{"id":"one","name":"team","labels":{"e2e-create-id":"marker"}}]}`)
	}))
	defer server.Close()
	t.Setenv(KonnectBaseAuthURLEnvName, server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "secret")
	s := &Step{cli: &CLI{TestDir: t.TempDir()}}
	_, err := s.recoverTeamCreate(map[string]any{"name": "team"}, "marker", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(".")
	if err != nil || len(files) != 0 {
		t.Fatalf("capture disabled wrote recovery artifacts: %v %v", files, err)
	}
}

func TestTeamRecoveryAbsentIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("unexpected mutation")
		}
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()
	t.Setenv(KonnectBaseAuthURLEnvName, server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "secret")
	s := &Step{cli: &CLI{TestDir: t.TempDir()}}
	start := time.Now()
	_, err := s.recoverTeamCreate(map[string]any{"name": "team"}, "missing", 50*time.Millisecond)
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("unbounded or accepted missing team: %v", err)
	}
}

func TestCreateTimeoutPreservesMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	t.Setenv("KONGCTL_E2E_KONNECT_BASE_URL", server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "secret")
	t.Setenv("KONGCTL_E2E_HTTP_TIMEOUT", "30ms")
	s := &Step{cli: &CLI{TestDir: t.TempDir()}}
	result, err := s.CreateResource("portal", nil, CreateResourceOptions{})
	if !IsCreateOutcomeUnknown(err) || !result.TimedOut ||
		result.Duration < 30*time.Millisecond || result.Method != "POST" {
		t.Fatalf("lost failure metadata: %+v %v", result, err)
	}
}

func TestTeamRecoveryChecksAllPages(t *testing.T) {
	var gets atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets.Add(1)
		team := map[string]any{"id": "one", "name": "team", "labels": map[string]any{teamCreateMarker: "marker"}}
		data := []map[string]any{team}
		if r.URL.Query().Get("page[number]") == "1" {
			for range 99 {
				data = append(data, map[string]any{"id": "unrelated"})
			}
		} else {
			team["id"] = "two"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	t.Setenv(KonnectBaseAuthURLEnvName, server.URL)
	t.Setenv("KONGCTL_E2E_KONNECT_PAT", "secret")
	s := &Step{cli: &CLI{TestDir: t.TempDir()}}
	_, err := s.recoverTeamCreate(map[string]any{"name": "team"}, "marker", time.Second)
	if err == nil || !strings.Contains(err.Error(), "multiple teams") || gets.Load() != 2 {
		t.Fatalf("accepted first page: %v", err)
	}
}

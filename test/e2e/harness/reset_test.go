//go:build e2e

package harness

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOnlyE2EEmailDomains(t *testing.T) {
	tests := []struct {
		name     string
		resource map[string]any
		want     bool
	}{
		{
			name:     "e2e mail domain",
			resource: map[string]any{"domain": "abc123.mail.kongctl-e2e.io"},
			want:     true,
		},
		{
			name:     "non e2e domain",
			resource: map[string]any{"domain": "example.com"},
			want:     false,
		},
		{
			name:     "missing domain",
			resource: map[string]any{"id": "abc123"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := onlyE2EEmailDomains(tt.resource); got != tt.want {
				t.Fatalf("onlyE2EEmailDomains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfiguredE2EUserEmails(t *testing.T) {
	t.Setenv("KONGCTL_E2E_ORG_USER_EMAIL_2", "user-2@example.com")
	t.Setenv("KONGCTL_E2E_ORG_USER_EMAIL_1", "user-1@example.com")
	t.Setenv("KONGCTL_E2E_ORG_USER_EMAIL_3", "user-2@example.com")
	t.Setenv("KONGCTL_E2E_ORG_USER_EMAIL_EMPTY", " ")
	t.Setenv("KONGCTL_E2E_OTHER_EMAIL", "ignored@example.com")

	got := configuredE2EUserEmails()
	want := []string{"user-1@example.com", "user-2@example.com"}
	if !slices.Equal(got, want) {
		t.Fatalf("configuredE2EUserEmails() = %#v, want %#v", got, want)
	}
}

func TestWriteResetDeletionSummary(t *testing.T) {
	tests := []struct {
		name    string
		summary ResetSummary
		want    string
	}{
		{
			name: "skipped",
			summary: ResetSummary{
				Executed: false,
				Reason:   "missing PAT",
			},
			want: "Reset skipped: missing PAT\n",
		},
		{
			name: "no top-level resources deleted",
			summary: ResetSummary{
				Executed: true,
				Details: []ResetEndpointSummary{
					{APIVersion: "v3", Endpoint: "e2e-user-assignments", Total: 2, Deleted: 2},
					{APIVersion: "v2", Endpoint: "directories", Total: 0, Deleted: 0},
				},
			},
			want: "No top-level resources deleted.\n",
		},
		{
			name: "top-level resources deleted",
			summary: ResetSummary{
				Executed: true,
				Details: []ResetEndpointSummary{
					{APIVersion: "v2", Endpoint: "directories", Total: 1, Deleted: 1},
					{APIVersion: "v2", Endpoint: "control-planes", Total: 2, Deleted: 2},
				},
			},
			want: "Deleted top-level resources:\n" +
				"  v2/directories: 1 deleted (1 found)\n" +
				"  v2/control-planes: 2 deleted (2 found)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := WriteResetDeletionSummary(&out, tt.summary); err != nil {
				t.Fatalf("WriteResetDeletionSummary() error = %v", err)
			}
			if got := out.String(); got != tt.want {
				t.Fatalf("WriteResetDeletionSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeleteAllDeletesFilteredItemsAcrossPages(t *testing.T) {
	type team struct {
		id         string
		systemTeam bool
	}

	teams := make([]team, 0, resetListPageSize+2)
	for i := range resetListPageSize {
		teams = append(teams, team{
			id:         "system-team-" + strconv.Itoa(i),
			systemTeam: true,
		})
	}
	teams = append(
		teams,
		team{id: "e2e-team-alpha"},
		team{id: "e2e-team-beta"},
	)

	deleted := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v3/teams":
			pageSize, pageNumber := resetListPageSize, 1
			if v := r.URL.Query().Get("page[size]"); v != "" {
				if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
					pageSize = parsed
				}
			}
			if v := r.URL.Query().Get("page[number]"); v != "" {
				if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
					pageNumber = parsed
				}
			}

			active := make([]map[string]any, 0, len(teams))
			for _, t := range teams {
				if deleted[t.id] {
					continue
				}
				active = append(active, map[string]any{
					"id":          t.id,
					"system_team": t.systemTeam,
				})
			}

			start := (pageNumber - 1) * pageSize
			end := min(start+pageSize, len(active))
			if start > len(active) {
				start = len(active)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": active[start:end],
			})

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v3/teams/"):
			id := strings.TrimPrefix(r.URL.Path, "/v3/teams/")
			for _, t := range teams {
				if t.id == id && !deleted[id] {
					deleted[id] = true
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			http.NotFound(w, r)

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	policy := HTTPRetryPolicy{
		RequestTimeout: time.Second,
		Backoff:        BackoffConfig{Attempts: 1},
	}
	total, deleteCount, err := deleteAll(
		t.Context(),
		server.URL,
		server.URL,
		"test-token",
		"v3",
		"teams",
		"",
		"",
		"",
		skipSystemTeams,
		nil,
		policy,
		HTTPTransportOptions{},
	)
	if err != nil {
		t.Fatalf("deleteAll() error = %v", err)
	}
	if total != resetListPageSize+2 {
		t.Fatalf("deleteAll() total = %d, want %d", total, resetListPageSize+2)
	}
	if deleteCount != 2 {
		t.Fatalf("deleteAll() deleteCount = %d, want 2", deleteCount)
	}
	if !deleted["e2e-team-alpha"] || !deleted["e2e-team-beta"] {
		t.Fatalf("deleteAll() deleted = %#v, want both e2e teams deleted", deleted)
	}
	if deleted["system-team-0"] {
		t.Fatal("deleteAll() deleted system-team-0, want system teams skipped")
	}
}

func TestTryDeleteDirectoryPrincipalsDeletesIdentitiesBeforePrincipals(t *testing.T) {
	deletedPrincipals := map[string]bool{}
	deletedIdentities := map[string]bool{}
	principalIDs := []string{"principal-1", "principal-2"}
	identityIDs := map[string][]string{
		"principal-1": {"identity-1", "identity-2"},
		"principal-2": {"identity-3"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/directories/dir-1/principals":
			data := make([]map[string]any, 0, len(principalIDs))
			for _, principalID := range principalIDs {
				if deletedPrincipals[principalID] {
					continue
				}
				data = append(data, map[string]any{"id": principalID})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data})

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v2/directories/dir-1/principals/") &&
			strings.HasSuffix(r.URL.Path, "/identities"):
			principalID := strings.TrimSuffix(
				strings.TrimPrefix(r.URL.Path, "/v2/directories/dir-1/principals/"),
				"/identities",
			)
			data := make([]map[string]any, 0, len(identityIDs[principalID]))
			for _, identityID := range identityIDs[principalID] {
				if deletedIdentities[identityID] {
					continue
				}
				data = append(data, map[string]any{"id": identityID})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data})

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/identities/"):
			identityID := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			deletedIdentities[identityID] = true
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v2/directories/dir-1/principals/"):
			principalID := strings.TrimPrefix(r.URL.Path, "/v2/directories/dir-1/principals/")
			for _, identityID := range identityIDs[principalID] {
				if !deletedIdentities[identityID] {
					http.Error(w, "cannot delete principal with associated identities", http.StatusConflict)
					return
				}
			}
			deletedPrincipals[principalID] = true
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	policy := HTTPRetryPolicy{
		RequestTimeout: time.Second,
		Backoff:        BackoffConfig{Attempts: 1},
	}
	session := newResetHTTPSession(policy.RequestTimeout, HTTPTransportOptions{})
	defer session.Close()

	tryDeleteDirectoryPrincipals(t.Context(), session, server.URL+"/v2/directories", "test-token", "dir-1", policy)

	for _, principalID := range principalIDs {
		if !deletedPrincipals[principalID] {
			t.Fatalf("expected principal %s to be deleted", principalID)
		}
	}
	for _, ids := range identityIDs {
		for _, identityID := range ids {
			if !deletedIdentities[identityID] {
				t.Fatalf("expected identity %s to be deleted", identityID)
			}
		}
	}
}

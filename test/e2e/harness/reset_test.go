//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
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

func TestSkipAIGatewaysResetForUnsupportedRegion(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    bool
	}{
		{
			name:    "tech eu",
			baseURL: "https://eu.api.konghq.tech",
			want:    true,
		},
		{
			name:    "production eu",
			baseURL: "https://eu.api.konghq.com",
			want:    true,
		},
		{
			name:    "tech us",
			baseURL: "https://us.api.konghq.tech",
			want:    false,
		},
		{
			name:    "global",
			baseURL: "https://global.api.konghq.tech",
			want:    false,
		},
		{
			name:    "invalid url",
			baseURL: "%",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := skipAIGatewaysResetForUnsupportedRegion(tt.baseURL); got != tt.want {
				t.Fatalf("skipAIGatewaysResetForUnsupportedRegion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResetResourcesDeletesFilteredItemsAcrossPages(t *testing.T) {
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
	details, err := resetResources(
		t.Context(),
		server.URL,
		server.URL,
		"test-token",
		[]resetResourceSpec{{Version: "v3", Endpoint: "teams", Filter: skipSystemTeams}},
		policy,
		HTTPTransportOptions{},
	)
	if err != nil {
		t.Fatalf("resetResources() error = %v", err)
	}
	metrics := details[0]
	if metrics.Total != resetListPageSize+2 {
		t.Fatalf("reset total = %d, want %d", metrics.Total, resetListPageSize+2)
	}
	if metrics.Deleted != 2 {
		t.Fatalf("reset deleteCount = %d, want 2", metrics.Deleted)
	}
	if metrics.ListCalls != 2 {
		t.Fatalf("reset list calls = %d, want 2", metrics.ListCalls)
	}
	if metrics.DeleteCalls != 2 {
		t.Fatalf("reset delete calls = %d, want 2", metrics.DeleteCalls)
	}
	if !deleted["e2e-team-alpha"] || !deleted["e2e-team-beta"] {
		t.Fatalf("reset deleted = %#v, want both e2e teams deleted", deleted)
	}
	if deleted["system-team-0"] {
		t.Fatal("reset deleted system-team-0, want system teams skipped")
	}
}

func TestResetResourcesBoundsReadsAndPreservesDeleteOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	sequence := make([]resetResourceSpec, 7)
	for i := range sequence {
		sequence[i] = resetResourceSpec{Version: "v3", Endpoint: fmt.Sprintf("resource-%d", i)}
	}
	started := make(chan struct{}, len(sequence))
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	var mu sync.Mutex
	active, peak, lists := 0, 0, 0
	var deletes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			mu.Lock()
			active++
			peak = max(peak, active)
			mu.Unlock()
			started <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
			}
			mu.Lock()
			active--
			lists++
			mu.Unlock()
			_, _ = fmt.Fprint(w, `{"data":[{"id":"delete-me"},{"id":"managed","konnect_managed":true}]}`)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if active != 0 || lists != len(sequence) {
			t.Errorf("deletion before inventory barrier: active=%d, lists=%d", active, lists)
		}
		deletes = append(deletes, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	type outcome struct {
		details []resetEndpoint
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		details, err := resetResources(ctx, server.URL, server.URL, "test-token", sequence,
			HTTPRetryPolicy{RequestTimeout: time.Second, Backoff: BackoffConfig{Attempts: 1}}, HTTPTransportOptions{})
		done <- outcome{details, err}
	}()
	for range resetInventoryConcurrency {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("inventory reads did not overlap")
		}
	}
	unblock()
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	mu.Lock()
	defer mu.Unlock()
	if peak != resetInventoryConcurrency {
		t.Fatalf("peak concurrent reads = %d, want %d", peak, resetInventoryConcurrency)
	}
	if len(deletes) != len(sequence) || len(got.details) != len(sequence) {
		t.Fatalf("deletes=%v, details=%+v", deletes, got.details)
	}
	for i, step := range sequence {
		if deletes[i] != "/v3/"+step.Endpoint+"/delete-me" {
			t.Errorf("delete[%d] = %s, want %s", i, deletes[i], step.Endpoint)
		}
		detail := got.details[i]
		if detail.Endpoint != step.Endpoint || detail.Total != 2 || detail.Deleted != 1 ||
			detail.ListCalls != 1 || detail.DeleteCalls != 1 {
			t.Errorf("detail[%d] = %+v", i, detail)
		}
	}
}

func TestResetResourcesKeepsRoutingFiltersAndPreDelete(t *testing.T) {
	var mu sync.Mutex
	var deletes []string
	newServer := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			if r.Method == http.MethodDelete {
				deletes = append(deletes, name+r.URL.Path)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			wantGlobal := slices.Contains([]string{"/v3/audit-log-destinations", "/v3/system-accounts", "/v3/teams"},
				r.URL.Path)
			if (name == "global") != wantGlobal {
				t.Errorf("incorrect list routing: %s %s", name, r.URL.Path)
			}
			switch r.URL.Path {
			case "/v3/portals/email-domains":
				_, _ = fmt.Fprint(w, `{"data":[{"domain":"test.mail.kongctl-e2e.io"},{"domain":"example.com"}]}`)
			case "/v3/teams":
				_, _ = fmt.Fprint(w, `{"data":[{"id":"user-team"},{"id":"system","system_team":true}]}`)
			case "/v1/ai-gateways":
				t.Error("skipped category received a request")
			default:
				_, _ = fmt.Fprint(w, `{"data":[{"id":"resource-id"}]}`)
			}
		}))
	}
	regional, global := newServer("regional"), newServer("global")
	t.Cleanup(regional.Close)
	t.Cleanup(global.Close)
	sequence := slices.Clone(resetSequence)
	sequence[len(sequence)-1].SkipForBaseURL = func(baseURL string) bool {
		if baseURL != regional.URL {
			t.Errorf("skip check URL = %s, want regional", baseURL)
		}
		return true
	}
	details, err := resetResources(t.Context(), regional.URL, global.URL, "test-token", sequence,
		HTTPRetryPolicy{RequestTimeout: time.Second, Backoff: BackoffConfig{Attempts: 1}}, HTTPTransportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"regional/v3/apis/resource-id",
		"regional/v3/portals/resource-id/custom-domain",
		"regional/v3/portals/resource-id",
		"regional/v3/portals/email-domains/test.mail.kongctl-e2e.io",
		"regional/v2/audit-log-destinations/resource-id",
		"global/v3/system-accounts/resource-id",
		"global/v3/teams/user-team",
		"regional/v2/application-auth-strategies/resource-id",
		"regional/v2/dcr-providers/resource-id",
		"regional/v2/control-planes/resource-id",
		"regional/v2/dashboards/resource-id",
		"regional/v1/catalog-services/resource-id",
		"regional/v1/event-gateways/resource-id",
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(deletes, want) {
		t.Errorf("deletes = %v, want %v", deletes, want)
	}
	if len(details) != len(sequence)-1 || details[1].DeleteCalls != 2 {
		t.Errorf("reset details = %+v", details)
	}
}

func TestResetResourcesDoesNotDeletePartialInventory(t *testing.T) {
	var mu sync.Mutex
	var deletes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodDelete {
			deletes = append(deletes, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		switch r.URL.Path {
		case "/v3/partial":
			if r.URL.Query().Get("page[number]") == "2" {
				http.Error(w, "second page failed", http.StatusInternalServerError)
				return
			}
			items := make([]map[string]any, resetListPageSize)
			for i := range items {
				items[i] = map[string]any{"id": strconv.Itoa(i)}
			}
			_ = json.NewEncoder(w).Encode(listResp{Data: items})
		case "/v3/forbidden":
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			_, _ = fmt.Fprint(w, `{"data":[{"id":"ok"}]}`)
		}
	}))
	t.Cleanup(server.Close)
	sequence := []resetResourceSpec{
		{Version: "v3", Endpoint: "partial"},
		{Version: "v3", Endpoint: "forbidden"},
		{Version: "v3", Endpoint: "healthy"},
	}
	details, err := resetResources(t.Context(), server.URL, server.URL, "test-token", sequence,
		HTTPRetryPolicy{RequestTimeout: time.Second, Backoff: BackoffConfig{Attempts: 1}}, HTTPTransportOptions{})
	var httpErr *httpError
	if !errors.As(err, &httpErr) || httpErr.status != http.StatusInternalServerError {
		t.Fatalf("first error = %v, want first category's page failure", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(deletes, []string{"/v3/healthy/ok"}) {
		t.Errorf("deleted from incomplete inventory: %v", deletes)
	}
	if details[0].ListCalls != 2 || details[0].Total != 0 || details[0].Error == "" ||
		details[1].Error == "" || details[2].Error != "" || details[2].Deleted != 1 {
		t.Errorf("details = %+v", details)
	}
}

func TestResetResourcesCancellationStopsActiveAndQueuedReads(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	started := make(chan struct{}, 7)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("delete after canceled inventory")
		}
		started <- struct{}{}
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	sequence := make([]resetResourceSpec, 7)
	for i := range sequence {
		sequence[i] = resetResourceSpec{Version: "v3", Endpoint: strconv.Itoa(i)}
	}
	done := make(chan struct{})
	var details []resetEndpoint
	var err error
	go func() {
		defer close(done)
		details, err = resetResources(ctx, server.URL, server.URL, "test-token", sequence,
			HTTPRetryPolicy{RequestTimeout: 10 * time.Second, Backoff: BackoffConfig{Attempts: 1}}, HTTPTransportOptions{})
	}()
	for range resetInventoryConcurrency {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("requests did not start")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled reset did not finish promptly")
	}
	if !errors.Is(err, context.Canceled) || len(details) != len(sequence) {
		t.Fatalf("err=%v, details=%+v", err, details)
	}
	calls := 0
	for _, detail := range details {
		calls += detail.ListCalls
		if detail.Error == "" || detail.DeleteCalls != 0 {
			t.Errorf("canceled category: %+v", detail)
		}
	}
	if calls != resetInventoryConcurrency || len(started) != 0 {
		t.Errorf("queued categories made requests: calls=%d, additional requests=%d", calls, len(started))
	}
}

func TestResetResourcesRefreshesInventoryAfterConflict(t *testing.T) {
	var mu sync.Mutex
	lists := 0
	var deletes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodGet {
			lists++
			_, _ = fmt.Fprintf(w, `{"data":[{"id":"revision-%d"}]}`, lists)
			return
		}
		deletes = append(deletes, r.URL.Path)
		if len(deletes) == 1 {
			http.Error(w, "conflict", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	details, err := resetResources(t.Context(), server.URL, server.URL, "test-token",
		[]resetResourceSpec{{Version: "v3", Endpoint: "apis"}},
		HTTPRetryPolicy{RequestTimeout: time.Second, Backoff: BackoffConfig{Attempts: 1}}, HTTPTransportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(deletes, []string{"/v3/apis/revision-1", "/v3/apis/revision-2"}) || lists != 2 {
		t.Errorf("conflict retry used stale inventory: lists=%d, deletes=%v", lists, deletes)
	}
	if details[0].ListCalls != 2 || details[0].DeleteCalls != 2 || details[0].Deleted != 1 {
		t.Errorf("details = %+v", details)
	}
}

func TestResetResourcesRetriesReadsAndToleratesDisappearingResources(t *testing.T) {
	var mu sync.Mutex
	lists := 0
	var deletes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodGet {
			lists++
			if lists == 1 {
				w.Header().Set("Retry-After", "0")
				http.Error(w, "rate limited", http.StatusTooManyRequests)
				return
			}
			_, _ = fmt.Fprint(w, `{"data":[{"id":"gone"},{"id":"remaining"}]}`)
			return
		}
		deletes = append(deletes, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/gone") {
			// Earlier dependency cleanup may have cascade-deleted an inventoried item.
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	details, err := resetResources(t.Context(), server.URL, server.URL, "test-token",
		[]resetResourceSpec{{Version: "v3", Endpoint: "apis"}},
		HTTPRetryPolicy{RequestTimeout: time.Second, Backoff: BackoffConfig{Attempts: 2, Base: time.Millisecond}},
		HTTPTransportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if lists != 2 || !slices.Equal(deletes, []string{"/v3/apis/gone", "/v3/apis/remaining"}) {
		t.Errorf("lists=%d, deletes=%v", lists, deletes)
	}
	if details[0].ListCalls != 2 || details[0].DeleteCalls != 2 || details[0].Deleted != 1 {
		t.Errorf("details = %+v", details)
	}
}

func TestResetResourcesCancellationStopsDeletes(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var mu sync.Mutex
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprint(w, `{"data":[{"id":"first"},{"id":"second"}]}`)
			return
		}
		mu.Lock()
		deletes++
		mu.Unlock()
		cancel()
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	details, err := resetResources(ctx, server.URL, server.URL, "test-token",
		[]resetResourceSpec{{Version: "v3", Endpoint: "apis"}, {Version: "v3", Endpoint: "portals"}},
		HTTPRetryPolicy{RequestTimeout: time.Second, Backoff: BackoffConfig{Attempts: 1}}, HTTPTransportOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want canceled", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if deletes != 1 || details[0].DeleteCalls != 1 || details[1].DeleteCalls != 0 || details[1].Error == "" {
		t.Errorf("deletes=%d, details=%+v", deletes, details)
	}
}

// BenchmarkResetResourcesInventoryLatency isolates inventory scheduling with
// empty categories and a synthetic 10ms API latency. It is not a Konnect forecast.
func BenchmarkResetResourcesInventoryLatency(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sleepWithContext(r.Context(), 10*time.Millisecond); err != nil {
			return
		}
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	}))
	b.Cleanup(server.Close)
	policy := HTTPRetryPolicy{RequestTimeout: time.Second, Backoff: BackoffConfig{Attempts: 1}}
	for _, parallel := range []bool{false, true} {
		name := "sequential"
		if parallel {
			name = "concurrent"
		}
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				groups := [][]resetResourceSpec{resetSequence}
				if !parallel {
					groups = nil
					for _, step := range resetSequence {
						groups = append(groups, []resetResourceSpec{step})
					}
				}
				for _, group := range groups {
					if _, err := resetResources(b.Context(), server.URL, server.URL, "test-token", group,
						policy, HTTPTransportOptions{}); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

func TestResetEventsRecordRecoveryWithoutSensitiveData(t *testing.T) {
	for _, recovered := range []bool{true, false} {
		t.Run(strconv.FormatBool(recovered), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				if calls == 1 || !recovered {
					http.Error(w, "503 Service Unavailable PRIVATE_RESPONSE_TOKEN", http.StatusServiceUnavailable)
					return
				}
				_, _ = fmt.Fprint(w, `{"data":[]}`)
			}))
			defer server.Close()
			session := newResetHTTPSession(time.Second, HTTPTransportOptions{})
			defer session.Close()
			_, err := retryListItems(t.Context(), session, server.URL+"?token=PRIVATE_QUERY", "PRIVATE_PAT", "apis",
				HTTPRetryPolicy{Backoff: BackoffConfig{Attempts: 2, Base: time.Millisecond, Max: time.Millisecond}})
			if (err == nil) != recovered {
				t.Fatalf("recovered=%t, err=%v", recovered, err)
			}
			events := session.Metrics().Events
			expected := "operation_failed"
			expectedEvents := 2
			if recovered {
				expected = "recovered"
				expectedEvents = 1
			}
			if len(events) != expectedEvents {
				t.Fatalf("events=%+v", events)
			}
			for i, event := range events {
				if event.Outcome != expected || event.Attempt != i+1 || event.Operation != "list" || event.Timestamp == "" {
					t.Fatalf("event=%+v", event)
				}
			}
			encoded, err := json.Marshal(events)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "PRIVATE") {
				t.Fatal("event leaked request or response data")
			}
		})
	}
}

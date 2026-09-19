//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	resetListPageSize = 100
	resetListMaxPages = 10000
	// Each worker owns a resource category, including its pagination and retries.
	resetInventoryConcurrency = 3
)

// truthy returns true if v is a typical truthy string.
func truthyEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on", "y":
		return true
	default:
		return false
	}
}

// ResetOrgIfRequested deletes top-level resources (application-auth-strategies, apis, portals)
// in the target Konnect org when KONGCTL_E2E_RESET is truthy. This is destructive.
func ResetOrgIfRequested() error { return resetOrg("unspecified", true) }

// ResetOrg performs the destructive reset without recording harness artifacts. Intended for
// developer utilities that only need to wipe the org state.
func ResetOrg(stage string) error { return resetOrg(stage, false) }

// ResetOrgWithCapture performs the same destructive reset as ResetOrgIfRequested and
// records a synthetic command under <run>/global/commands documenting execution.
// The stage parameter is recorded in artifacts (e.g., "before_suite", "before_test").
func ResetOrgWithCapture(stage string) error { return resetOrg(stage, true) }

func resetOrg(stage string, capture bool) error {
	// Default: reset is ON unless explicitly disabled.
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("KONGCTL_E2E_RESET"))); v != "" {
		if !truthyEnv(v) { // values like 0,false,off,no
			Infof("Reset disabled by KONGCTL_E2E_RESET=%s", v)
			if capture {
				captureResetEvent(stage, false, "skipped", "reset disabled", "", resetResult{})
			}
			return nil
		}
	}
	baseURL, err := KonnectBaseURL()
	if err != nil {
		return err
	}
	token := os.Getenv("KONGCTL_E2E_KONNECT_PAT")
	if token == "" {
		Warnf("reset requested, but KONGCTL_E2E_KONNECT_PAT is not set; skipping reset")
		if capture {
			captureResetEvent(stage, false, "skipped", "missing PAT", baseURL, resetResult{})
		}
		return nil
	}

	Infof("Resetting Konnect org at %s", baseURL)
	policy := resetHTTPPolicyFromEnv()
	result, err := executeReset(baseURL, token, policy)
	if capture {
		status := "ok"
		reason := ""
		if err != nil {
			status = "error"
			reason = err.Error()
		}
		captureResetEvent(stage, true, status, reason, baseURL, result)
	}
	if err != nil {
		return err
	}

	Infof("Reset complete")
	return nil
}

type listResp struct {
	Data []map[string]any `json:"data"`
}

// filterFunc determines whether a resource should be included for deletion.
// Return true to DELETE the resource, false to SKIP it.
type filterFunc func(resource map[string]any) bool

// shouldDeleteResource is the default filter that excludes konnect-managed resources.
func shouldDeleteResource(resource map[string]any) bool {
	if managed, ok := resource["konnect_managed"].(bool); ok && managed {
		return false
	}
	return true
}

func skipSystemTeams(resource map[string]any) bool {
	if isSystemTeam, ok := resource["system_team"].(bool); ok && isSystemTeam {
		return false
	}
	return true
}

type resetResource struct {
	spec      resetResourceSpec
	listURL   string
	deleteURL string
	session   *resetHTTPSession
	items     []map[string]any
	listErr   error
}

func (r *resetResource) list(ctx context.Context, token string, policy HTTPRetryPolicy) {
	startedAt := time.Now()
	Infof("Fetching %s for deletion...", r.spec.Endpoint)
	r.items, r.listErr = retryListAllItems(ctx, r.session, r.listURL, token, r.spec.Endpoint, policy)
	r.session.metrics.Duration = time.Since(startedAt)
}

func (r *resetResource) deleteAll(
	ctx context.Context,
	token string,
	policy HTTPRetryPolicy,
) (total int, deleted int, err error) {
	startedAt := time.Now()
	defer func() {
		// Exclude time waiting for other categories to list or delete.
		r.session.metrics.Duration += time.Since(startedAt)
	}()
	if r.listErr != nil {
		return 0, 0, r.listErr
	}
	session := r.session
	endpoint := r.spec.Endpoint
	deleteEndpoint := r.spec.DeleteEndpoint
	if deleteEndpoint == "" {
		deleteEndpoint = endpoint
	}
	filter := r.spec.Filter

	if filter == nil {
		filter = shouldDeleteResource
	}
	idField := strings.TrimSpace(r.spec.IDField)
	if idField == "" {
		idField = "id"
	}

	const maxAttempts = 5
	const retryDelay = 2 * time.Second

	attempt := 0
	items := r.items

	for {
		if err := ctx.Err(); err != nil {
			return total, deleted, err
		}
		// A conflict can reflect changing dependencies or visibility. Never reuse
		// the initial inventory for a conflict retry.
		if attempt > 0 {
			items, err = retryListAllItems(ctx, session, r.listURL, token, endpoint, policy)
			if err != nil {
				return total, deleted, err
			}
		}

		if len(items) == 0 {
			if total == 0 {
				Infof("No %s found", endpoint)
				return 0, 0, nil
			}
			return total, deleted, nil
		}

		// Filter items based on the filter function
		var idsToDelete []string
		var skipped int
		for _, item := range items {
			id, ok := item[idField].(string)
			if !ok || id == "" {
				continue
			}
			if filter(item) {
				idsToDelete = append(idsToDelete, id)
			} else {
				skipped++
			}
		}

		if attempt == 0 {
			total = len(items)
			if skipped > 0 {
				Infof("Skipping %d filtered %s resources", skipped, endpoint)
			}
		}

		if len(idsToDelete) == 0 {
			Infof("No %s matched deletion filter; nothing to delete", endpoint)
			return total, deleted, nil
		}

		Infof("Attempt %d deleting %d %s", attempt+1, len(idsToDelete), endpoint)

		conflicts := 0
		for _, id := range idsToDelete {
			if err := ctx.Err(); err != nil {
				return total, deleted, err
			}
			if r.spec.PreDeleteFn != nil {
				r.spec.PreDeleteFn(ctx, session, r.deleteURL, token, id)
			}
			if err := retryDeleteOne(ctx, session, r.deleteURL, token, deleteEndpoint, id, policy); err != nil {
				Warnf("delete %s %s failed: %v", deleteEndpoint, id, err)
				var he *httpError
				if errors.As(err, &he) && he.status == http.StatusConflict {
					conflicts++
				}
			} else {
				Debugf("deleted %s %s", deleteEndpoint, id)
				deleted++
			}
		}

		if err := ctx.Err(); err != nil {
			return total, deleted, err
		}
		if conflicts == 0 {
			return total, deleted, nil
		}

		attempt++
		if attempt >= maxAttempts {
			return total, deleted, fmt.Errorf(
				"failed to delete all %s after %d attempts (conflicts remain)",
				endpoint,
				maxAttempts,
			)
		}

		Infof("Retrying deletion of %s (%d conflicts remaining)", endpoint, conflicts)
		if err := sleepWithContext(ctx, retryDelay); err != nil {
			return total, deleted, err
		}
	}
}

func retryListAllItems(
	ctx context.Context,
	session *resetHTTPSession,
	rawURL string,
	token string,
	endpoint string,
	policy HTTPRetryPolicy,
) ([]map[string]any, error) {
	var allItems []map[string]any
	for pageNumber := 1; pageNumber <= resetListMaxPages; pageNumber++ {
		items, err := retryListItems(
			ctx,
			session,
			pagedURL(rawURL, resetListPageSize, pageNumber),
			token,
			endpoint,
			policy,
		)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, items...)
		if len(items) < resetListPageSize {
			return allItems, nil
		}
	}
	return nil, fmt.Errorf("%s pagination exceeded safety limit", endpoint)
}

type httpError struct {
	status int
	body   string
	header http.Header
}

func (e *httpError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("unexpected status %d: %s", e.status, e.body)
}

// resetHTTPEvent records failed attempts without URLs, resource IDs, or error text.
type resetHTTPEvent struct {
	Timestamp  string `json:"timestamp"`
	Operation  string `json:"operation"`
	Class      string `json:"class"`
	DurationMS int64  `json:"duration_ms"`
	Attempt    int    `json:"attempt"`
	Outcome    string `json:"outcome"`
}

type resetEndpoint struct {
	Events           []resetHTTPEvent `json:"events,omitempty"`
	APIVersion       string           `json:"api_version"`
	Endpoint         string           `json:"endpoint"`
	Total            int              `json:"total"`
	Deleted          int              `json:"deleted"`
	DurationMS       int64            `json:"duration_ms"`
	ListCalls        int              `json:"list_calls"`
	ListDurationMS   int64            `json:"list_duration_ms"`
	DeleteCalls      int              `json:"delete_calls"`
	DeleteDurationMS int64            `json:"delete_duration_ms"`
	Error            string           `json:"error,omitempty"`
}

type resetResult struct {
	DurationMS int64           `json:"duration_ms"`
	Details    []resetEndpoint `json:"details"`
}

type resetHTTPMetrics struct {
	Events         []resetHTTPEvent
	Duration       time.Duration
	ListCalls      int
	ListDuration   time.Duration
	DeleteCalls    int
	DeleteDuration time.Duration
}

func resetEndpointResult(
	apiVersion, endpoint string,
	total, deleted int,
	metrics resetHTTPMetrics,
	err error,
) resetEndpoint {
	return resetEndpoint{
		APIVersion:       apiVersion,
		Events:           metrics.Events,
		Endpoint:         endpoint,
		Total:            total,
		Deleted:          deleted,
		DurationMS:       metrics.Duration.Milliseconds(),
		ListCalls:        metrics.ListCalls,
		ListDurationMS:   metrics.ListDuration.Milliseconds(),
		DeleteCalls:      metrics.DeleteCalls,
		DeleteDurationMS: metrics.DeleteDuration.Milliseconds(),
		Error:            errorString(err),
	}
}

type resetHTTPSession struct {
	newClient func() *http.Client
	client    *http.Client
	metrics   resetHTTPMetrics
}

func newResetHTTPSession(timeout time.Duration, options HTTPTransportOptions) *resetHTTPSession {
	return &resetHTTPSession{
		newClient: func() *http.Client {
			return newHTTPClientWithOptions(timeout, options)
		},
	}
}

func (s *resetHTTPSession) Client() *http.Client {
	if s == nil {
		return nil
	}
	if s.client == nil {
		s.client = s.newClient()
	}
	return s.client
}

func (s *resetHTTPSession) Metrics() resetHTTPMetrics {
	if s == nil {
		return resetHTTPMetrics{}
	}
	return s.metrics
}

func (s *resetHTTPSession) RecordList(duration time.Duration) {
	if s == nil {
		return
	}
	s.metrics.ListCalls++
	s.metrics.ListDuration += duration
}

func (s *resetHTTPSession) RecordDelete(duration time.Duration) {
	if s == nil {
		return
	}
	s.metrics.DeleteCalls++
	s.metrics.DeleteDuration += duration
}

func (s *resetHTTPSession) Rebuild(err error) {
	if s == nil || s.client == nil {
		return
	}
	s.client.CloseIdleConnections()
	if err != nil {
		Debugf("reset: rebuilt HTTP client after error: %v", err)
	}
	s.client = nil
}

func (s *resetHTTPSession) Close() {
	if s == nil || s.client == nil {
		return
	}
	s.client.CloseIdleConnections()
	s.client = nil
}

type resetResourceSpec struct {
	Version  string
	Endpoint string
	// Use the global Konnect URL instead of the regional URL.
	UseGlobal bool
	// Optional delete override for resources whose list and delete APIs differ.
	DeleteVersion     string
	DeleteEndpoint    string
	DeleteUseRegional bool
	// Optional filter to exclude resources from deletion
	// if nothing is passed default filter that skips konnect-managed resources is used
	Filter filterFunc
	// Optional resource identifier field. Defaults to "id".
	IDField string
	// PreDeleteFn is called for each resource ID before deletion. It is used to clean
	// up sub-resources that Konnect does not cascade-delete automatically. Errors are
	// logged but do not stop the deletion.
	PreDeleteFn func(ctx context.Context, session *resetHTTPSession, endpointURL, token, id string)
	// SkipForBaseURL skips reset for resources that are not available in every
	// regional Konnect environment.
	SkipForBaseURL func(baseURL string) bool
}

var resetSequence = []resetResourceSpec{
	{Version: "v3", Endpoint: "apis"},
	{Version: "v3", Endpoint: "portals", PreDeleteFn: tryDeletePortalCustomDomain},
	{Version: "v3", Endpoint: "portals/email-domains", Filter: onlyE2EEmailDomains, IDField: "domain"},
	{
		Version:           "v3",
		Endpoint:          "audit-log-destinations",
		UseGlobal:         true,
		DeleteVersion:     "v2",
		DeleteEndpoint:    "audit-log-destinations",
		DeleteUseRegional: true,
	},
	{Version: "v3", Endpoint: "system-accounts", UseGlobal: true},
	{Version: "v3", Endpoint: "teams", UseGlobal: true, Filter: skipSystemTeams},
	{Version: "v2", Endpoint: "application-auth-strategies"},
	{Version: "v2", Endpoint: "dcr-providers"},
	{Version: "v2", Endpoint: "control-planes"},
	{Version: "v2", Endpoint: "dashboards"},
	{Version: "v1", Endpoint: "catalog-services"},
	{Version: "v1", Endpoint: "event-gateways"},
	{Version: "v1", Endpoint: "ai-gateways", SkipForBaseURL: skipAIGatewaysResetForUnsupportedRegion},
}

func skipAIGatewaysResetForUnsupportedRegion(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToLower(u.Hostname()), "eu.api.")
}

func onlyE2EEmailDomains(resource map[string]any) bool {
	domain, ok := resource["domain"].(string)
	if !ok {
		return false
	}
	return strings.HasSuffix(domain, ".mail.kongctl-e2e.io")
}

// tryDeletePortalCustomDomain attempts to delete the custom domain for a portal before
// the portal itself is deleted. Konnect does not release the custom domain hostname
// reservation when a portal is deleted, so this prevents 409 conflicts on subsequent
// test runs that try to register the same hostname.
func tryDeletePortalCustomDomain(
	ctx context.Context,
	session *resetHTTPSession,
	portalsURL, token, portalID string,
) {
	url := strings.TrimRight(portalsURL, "/") + "/" + portalID + "/custom-domain"
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		Warnf("pre-delete portal custom domain %s: build request: %v", portalID, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	startedAt := time.Now()
	defer func() {
		session.RecordDelete(time.Since(startedAt))
	}()
	resp, err := session.Client().Do(req)
	if err != nil {
		Warnf("pre-delete portal custom domain %s: %v", portalID, err)
		return
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent, http.StatusNotFound:
		// expected: deleted or no custom domain set
	default:
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		Warnf("pre-delete portal custom domain %s: unexpected status %d: %s", portalID, resp.StatusCode, b)
	}
}

func executeReset(baseURL, token string, policy HTTPRetryPolicy) (result resetResult, firstErr error) {
	startedAt := time.Now()
	defer func() {
		result.DurationMS = time.Since(startedAt).Milliseconds()
	}()
	transportOptions := HTTPTransportOptionsFromEnv()
	globalBaseURL, envErr := KonnectBaseAuthURL()
	if envErr != nil {
		return result, envErr
	}
	ctx := context.Background()
	cancel := func() {}
	if policy.TotalTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, policy.TotalTimeout)
	}
	defer cancel()

	tot, del, metrics, err := resetConfiguredE2EUserAssignments(
		ctx,
		globalBaseURL,
		token,
		policy,
		transportOptions,
	)
	if err != nil && firstErr == nil {
		firstErr = err
	}
	result.Details = append(
		result.Details,
		resetEndpointResult("v3", "e2e-user-assignments", tot, del, metrics, err),
	)

	details, err := resetResources(ctx, baseURL, globalBaseURL, token, resetSequence, policy, transportOptions)
	result.Details = append(result.Details, details...)
	if firstErr == nil {
		firstErr = err
	}
	return result, firstErr
}

// resetResources inventories independent resource categories concurrently, then
// deletes in dependency order. Sessions are owned by one category and never used
// concurrently. The inventory barrier prevents reads from racing with deletes.
func resetResources(
	ctx context.Context,
	baseURL, globalBaseURL, token string,
	sequence []resetResourceSpec,
	policy HTTPRetryPolicy,
	transportOptions HTTPTransportOptions,
) ([]resetEndpoint, error) {
	resources := make([]*resetResource, 0, len(sequence))
	for _, step := range sequence {
		targetURL := baseURL
		if step.UseGlobal {
			targetURL = globalBaseURL
		}
		if step.SkipForBaseURL != nil && step.SkipForBaseURL(targetURL) {
			Debugf("Skipping %s reset for %s", step.Endpoint, targetURL)
			continue
		}
		deleteTargetURL := targetURL
		if step.DeleteUseRegional {
			deleteTargetURL = baseURL
		}
		deleteVersion, deleteEndpoint := step.DeleteVersion, step.DeleteEndpoint
		if deleteVersion == "" {
			deleteVersion = step.Version
		}
		if deleteEndpoint == "" {
			deleteEndpoint = step.Endpoint
		}
		session := newResetHTTPSession(policy.RequestTimeout, transportOptions)
		defer session.Close()
		resources = append(resources, &resetResource{
			spec:      step,
			listURL:   fmt.Sprintf("%s/%s/%s", strings.TrimRight(targetURL, "/"), step.Version, step.Endpoint),
			deleteURL: fmt.Sprintf("%s/%s/%s", strings.TrimRight(deleteTargetURL, "/"), deleteVersion, deleteEndpoint),
			session:   session,
		})
	}

	jobs := make(chan *resetResource, len(resources))
	for _, resource := range resources {
		jobs <- resource
	}
	close(jobs)
	var workers sync.WaitGroup
	for range min(resetInventoryConcurrency, len(resources)) {
		workers.Go(func() {
			for resource := range jobs {
				resource.list(ctx, token, policy)
			}
		})
	}
	workers.Wait()

	details := make([]resetEndpoint, 0, len(resources))
	var firstErr error
	for _, resource := range resources {
		total, deleted, err := resource.deleteAll(ctx, token, policy)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		details = append(details, resetEndpointResult(
			resource.spec.Version, resource.spec.Endpoint, total, deleted, resource.session.Metrics(), err,
		))
	}
	return details, firstErr
}

type resetUser struct {
	ID string
}

func resetConfiguredE2EUserAssignments(
	ctx context.Context,
	globalBaseURL string,
	token string,
	policy HTTPRetryPolicy,
	transportOptions HTTPTransportOptions,
) (total int, deleted int, metrics resetHTTPMetrics, err error) {
	startedAt := time.Now()
	emails := configuredE2EUserEmails()
	if len(emails) == 0 {
		Debugf("No KONGCTL_E2E_ORG_USER_EMAIL_* values configured; skipping user assignment reset")
		return 0, 0, metrics, nil
	}

	session := newResetHTTPSession(policy.RequestTimeout, transportOptions)
	defer func() {
		session.Close()
		metrics = session.Metrics()
		metrics.Duration = time.Since(startedAt)
	}()

	usersByEmail, err := listUsersByEmail(ctx, session, globalBaseURL, token, policy)
	if err != nil {
		return len(emails), 0, metrics, err
	}

	for _, email := range emails {
		user, ok := usersByEmail[email]
		if !ok {
			continue
		}
		userTotal, userDeleted, err := resetUserAssignments(ctx, session, globalBaseURL, token, policy, user)
		total += userTotal
		deleted += userDeleted
		if err != nil {
			return total, deleted, metrics, err
		}
	}
	return total, deleted, metrics, nil
}

func configuredE2EUserEmails() []string {
	seen := map[string]struct{}{}
	for _, kv := range os.Environ() {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(key, "KONGCTL_E2E_ORG_USER_EMAIL_") {
			continue
		}
		email := strings.TrimSpace(value)
		if email == "" {
			continue
		}
		seen[email] = struct{}{}
	}
	emails := make([]string, 0, len(seen))
	for email := range seen {
		emails = append(emails, email)
	}
	slices.Sort(emails)
	return emails
}

func listUsersByEmail(
	ctx context.Context,
	session *resetHTTPSession,
	globalBaseURL string,
	token string,
	policy HTTPRetryPolicy,
) (map[string]resetUser, error) {
	const pageSize = 100
	usersByEmail := map[string]resetUser{}
	for pageNumber := 1; pageNumber <= 10000; pageNumber++ {
		items, err := retryListItems(
			ctx,
			session,
			pagedURL(strings.TrimRight(globalBaseURL, "/")+"/v3/users", pageSize, pageNumber),
			token,
			"users",
			policy,
		)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			id, _ := item["id"].(string)
			email, _ := item["email"].(string)
			if id == "" || email == "" {
				continue
			}
			usersByEmail[email] = resetUser{ID: id}
		}
		if len(items) < pageSize {
			return usersByEmail, nil
		}
	}
	return nil, fmt.Errorf("organization users pagination exceeded safety limit")
}

func resetUserAssignments(
	ctx context.Context,
	session *resetHTTPSession,
	globalBaseURL string,
	token string,
	policy HTTPRetryPolicy,
	user resetUser,
) (int, int, error) {
	total := 0
	deleted := 0

	teamItems, err := retryListItems(
		ctx,
		session,
		pagedURL(
			fmt.Sprintf("%s/v3/users/%s/teams", strings.TrimRight(globalBaseURL, "/"), url.PathEscape(user.ID)),
			100,
			1,
		),
		token,
		"user teams",
		policy,
	)
	if err != nil {
		return total, deleted, err
	}
	total += len(teamItems)
	for _, team := range teamItems {
		teamID, _ := team["id"].(string)
		if teamID == "" {
			continue
		}
		deleteURL := fmt.Sprintf("%s/v3/teams/%s/users", strings.TrimRight(globalBaseURL, "/"), url.PathEscape(teamID))
		if err := retryDeleteOne(ctx, session, deleteURL, token, "user team assignment", user.ID, policy); err != nil {
			if he, ok := err.(*httpError); ok && he.status == http.StatusNotFound {
				continue
			}
			return total, deleted, err
		}
		deleted++
	}

	roleItems, err := retryListItems(
		ctx,
		session,
		fmt.Sprintf("%s/v3/users/%s/assigned-roles", strings.TrimRight(globalBaseURL, "/"), url.PathEscape(user.ID)),
		token,
		"user roles",
		policy,
	)
	if err != nil {
		return total, deleted, err
	}
	total += len(roleItems)
	for _, role := range roleItems {
		roleID, _ := role["id"].(string)
		if roleID == "" {
			continue
		}
		deleteURL := fmt.Sprintf(
			"%s/v3/users/%s/assigned-roles",
			strings.TrimRight(globalBaseURL, "/"),
			url.PathEscape(user.ID),
		)
		if err := retryDeleteOne(ctx, session, deleteURL, token, "user role assignment", roleID, policy); err != nil {
			if he, ok := err.(*httpError); ok && he.status == http.StatusNotFound {
				continue
			}
			return total, deleted, err
		}
		deleted++
	}

	if deleted > 0 {
		Infof("Reset %d assignments for configured E2E user", deleted)
	}
	return total, deleted, nil
}

func pagedURL(rawURL string, pageSize, pageNumber int) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	q.Set("page[size]", fmt.Sprintf("%d", pageSize))
	q.Set("page[number]", fmt.Sprintf("%d", pageNumber))
	u.RawQuery = q.Encode()
	return u.String()
}

func errorString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func captureResetEvent(
	stage string,
	executed bool,
	status string,
	reason string,
	baseURL string,
	result resetResult,
) {
	// Best-effort capture; never fail tests from here.
	rd, err := ensureRunDir()
	if err != nil || rd == "" {
		return
	}
	globalDir := rd + string(os.PathSeparator) + "global" + string(os.PathSeparator) + "commands"
	_ = os.MkdirAll(globalDir, 0o755)
	// Compute next seq by counting existing dirs
	entries, _ := os.ReadDir(globalDir)
	seq := 0
	for range entries {
		seq++
	}
	dir := fmt.Sprintf("%s%c%03d-reset_org", globalDir, os.PathSeparator, seq)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	// command.txt
	_ = os.WriteFile(
		dir+string(os.PathSeparator)+"command.txt",
		fmt.Appendf(nil, "RESET ORG (stage=%s)\n", stage),
		0o644,
	)
	// meta.json
	meta := map[string]any{
		"stage":    stage,
		"executed": executed,
		"status":   status,
		"base_url": baseURL,
		"time":     time.Now(),
	}
	if b, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(dir+string(os.PathSeparator)+"meta.json", b, 0o644)
	}
	// env.json (sanitized)
	envMap := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			k := kv[:i]
			v := kv[i+1:]
			ku := strings.ToUpper(k)
			if strings.Contains(ku, "TOKEN") || strings.Contains(ku, "PAT") || strings.Contains(ku, "PASSWORD") ||
				strings.Contains(ku, "SECRET") || strings.Contains(ku, "EMAIL") {
				if v != "" {
					v = "***"
				}
			}
			envMap[k] = v
		}
	}
	if b, err := json.MarshalIndent(envMap, "", "  "); err == nil {
		_ = os.WriteFile(dir+string(os.PathSeparator)+"env.json", b, 0o644)
	}
	// observation.json
	obs := map[string]any{
		"type":        "reset_summary",
		"executed":    executed,
		"status":      status,
		"reason":      reason,
		"duration_ms": result.DurationMS,
		"details":     result.Details,
	}
	if b, err := json.MarshalIndent(obs, "", "  "); err == nil {
		_ = os.WriteFile(dir+string(os.PathSeparator)+"observation.json", b, 0o644)
	}
}

func listItems(client *http.Client, url, token string) ([]map[string]any, error) {
	return listItemsWithContext(context.Background(), client, url, token)
}

func listItemsWithContext(ctx context.Context, client *http.Client, url, token string) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, &httpError{
			status: resp.StatusCode,
			body:   strings.TrimSpace(string(b)),
			header: resp.Header.Clone(),
		}
	}
	var lr listResp
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, err
	}
	return lr.Data, nil
}

func deleteOne(client *http.Client, baseURL, token, id string) error {
	return deleteOneWithContext(context.Background(), client, baseURL, token, id)
}

func deleteOneWithContext(ctx context.Context, client *http.Client, baseURL, token, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, baseURL+"/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return &httpError{
			status: resp.StatusCode,
			body:   strings.TrimSpace(string(b)),
			header: resp.Header.Clone(),
		}
	}
	return nil
}

func retryListItems(
	ctx context.Context,
	session *resetHTTPSession,
	url string,
	token string,
	endpoint string,
	policy HTTPRetryPolicy,
) (items []map[string]any, err error) {
	firstEvent := len(session.metrics.Events)
	defer func() { session.finishEvents(firstEvent, err) }()
	cfg := NormalizeBackoffConfig(policy.Backoff)
	attempts := cfg.Attempts
	backoff := BuildBackoffSchedule(cfg)
	for atry := range attempts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		start := time.Now()
		client := session.Client()
		items, err = listItemsWithContext(ctx, client, url, token)
		duration := time.Since(start)
		session.RecordList(duration)
		session.recordEvent("list", atry+1, duration, err)
		if err == nil {
			return items, nil
		}
		detail := err.Error()
		session.Rebuild(err)
		if !ShouldRetryResetHTTPAttempt(err, detail) || atry+1 >= attempts {
			return nil, err
		}
		delay := RetryDelayForError(err, backoff, atry)
		Warnf(
			"reset: list %s attempt %d/%d failed (%s, duration=%s): %v; retrying in %s",
			endpoint,
			atry+1,
			attempts,
			ClassifyRetry(err, detail),
			duration.Round(time.Millisecond),
			err,
			delay,
		)
		if err := sleepWithContext(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, err
}

func retryDeleteOne(
	ctx context.Context,
	session *resetHTTPSession,
	baseURL string,
	token string,
	endpoint string,
	id string,
	policy HTTPRetryPolicy,
) (err error) {
	firstEvent := len(session.metrics.Events)
	defer func() { session.finishEvents(firstEvent, err) }()
	cfg := NormalizeBackoffConfig(policy.Backoff)
	attempts := cfg.Attempts
	backoff := BuildBackoffSchedule(cfg)
	for atry := range attempts {
		if err := ctx.Err(); err != nil {
			return err
		}
		start := time.Now()
		client := session.Client()
		err = deleteOneWithContext(ctx, client, baseURL, token, id)
		duration := time.Since(start)
		session.RecordDelete(duration)
		session.recordEvent("delete", atry+1, duration, err)
		if err == nil {
			return nil
		}
		if he, ok := err.(*httpError); ok && he.status == http.StatusConflict {
			return err
		}
		detail := err.Error()
		session.Rebuild(err)
		if !ShouldRetryResetHTTPAttempt(err, detail) || atry+1 >= attempts {
			return err
		}
		delay := RetryDelayForError(err, backoff, atry)
		Warnf(
			"reset: delete %s/%s attempt %d/%d failed (%s, duration=%s): %v; retrying in %s",
			endpoint,
			id,
			atry+1,
			attempts,
			ClassifyRetry(err, detail),
			duration.Round(time.Millisecond),
			err,
			delay,
		)
		if err := sleepWithContext(ctx, delay); err != nil {
			return err
		}
	}
	return err
}

func (s *resetHTTPSession) recordEvent(operation string, attempt int, duration time.Duration, err error) {
	if err == nil {
		return
	}
	class := string(ClassifyRetry(err, err.Error()))
	if strings.Contains(strings.ToLower(err.Error()), "tls handshake timeout") {
		class = "tls_timeout"
	}
	if strings.Contains(strings.ToLower(err.Error()), "connection reset") {
		class = "connection_reset"
	}
	if class == "" || class == string(RetryClassNone) {
		return
	}
	s.metrics.Events = append(s.metrics.Events, resetHTTPEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Operation: operation, Class: class,
		DurationMS: duration.Milliseconds(), Attempt: attempt, Outcome: "unknown",
	})
}

func (s *resetHTTPSession) finishEvents(first int, err error) {
	for i := first; i < len(s.metrics.Events); i++ {
		s.metrics.Events[i].Outcome = "operation_failed"
		if err == nil {
			s.metrics.Events[i].Outcome = "recovered"
		}
	}
}

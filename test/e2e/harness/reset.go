//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
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
				captureResetEvent(stage, false, "skipped", "reset disabled", "", nil)
			}
			return nil
		}
	}
	baseURL := os.Getenv("KONGCTL_E2E_KONNECT_BASE_URL")
	if baseURL == "" {
		baseURL = "https://us.api.konghq.com"
	}
	token := os.Getenv("KONGCTL_E2E_KONNECT_PAT")
	if token == "" {
		Warnf("reset requested, but KONGCTL_E2E_KONNECT_PAT is not set; skipping reset")
		if capture {
			captureResetEvent(stage, false, "skipped", "missing PAT", baseURL, nil)
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
		captureResetEvent(stage, true, status, reason, baseURL, result.Details)
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

func deleteAll(
	ctx context.Context,
	baseURL string,
	deleteBaseURL string,
	token string,
	apiVersion string,
	endpoint string,
	deleteAPIVersion string,
	deleteEndpoint string,
	idField string,
	filter filterFunc,
	// preDeleteFn is called for each resource ID before deletion. It is used to clean up
	// sub-resources. Any errors must be logged inside the function; the function must not
	// return an error (it must not block deletion of the parent resource).
	preDeleteFn func(ctx context.Context, session *resetHTTPSession, endpointURL, token, id string),
	policy HTTPRetryPolicy,
	transportOptions HTTPTransportOptions,
) (int, int, error) {
	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(baseURL, "/"), apiVersion, endpoint)
	if deleteBaseURL == "" {
		deleteBaseURL = baseURL
	}
	if deleteAPIVersion == "" {
		deleteAPIVersion = apiVersion
	}
	if deleteEndpoint == "" {
		deleteEndpoint = endpoint
	}
	deleteURL := fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimRight(deleteBaseURL, "/"),
		deleteAPIVersion,
		deleteEndpoint,
	)
	Infof("Fetching %s for deletion...", endpoint)
	session := newResetHTTPSession(policy.RequestTimeout, transportOptions)
	defer session.Close()

	if filter == nil {
		filter = shouldDeleteResource
	}
	idField = strings.TrimSpace(idField)
	if idField == "" {
		idField = "id"
	}

	const maxAttempts = 5
	const retryDelay = 2 * time.Second

	total := 0
	deleted := 0
	attempt := 0

	for {
		if err := ctx.Err(); err != nil {
			return total, deleted, err
		}
		items, err := retryListItems(ctx, session, url, token, endpoint, policy)
		if err != nil {
			return total, deleted, err
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
		} else if len(items)+deleted > total {
			total = len(items) + deleted
		}

		if len(idsToDelete) == 0 {
			Infof("No %s matched deletion filter; nothing to delete", endpoint)
			return total, deleted, nil
		}

		Infof("Attempt %d deleting %d %s", attempt+1, len(idsToDelete), endpoint)

		conflicts := 0
		for _, id := range idsToDelete {
			if preDeleteFn != nil {
				preDeleteFn(ctx, session, deleteURL, token, id)
			}
			if err := retryDeleteOne(ctx, session, deleteURL, token, deleteEndpoint, id, policy); err != nil {
				Warnf("delete %s %s failed: %v", deleteEndpoint, id, err)
				if he, ok := err.(*httpError); ok && he.status == http.StatusConflict {
					conflicts++
				}
			} else {
				Debugf("deleted %s %s", deleteEndpoint, id)
				deleted++
			}
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

type resetEndpoint struct {
	APIVersion string `json:"api_version"`
	Endpoint   string `json:"endpoint"`
	Total      int    `json:"total"`
	Deleted    int    `json:"deleted"`
	Error      string `json:"error,omitempty"`
}

type resetResult struct {
	Details []resetEndpoint
}

type resetHTTPSession struct {
	newClient func() *http.Client
	client    *http.Client
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
	// Use global.api.konghq.com instead of regional URL
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

func executeReset(baseURL, token string, policy HTTPRetryPolicy) (resetResult, error) {
	transportOptions := HTTPTransportOptionsFromEnv()
	ctx := context.Background()
	cancel := func() {}
	if policy.TotalTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, policy.TotalTimeout)
	}
	defer cancel()

	var result resetResult
	var firstErr error

	tot, del, err := resetConfiguredE2EUserAssignments(ctx, token, policy, transportOptions)
	if err != nil && firstErr == nil {
		firstErr = err
	}
	result.Details = append(result.Details, resetEndpoint{
		APIVersion: "v3",
		Endpoint:   "e2e-user-assignments",
		Total:      tot,
		Deleted:    del,
		Error:      errorString(err),
	})

	for _, step := range resetSequence {
		if err := ctx.Err(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			result.Details = append(result.Details, resetEndpoint{
				APIVersion: step.Version,
				Endpoint:   step.Endpoint,
				Error:      errorString(err),
			})
			break
		}
		targetURL := baseURL
		if step.UseGlobal {
			targetURL = "https://global.api.konghq.com"
		}
		deleteTargetURL := targetURL
		if step.DeleteUseRegional {
			deleteTargetURL = baseURL
		}
		tot, del, err := deleteAll(
			ctx,
			targetURL,
			deleteTargetURL,
			token,
			step.Version,
			step.Endpoint,
			step.DeleteVersion,
			step.DeleteEndpoint,
			step.IDField,
			step.Filter,
			step.PreDeleteFn,
			policy,
			transportOptions,
		)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		result.Details = append(result.Details, resetEndpoint{
			APIVersion: step.Version,
			Endpoint:   step.Endpoint,
			Total:      tot,
			Deleted:    del,
			Error:      errorString(err),
		})
	}

	return result, firstErr
}

type resetUser struct {
	ID string
}

func resetConfiguredE2EUserAssignments(
	ctx context.Context,
	token string,
	policy HTTPRetryPolicy,
	transportOptions HTTPTransportOptions,
) (int, int, error) {
	emails := configuredE2EUserEmails()
	if len(emails) == 0 {
		Debugf("No KONGCTL_E2E_ORG_USER_EMAIL_* values configured; skipping user assignment reset")
		return 0, 0, nil
	}

	session := newResetHTTPSession(policy.RequestTimeout, transportOptions)
	defer session.Close()

	usersByEmail, err := listUsersByEmail(ctx, session, token, policy)
	if err != nil {
		return len(emails), 0, err
	}

	total := 0
	deleted := 0
	for _, email := range emails {
		user, ok := usersByEmail[email]
		if !ok {
			Warnf("reset: configured E2E user was not found; skipping assignment reset")
			continue
		}
		userTotal, userDeleted, err := resetUserAssignments(ctx, session, token, policy, user)
		total += userTotal
		deleted += userDeleted
		if err != nil {
			return total, deleted, err
		}
	}
	return total, deleted, nil
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
	token string,
	policy HTTPRetryPolicy,
) (map[string]resetUser, error) {
	const pageSize = 100
	usersByEmail := map[string]resetUser{}
	for pageNumber := 1; pageNumber <= 10000; pageNumber++ {
		items, err := retryListItems(
			ctx,
			session,
			pagedURL("https://global.api.konghq.com/v3/users", pageSize, pageNumber),
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
	token string,
	policy HTTPRetryPolicy,
	user resetUser,
) (int, int, error) {
	total := 0
	deleted := 0

	teamItems, err := retryListItems(
		ctx,
		session,
		pagedURL(fmt.Sprintf("https://global.api.konghq.com/v3/users/%s/teams", url.PathEscape(user.ID)), 100, 1),
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
		deleteURL := fmt.Sprintf("https://global.api.konghq.com/v3/teams/%s/users", url.PathEscape(teamID))
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
		fmt.Sprintf("https://global.api.konghq.com/v3/users/%s/assigned-roles", url.PathEscape(user.ID)),
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
		deleteURL := fmt.Sprintf("https://global.api.konghq.com/v3/users/%s/assigned-roles", url.PathEscape(user.ID))
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
	details []resetEndpoint,
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
		[]byte(fmt.Sprintf("RESET ORG (stage=%s)\n", stage)),
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
		"type":     "reset_summary",
		"executed": executed,
		"status":   status,
		"reason":   reason,
		"details":  details,
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
) ([]map[string]any, error) {
	cfg := NormalizeBackoffConfig(policy.Backoff)
	attempts := cfg.Attempts
	backoff := BuildBackoffSchedule(cfg)
	var (
		items []map[string]any
		err   error
	)
	for atry := 0; atry < attempts; atry++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		start := time.Now()
		client := session.Client()
		items, err = listItemsWithContext(ctx, client, url, token)
		duration := time.Since(start)
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
) error {
	cfg := NormalizeBackoffConfig(policy.Backoff)
	attempts := cfg.Attempts
	backoff := BuildBackoffSchedule(cfg)
	var err error
	for atry := 0; atry < attempts; atry++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		start := time.Now()
		client := session.Client()
		err = deleteOneWithContext(ctx, client, baseURL, token, id)
		duration := time.Since(start)
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

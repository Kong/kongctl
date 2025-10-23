//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	client := &http.Client{Timeout: 30 * time.Second}

	result, err := executeReset(client, baseURL, token)
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
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func deleteAll(client *http.Client, baseURL, token, apiVersion, endpoint string) (int, int, error) {
	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(baseURL, "/"), apiVersion, endpoint)
	Infof("Fetching %s for deletion...", endpoint)

	const maxAttempts = 5
	const retryDelay = 2 * time.Second

	total := 0
	deleted := 0
	attempt := 0

	for {
		ids, err := listIDs(client, url, token)
		if err != nil {
			return total, deleted, err
		}

		if len(ids) == 0 {
			if total == 0 {
				Infof("No %s found", endpoint)
				return 0, 0, nil
			}
			return total, deleted, nil
		}

		if attempt == 0 {
			total = len(ids)
		} else if len(ids)+deleted > total {
			total = len(ids) + deleted
		}

		Infof("Attempt %d deleting %d %s", attempt+1, len(ids), endpoint)

		conflicts := 0
		for _, id := range ids {
			if err := deleteOne(client, url, token, id); err != nil {
				Warnf("delete %s %s failed: %v", endpoint, id, err)
				if he, ok := err.(*httpError); ok && he.status == http.StatusConflict {
					conflicts++
				}
			} else {
				Debugf("deleted %s %s", endpoint, id)
				deleted++
			}
		}

		if conflicts == 0 {
			return total, deleted, nil
		}

		attempt++
		if attempt >= maxAttempts {
			return total, deleted, fmt.Errorf("failed to delete all %s after %d attempts (conflicts remain)", endpoint, maxAttempts)
		}

		Infof("Retrying deletion of %s (%d conflicts remaining)", endpoint, conflicts)
		time.Sleep(retryDelay)
	}
}

type httpError struct {
	status int
	body   string
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

var resetSequence = []struct {
	Version  string
	Endpoint string
}{
	{"v3", "apis"},
	{"v3", "portals"},
	{"v2", "application-auth-strategies"},
	{"v2", "control-planes"},
}

func executeReset(client *http.Client, baseURL, token string) (resetResult, error) {
	var result resetResult
	var firstErr error

	for _, step := range resetSequence {
		tot, del, err := deleteAll(client, baseURL, token, step.Version, step.Endpoint)
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
				strings.Contains(ku, "SECRET") {
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

func listIDs(client *http.Client, url, token string) ([]string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
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
		return nil, fmt.Errorf("list failed: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var lr listResp
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(lr.Data))
	for _, d := range lr.Data {
		if d.ID != "" {
			ids = append(ids, d.ID)
		}
	}
	return ids, nil
}

func deleteOne(client *http.Client, baseURL, token, id string) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, baseURL+"/"+id, nil)
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
		return &httpError{status: resp.StatusCode, body: strings.TrimSpace(string(b))}
	}
	return nil
}

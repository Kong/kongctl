//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"errors"
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
func ResetOrgIfRequested() error { return ResetOrgWithCapture("unspecified") }

// ResetOrgWithCapture performs the same destructive reset as ResetOrgIfRequested and
// records a synthetic command under <run>/global/commands documenting execution.
// The stage parameter is recorded in artifacts (e.g., "before_suite", "before_test").
func ResetOrgWithCapture(stage string) error {
	// Default: reset is ON unless explicitly disabled.
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("KONGCTL_E2E_RESET"))); v != "" {
		if !truthyEnv(v) { // values like 0,false,off,no
			Infof("Reset disabled by KONGCTL_E2E_RESET=%s", v)
			captureResetEvent(stage, false, "skipped", "reset disabled", "", nil)
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
		captureResetEvent(stage, false, "skipped", "missing PAT", baseURL, nil)
		return nil
	}

	Infof("Resetting Konnect org at %s", baseURL)
	client := &http.Client{Timeout: 30 * time.Second}

	// Order can matter; follow provided script order.
	var details []resetEndpoint
	if tot, del, err := deleteAll(client, baseURL, token, "v2", "application-auth-strategies"); err != nil {
		captureResetEvent(stage, true, "error", err.Error(), baseURL, append(details, resetEndpoint{"v2", "application-auth-strategies", tot, del, err.Error()}))
		return err
	} else {
		details = append(details, resetEndpoint{"v2", "application-auth-strategies", tot, del, ""})
	}
	if tot, del, err := deleteAll(client, baseURL, token, "v3", "apis"); err != nil {
		captureResetEvent(stage, true, "error", err.Error(), baseURL, append(details, resetEndpoint{"v3", "apis", tot, del, err.Error()}))
		return err
	} else {
		details = append(details, resetEndpoint{"v3", "apis", tot, del, ""})
	}
	if tot, del, err := deleteAll(client, baseURL, token, "v3", "portals"); err != nil {
		captureResetEvent(stage, true, "error", err.Error(), baseURL, append(details, resetEndpoint{"v3", "portals", tot, del, err.Error()}))
		return err
	} else {
		details = append(details, resetEndpoint{"v3", "portals", tot, del, ""})
	}
	Infof("Reset complete")
	captureResetEvent(stage, true, "ok", "", baseURL, details)
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

	ids, err := listIDs(client, url, token)
	if err != nil {
		return 0, 0, err
	}
	if len(ids) == 0 {
		Infof("No %s found", endpoint)
		return 0, 0, nil
	}
	Infof("Found %d %s", len(ids), endpoint)

	succ := 0
	for _, id := range ids {
		if err := deleteOne(client, url, token, id); err != nil {
			Warnf("delete %s %s failed: %v", endpoint, id, err)
		} else {
			Debugf("deleted %s %s", endpoint, id)
			succ++
		}
	}
	return len(ids), succ, nil
}

type resetEndpoint struct {
	APIVersion string `json:"api_version"`
	Endpoint   string `json:"endpoint"`
	Total      int    `json:"total"`
	Deleted    int    `json:"deleted"`
	Error      string `json:"error,omitempty"`
}

func captureResetEvent(stage string, executed bool, status string, reason string, baseURL string, details []resetEndpoint) {
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
	_ = os.WriteFile(dir+string(os.PathSeparator)+"command.txt", []byte(fmt.Sprintf("RESET ORG (stage=%s)\n", stage)), 0o644)
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
			if strings.Contains(ku, "TOKEN") || strings.Contains(ku, "PAT") || strings.Contains(ku, "PASSWORD") || strings.Contains(ku, "SECRET") {
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
		return errors.New(fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(b))))
	}
	return nil
}

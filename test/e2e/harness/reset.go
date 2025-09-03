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
func ResetOrgIfRequested() error {
	// Default: reset is ON unless explicitly disabled.
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("KONGCTL_E2E_RESET"))); v != "" {
		if !truthyEnv(v) { // values like 0,false,off,no
			Infof("Reset disabled by KONGCTL_E2E_RESET=%s", v)
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
		return nil
	}

	Infof("Resetting Konnect org at %s", baseURL)
	client := &http.Client{Timeout: 30 * time.Second}

	// Order can matter; follow provided script order.
	if err := deleteAll(client, baseURL, token, "v2", "application-auth-strategies"); err != nil {
		return err
	}
	if err := deleteAll(client, baseURL, token, "v3", "apis"); err != nil {
		return err
	}
	if err := deleteAll(client, baseURL, token, "v3", "portals"); err != nil {
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

func deleteAll(client *http.Client, baseURL, token, apiVersion, endpoint string) error {
	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(baseURL, "/"), apiVersion, endpoint)
	Infof("Fetching %s for deletion...", endpoint)

	ids, err := listIDs(client, url, token)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		Infof("No %s found", endpoint)
		return nil
	}
	Infof("Found %d %s", len(ids), endpoint)

	for _, id := range ids {
		if err := deleteOne(client, url, token, id); err != nil {
			Warnf("delete %s %s failed: %v", endpoint, id, err)
		} else {
			Debugf("deleted %s %s", endpoint, id)
		}
	}
	return nil
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

//go:build e2e

package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/util"
)

var errIdentityResetUnavailable = errors.New("identity directory API is unavailable")

// IdentityAdditionalDirectoryCapacity opts into fixtures that create and delete
// directories alongside default. The normal suite preserves default's ID.
func IdentityAdditionalDirectoryCapacity() bool {
	return truthyEnv(os.Getenv("KONGCTL_E2E_IDENTITY_MULTI_DIRECTORY"))
}

type identityResetClient struct {
	session           *resetHTTPSession
	baseURL           string
	token             string
	expectedDefaultID string
	counts            *identityResetCounts
}

type identityResetCounts struct {
	listed, deleted int
}

func (c identityResetClient) request(ctx context.Context, method, path string, body any) (map[string]any, error) {
	started := time.Now()
	defer func() {
		switch method {
		case http.MethodGet:
			c.session.RecordList(time.Since(started))
		case http.MethodDelete:
			c.session.RecordDelete(time.Since(started))
		}
	}()
	var input io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		input = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.baseURL, "/")+path, input)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.session.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("identity reset %s: request failed", method)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("identity reset %s %s: %w", method, path, &httpError{status: response.StatusCode})
	}
	if response.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	var value map[string]any
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		return nil, fmt.Errorf("identity reset %s %s: invalid JSON response", method, path)
	}
	return value, nil
}

func (c identityResetClient) list(ctx context.Context, path string) ([]map[string]any, error) {
	var all []map[string]any
	after := ""
	seen := map[string]bool{}
	for range resetListMaxPages {
		query := url.Values{"page[size]": {"100"}}
		if after != "" {
			query.Set("page[after]", after)
		}
		value, err := c.request(ctx, http.MethodGet, path+"?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		data, ok := value["data"].([]any)
		if !ok {
			return nil, fmt.Errorf("identity reset %s: missing collection", path)
		}
		for _, item := range data {
			record, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("identity reset %s: invalid collection item", path)
			}
			if _, err := identityResetID(record); err != nil {
				return nil, err
			}
			all = append(all, record)
		}
		meta, _ := value["meta"].(map[string]any)
		page, _ := meta["page"].(map[string]any)
		next, _ := page["next"].(string)
		if next == "" {
			return all, nil
		}
		if seen[next] {
			return nil, fmt.Errorf("identity reset %s: repeated pagination cursor", path)
		}
		seen[next], after = true, next
	}
	return nil, fmt.Errorf("identity reset %s: pagination exceeded safety limit", path)
}

func identityResetID(value map[string]any) (string, error) {
	id, _ := value["id"].(string)
	if !util.IsValidUUID(id) {
		return "", fmt.Errorf("identity reset: response has no valid resource ID")
	}
	return id, nil
}

func (c identityResetClient) clear(ctx context.Context, path string) error {
	values, err := c.list(ctx, path)
	if err != nil {
		return err
	}
	for _, value := range values {
		id, err := identityResetID(value)
		if err != nil {
			return err
		}
		if _, err := c.request(ctx, http.MethodDelete, path+"/"+id, nil); err != nil {
			return err
		}
	}
	return nil
}

func identityDirectoryBaseline(vault bool) map[string]any {
	return map[string]any{
		"name": "default", "description": "", "allow_all_control_planes": true,
		"allowed_control_planes": []any{}, "ttl_secs": 600, "negative_ttl_secs": 600,
		"vault_enabled": vault, "labels": map[string]any{}, "managed_by": map[string]any{},
	}
}

func (c identityResetClient) clearDirectory(ctx context.Context, value map[string]any) error {
	id, err := identityResetID(value)
	if err != nil {
		return err
	}
	root := "/v2/directories/" + id
	// Vault must be visible while discovering dependencies, even when it was
	// disabled after credentials had already been created.
	body := identityDirectoryBaseline(true)
	body["name"] = value["name"]
	if _, err := c.request(ctx, http.MethodPut, root, body); err != nil {
		return err
	}
	principals, err := c.list(ctx, root+"/principals")
	if err != nil {
		return err
	}
	for _, principal := range principals {
		if principal["provisioned_by"] == "imported" {
			principalID, _ := identityResetID(principal)
			released, err := c.request(ctx, http.MethodPatch, root+"/principals/"+principalID,
				map[string]any{"active": false})
			if err != nil {
				return err
			}
			if released["active"] != false || released["provisioned_by"] != "konnect_user" {
				return fmt.Errorf("identity reset: imported principal was not released for deletion")
			}
		}
	}
	groups, err := c.list(ctx, root+"/groups")
	if err != nil {
		return err
	}
	for _, group := range groups {
		groupID, _ := identityResetID(group)
		for _, child := range []string{"membership", "api-keys"} {
			if err := c.clear(ctx, root+"/groups/"+groupID+"/"+child); err != nil {
				var status *httpError
				if child == "api-keys" && errors.As(err, &status) && status.status == http.StatusNotImplemented {
					// Destructive reset can ask the API to delete the whole group
					// on deployments without group-key inventory. This does not
					// assert that the group has no credentials.
					continue
				}
				return err
			}
		}
	}
	for _, principal := range principals {
		principalID, _ := identityResetID(principal)
		parent := root + "/principals/" + principalID
		for _, child := range []string{"vault-credentials", "api-keys", "identities"} {
			if err := c.clear(ctx, parent+"/"+child); err != nil {
				return err
			}
		}
		basicAuths, err := c.list(ctx, parent+"/basic-auths")
		if err != nil {
			return err
		}
		for _, basic := range basicAuths {
			basicID, _ := identityResetID(basic)
			basicPath := parent + "/basic-auths/" + basicID
			if err := c.clear(ctx, basicPath+"/passwords"); err != nil {
				return err
			}
			if _, err := c.request(ctx, http.MethodDelete, basicPath, nil); err != nil {
				return err
			}
		}
		if _, err := c.request(ctx, http.MethodDelete, parent, nil); err != nil {
			return err
		}
	}
	for _, group := range groups {
		groupID, _ := identityResetID(group)
		if _, err := c.request(ctx, http.MethodDelete, root+"/groups/"+groupID, nil); err != nil {
			return err
		}
	}
	providers, err := c.list(ctx, root+"/vault/providers")
	if err != nil {
		return err
	}
	for _, provider := range providers {
		providerID, _ := identityResetID(provider)
		parent := root + "/vault/providers/" + providerID
		if err := c.clear(ctx, parent+"/credentials"); err != nil {
			return err
		}
		if _, err := c.request(ctx, http.MethodDelete, parent, nil); err != nil {
			return err
		}
	}
	for _, child := range []string{"trusted-idps", "vault/keys"} {
		if err := c.clear(ctx, root+"/"+child); err != nil {
			return err
		}
	}
	return nil
}

func resetIdentityDirectories(
	ctx context.Context, client identityResetClient, onlyDefault bool,
) (map[string]any, error) {
	directories, err := client.list(ctx, "/v2/directories")
	if err != nil {
		var status *httpError
		if errors.As(err, &status) && status.status == http.StatusNotFound {
			return nil, fmt.Errorf("%w: %w", errIdentityResetUnavailable, err)
		}
		return nil, err
	}
	if client.counts != nil {
		client.counts.listed = len(directories)
	}
	var current map[string]any
	for _, directory := range directories {
		if directory["name"] == "default" {
			if current != nil {
				return nil, fmt.Errorf("identity reset: multiple directories named default")
			}
			current = directory
		}
	}
	if client.expectedDefaultID != "" && (current == nil || current["id"] != client.expectedDefaultID) {
		return nil, fmt.Errorf("identity reset: shared default directory ID changed or disappeared")
	}
	for _, directory := range directories {
		if onlyDefault && directory["name"] != "default" {
			continue
		}
		if err := client.clearDirectory(ctx, directory); err != nil {
			return nil, err
		}
		if directory["name"] != "default" {
			id, _ := identityResetID(directory)
			if _, err := client.request(ctx, http.MethodDelete, "/v2/directories/"+id+"?force=false", nil); err != nil {
				return nil, err
			}
			if client.counts != nil {
				client.counts.deleted++
			}
		}
	}
	if current == nil {
		created, err := client.request(ctx, http.MethodPost, "/v2/directories", identityDirectoryBaseline(false))
		if err != nil {
			return nil, err
		}
		if _, err := identityResetID(created); err != nil {
			return nil, err
		}
		return created, nil
	}
	id, _ := identityResetID(current)
	restored, err := client.request(ctx, http.MethodPut, "/v2/directories/"+id, identityDirectoryBaseline(false))
	if err != nil {
		return nil, err
	}
	if restored["id"] != id || restored["name"] != "default" || restored["vault_enabled"] != false {
		return nil, fmt.Errorf("identity reset: default directory baseline was not restored")
	}
	return restored, nil
}

// ResetDefaultIdentityDirectory empties the shared E2E directory while preserving
// its ID. Like ResetOrg, this is destructive and intended for test organizations.
func ResetDefaultIdentityDirectory(ctx context.Context, expectedID string) (map[string]any, error) {
	baseURL, err := KonnectBaseURL()
	if err != nil {
		return nil, err
	}
	session := newResetHTTPSession(resetHTTPPolicyFromEnv().RequestTimeout, HTTPTransportOptionsFromEnv())
	defer session.Close()
	token := os.Getenv("KONGCTL_E2E_KONNECT_PAT")
	if token == "" {
		return nil, fmt.Errorf("default-directory fixture requires KONGCTL_E2E_KONNECT_PAT")
	}
	return resetIdentityDirectories(ctx, identityResetClient{
		session: session, baseURL: baseURL, token: token, expectedDefaultID: expectedID,
	}, true)
}

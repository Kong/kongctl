package hash

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kong/kongctl/internal/declarative/labels"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// CalculatePortalHash generates deterministic hash for portal config
func CalculatePortalHash(portal kkInternalComps.CreatePortal) (string, error) {
	// Create hashable structure with sorted fields
	hashable := map[string]interface{}{
		"name":                               portal.Name,
		"display_name":                       portal.DisplayName,
		"description":                        portal.Description,
		"authentication_enabled":             portal.AuthenticationEnabled,
		"rbac_enabled":                      portal.RbacEnabled,
		"default_api_visibility":            portal.DefaultAPIVisibility,
		"default_page_visibility":           portal.DefaultPageVisibility,
		"default_application_auth_strategy_id": portal.DefaultApplicationAuthStrategyID,
		"auto_approve_developers":           portal.AutoApproveDevelopers,
		"auto_approve_applications":         portal.AutoApproveApplications,
	}

	// Add user labels only (exclude KONGCTL labels)
	if portal.Labels != nil {
		userLabels := make(map[string]string)
		normalized := labels.NormalizeLabels(portal.Labels)

		for k, v := range normalized {
			if !labels.IsKongctlLabel(k) {
				userLabels[k] = v
			}
		}

		if len(userLabels) > 0 {
			hashable["user_labels"] = sortedMap(userLabels)
		}
	}

	return calculateHash(hashable)
}

// sortedMap returns map with keys in sorted order for deterministic JSON
func sortedMap(m map[string]string) map[string]string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sorted := make(map[string]string)
	for _, k := range keys {
		sorted[k] = m[k]
	}
	return sorted
}

// calculateHash generates SHA256 hash from data structure
func calculateHash(data interface{}) (string, error) {
	// Marshal to JSON with sorted keys
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal for hash: %w", err)
	}

	// Generate SHA256 hash
	hash := sha256.Sum256(jsonBytes)

	// Return base64 encoded string
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// ComparePortalHash checks if portal config matches expected hash
func ComparePortalHash(portal kkInternalComps.PortalResponse, expectedHash string) (bool, error) {
	// Convert response to create structure for hashing
	// Need to convert between different enum types
	var apiVisibility *kkInternalComps.DefaultAPIVisibility
	if portal.DefaultAPIVisibility == kkInternalComps.PortalResponseDefaultAPIVisibilityPublic {
		apiVisibility = (*kkInternalComps.DefaultAPIVisibility)(ptrString("public"))
	} else if portal.DefaultAPIVisibility == kkInternalComps.PortalResponseDefaultAPIVisibilityPrivate {
		apiVisibility = (*kkInternalComps.DefaultAPIVisibility)(ptrString("private"))
	}

	var pageVisibility *kkInternalComps.DefaultPageVisibility
	if portal.DefaultPageVisibility == kkInternalComps.PortalResponseDefaultPageVisibilityPublic {
		pageVisibility = (*kkInternalComps.DefaultPageVisibility)(ptrString("public"))
	} else if portal.DefaultPageVisibility == kkInternalComps.PortalResponseDefaultPageVisibilityPrivate {
		pageVisibility = (*kkInternalComps.DefaultPageVisibility)(ptrString("private"))
	}

	createPortal := kkInternalComps.CreatePortal{
		Name:                            portal.Name,
		DisplayName:                     &portal.DisplayName,
		Description:                     portal.Description,
		AuthenticationEnabled:           &portal.AuthenticationEnabled,
		RbacEnabled:                    &portal.RbacEnabled,
		DefaultAPIVisibility:           apiVisibility,
		DefaultPageVisibility:          pageVisibility,
		DefaultApplicationAuthStrategyID: portal.DefaultApplicationAuthStrategyID,
		AutoApproveDevelopers:          &portal.AutoApproveDevelopers,
		AutoApproveApplications:        &portal.AutoApproveApplications,
		Labels:                         labels.DenormalizeLabels(portal.Labels),
	}

	actualHash, err := CalculatePortalHash(createPortal)
	if err != nil {
		return false, err
	}

	return actualHash == expectedHash, nil
}

// Helper function
func ptrString(s string) *string {
	return &s
}
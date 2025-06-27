package hash

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
)

// CalculateResourceHash computes a deterministic hash for any resource
func CalculateResourceHash(resource interface{}) (string, error) {
	// Step 1: Use json.Marshal for serialization
	jsonBytes, err := json.Marshal(resource)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource: %w", err)
	}

	// Step 2: Parse into generic map to filter fields
	var data map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return "", fmt.Errorf("failed to unmarshal for filtering: %w", err)
	}

	// Step 3: Filter out system fields and KONGCTL labels
	filtered := filterForHashing(data)

	// Step 4: Re-marshal with deterministic ordering
	// json.Marshal on maps already sorts keys alphabetically
	canonicalJSON, err := json.Marshal(filtered)
	if err != nil {
		return "", fmt.Errorf("failed to marshal filtered data: %w", err)
	}

	// Step 5: Calculate SHA256 hash
	hash := sha256.Sum256(canonicalJSON)
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// CalculatePortalHash generates deterministic hash for portal config
func CalculatePortalHash(portal kkInternalComps.CreatePortal) (string, error) {
	return CalculateResourceHash(portal)
}

// CalculateAPIHash generates deterministic hash for API config
func CalculateAPIHash(api kkInternalComps.CreateAPIRequest) (string, error) {
	return CalculateResourceHash(api)
}

// CalculateAPIVersionHash generates deterministic hash for API version config
func CalculateAPIVersionHash(version kkInternalComps.CreateAPIVersionRequest) (string, error) {
	return CalculateResourceHash(version)
}

// CalculateAPIDocumentHash generates deterministic hash for API document config
func CalculateAPIDocumentHash(doc kkInternalComps.CreateAPIDocumentRequest) (string, error) {
	return CalculateResourceHash(doc)
}


// filterForHashing removes system fields and KONGCTL labels
func filterForHashing(data map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})

	for key, value := range data {
		// Skip system-generated fields
		if isSystemField(key) {
			continue
		}

		// Special handling for labels
		if key == "labels" {
			if labelMap, ok := value.(map[string]interface{}); ok {
				userLabels := filterKONGCTLLabels(labelMap)
				if len(userLabels) > 0 {
					filtered[key] = userLabels
				}
			}
			continue
		}

		// Recursively filter nested objects
		if nestedMap, ok := value.(map[string]interface{}); ok {
			filtered[key] = filterForHashing(nestedMap)
		} else if nestedArray, ok := value.([]interface{}); ok {
			// Handle arrays of objects
			filteredArray := make([]interface{}, 0, len(nestedArray))
			for _, item := range nestedArray {
				if itemMap, ok := item.(map[string]interface{}); ok {
					filteredArray = append(filteredArray, filterForHashing(itemMap))
				} else {
					filteredArray = append(filteredArray, item)
				}
			}
			filtered[key] = filteredArray
		} else {
			filtered[key] = value
		}
	}

	return filtered
}

// filterKONGCTLLabels returns only user-defined labels
func filterKONGCTLLabels(labelMap map[string]interface{}) map[string]interface{} {
	userLabels := make(map[string]interface{})

	for k, v := range labelMap {
		// Skip KONGCTL-managed labels
		if !strings.HasPrefix(k, "KONGCTL/") {
			userLabels[k] = v
		}
	}

	return userLabels
}

// isSystemField identifies fields that should be excluded from hash
func isSystemField(fieldName string) bool {
	systemFields := map[string]bool{
		"id":                   true,
		"created_at":           true,
		"updated_at":           true,
		"default_domain":       true, // Portal-specific generated field
		"canonical_domain":     true, // Portal-specific generated field
		"status":               true, // Various resources have status fields
		"state":                true, // State is often system-managed
		"verification_status":  true, // Various verification statuses
	}
	return systemFields[fieldName]
}

// ComparePortalHash checks if portal config matches expected hash
func ComparePortalHash(portal kkInternalComps.PortalResponse, expectedHash string) (bool, error) {
	// Convert response to create structure for hashing
	// Need to convert between different enum types
	var apiVisibility *kkInternalComps.DefaultAPIVisibility
	switch portal.DefaultAPIVisibility {
	case kkInternalComps.PortalResponseDefaultAPIVisibilityPublic:
		apiVisibility = (*kkInternalComps.DefaultAPIVisibility)(ptrString("public"))
	case kkInternalComps.PortalResponseDefaultAPIVisibilityPrivate:
		apiVisibility = (*kkInternalComps.DefaultAPIVisibility)(ptrString("private"))
	}

	var pageVisibility *kkInternalComps.DefaultPageVisibility
	switch portal.DefaultPageVisibility {
	case kkInternalComps.PortalResponseDefaultPageVisibilityPublic:
		pageVisibility = (*kkInternalComps.DefaultPageVisibility)(ptrString("public"))
	case kkInternalComps.PortalResponseDefaultPageVisibilityPrivate:
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
package resources

import (
	"fmt"
	"maps"
)

const (
	ManagedByToolKey    = "tool"
	ManagedByVersionKey = "version"
	ManagedByTool       = "kongctl"
)

// ManagedByWithProvenance stamps the writer without adopting the resource.
func ManagedByWithProvenance(metadata map[string]string, version string) map[string]string {
	result := maps.Clone(metadata)
	if result == nil {
		result = make(map[string]string)
	}
	if version == "" {
		version = "dev"
	}
	result[ManagedByToolKey] = ManagedByTool
	result[ManagedByVersionKey] = version
	return result
}

// ManagedByPolicy records the verified metadata limit of one resource API.
// Writing tool/version is independent of namespace and ownership capabilities.
type ManagedByPolicy struct {
	MaxEntries int
}

// WithManagedBy opts a resource into shared tool/version writing. Existing
// labels, protection, and ownership decisions remain with their current adapters.
func WithManagedBy(policy ManagedByPolicy) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if policy.MaxEntries < 2 {
			return fmt.Errorf("managed_by entry limit must accommodate tool and version")
		}
		if ops.managedBy != nil {
			return fmt.Errorf("managed_by capability already registered")
		}
		ops.managedBy = &policy
		return nil
	}
}

// SupportsManagedBy reports explicit write enrollment, not inferred SDK support.
func SupportsManagedBy(kind ResourceType) bool {
	return registry[kind].managedBy != nil
}

// PrepareManagedBy preserves unrelated entries and applies the registered API's
// capacity policy. Unenrolled resource types remain unchanged.
func PrepareManagedBy(kind ResourceType, metadata map[string]string, version string) (map[string]string, error) {
	policy := registry[kind].managedBy
	if policy == nil {
		return metadata, nil
	}
	result := ManagedByWithProvenance(metadata, version)
	if err := ValidateManagedBy(result, *policy); err != nil {
		return nil, err
	}
	return result, nil
}

// ValidateManagedBy validates Konnect metadata keys and a resource-specific limit.
func ValidateManagedBy(metadata map[string]string, policy ManagedByPolicy) error {
	if len(metadata) > policy.MaxEntries {
		return fmt.Errorf("managed_by exceeds the %d-entry limit after merging generated metadata (%d entries)",
			policy.MaxEntries, len(metadata))
	}
	for key := range metadata {
		if len(key) < 1 || len(key) > 63 || !isMetadataAlphanumeric(key[0]) {
			return fmt.Errorf("invalid managed_by key: keys must be 1–63 characters and start with an alphanumeric character")
		}
	}
	return nil
}

func isMetadataAlphanumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

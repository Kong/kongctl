package resources

import (
	"fmt"
)

const (
	ManagedByToolKey    = "tool"
	ManagedByVersionKey = "version"
	ManagedByTool       = "kongctl"
	ManagedByRefKey     = "kongctl-ref"
)

// ManagedByWithProvenance builds the complete tool-owned map without adopting a resource.
func ManagedByWithProvenance(metadata map[string]string, version string) map[string]string {
	result := make(map[string]string)
	for _, key := range []string{ChildNamespaceKey, ChildProtectedKey} {
		if value, present := metadata[key]; present {
			result[key] = value
		}
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
	// PersistsRef reserves a fifth key for resources without a unique public moniker.
	PersistsRef bool
	// OwnsLifecycle uses managed_by for namespace/protection on a managed root.
	OwnsLifecycle bool
}

// WithManagedBy opts a resource into shared tool/version writing. Existing
// labels, protection, and ownership decisions remain with their current adapters.
func WithManagedBy(policy ManagedByPolicy) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if ops.independentChildLabels {
			return fmt.Errorf("managed_by cannot also use independent label ownership")
		}
		if policy.MaxEntries < 2 {
			return fmt.Errorf("managed_by entry limit must accommodate tool and version")
		}
		if policy.OwnsLifecycle && (ops.namespace == nil || policy.MaxEntries < 4) {
			return fmt.Errorf(
				"managed_by lifecycle ownership requires namespace participation and capacity for four keys",
			)
		}
		if policy.PersistsRef && (policy.MaxEntries < 5 || ops.namespace == nil) {
			return fmt.Errorf("persisted ref requires namespace participation and capacity for five keys")
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

// PrepareManagedBy replaces the map with the registered resource's generated
// metadata. Unenrolled resource types remain unchanged.
func PrepareManagedBy(kind ResourceType, metadata map[string]string, version string) (map[string]string, error) {
	policy := registry[kind].managedBy
	if policy == nil {
		return metadata, nil
	}
	if !UsesManagedByOwnership(kind) {
		metadata = nil
	}
	result := ManagedByWithProvenance(metadata, version)
	if policy.PersistsRef {
		ref := metadata[ManagedByRefKey]
		if err := ValidateRef(ref); err != nil {
			return nil, fmt.Errorf("invalid persisted managed_by ref: %w", err)
		}
		result[ManagedByRefKey] = ref
	}
	if err := ValidateManagedBy(result, *policy); err != nil {
		return nil, err
	}
	return result, nil
}

// UsesManagedByOwnership distinguishes metadata ownership from legacy label ownership.
func UsesManagedByOwnership(kind ResourceType) bool {
	policy := registry[kind].managedBy
	return (HasIndependentChildOwnership(kind) && !registry[kind].independentChildLabels) ||
		policy != nil && policy.OwnsLifecycle
}

// ValidateManagedBy validates Konnect metadata keys and a resource-specific limit.
func ValidateManagedBy(metadata map[string]string, policy ManagedByPolicy) error {
	if len(metadata) > policy.MaxEntries {
		return fmt.Errorf("managed_by exceeds the %d-entry limit after merging generated metadata (%d entries)",
			policy.MaxEntries, len(metadata))
	}
	for key := range metadata {
		if len(key) < 1 || len(key) > 63 || !isMetadataAlphanumeric(key[0]) {
			return fmt.Errorf(
				"invalid managed_by key: keys must be 1–63 characters and start with an alphanumeric character",
			)
		}
	}
	return nil
}

func isMetadataAlphanumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

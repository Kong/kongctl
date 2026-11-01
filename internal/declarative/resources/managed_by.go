package resources

import (
	"fmt"
	"maps"
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
	// PersistsRef reserves a lookup key for resources without a unique moniker.
	// Independent ownership needs five keys; inherited lifecycle needs no copy
	// of the parent's namespace/protection fields.
	PersistsRef bool
	// OwnsLifecycle uses managed_by for namespace/protection on a managed root.
	OwnsLifecycle bool
	// CreateOnly leaves an immutable remote map unchanged on later updates.
	CreateOnly bool
	// CreationValues records fixed creation intent not otherwise observable.
	// These fields do not supply namespace or protection authority.
	CreationValues map[string]string
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
		if policy.PersistsRef {
			if ops.namespace == nil && ops.namespaceOwner == "" {
				return fmt.Errorf("persisted ref requires namespace participation or a namespace owner")
			}
			if ops.namespace != nil && policy.MaxEntries < 5 {
				return fmt.Errorf("persisted ref requires namespace participation and capacity for five keys")
			}
		}
		reserved := map[string]bool{
			ManagedByToolKey: true, ManagedByVersionKey: true, ManagedByRefKey: true,
			ChildNamespaceKey: true, ChildProtectedKey: true,
		}
		for key := range policy.CreationValues {
			if reserved[key] {
				return fmt.Errorf("creation metadata cannot override reserved management keys")
			}
		}
		minimum := 2 + len(policy.CreationValues)
		if policy.PersistsRef {
			minimum++
		}
		if policy.OwnsLifecycle || ops.independentChildMeta != nil {
			minimum += 2
		}
		if policy.MaxEntries < minimum {
			return fmt.Errorf("managed_by entry limit cannot accommodate generated metadata (%d entries)", minimum)
		}
		if err := ValidateManagedBy(policy.CreationValues, policy); err != nil {
			return err
		}
		if ops.managedBy != nil {
			return fmt.Errorf("managed_by capability already registered")
		}
		policy.CreationValues = maps.Clone(policy.CreationValues)
		ops.managedBy = &policy
		return nil
	}
}

// SupportsManagedBy reports explicit write enrollment, not inferred SDK support.
func SupportsManagedBy(kind ResourceType) bool {
	return registry[kind].managedBy != nil
}

// UpdatesManagedBy separates mutable ownership from create-only provenance.
func UpdatesManagedBy(kind ResourceType) bool {
	policy := registry[kind].managedBy
	return policy != nil && !policy.CreateOnly
}

// PrepareManagedBy replaces the map with the registered resource's generated
// metadata. Unenrolled resource types remain unchanged.
func PrepareManagedBy(kind ResourceType, metadata map[string]string, version string) (map[string]string, error) {
	policy := registry[kind].managedBy
	if policy == nil {
		return metadata, nil
	}
	ref := metadata[ManagedByRefKey]
	if !UsesManagedByOwnership(kind) {
		metadata = nil
	}
	result := ManagedByWithProvenance(metadata, version)
	if policy.PersistsRef {
		if err := ValidateRef(ref); err != nil {
			return nil, fmt.Errorf("invalid persisted managed_by ref: %w", err)
		}
		result[ManagedByRefKey] = ref
	}
	maps.Copy(result, policy.CreationValues)
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

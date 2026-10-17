package resources

import (
	"fmt"
	"maps"
	"reflect"
	"strconv"

	"github.com/kong/kongctl/internal/declarative/labels"
)

// These keys are lifecycle metadata, independent of gateway ownership.
const (
	ChildNamespaceKey   = "kongctl-namespace"
	ChildProtectedKey   = "kongctl-protected"
	ChildManagedByLimit = 5
)

// BuildChildManagedBy preserves metadata belonging to other tools. Validate the
// resulting map before planning or writing: the limit applies after merging.
func BuildChildManagedBy(
	current, desired map[string]string,
	namespace string,
	protected bool,
	version string,
) map[string]string {
	metadata := maps.Clone(current)
	if metadata == nil {
		metadata = make(map[string]string)
	}
	maps.Copy(metadata, desired)
	metadata[ChildNamespaceKey] = namespace
	metadata[ChildProtectedKey] = strconv.FormatBool(protected)
	return ManagedByWithProvenance(metadata, version)
}

func ValidateChildManagedBy(metadata map[string]string, raw bool) error {
	if err := ValidateManagedBy(metadata, ManagedByPolicy{MaxEntries: ChildManagedByLimit}); err != nil {
		return err
	}
	for key := range metadata {
		if raw && (key == ChildNamespaceKey || key == ChildProtectedKey ||
			key == ManagedByToolKey || key == ManagedByVersionKey) {
			return fmt.Errorf("managed_by key %q is reserved for kongctl", key)
		}
	}
	return nil
}

// Unowned objects require explicit out-of-band adoption. Structural parenthood
// and source defaults never authorize a write or an ownership transfer.
func ValidateChildOwnership(kind string, metadata map[string]string, namespace string) error {
	owner := metadata[ChildNamespaceKey]
	if owner == "" {
		return fmt.Errorf("%s is unowned; explicitly adopt it into namespace %q before managing it", kind, namespace)
	}
	if namespace == "" || owner != namespace {
		return fmt.Errorf("%s belongs to namespace %q, not %q; explicit ownership transfer is required",
			kind,
			owner,
			namespace)
	}
	if value, ok := metadata[ChildProtectedKey]; ok && value != "true" && value != "false" {
		return fmt.Errorf("%s has invalid %s metadata", kind, ChildProtectedKey)
	}
	return nil
}

func NormalizedChildOwnership(metadata map[string]string) map[string]string {
	if metadata[ChildNamespaceKey] == "" {
		return nil
	}
	return map[string]string{
		labels.NamespaceKey: metadata[ChildNamespaceKey],
		labels.ProtectedKey: metadata[ChildProtectedKey],
	}
}

// WithIndependentChildOwnership explicitly enrolls one verified child family.
// Namespace participation and source collection presence are separate contracts;
// registering one must not implicitly enable the other for every gateway child.
func WithIndependentChildOwnership[R any](capture func(*ResourceSet,
	string),
	metadata func(*R) *KongctlMeta,
) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if ops.namespace == nil || capture == nil || metadata == nil || ops.captureOwnedChildScope != nil ||
			ops.explain.typ != reflect.TypeFor[R]() {
			return fmt.Errorf("independent child ownership requires namespace participation and one scope capture")
		}
		ops.captureOwnedChildScope = capture
		ops.independentChildMeta = func(resource Resource) *KongctlMeta { return metadata(any(resource).(*R)) }
		return nil
	}
}

func HasIndependentChildOwnership(kind ResourceType) bool {
	return registry[kind].captureOwnedChildScope != nil
}

// CaptureOwnedChildCollection applies source defaults only to explicitly empty
// collections. Nonempty collections contribute each child's own namespace.
func CaptureOwnedChildCollection(scope *SyncScope,
	kind ResourceType,
	parent,
	defaultNamespace string,
	present bool,
	namespaces []string,
) {
	if !present {
		return
	}
	if len(namespaces) == 0 {
		namespaces = []string{defaultNamespace}
	}
	for _, namespace := range namespaces {
		if kind == ResourceTypeAIGatewayModel {
			scope.AddModelNamespace(parent, namespace)
		} else {
			scope.AddChildNamespace(kind, parent, namespace)
		}
	}
}

// IndependentChildMetadata is the shared source of namespace/protection for
// auxiliary writes, including secret-only updates, without inheriting a parent.
func IndependentChildMetadata(resource Resource) (*KongctlMeta, bool) {
	accessor := registry[resource.GetType()].independentChildMeta
	if accessor == nil {
		return nil, false
	}
	return accessor(resource), true
}

// ChildUnprotectionMetadataOnly proves the only metadata mutation is removing
// protection. Payload preservation and secret writes are adapter-specific checks.
func ChildUnprotectionMetadataOnly(current, desired map[string]string) bool {
	if desired[ChildProtectedKey] != "false" {
		return false
	}
	before, after := maps.Clone(current), maps.Clone(desired)
	delete(before, ChildProtectedKey)
	delete(after, ChildProtectedKey)
	// Provenance changes automatically with the executing binary; it does not
	// authorize any other metadata, payload, or secret mutation.
	delete(before, ManagedByToolKey)
	delete(before, ManagedByVersionKey)
	delete(after, ManagedByToolKey)
	delete(after, ManagedByVersionKey)
	return maps.Equal(before, after)
}

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

// BuildChildManagedBy generates the complete map from kongctl ownership settings.
func BuildChildManagedBy(namespace string, protected bool, version string) map[string]string {
	return ManagedByWithProvenance(map[string]string{
		ChildNamespaceKey: namespace,
		ChildProtectedKey: strconv.FormatBool(protected),
	}, version)
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

// IsChildProtected reads protection from the child's ownership metadata.
func IsChildProtected(metadata map[string]string) bool {
	return metadata[ChildProtectedKey] == "true"
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

// WithIndependentChildLabelOwnership enrolls a child that persists lifecycle
// metadata in labels rather than managed_by. Source scope remains explicit.
func WithIndependentChildLabelOwnership[R any](capture func(*ResourceSet, string),
	metadata func(*R) *KongctlMeta,
) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if ops.managedBy != nil {
			return fmt.Errorf("independent label ownership cannot also use managed_by")
		}
		if err := WithIndependentChildOwnership(capture, metadata)(ops); err != nil {
			return err
		}
		ops.independentChildLabels = true
		return nil
	}
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

// ChildUnprotectionMetadataOnly allows only unprotection among lifecycle fields.
// Canonicalization may discard outside keys; payload and secret checks are separate.
func ChildUnprotectionMetadataOnly(current, desired map[string]string) bool {
	if desired[ChildProtectedKey] != "false" {
		return false
	}
	before, after := ManagedByWithProvenance(current, ""), ManagedByWithProvenance(desired, "")
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

// WithAIGatewayOwnedChildNamespace binds independent ownership to the original
// nested slices and root declarations. Scope capture runs before extraction.
func WithAIGatewayOwnedChildNamespace[R any](
	order int, kind ResourceType,
	nested func(*AIGatewayResource) *[]R,
	base func(*R) *BaseResource,
	parent func(*R) string,
) ResourceRegistrationOption {
	participant := func(r *R) NamespaceParticipant {
		b := base(r)
		return NamespaceParticipant{
			Ref:               b.Ref,
			Label:             string(kind),
			SupportsProtected: true,
			Meta:              &b.Kongctl,
		}
	}
	groups := func(rs *ResourceSet) [][]R {
		result := make([][]R, 0, len(rs.AIGateways))
		for i := range rs.AIGateways {
			result = append(result, *nested(&rs.AIGateways[i]))
		}
		return result
	}
	capture := func(rs *ResourceSet, defaultNamespace string) {
		scope := rs.EnsureSyncScope()
		for i := range rs.AIGateways {
			gateway := &rs.AIGateways[i]
			var namespaces []string
			for j := range *nested(gateway) {
				namespaces = append(namespaces, GetNamespace(base(&(*nested(gateway))[j]).Kongctl))
			}
			CaptureOwnedChildCollection(
				scope,
				kind,
				gateway.Ref,
				defaultNamespace,
				scope.ChildInScope(ResourceTypeAIGateway, gateway.Ref, kind),
				namespaces,
			)
		}
		for _, resource := range rs.AllResourcesByType(kind) {
			r := any(resource).(*R)
			scope.AddChildNamespace(kind, parent(r), GetNamespace(base(r).Kongctl))
		}
	}
	return func(ops *resourceOps) error {
		for _, option := range []ResourceRegistrationOption{
			WithNamespace(order,
				participant),
			WithNestedNamespaceGroups(groups,
				participant),
			WithIndependentChildOwnership(capture,
				func(r *R) *KongctlMeta { return base(r).Kongctl }),
		} {
			if err := option(ops); err != nil {
				return err
			}
		}
		return nil
	}
}

func validateChildDeclarationManagedBy(metadata map[string]string, meta *KongctlMeta) error {
	if err := ValidateChildManagedBy(metadata, true); err != nil {
		return err
	}
	protected := meta.IsProtected()
	return ValidateChildManagedBy(
		BuildChildManagedBy(GetNamespace(meta), protected, ""),
		false,
	)
}

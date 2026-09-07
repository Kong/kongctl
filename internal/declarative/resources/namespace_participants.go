package resources

import (
	"cmp"
	"fmt"
	"maps"
	"reflect"
	"slices"
)

// NamespaceParticipant is a namespace-bearing declarative parent resource.
// Resource registrations supply these values; callers retain their own
// defaulting, validation, and external-resource policies.
type NamespaceParticipant struct {
	Type ResourceType
	Ref  string
	// External reports whether the resource is declared as an external reference.
	External bool
	// SupportsProtected excludes namespace-only organization selectors.
	SupportsProtected bool
	// Label is the human-facing name used in loader defaulting errors.
	Label string
	// Meta addresses the resource's Kongctl field for in-place defaulting.
	Meta **KongctlMeta
}

type namespaceRegistration struct {
	kind        ResourceType
	order       int
	visit       func(*ResourceSet, func(NamespaceParticipant) error) error
	visitNested func(*ResourceSet, func(NamespaceParticipant) error) error
}

var namespaceRegistrations []namespaceRegistration

// WithNamespace registers typed metadata access alongside the resource's
// existing slice accessor. Order preserves namespace diagnostic traversal.
// Additional sources are grouping locations visited before nested extraction.
func WithNamespace[R any](
	order int,
	participant func(*R) NamespaceParticipant,
	nested ...func(*ResourceSet) []R,
) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if ops.namespace != nil {
			return fmt.Errorf("namespace capability is already registered")
		}
		if participant == nil || ops.explain.typ != reflect.TypeFor[R]() {
			return fmt.Errorf("namespace accessor must match the registered resource type")
		}
		ops.namespace = &namespaceRegistration{
			order: order,
			visit: func(rs *ResourceSet, fn func(NamespaceParticipant) error) error {
				var err error
				ops.forEach(rs, func(resource Resource) bool {
					err = fn(participant(any(resource).(*R)))
					return err == nil
				})
				return err
			},
			visitNested: func(rs *ResourceSet, fn func(NamespaceParticipant) error) error {
				for _, source := range nested {
					if err := visitNamespaceSlice(source(rs), participant, fn); err != nil {
						return err
					}
				}
				return nil
			},
		}
		return nil
	}
}

// Organization selectors do not implement Resource and must not enter the
// declaration registry merely to participate in namespace processing.
func registerNamespaceSelector[R any](
	kind ResourceType,
	order int,
	source func(*ResourceSet) []R,
	participant func(*R) NamespaceParticipant,
) {
	registerNamespaceParticipant(kind, namespaceRegistration{
		order: order,
		visit: func(rs *ResourceSet, fn func(NamespaceParticipant) error) error {
			return visitNamespaceSlice(source(rs), participant, fn)
		},
	})
}

func visitNamespaceSlice[R any](
	values []R,
	participant func(*R) NamespaceParticipant,
	fn func(NamespaceParticipant) error,
) error {
	for i := range values {
		if err := fn(participant(&values[i])); err != nil {
			return err
		}
	}
	return nil
}

func registerNamespaceParticipant(kind ResourceType, registration namespaceRegistration) {
	if kind == "" || registration.order <= 0 || registration.visit == nil {
		panic("namespace registration requires a resource type, positive order, and visitor")
	}
	for _, existing := range namespaceRegistrations {
		if existing.kind == kind || existing.order == registration.order {
			panic("duplicate namespace resource type or traversal order: " + string(kind))
		}
	}
	registration.kind = kind
	namespaceRegistrations = append(namespaceRegistrations, registration)
	slices.SortFunc(namespaceRegistrations, func(a, b namespaceRegistration) int {
		return cmp.Compare(a.order, b.order)
	})
}

// ForEachNamespaceParticipant visits namespace-bearing resources in diagnostic
// order and stops on error. Both flattened and nested grouping locations are
// visited, preserving behavior before and after nested extraction.
func (rs *ResourceSet) ForEachNamespaceParticipant(fn func(NamespaceParticipant) error) error {
	if rs == nil {
		return nil
	}
	for _, registration := range namespaceRegistrations {
		visit := func(participant NamespaceParticipant) error {
			participant.Type = registration.kind
			return fn(participant)
		}
		if err := registration.visit(rs, visit); err != nil {
			return err
		}
		if registration.visitNested != nil {
			if err := registration.visitNested(rs, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

// NamespaceValues returns present kongctl.namespace values after extraction.
// It reads flattened resources and organization selectors, without revisiting
// grouping locations or applying the planner's external-namespace policy.
func (rs *ResourceSet) NamespaceValues() []string {
	if rs == nil {
		return nil
	}
	namespaces := make(map[string]struct{})
	for _, registration := range namespaceRegistrations {
		_ = registration.visit(rs, func(participant NamespaceParticipant) error {
			if meta := *participant.Meta; meta != nil && meta.Namespace != nil {
				namespaces[*meta.Namespace] = struct{}{}
			}
			return nil
		})
	}
	return slices.Collect(maps.Keys(namespaces))
}

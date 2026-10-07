package resources

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"sync"
)

// Reference validation enrollment is independent of reference metadata: adding a
// mapping must not silently add an earlier loader check to an existing resource.
type referenceValidationRegistration struct {
	phase      int
	order      int
	parent     ResourceType
	parentType reflect.Type
	visit      func(Resource, func(Resource) error) error
}

func withReferenceValidation(phase int) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if ops.referenceValidation != nil {
			return fmt.Errorf("reference validation is already registered")
		}
		if phase <= 0 || !reflect.PointerTo(ops.explain.typ).Implements(reflect.TypeFor[ReferenceMapping]()) {
			return fmt.Errorf("reference validation requires a positive phase and reference mappings")
		}
		ops.referenceValidation = &referenceValidationRegistration{phase: phase}
		return nil
	}
}

// Nested visits at one phase run within each parent, before advancing to the
// next parent. Flattened validation remains an independently ordered visit.
func withNestedReferenceValidation[P, R any, PPtr interface {
	*P
	Resource
}, RPtr interface {
	*R
	Resource
	ReferenceMapping
}](phase, order int, nested func(*P) *[]R) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if ops.nestedReferenceValidation != nil {
			return fmt.Errorf("nested reference validation is already registered")
		}
		if phase <= 0 || order <= 0 || nested == nil || ops.explain.typ != reflect.TypeFor[R]() {
			return fmt.Errorf("nested reference validation requires positive phase/order and matching typed accessor")
		}
		ops.nestedReferenceValidation = &referenceValidationRegistration{
			phase: phase, order: order, parent: PPtr(new(P)).GetType(), parentType: reflect.TypeFor[P](),
			visit: func(parent Resource, validate func(Resource) error) error {
				children := *nested(any(parent).(*P))
				for i := range children {
					if err := validate(RPtr(&children[i])); err != nil {
						return err
					}
				}
				return nil
			},
		}
		return nil
	}
}

type referenceValidationStep struct {
	kind ResourceType
	*referenceValidationRegistration
}

var referenceValidationSteps = sync.OnceValue(buildReferenceValidationSteps)

func buildReferenceValidationSteps() []referenceValidationStep {
	var steps []referenceValidationStep
	kinds := RegisteredTypes()
	slices.Sort(kinds)
	for _, kind := range kinds {
		ops := registry[kind]
		if ops.referenceValidation != nil {
			steps = append(steps, referenceValidationStep{kind, ops.referenceValidation})
		}
		if nested := ops.nestedReferenceValidation; nested != nil {
			parent, exists := registry[nested.parent]
			if !exists || parent.explain.typ != nested.parentType {
				panic("nested reference validation requires a matching registered parent: " + string(kind))
			}
			steps = append(steps, referenceValidationStep{kind, nested})
		}
	}
	slices.SortFunc(steps, func(a, b referenceValidationStep) int {
		return cmp.Or(cmp.Compare(a.phase, b.phase), cmp.Compare(a.order, b.order), cmp.Compare(a.kind, b.kind))
	})
	for i := 1; i < len(steps); i++ {
		previous, current := steps[i-1], steps[i]
		if previous.phase != current.phase {
			continue
		}
		if previous.parent == "" || previous.parent != current.parent || previous.order == current.order {
			panic(fmt.Sprintf("conflicting reference validation position for %s and %s", previous.kind, current.kind))
		}
	}
	return steps
}

// ValidateRegisteredReferences visits only explicitly enrolled resources in
// diagnostic order. The caller retains reference-field validation policy.
func (rs *ResourceSet) ValidateRegisteredReferences(validate func(Resource) error) error {
	steps := referenceValidationSteps()
	for i := 0; i < len(steps); {
		step := steps[i]
		end := i + 1
		kind := step.kind
		visit := validate
		if step.parent != "" {
			kind = step.parent
			for end < len(steps) && steps[end].phase == step.phase {
				end++
			}
			visit = func(parent Resource) error {
				for _, child := range steps[i:end] {
					if err := child.visit(parent, validate); err != nil {
						return err
					}
				}
				return nil
			}
		}
		var err error
		registry[kind].forEach(rs, func(resource Resource) bool {
			err = visit(resource)
			return err == nil
		})
		if err != nil {
			return err
		}
		i = end
	}
	return nil
}

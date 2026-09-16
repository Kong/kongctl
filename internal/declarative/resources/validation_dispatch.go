package resources

import (
	"cmp"
	"fmt"
	"slices"
	"sync"
)

type collectionValidationStep struct {
	kind     ResourceType
	phase    int
	order    int
	validate func(*ResourceSet) error
}

func withChildValidationPhase(phase int) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if phase <= 0 || ops.childValidationPhase != 0 {
			return fmt.Errorf("child validation requires one positive phase")
		}
		ops.childValidationPhase = phase
		return nil
	}
}

// Resolve family dependencies after init, independently of resource file order.
var collectionValidationSteps = sync.OnceValue(buildCollectionValidationSteps)

func buildCollectionValidationSteps() []collectionValidationStep {
	var steps []collectionValidationStep
	for _, kind := range RegisteredTypes() {
		ops := registry[kind]
		if validation := ops.collectionValidation; validation != nil {
			steps = append(steps, collectionValidationStep{
				kind: kind, phase: validation.phase, validate: validation.validate,
			})
		}
		if load := ops.load; load != nil && load.validate != nil {
			phase := load.validatePhase
			if phase == 0 {
				phase = registry[load.family].childValidationPhase
				if selector, exists := selectorLoaders[load.family]; exists {
					phase = selector.validationPhase
				}
			}
			if phase <= 0 {
				panic("child validation family has no registered phase: " + string(load.family))
			}
			steps = append(steps, collectionValidationStep{
				kind: kind, phase: phase, order: load.validateOrder, validate: load.validate,
			})
		}
	}
	for kind, selector := range selectorLoaders {
		if selector.validationPhase <= 0 {
			panic("selector validation has no registered phase: " + string(kind))
		}
		steps = append(steps, collectionValidationStep{
			kind: kind, phase: selector.validationPhase, validate: selector.validate,
		})
	}
	slices.SortFunc(steps, func(a, b collectionValidationStep) int {
		return cmp.Or(cmp.Compare(a.phase, b.phase), cmp.Compare(a.order, b.order))
	})
	for i := 1; i < len(steps); i++ {
		previous, current := steps[i-1], steps[i]
		if previous.phase == current.phase && previous.order == current.order {
			panic(fmt.Sprintf("duplicate collection validation position for %s and %s", previous.kind, current.kind))
		}
	}
	return steps
}

// ValidateRegisteredCollections runs root, child, and selector collection rules
// in diagnostic order. Normalization, cross-references, and namespaces remain
// loader phases outside this pass; nested API checks run within their root.
func (rs *ResourceSet) ValidateRegisteredCollections() error {
	for _, step := range collectionValidationSteps() {
		if err := step.validate(rs); err != nil {
			return err
		}
	}
	return nil
}

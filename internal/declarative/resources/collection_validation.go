package resources

import (
	"fmt"
	"reflect"
	"strings"
)

// A fresh visitor owns collection-wide uniqueness state for one validation pass.
type collectionValidationRegistration struct {
	phase        int
	newValidator func(*ResourceSet) func(Resource) error
	validate     func(*ResourceSet) error
}

func withCollectionValidation[R any](
	phase int,
	newValidator func(*ResourceSet) func(*R) error,
) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if ops.collectionValidation != nil || ops.collectionValidationOmittedReason != "" {
			return fmt.Errorf("collection validation is already registered")
		}
		if phase <= 0 || newValidator == nil || ops.explain.typ != reflect.TypeFor[R]() {
			return fmt.Errorf("collection validation requires a positive phase and matching typed validator")
		}
		factory := func(rs *ResourceSet) func(Resource) error {
			validate := newValidator(rs)
			return func(resource Resource) error { return validate(any(resource).(*R)) }
		}
		ops.collectionValidation = &collectionValidationRegistration{
			phase:        phase,
			newValidator: factory,
			validate: func(rs *ResourceSet) error {
				validate := factory(rs)
				var err error
				ops.forEach(rs, func(resource Resource) bool {
					err = validate(resource)
					return err == nil
				})
				return err
			},
		}
		return nil
	}
}

func withCollectionValidationOmitted(reason string) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		if ops.collectionValidation != nil || ops.collectionValidationOmittedReason != "" {
			return fmt.Errorf("collection validation is already registered")
		}
		if strings.TrimSpace(reason) == "" {
			return fmt.Errorf("omitted collection validation requires a reason")
		}
		ops.collectionValidationOmittedReason = reason
		return nil
	}
}

// ValidateResourceCollection applies a kind's registered collection rules to an
// explicit slice. Retained loader entry points use it without duplicating policy.
func ValidateResourceCollection[R any, RPtr interface {
	*R
	Resource
}](rs *ResourceSet, values []R) error {
	kind := RPtr(new(R)).GetType()
	registration := registry[kind].collectionValidation
	if registration == nil {
		return fmt.Errorf("resource type %s has no registered collection validation", kind)
	}
	validate := registration.newValidator(rs)
	for i := range values {
		if err := validate(RPtr(&values[i])); err != nil {
			return err
		}
	}
	return nil
}

// namedCollectionValidation preserves validation/ref/name precedence for roots
// with type-wide name uniqueness. Namespace-aware and ref-unique rules stay local.
func namedCollectionValidation[R any, RPtr interface {
	*R
	Resource
}](name func(*R) string) func(*ResourceSet) func(*R) error {
	return func(rs *ResourceSet) func(*R) error {
		names := make(map[string]string)
		return func(value *R) error {
			resource := RPtr(value)
			if err := validateCollectionResource(rs, resource); err != nil {
				return err
			}
			moniker := name(value)
			if existingRef, exists := names[moniker]; exists {
				return fmt.Errorf("duplicate %s name '%s' (ref: %s conflicts with ref: %s)",
					resource.GetType(), moniker, resource.GetRef(), existingRef)
			}
			names[moniker] = resource.GetRef()
			return nil
		}
	}
}

func validateCollectionResource(rs *ResourceSet, resource Resource) error {
	if err := resource.Validate(); err != nil {
		return fmt.Errorf("invalid %s %q: %w", resource.GetType(), resource.GetRef(), err)
	}
	if existing, found := rs.GetResourceByRef(resource.GetRef()); found && existing.GetType() != resource.GetType() {
		return fmt.Errorf("duplicate ref '%s' (already defined as %s)", resource.GetRef(), existing.GetType())
	}
	return nil
}

func validateNestedSlice[R interface {
	Validate() error
	GetRef() string
}](items []R, label string) error {
	refs := make(map[string]bool, len(items))
	for i, item := range items {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("invalid %s %d: %w", label, i, err)
		}
		if refs[item.GetRef()] {
			return fmt.Errorf("duplicate %s ref: %s", label, item.GetRef())
		}
		refs[item.GetRef()] = true
	}
	return nil
}

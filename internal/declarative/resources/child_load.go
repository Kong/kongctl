package resources

import (
	"cmp"
	"fmt"
	"slices"
)

// childLoad pairs typed nested extraction with resource-set validation.
// Exceptional sources supply extract instead of nested/setParent/beforeAppend.
// Kinds without loader validation must record why that phase is omitted.
type childLoad[R, P any] struct {
	family                  ResourceType
	extractOrder            int
	validateOrder           int
	nested                  func(*P) *[]R
	setParent               func(*R, string)
	beforeAppend            func(*ResourceSet, *R)
	validate                func(*ResourceSet, []R) error
	validateNested          func(*ResourceSet, *P) error
	extract                 func(*ResourceSet, *P, *[]R)
	validationOmittedReason string
}

type childLoadRegistration struct {
	kind                    ResourceType
	parent                  ResourceType
	family                  ResourceType
	extractOrder            int
	validateOrder           int
	extract                 func(*ResourceSet, Resource)
	validate                func(*ResourceSet) error
	validateNested          func(*ResourceSet, Resource) error
	validationOmittedReason string
}

var (
	childExtractors = make(map[ResourceType][]childLoadRegistration)
	childValidators = make(map[ResourceType][]childLoadRegistration)
)

func registerChildResourceType[R, P any, RPtr interface {
	*R
	Resource
}, PPtr interface {
	*P
	Resource
}](
	rt ResourceType,
	storage func(*ResourceSet) *[]R,
	explain ExplainRegistration,
	load childLoad[R, P],
	options ...ResourceRegistrationOption,
) {
	if load.extract != nil {
		if load.nested != nil || load.setParent != nil || load.beforeAppend != nil {
			panic("register resource type " + string(rt) + ": custom extraction cannot also supply slice extraction")
		}
	} else if load.nested == nil || load.setParent == nil {
		panic("register resource type " + string(rt) + ": child loader requires extraction")
	}
	capability := ResourceRegistrationOption(func(ops *resourceOps) error {
		if ops.load != nil {
			return fmt.Errorf("child loader is already registered")
		}
		registration := &childLoadRegistration{
			parent:                  PPtr(new(P)).GetType(),
			family:                  load.family,
			extractOrder:            load.extractOrder,
			validateOrder:           load.validateOrder,
			validationOmittedReason: load.validationOmittedReason,
		}
		if load.extract != nil {
			registration.extract = func(rs *ResourceSet, parent Resource) {
				load.extract(rs, any(parent).(*P), storage(rs))
			}
		} else {
			registration.extract = func(rs *ResourceSet, parent Resource) {
				nested := load.nested(any(parent).(*P))
				for _, child := range *nested {
					load.setParent(&child, parent.GetRef())
					if load.beforeAppend != nil {
						load.beforeAppend(rs, &child)
					}
					destination := storage(rs)
					*destination = append(*destination, child)
				}
				*nested = nil
			}
		}
		if load.validate != nil {
			registration.validate = func(rs *ResourceSet) error {
				return load.validate(rs, *storage(rs))
			}
		}
		if load.validateNested != nil {
			registration.validateNested = func(rs *ResourceSet, parent Resource) error {
				return load.validateNested(rs, any(parent).(*P))
			}
		}
		ops.load = registration
		return nil
	})
	registerResourceType[R, RPtr](rt, storage, explain, append(options, capability)...)
}

func registerChildLoader(kind ResourceType, registration childLoadRegistration) {
	if registration.parent == "" || registration.family == "" ||
		registration.extractOrder <= 0 || registration.extract == nil {
		panic("child loader requires parent, family, and positive extraction order: " + string(kind))
	}
	if registration.validate == nil {
		if registration.validationOmittedReason == "" || registration.validateOrder != 0 {
			panic(
				"child loader without validation requires an omission reason and no validation order: " + string(kind),
			)
		}
	} else if registration.validateOrder <= 0 || registration.validationOmittedReason != "" {
		panic("child validation requires a positive order and no omission reason: " + string(kind))
	}
	for _, existing := range childExtractors[registration.parent] {
		if existing.kind == kind || existing.extractOrder == registration.extractOrder {
			panic("duplicate child loader resource type or extraction order: " + string(kind))
		}
	}
	if registration.validate != nil {
		for _, existing := range childValidators[registration.family] {
			if existing.kind == kind || existing.validateOrder == registration.validateOrder {
				panic("duplicate child loader resource type or validation order: " + string(kind))
			}
		}
	}
	registration.kind = kind
	childExtractors[registration.parent] = append(childExtractors[registration.parent], registration)
	slices.SortFunc(childExtractors[registration.parent], func(a, b childLoadRegistration) int {
		return cmp.Compare(a.extractOrder, b.extractOrder)
	})
	if registration.validate != nil {
		childValidators[registration.family] = append(childValidators[registration.family], registration)
		slices.SortFunc(childValidators[registration.family], func(a, b childLoadRegistration) int {
			return cmp.Compare(a.validateOrder, b.validateOrder)
		})
	}
}

// ExtractRegisteredChildren applies a parent's registered child loaders.
// Ordinary collections append to root storage and clear their nested fields;
// custom handlers may retain normalized children. Callers own phase ordering.
func (rs *ResourceSet) ExtractRegisteredChildren(parent Resource) {
	for _, registration := range childExtractors[parent.GetType()] {
		registration.extract(rs, parent)
	}
}

// ValidateRegisteredNestedChildren runs optional per-parent validation in
// extraction order. Callers choose its phase independently of root validation.
func (rs *ResourceSet) ValidateRegisteredNestedChildren(parent Resource) error {
	for _, registration := range childExtractors[parent.GetType()] {
		if registration.validateNested != nil {
			if err := registration.validateNested(rs, parent); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateRegisteredChildren validates a family's flattened child collections
// in registration order, stopping at the first error. Validation families can
// include grandchildren without changing their extraction parent.
func (rs *ResourceSet) ValidateRegisteredChildren(family ResourceType) error {
	for _, registration := range childValidators[family] {
		if err := rs.ValidateRegisteredResource(registration.kind); err != nil {
			return err
		}
	}
	return nil
}

// ValidateRegisteredResource runs the load validation for one registered kind.
func (rs *ResourceSet) ValidateRegisteredResource(kind ResourceType) error {
	load := registry[kind].load
	if load == nil || load.validate == nil {
		return fmt.Errorf("resource type %s has no registered load validation", kind)
	}
	return load.validate(rs)
}

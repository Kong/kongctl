package resources

import "fmt"

// selectorLoader binds assignment registration to a typed selector source.
// Selectors own assignments without joining the managed resource registry.
type selectorLoader[P any] struct {
	kind ResourceType
}

type selectorLoadRegistration struct {
	extract         func(*ResourceSet)
	validate        func(*ResourceSet) error
	validationPhase int
}

var selectorLoaders = make(map[ResourceType]selectorLoadRegistration)

func (loader selectorLoader[P]) withValidationPhase(phase int) selectorLoader[P] {
	registration, exists := selectorLoaders[loader.kind]
	if !exists || phase <= 0 || registration.validationPhase != 0 {
		panic("selector validation requires one positive phase: " + string(loader.kind))
	}
	registration.validationPhase = phase
	selectorLoaders[loader.kind] = registration
	return loader
}

func registerSelectorLoader[P any](
	kind ResourceType,
	source func(*ResourceSet) []P,
	validate func(*ResourceSet) error,
) selectorLoader[P] {
	if kind == "" || source == nil || validate == nil {
		panic("selector loader requires kind, source, and validation")
	}
	if _, exists := selectorLoaders[kind]; exists {
		panic("duplicate selector loader: " + string(kind))
	}
	selectorLoaders[kind] = selectorLoadRegistration{
		extract: func(rs *ResourceSet) {
			parents := source(rs)
			for i := range parents {
				for _, child := range childExtractors[kind] {
					child.extractSelector(rs, &parents[i])
				}
			}
		},
		validate: validate,
	}
	return selectorLoader[P]{kind: kind}
}

func registerSelectorChildResourceType[R, P any, RPtr interface {
	*R
	Resource
}, PPtr interface {
	*P
	GetRef() string
}](
	selector selectorLoader[P],
	rt ResourceType,
	storage func(*ResourceSet) *[]R,
	explain ExplainRegistration,
	load childLoad[R, P],
	options ...ResourceRegistrationOption,
) {
	if _, exists := selectorLoaders[selector.kind]; !exists || load.family != selector.kind {
		panic("selector child loader requires its registered selector family: " + string(rt))
	}
	if load.validateNested != nil {
		panic("selector child loader validates selectors through their owner registration: " + string(rt))
	}
	registration := load.registration(rt, selector.kind, storage)
	registration.extractSelector = func(rs *ResourceSet, parent any) {
		typed := parent.(*P)
		load.extractInto(rs, typed, storage, PPtr(typed).GetRef())
	}
	registerResourceType[R, RPtr](rt, storage, explain, append(options, withChildLoadRegistration(registration))...)
}

// ExtractRegisteredSelectorChildren extracts assignments in selector source
// order, then child extraction order, leaving the selectors in their group.
func (rs *ResourceSet) ExtractRegisteredSelectorChildren(kind ResourceType) {
	selector, exists := selectorLoaders[kind]
	if !exists {
		panic("resource type " + string(kind) + " has no selector loader")
	}
	selector.extract(rs)
}

// ValidateRegisteredSelectorChildren validates selectors before their flattened
// assignment collections. Callers retain sequencing between selector families.
func (rs *ResourceSet) ValidateRegisteredSelectorChildren(kind ResourceType) error {
	selector, exists := selectorLoaders[kind]
	if !exists {
		return fmt.Errorf("resource type %s has no selector loader", kind)
	}
	if err := selector.validate(rs); err != nil {
		return err
	}
	return rs.ValidateRegisteredChildren(kind)
}

func (rs *ResourceSet) hasRegisteredSelectorAssignments(kind ResourceType) bool {
	for _, child := range childExtractors[kind] {
		if registry[child.kind].count(rs) > 0 {
			return true
		}
	}
	return false
}

func selectorRefs[P interface{ GetRef() string }](selectors []P) map[string]bool {
	refs := make(map[string]bool, len(selectors))
	for _, selector := range selectors {
		refs[selector.GetRef()] = true
	}
	return refs
}

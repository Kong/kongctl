package resources

import "fmt"

func apiChildLoad[R any, RPtr interface {
	*R
	Resource
}](
	order int,
	nested func(*APIResource) *[]R,
	setParent func(*R, string),
) childLoad[R, APIResource] {
	return childLoad[R, APIResource]{
		family:         ResourceTypeAPI,
		extractOrder:   order,
		validateOrder:  order,
		nested:         nested,
		setParent:      setParent,
		validate:       validateChildRefs[R, RPtr],
		validateNested: validateNestedAPIChildren[R, RPtr](nested),
	}
}

func validateNestedAPIChildren[R any, RPtr interface {
	*R
	Resource
}](nested func(*APIResource) *[]R) func(*ResourceSet, *APIResource) error {
	return func(rs *ResourceSet, api *APIResource) error {
		children := *nested(api)
		for i := range children {
			child := RPtr(&children[i])
			if err := child.Validate(); err != nil {
				return fmt.Errorf("invalid %s %q in api %q: %w", child.GetType(), child.GetRef(), api.GetRef(), err)
			}
			if existing, found := rs.GetResourceByRef(child.GetRef()); found {
				if existing.GetType() != child.GetType() {
					return fmt.Errorf("duplicate ref '%s' (already defined as %s)", child.GetRef(), existing.GetType())
				}
			}
		}
		return nil
	}
}

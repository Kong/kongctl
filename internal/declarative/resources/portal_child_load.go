package resources

import "fmt"

func extractPortalSingleton[R any](
	nested func(*PortalResource) **R,
	setParent func(*R, string),
) func(*ResourceSet, *PortalResource, *[]R) {
	if nested == nil || setParent == nil {
		panic("portal singleton extraction requires a source and parent setter")
	}
	return func(_ *ResourceSet, portal *PortalResource, destination *[]R) {
		source := nested(portal)
		if *source == nil {
			return
		}
		child := **source
		setParent(&child, portal.Ref)
		*destination = append(*destination, child)
		*source = nil
	}
}

// Validate each resource before checking its later siblings, preserving the
// loader's precedence between invalid resources and duplicate references.
func validatePortalChildRefs[R any, RPtr interface {
	*R
	Resource
}](_ *ResourceSet, children []R) error {
	for i := range children {
		child := RPtr(&children[i])
		if err := child.Validate(); err != nil {
			return fmt.Errorf("invalid %s %q: %w", child.GetType(), child.GetRef(), err)
		}
		for j := i + 1; j < len(children); j++ {
			if RPtr(&children[j]).GetRef() == child.GetRef() {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)", child.GetRef(), child.GetType())
			}
		}
	}
	return nil
}

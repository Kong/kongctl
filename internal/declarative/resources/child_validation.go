package resources

import "fmt"

// Validate each resource before checking its later siblings, preserving the
// loader's precedence between invalid resources and duplicate references.
func validateChildRefs[R any, RPtr interface {
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

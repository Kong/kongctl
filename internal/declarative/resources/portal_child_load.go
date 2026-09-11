package resources

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

func validatePortalChildRefs[R any, RPtr interface {
	*R
	Resource
}](rs *ResourceSet, children []R) error {
	return validateChildRefs[R, RPtr](rs, children)
}

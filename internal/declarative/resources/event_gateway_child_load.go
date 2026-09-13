package resources

// Event Gateway extraction retains its existing phase without introducing
// family-level loader validation.
func eventGatewayChildLoad[R any](
	order int,
	nested func(*EventGatewayControlPlaneResource) *[]R,
	setParent func(*R, string),
) childLoad[R, EventGatewayControlPlaneResource] {
	return childLoad[R, EventGatewayControlPlaneResource]{
		family:                  ResourceTypeEventGatewayControlPlane,
		extractOrder:            order,
		nested:                  nested,
		setParent:               setParent,
		validationOmittedReason: "Event Gateway children retain loading without family-level resource validation",
	}
}

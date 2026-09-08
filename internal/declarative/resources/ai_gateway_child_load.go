package resources

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/tags"
)

// aiGatewayChildLoad preserves the independent extraction and diagnostic order
// of direct AI Gateway children. Their API moniker is unique within a gateway.
type aiGatewayChildLoad[R any] struct {
	extractOrder  int
	validateOrder int
	nested        func(*AIGatewayResource) *[]R
	setParent     func(*R, string)
	uniqueField   string
	beforeAppend  func(*ResourceSet, *R)
}

func registerAIGatewayChildResource[R any, RPtr interface {
	*R
	ResourceWithParent
}](
	rt ResourceType,
	storage func(*ResourceSet) *[]R,
	explain ExplainRegistration,
	load aiGatewayChildLoad[R],
	options ...ResourceRegistrationOption,
) {
	uniqueField := load.uniqueField
	if uniqueField == "" {
		uniqueField = SchemaFieldName
	}
	registerChildResourceType[R, AIGatewayResource, RPtr](rt, storage, explain, childLoad[R, AIGatewayResource]{
		family:        ResourceTypeAIGateway,
		extractOrder:  load.extractOrder,
		validateOrder: load.validateOrder,
		nested:        load.nested,
		setParent:     load.setParent,
		beforeAppend:  load.beforeAppend,
		validate: func(rs *ResourceSet, children []R) error {
			return validateAIGatewayChildren[R, RPtr](rs, children, uniqueField)
		},
	}, options...)
}

type aiGatewayChildResource[T any] interface {
	*T
	ResourceWithParent
}

func validateAIGatewayChildren[T any, P aiGatewayChildResource[T]](
	rs *ResourceSet,
	children []T,
	uniqueFieldLabel string,
) error {
	namesByGateway := make(map[string]string)

	for i := range children {
		childPtr := &children[i]
		child := P(childPtr)
		resourceType := child.GetType()
		resourceLabel := string(resourceType)

		if err := child.Validate(); err != nil {
			return fmt.Errorf("invalid %s %q: %w", resourceLabel, child.GetRef(), err)
		}

		if existing, found := rs.GetResourceByRef(child.GetRef()); found {
			if existing.GetType() != resourceType {
				return fmt.Errorf("duplicate ref '%s' (already defined as %s)",
					child.GetRef(), existing.GetType())
			}
		}

		parentRef := child.GetParentRef()
		if parentRef == nil || parentRef.Ref == "" {
			return fmt.Errorf("%s %q must specify ai_gateway", resourceLabel, child.GetRef())
		}

		gatewayRef := NormalizeResourceRef(parentRef.Ref)
		if tags.IsExternalPlaceholder(parentRef.Ref) {
			lookup, ok := tags.ParseExternalPlaceholder(parentRef.Ref)
			if !ok {
				return fmt.Errorf(
					"%s %q has invalid external lookup placeholder in ai_gateway",
					resourceLabel,
					child.GetRef(),
				)
			}
			gatewayRef = "external:" + tags.ExternalLookupKey(lookup.MatchFields)
		} else {
			gateway, found := rs.GetResourceByRef(gatewayRef)
			if !found {
				return fmt.Errorf(
					"%s %q references unknown ai_gateway %q",
					resourceLabel,
					child.GetRef(),
					parentRef.Ref,
				)
			}
			if gateway.GetType() != ResourceTypeAIGateway {
				return fmt.Errorf(
					"%s %q references %q which is %s, not ai_gateway",
					resourceLabel,
					child.GetRef(),
					parentRef.Ref,
					gateway.GetType(),
				)
			}
		}

		value := child.GetMoniker()
		nameKey := gatewayRef + "\x00" + value
		if existingRef, exists := namesByGateway[nameKey]; exists {
			return fmt.Errorf(
				"duplicate %s %s %q for ai_gateway %q (ref: %s conflicts with ref: %s)",
				resourceLabel,
				uniqueFieldLabel,
				value,
				gatewayRef,
				child.GetRef(),
				existingRef,
			)
		}
		namesByGateway[nameKey] = child.GetRef()
	}

	return nil
}

package planner

import (
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func firstResourceSet(sets []*resources.ResourceSet) *resources.ResourceSet {
	if len(sets) == 0 {
		return nil
	}
	return sets[0]
}

// Compare name references against API names while retaining placeholders in the
// actual plan, where the relationship resolver owns validation and dependencies.
func normalizeAIGateway23References(
	payload map[string]any,
	rs *resources.ResourceSet,
	gatewayRef string,
) {
	if rs == nil {
		return
	}
	gatewayIDs := aiGateway23GatewayIDs(rs)
	gatewayRef = resources.NormalizeResourceRef(gatewayRef)
	resolve := func(value string) string {
		ref, _, ok := tags.ParseRefPlaceholder(value)
		if !ok {
			return value
		}
		resource, found := rs.GetResourceByRef(ref)
		if !found {
			return value
		}
		child, ok := resource.(resources.ResourceWithParent)
		if !ok || child.GetParentRef() == nil {
			return value
		}
		parentRef := resources.NormalizeResourceRef(child.GetParentRef().Ref)
		if !aiGateway23SameGateway(gatewayIDs, parentRef, gatewayRef) {
			return value
		}
		return resource.GetMoniker()
	}
	stores, _ := payload[FieldDatastores].([]any)
	for _, raw := range stores {
		if store, ok := raw.(map[string]any); ok {
			if name, ok := store[FieldName].(string); ok {
				store[FieldName] = resolve(name)
			}
		}
	}
	config, _ := payload[FieldConfig].(map[string]any)
	for _, field := range []string{FieldCertificates, FieldCACertificates} {
		if values, ok := config[field].([]any); ok {
			for i, raw := range values {
				if name, ok := raw.(string); ok {
					values[i] = resolve(name)
				}
			}
		}
	}
}

func aiGateway23GatewayIDs(rs *resources.ResourceSet) map[string]string {
	ids := make(map[string]string)
	if rs != nil {
		for _, gateway := range rs.AIGateways {
			ids[resources.NormalizeResourceRef(gateway.Ref)] = gateway.GetKonnectID()
		}
	}
	return ids
}

func aiGateway23SameGateway(ids map[string]string, first, second string) bool {
	first, second = resources.NormalizeResourceRef(first), resources.NormalizeResourceRef(second)
	return first == second || (ids[first] != "" && ids[first] == ids[second])
}

package planner

import (
	"fmt"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// Add semantic replacement edges before the resolver serializes gateway siblings.
// Requiring every matching create also protects ambiguous many-to-many renames.
func addAIGatewayMCPReplacementDependencies(
	namespace, gatewayRef, gatewayID string,
	current []state.AIGatewayMCPServer,
	plan *Plan,
) error {
	matchesScope := func(change PlannedChange) bool {
		return change.ResourceType == ResourceTypeAIGatewayMCPServer &&
			change.Namespace == namespace && change.Parent != nil &&
			change.Parent.ID == gatewayID &&
			aiGatewayChildChangeMatchesParent(change, gatewayRef)
	}
	for i := range plan.Changes {
		deletion := &plan.Changes[i]
		if !matchesScope(*deletion) || deletion.Action != ActionDelete {
			continue
		}
		for _, server := range current {
			if resources.AIGatewayMCPServerID(server.AIGatewayMCPServer) != deletion.ResourceID {
				continue
			}
			payload, err := resources.AIGatewayMCPServerMutablePayloadMap(server.AIGatewayMCPServer)
			if err != nil {
				return fmt.Errorf("inspect MCP server %q for route replacement: %w", deletion.ResourceRef, err)
			}
			for _, creation := range plan.Changes {
				if !matchesScope(creation) || creation.Action != ActionCreate {
					continue
				}
				if aiGatewayMCPSameRoute(payload, creation.Fields) {
					deletion.DependsOn = appendDependsOn(deletion.DependsOn, creation.ID)
				}
			}
		}
	}
	return nil
}

func aiGatewayMCPSameRoute(old, replacement map[string]any) bool {
	oldRoute := aiGatewayMCPServingRoute(old)
	newRoute := aiGatewayMCPServingRoute(replacement)
	if len(oldRoute) == 0 || len(newRoute) == 0 {
		return false
	}
	oldRoute, newRoute = normalizeAIGatewayPayloadsForComparison(oldRoute, newRoute)
	return reflect.DeepEqual(normalizeAIGatewayMCPRouteSets(oldRoute), normalizeAIGatewayMCPRouteSets(newRoute))
}

func aiGatewayMCPServingRoute(payload map[string]any) map[string]any {
	if boolValueEqual(payload[FieldEnabled], false) {
		return nil
	}
	switch payload[FieldType] {
	case "listener", "conversion-listener", "passthrough-listener":
	default:
		// conversion-only and upstream-server are sources, without a serving route.
		return nil
	}
	config, _ := payload[FieldConfig].(map[string]any)
	route, _ := config[FieldRoute].(map[string]any)
	return route
}

// Route matcher lists (including header values) are sets. Preserve case and
// exact path/regex values; overlapping but unequal matchers are not replacements.
func normalizeAIGatewayMCPRouteSets(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = normalizeAIGatewayMCPRouteSets(child)
		}
	case []any:
		return sortAIGatewayStringSlice(typed)
	}
	return value
}

package planner

import (
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// CurrentPlanVersion is the only plan format version accepted by this build.
// Plans are intentionally rejected rather than migrated when their serialized
// contract is not compatible with the current planner-to-executor boundary.
const CurrentPlanVersion = "1.0"

// ValidatePlanCompatibility validates the portion of the serialized plan
// contract that is independent of executor SDK request types.
func ValidatePlanCompatibility(plan *Plan) error {
	if plan == nil {
		return fmt.Errorf("invalid plan: plan is required")
	}
	if plan.Metadata.Version == "" {
		return fmt.Errorf("invalid plan: missing version")
	}
	if plan.Metadata.Version != CurrentPlanVersion {
		return fmt.Errorf(
			"plan version %q is not supported by this kongctl version; regenerate the plan",
			plan.Metadata.Version,
		)
	}
	if err := validatePlanMode(plan.Metadata.Mode); err != nil {
		return err
	}

	for i := range plan.Changes {
		if err := validateChangeCompatibility(&plan.Changes[i]); err != nil {
			return fmt.Errorf("incompatible plan change %d (%q): %w; regenerate the plan", i, plan.Changes[i].ID, err)
		}
	}
	return nil
}

func validatePlanMode(mode PlanMode) error {
	switch mode {
	case PlanModeSync, PlanModeApply, PlanModeDelete:
		return nil
	case "":
		return fmt.Errorf("invalid plan: missing mode")
	default:
		return fmt.Errorf("invalid plan: unsupported mode %q", mode)
	}
}

func validateChangeCompatibility(change *PlannedChange) error {
	if change == nil {
		return fmt.Errorf("change is required")
	}
	if strings.TrimSpace(change.ID) == "" {
		return fmt.Errorf("missing change ID")
	}
	if strings.TrimSpace(change.ResourceType) == "" {
		return fmt.Errorf("missing resource type")
	}
	switch change.Action {
	case ActionCreate, ActionUpdate, ActionDelete:
		if change.ResourceType == ResourceTypeDeck && change.Action != ActionCreate {
			return fmt.Errorf("action %q is not supported for resource type %q", change.Action, change.ResourceType)
		}
	case ActionExternalTool:
		if change.ResourceType != ResourceTypeDeck {
			return fmt.Errorf("action %q is only supported for resource type %q", change.Action, ResourceTypeDeck)
		}
	default:
		return fmt.Errorf("unsupported action %q", change.Action)
	}

	return validateRoutingBoundary(change)
}

func validateRoutingBoundary(change *PlannedChange) error {
	switch change.ResourceType {
	case ResourceTypeOrganizationUserTeamMembership:
		if hasPayloadPath(change.Fields, FieldUserID) {
			return fmt.Errorf("routing-only field %q must not appear in fields", FieldUserID)
		}
	case ResourceTypeOrganizationSystemAccountTeamMembership:
		if hasPayloadPath(change.Fields, FieldSystemAccountID) {
			return fmt.Errorf("routing-only field %q must not appear in fields", FieldSystemAccountID)
		}
	}

	if change.ResourceType == ResourceTypeAPIPublication {
		if hasPayloadPath(change.Fields, FieldAPIID) || hasPayloadPath(change.Fields, FieldPortalID) {
			return fmt.Errorf("API and portal routing IDs must not appear in fields")
		}
		if _, ok := change.References[FieldPortalID]; !ok {
			return fmt.Errorf("missing portal routing reference")
		}
		if _, ok := change.References[FieldAPIID]; !ok && change.Parent == nil {
			return fmt.Errorf("missing API routing reference or parent")
		}
	}

	descriptors := resources.RelationshipDescriptorsForType(resources.ResourceType(change.ResourceType))
	for _, descriptor := range descriptors {
		if descriptor.Kind == resources.RelationshipKindKongctlParentSelector &&
			hasPayloadPath(change.Fields, descriptor.FieldPath) {
			return fmt.Errorf("routing-only field %q must not appear in fields", descriptor.FieldPath)
		}
		if descriptor.Kind != resources.RelationshipKindKongctlParentSelector {
			continue
		}
		for _, routingField := range routingIDFields(descriptor) {
			if hasPayloadPath(change.Fields, routingField) {
				return fmt.Errorf("routing-only field %q must not appear in fields", routingField)
			}
		}
	}
	return nil
}

func routingIDFields(descriptor resources.RelationshipDescriptor) []string {
	// Only resource types that can be API-routing parents have a corresponding
	// planner routing ID field.
	//exhaustive:ignore
	switch descriptor.TargetType {
	case resources.ResourceTypeAIGateway:
		return []string{FieldAIGatewayID}
	case resources.ResourceTypeAIGatewayConsumer:
		return []string{FieldAIGatewayConsumerID}
	case resources.ResourceTypeControlPlane:
		return []string{FieldControlPlaneID}
	case resources.ResourceTypeEventGatewayControlPlane:
		return []string{FieldEventGatewayID}
	case resources.ResourceTypeEventGatewayListener:
		return []string{FieldEventGatewayListenerID}
	case resources.ResourceTypeEventGatewayVirtualCluster:
		return []string{FieldEventGatewayVirtualClusterID}
	case resources.ResourceTypeOrganizationTeam, resources.ResourceTypePortalTeam:
		return []string{FieldTeamID}
	case resources.ResourceTypePortal:
		return []string{FieldPortalID}
	default:
		return nil
	}
}

func hasPayloadPath(fields map[string]any, path string) bool {
	if len(fields) == 0 || path == "" {
		return false
	}
	current := any(fields)
	for part := range strings.SplitSeq(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = object[part]
		if !ok {
			return false
		}
	}
	return true
}

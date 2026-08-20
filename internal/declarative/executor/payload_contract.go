package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/planner"
)

type payloadContract interface {
	ResourceType() string
	ValidatePayload(context.Context, planner.PlannedChange) error
}

type upsertPayloadContract[TRequest any] struct {
	base *BaseCreateDeleteExecutor[TRequest]
}

func (c upsertPayloadContract[TRequest]) ResourceType() string {
	return c.base.ResourceType()
}

func (c upsertPayloadContract[TRequest]) ValidatePayload(
	ctx context.Context,
	change planner.PlannedChange,
) error {
	switch change.Action {
	case planner.ActionCreate, planner.ActionUpdate:
		execCtx := NewExecutionContext(&change)
		var request TRequest
		if err := c.base.ops.MapCreateFields(ctx, execCtx, change.Fields, &request); err != nil {
			return err
		}
		return validateMappedPayload(c.base.ops.ResourceType(), change.Action, change.Fields, request)
	case planner.ActionDelete:
		return nil
	case planner.ActionExternalTool:
		return fmt.Errorf("action %q is not supported", change.Action)
	default:
		return fmt.Errorf("action %q is not supported", change.Action)
	}
}

type payloadContractKey struct {
	resourceType string
	action       planner.ActionType
}

// translatedPayloadFields records planner fields that are intentionally consumed
// outside the same JSON path in the SDK request. Entries are action-specific so
// adding a planner field still requires an explicit decision about its execution.
var translatedPayloadFields = map[payloadContractKey][]string{
	{planner.ResourceTypeAPI, planner.ActionUpdate}:                    {planner.FieldCurrentLabels},
	{planner.ResourceTypeAIGateway, planner.ActionUpdate}:              {planner.FieldCurrentLabels},
	{planner.ResourceTypeAIGatewayConsumerGroup, planner.ActionCreate}: {planner.FieldConsumers},
	{planner.ResourceTypeAIGatewayConsumerGroup, planner.ActionUpdate}: {planner.FieldConsumers},
	{planner.ResourceTypeApplicationAuthStrategy, planner.ActionUpdate}: {
		planner.FieldCurrentLabels,
		planner.FieldError,
		planner.FieldName,
		planner.FieldStrategyType,
	},
	{planner.ResourceTypeCatalogService, planner.ActionUpdate}: {planner.FieldCurrentLabels},
	{planner.ResourceTypeControlPlane, planner.ActionCreate}:   {planner.FieldMembers},
	{planner.ResourceTypeControlPlane, planner.ActionUpdate}: {
		planner.FieldCurrentLabels,
		planner.FieldMembers,
	},
	{planner.ResourceTypeDashboard, planner.ActionUpdate}: {planner.FieldCurrentLabels},
	{planner.ResourceTypeDCRProvider, planner.ActionUpdate}: {
		planner.FieldCurrentLabels,
		planner.FieldDCRProviderUpdateType,
		planner.FieldError,
		planner.FieldName,
	},
	{planner.ResourceTypeEventGatewayVirtualCluster, planner.ActionCreate}: {planner.FieldDestination + ".name"},
	{planner.ResourceTypeEventGatewayVirtualCluster, planner.ActionUpdate}: {planner.FieldDestination + ".name"},
	{planner.ResourceTypeEventGatewayClusterPolicy, planner.ActionUpdate}:  {planner.FieldCurrentLabels},
	{planner.ResourceTypeEventGatewayConsumePolicy, planner.ActionUpdate}:  {planner.FieldCurrentLabels},
	{planner.ResourceTypeEventGatewayControlPlane, planner.ActionUpdate}:   {planner.FieldCurrentLabels},
	{planner.ResourceTypeEventGatewayProducePolicy, planner.ActionUpdate}:  {planner.FieldCurrentLabels},
	{planner.ResourceTypeOrganizationTeam, planner.ActionUpdate}:           {planner.FieldCurrentLabels},
	{planner.ResourceTypePortal, planner.ActionUpdate}:                     {planner.FieldCurrentLabels},
	{planner.ResourceTypePortalAssetFavicon, planner.ActionUpdate}:         {planner.FieldDataURL},
	{planner.ResourceTypePortalAssetLogo, planner.ActionUpdate}:            {planner.FieldDataURL},
	{planner.ResourceTypePortalEmailTemplate, planner.ActionCreate}: {
		planner.FieldContent,
		planner.FieldEnabled,
		planner.FieldName,
	},
	{planner.ResourceTypePortalEmailTemplate, planner.ActionUpdate}: {
		planner.FieldContent,
		planner.FieldEnabled,
		planner.FieldName,
	},
	{planner.ResourceTypePortalIdentityProvider, planner.ActionCreate}: {planner.FieldConfig + ".type"},
	{planner.ResourceTypePortalIdentityProvider, planner.ActionUpdate}: {planner.FieldConfig + ".type"},
}

func validateMappedPayload(
	resourceType string,
	action planner.ActionType,
	fields map[string]any,
	request any,
) error {
	sourceData, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("failed to encode planner payload: %w", err)
	}
	mappedData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to encode SDK request: %w", err)
	}

	var sourceValue map[string]any
	if err := json.Unmarshal(sourceData, &sourceValue); err != nil {
		return fmt.Errorf("failed to inspect planner payload: %w", err)
	}
	for _, field := range translatedPayloadFields[payloadContractKey{resourceType, action}] {
		removePayloadPath(sourceValue, strings.Split(field, "."))
	}
	var mappedValue any
	if err := json.Unmarshal(mappedData, &mappedValue); err != nil {
		return fmt.Errorf("failed to inspect SDK request: %w", err)
	}

	dropped := droppedPayloadPaths(sourceValue, mappedValue, "")
	if len(dropped) == 0 {
		return nil
	}
	slices.Sort(dropped)
	return fmt.Errorf(
		"%s contains fields not represented by the action-specific SDK request: %s",
		resourceType,
		strings.Join(dropped, ", "),
	)
}

func removePayloadPath(value any, path []string) {
	if len(path) == 0 {
		return
	}
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	if len(path) == 1 {
		delete(object, path[0])
		return
	}
	removePayloadPath(object[path[0]], path[1:])
}

func validationChange(change planner.PlannedChange) planner.PlannedChange {
	result := change
	result.Fields = cloneValidationFields(change.Fields)
	result.References = cloneValidationReferences(change.References)
	for field, reference := range result.References {
		if reference.IsArray {
			if len(reference.ResolvedIDs) < len(reference.Refs) {
				reference.ResolvedIDs = append(
					reference.ResolvedIDs,
					make([]string, len(reference.Refs)-len(reference.ResolvedIDs))...,
				)
			}
			for i := range reference.Refs {
				if reference.ResolvedIDs[i] == "" {
					reference.ResolvedIDs[i] = "payload-validation-id"
				}
			}
			result.References[field] = reference
			continue
		}
		if !reference.HasResolvedID() {
			reference.ID = "payload-validation-id"
		}
		result.References[field] = reference
		setResolvedFieldValue(result.Fields, field, reference.ID)
	}
	if result.Parent != nil && unresolvedReferenceID(result.Parent.ID) {
		parent := *result.Parent
		parent.ID = "payload-validation-id"
		result.Parent = &parent
	}
	return result
}

func cloneValidationReferences(references map[string]planner.ReferenceInfo) map[string]planner.ReferenceInfo {
	if references == nil {
		return nil
	}
	result := make(map[string]planner.ReferenceInfo, len(references))
	for field, reference := range references {
		reference.Refs = slices.Clone(reference.Refs)
		reference.ResolvedIDs = slices.Clone(reference.ResolvedIDs)
		reference.LookupFields = maps.Clone(reference.LookupFields)
		if reference.LookupArrays != nil {
			lookupArrays := make(map[string][]string, len(reference.LookupArrays))
			for name, values := range reference.LookupArrays {
				lookupArrays[name] = slices.Clone(values)
			}
			reference.LookupArrays = lookupArrays
		}
		result[field] = reference
	}
	return result
}

func cloneValidationFields(fields map[string]any) map[string]any {
	if fields == nil {
		return nil
	}
	result := make(map[string]any, len(fields))
	for field, value := range fields {
		result[field] = cloneValidationValue(value)
	}
	return result
}

func cloneValidationValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneValidationFields(typed)
	case []any:
		result := make([]any, len(typed))
		for i := range typed {
			result[i] = cloneValidationValue(typed[i])
		}
		return result
	case planner.FieldChange:
		typed.Old = cloneValidationValue(typed.Old)
		typed.New = cloneValidationValue(typed.New)
		return typed
	default:
		return typed
	}
}

func (e *Executor) registerPayloadContracts(contracts ...payloadContract) {
	for _, contract := range contracts {
		resourceType := contract.ResourceType()
		if _, exists := e.payloadContracts[resourceType]; exists {
			panic("duplicate payload contract for resource type " + resourceType)
		}
		e.payloadContracts[resourceType] = contract
	}
}

func (e *Executor) validatePlanPayloads(ctx context.Context, plan *planner.Plan) error {
	if err := planner.ValidatePlanCompatibility(plan); err != nil {
		return e.withPlanCompatibilityGuidance(err)
	}
	for i := range plan.Changes {
		executionChange, err := cloneChangeForExecution(&plan.Changes[i])
		if err != nil {
			return fmt.Errorf(
				"incompatible plan change %d (%q): failed to normalize payload: %w",
				i,
				plan.Changes[i].ID,
				err,
			)
		}
		change := *executionChange
		if err := e.resolveDeferredEnvPlaceholders(&change); err != nil {
			return fmt.Errorf(
				"incompatible plan change %d (%q): failed to resolve deferred environment values: %w",
				i,
				change.ID,
				err,
			)
		}
		change = validationChange(change)
		if err := injectSecretWriteValidationPlaceholders(&change); err != nil {
			return fmt.Errorf(
				"incompatible plan change %d (%q): %w",
				i,
				change.ID,
				err,
			)
		}
		if change.ResourceType == planner.ResourceTypeDeck {
			continue
		}
		contract, ok := e.payloadContracts[change.ResourceType]
		if !ok {
			// Unsupported resource types retain the existing execution error path.
			// Every resource dispatched by this executor is registered above.
			continue
		}
		if err := contract.ValidatePayload(ctx, change); err != nil {
			err = fmt.Errorf(
				"incompatible plan change %d (%q) for %s %s: %w",
				i,
				change.ID,
				change.Action,
				change.ResourceType,
				err,
			)
			return e.withPlanCompatibilityGuidance(err)
		}
	}
	return nil
}

func (e *Executor) withPlanCompatibilityGuidance(err error) error {
	message := strings.TrimSuffix(err.Error(), "; regenerate the plan")
	if e.planBaseDir != "" {
		return fmt.Errorf("%s; regenerate the plan", message)
	}
	return fmt.Errorf("%s; this is an internal planner-to-executor contract violation", message)
}

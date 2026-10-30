package planner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) planAIGatewayDatastoreChanges(
	ctx context.Context,
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	gatewayChangeID string,
	desired []resources.AIGatewayDatastoreResource,
	plan *Plan,
) error {
	p.logger.Debug(
		"Planning AI Gateway Datastore changes",
		slog.String("gateway_ref", gatewayRef),
		slog.String("gateway_id", gatewayID),
		slog.String("gateway_change_id", gatewayChangeID),
		slog.Int("desired_count", len(desired)),
	)

	if gatewayID == "" {
		p.planAIGatewayDatastoreCreatesForNewGateway(
			namespace,
			gatewayRef,
			gatewayName,
			gatewayChangeID,
			desired,
			plan,
		)
		return nil
	}

	currentDatastores, err := p.client.ListAIGatewayDatastores(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list AI Gateway Datastores for gateway %s: %w", gatewayID, err)
	}

	return reconcileNameMatchedChildren(
		p,
		ResourceTypeAIGatewayDatastore,
		desired,
		currentDatastores,
		nameMatchedChildOperations[resources.AIGatewayDatastoreResource, state.AIGatewayDatastore]{
			desiredName: func(desired resources.AIGatewayDatastoreResource) string {
				return desired.Name()
			},
			currentName: func(current state.AIGatewayDatastore) string {
				return resources.AIGatewayDatastoreName(current.AIGatewayDatastore)
			},
			fetch: func(
				_ resources.AIGatewayDatastoreResource, current state.AIGatewayDatastore,
			) (*state.AIGatewayDatastore, error) {
				return &current, nil
			},
			diff: func(current state.AIGatewayDatastore, desired resources.AIGatewayDatastoreResource) (
				bool, map[string]any, map[string]FieldChange, error,
			) {
				return shouldUpdateAIGatewayDatastore(current, desired, p.resources)
			},
			create: func(desired resources.AIGatewayDatastoreResource) {
				p.planAIGatewayDatastoreCreate(
					namespace,
					gatewayRef,
					gatewayName,
					gatewayID,
					desired,
					nil,
					plan,
				)
			},
			update: func(current state.AIGatewayDatastore, desired resources.AIGatewayDatastoreResource,
				fields map[string]any, changed map[string]FieldChange,
			) {
				p.planAIGatewayDatastoreUpdate(
					namespace,
					gatewayRef,
					gatewayID,
					resources.AIGatewayDatastoreID(
						current.AIGatewayDatastore,
					),
					desired,
					fields,
					changed,
					nil,
					plan,
				)
			},
			protected: func(current state.AIGatewayDatastore) bool {
				return current.KonnectManaged ||
					labels.IsProtectedResource(current.NormalizedLabels)
			},
			remove: func(current state.AIGatewayDatastore) {
				p.planAIGatewayDatastoreDelete(namespace, gatewayRef, gatewayID,
					resources.AIGatewayDatastoreID(current.AIGatewayDatastore),
					resources.AIGatewayDatastoreName(current.AIGatewayDatastore), plan)
			},
		},
		plan,
	)
}

func (p *Planner) planAIGatewayDatastoreCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	datastores []resources.AIGatewayDatastoreResource,
	plan *Plan,
) {
	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}
	for _, datastore := range datastores {
		p.planAIGatewayDatastoreCreate(
			namespace,
			gatewayRef,
			gatewayName,
			"",
			datastore,
			dependsOn,
			plan,
		)
	}
}

func (p *Planner) planAIGatewayDatastoreCreate(
	namespace string,
	gatewayRef string,
	_ string,
	gatewayID string,
	datastore resources.AIGatewayDatastoreResource,
	dependsOn []string,
	plan *Plan,
) {
	fields, err := datastore.MutablePayloadMap()
	if err != nil {
		plan.AddWarning(
			datastore.GetRef(),
			fmt.Sprintf("failed to build AI Gateway Datastore create payload: %s", err),
		)
		return
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeAIGatewayDatastore, datastore.Ref),
		ResourceType: ResourceTypeAIGatewayDatastore,
		ResourceRef:  datastore.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}
	if gatewayID != "" {
		change.Parent = &ParentInfo{Ref: gatewayRef, ID: gatewayID}
	} else {
		change.References = map[string]ReferenceInfo{
			FieldAIGatewayID: {
				Ref: gatewayRef,
				LookupFields: map[string]string{
					FieldName: gatewayRef,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planAIGatewayDatastoreUpdate(
	namespace string,
	gatewayRef string,
	gatewayID string,
	datastoreID string,
	datastore resources.AIGatewayDatastoreResource,
	updateFields map[string]any,
	changedFields map[string]FieldChange,
	dependsOn []string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:            p.nextChangeID(ActionUpdate, ResourceTypeAIGatewayDatastore, datastore.Ref),
		ResourceType:  ResourceTypeAIGatewayDatastore,
		ResourceRef:   datastore.Ref,
		ResourceID:    datastoreID,
		Action:        ActionUpdate,
		Fields:        updateFields,
		ChangedFields: changedFields,
		Namespace:     namespace,
		DependsOn:     dependsOn,
		Parent:        &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func (p *Planner) planAIGatewayDatastoreDelete(
	namespace string,
	gatewayRef string,
	gatewayID string,
	datastoreID string,
	datastoreName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeAIGatewayDatastore, datastoreName),
		ResourceType: ResourceTypeAIGatewayDatastore,
		ResourceRef:  datastoreName,
		ResourceID:   datastoreID,
		Action:       ActionDelete,
		Namespace:    namespace,
		Fields: map[string]any{
			FieldName: datastoreName,
		},
		Parent: &ParentInfo{Ref: gatewayRef, ID: gatewayID},
	}
	plan.AddChange(change)
}

func shouldUpdateAIGatewayDatastore(
	current state.AIGatewayDatastore,
	desired resources.AIGatewayDatastoreResource,
	_ *resources.ResourceSet,
) (bool, map[string]any, map[string]FieldChange, error) {
	currentPayload, err := resources.AIGatewayDatastoreMutablePayloadMap(current.AIGatewayDatastore)
	if err != nil {
		return false, nil, nil, fmt.Errorf(
			"failed to normalize current AI Gateway Datastore: %w",
			err,
		)
	}
	desiredPayload, err := desired.MutablePayloadMap()
	if err != nil {
		return false, nil, nil, fmt.Errorf(
			"failed to normalize desired AI Gateway Datastore %q: %w",
			desired.Ref,
			err,
		)
	}

	currentCompare, desiredCompare := comparableAIGatewayDatastorePayloads(
		currentPayload,
		desiredPayload,
	)

	currentPlanPayload := scrubAIGatewayDatastoreWriteOnlyFields(currentPayload).(map[string]any)
	desiredPlanPayload := scrubAIGatewayDatastoreWriteOnlyFields(desiredPayload).(map[string]any)

	changedFields := diffAIGatewayPayloads(
		currentPlanPayload,
		desiredPlanPayload,
		currentCompare,
		desiredCompare,
	)
	if len(changedFields) == 0 {
		return false, nil, nil, nil
	}

	return true, desiredPayload, changedFields, nil
}

func comparableAIGatewayDatastorePayloads(
	current, desired map[string]any,
) (map[string]any, map[string]any) {
	return comparableAIGatewayWriteOnlyPayloads(
		current,
		desired,
		normalizeAIGatewayPayloadsForComparison,
		isAIGatewayDatastoreWriteOnlyField,
	)
}

func scrubAIGatewayDatastoreWriteOnlyFields(value any) any {
	return scrubAIGatewayWriteOnlyFields(value, isAIGatewayDatastoreWriteOnlyField)
}

func isAIGatewayDatastoreWriteOnlyField(key string) bool {
	switch strings.ToLower(key) {
	case "password",
		"ssl_cert_key",
		"key",
		"sentinel_password",
		"aws_secret_access_key",
		"gcp_service_account_json",
		"azure_client_secret":
		return true
	default:
		return false
	}
}

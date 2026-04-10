package planner

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// planEventGatewaySchemaRegistryChanges plans changes for Event Gateway Schema Registries
// for a specific gateway.
func (p *Planner) planEventGatewaySchemaRegistryChanges(
	ctx context.Context,
	_ *Config,
	namespace string,
	gatewayName string,
	gatewayID string,
	gatewayRef string,
	gatewayChangeID string,
	desired []resources.EventGatewaySchemaRegistryResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning Event Gateway Schema Registry changes",
		"gateway_name", gatewayName,
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"desired_count", len(desired),
		"namespace", namespace,
	)

	if gatewayID != "" {
		return p.planSchemaRegistryChangesForExistingGateway(
			ctx, namespace, gatewayID, gatewayRef, gatewayName, desired, plan,
		)
	}

	// Gateway doesn't exist yet: plan creates only with dependency on gateway creation
	p.planSchemaRegistryCreatesForNewGateway(
		namespace, gatewayRef, gatewayName, gatewayChangeID, desired, plan)
	return nil
}

// planSchemaRegistryChangesForExistingGateway handles full diff for schema registries of
// an existing gateway.
func (p *Planner) planSchemaRegistryChangesForExistingGateway(
	ctx context.Context,
	namespace string,
	gatewayID string,
	gatewayRef string,
	gatewayName string,
	desired []resources.EventGatewaySchemaRegistryResource,
	plan *Plan,
) error {
	p.logger.Debug("Planning changes for existing gateway schema registries",
		"gateway_id", gatewayID,
		"gateway_ref", gatewayRef,
		"desired_count", len(desired),
	)

	currentRegistries, err := p.client.ListEventGatewaySchemaRegistries(ctx, gatewayID)
	if err != nil {
		return fmt.Errorf("failed to list schema registries for gateway %s: %w", gatewayID, err)
	}

	p.logger.Debug("Fetched current schema registries",
		"gateway_id", gatewayID,
		"current_count", len(currentRegistries),
	)

	currentByName := make(map[string]state.EventGatewaySchemaRegistry)
	for _, sr := range currentRegistries {
		currentByName[sr.Name] = sr
	}

	desiredNames := make(map[string]bool)

	for _, desiredSR := range desired {
		name := desiredSR.GetMoniker()
		desiredNames[name] = true

		current, exists := currentByName[name]

		if !exists {
			p.logger.Debug("Planning schema registry CREATE",
				"registry_name", name,
				"gateway_ref", gatewayRef,
			)
			p.planSchemaRegistryCreate(namespace, gatewayRef, gatewayName, gatewayID, desiredSR, []string{}, plan)
		} else {
			p.logger.Debug("Checking if schema registry needs update",
				"registry_name", name,
				"registry_id", current.ID,
			)

			needsUpdate, updateFields := p.shouldUpdateSchemaRegistry(current, desiredSR)
			if needsUpdate {
				p.logger.Debug("Planning schema registry UPDATE",
					"registry_name", name,
					"registry_id", current.ID,
					"update_fields", updateFields,
				)
				p.planSchemaRegistryUpdate(
					namespace, gatewayRef, gatewayName, gatewayID,
					current.ID, desiredSR, updateFields, plan)
			}
		}
	}

	// SYNC MODE: Delete unmanaged registries
	if plan.Metadata.Mode == PlanModeSync {
		for name, current := range currentByName {
			if !desiredNames[name] {
				p.logger.Debug("Planning schema registry DELETE (sync mode)",
					"registry_name", name,
					"registry_id", current.ID,
				)
				p.planSchemaRegistryDelete(gatewayRef, gatewayName, gatewayID, current.ID, name, plan)
			}
		}
	}

	return nil
}

// planSchemaRegistryCreatesForNewGateway plans creates for schema registries when the gateway
// doesn't exist yet.
func (p *Planner) planSchemaRegistryCreatesForNewGateway(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayChangeID string,
	registries []resources.EventGatewaySchemaRegistryResource,
	plan *Plan,
) {
	p.logger.Debug("Planning schema registry creates for new gateway",
		"gateway_ref", gatewayRef,
		"gateway_change_id", gatewayChangeID,
		"registry_count", len(registries),
	)

	var dependsOn []string
	if gatewayChangeID != "" {
		dependsOn = []string{gatewayChangeID}
	}

	for _, sr := range registries {
		p.planSchemaRegistryCreate(namespace, gatewayRef, gatewayName, "", sr, dependsOn, plan)
	}
}

// planSchemaRegistryCreate plans a CREATE change for a schema registry.
func (p *Planner) planSchemaRegistryCreate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	sr resources.EventGatewaySchemaRegistryResource,
	dependsOn []string,
	plan *Plan,
) {
	fields := buildSchemaRegistryFields(sr)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, ResourceTypeEventGatewaySchemaRegistry, sr.Ref),
		ResourceType: ResourceTypeEventGatewaySchemaRegistry,
		ResourceRef:  sr.Ref,
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		DependsOn:    dependsOn,
	}

	if gatewayID != "" {
		change.Parent = &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		}
	} else {
		change.References = map[string]ReferenceInfo{
			"event_gateway_id": {
				Ref: gatewayRef,
				ID:  "", // to be resolved at runtime
				LookupFields: map[string]string{
					"name": gatewayName,
				},
			},
		}
	}

	plan.AddChange(change)
}

// planSchemaRegistryUpdate plans an UPDATE change for a schema registry.
func (p *Planner) planSchemaRegistryUpdate(
	namespace string,
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	registryID string,
	sr resources.EventGatewaySchemaRegistryResource,
	updateFields map[string]any,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, ResourceTypeEventGatewaySchemaRegistry, sr.Ref),
		ResourceType: ResourceTypeEventGatewaySchemaRegistry,
		ResourceRef:  sr.Ref,
		ResourceID:   registryID,
		Action:       ActionUpdate,
		Fields:       updateFields,
		Namespace:    namespace,
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
		ChangedFields: map[string]FieldChange{
			"updated": {Old: gatewayName, New: ""},
		},
	}

	plan.AddChange(change)
}

// planSchemaRegistryDelete plans a DELETE change for a schema registry.
func (p *Planner) planSchemaRegistryDelete(
	gatewayRef string,
	gatewayName string,
	gatewayID string,
	registryID string,
	registryName string,
	plan *Plan,
) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, ResourceTypeEventGatewaySchemaRegistry, registryName),
		ResourceType: ResourceTypeEventGatewaySchemaRegistry,
		ResourceRef:  registryName,
		ResourceID:   registryID,
		Action:       ActionDelete,
		Namespace:    "",
		Parent: &ParentInfo{
			Ref: gatewayRef,
			ID:  gatewayID,
		},
	}

	plan.AddChange(change)
}

// shouldUpdateSchemaRegistry compares current and desired schema registry state.
// It returns whether an update is needed and the fields to send in the PUT request.
func (p *Planner) shouldUpdateSchemaRegistry(
	current state.EventGatewaySchemaRegistry,
	desired resources.EventGatewaySchemaRegistryResource,
) (bool, map[string]any) {
	updates := make(map[string]any)
	needsUpdate := false

	// Only the confluent variant is currently supported.
	conf := desired.SchemaRegistryCreate.SchemaRegistryConfluent
	if conf == nil {
		return false, nil
	}

	// Compare name
	if current.Name != conf.Name {
		needsUpdate = true
	}

	// Compare description
	currentDesc := ""
	if current.Description != nil {
		currentDesc = *current.Description
	}
	desiredDesc := ""
	if conf.Description != nil {
		desiredDesc = *conf.Description
	}
	if currentDesc != desiredDesc {
		needsUpdate = true
	}

	// Compare type (structural)
	if current.Type != conf.GetType() {
		needsUpdate = true
	}

	// Compare config fields using RawConfig (SDK Config struct is opaque/empty).
	// password is write-only and is never returned by the API, so it is skipped.
	desiredConf := conf.Config
	currentSchemaType, _ := current.RawConfig["schema_type"].(string)
	if currentSchemaType != string(desiredConf.SchemaType) {
		needsUpdate = true
	}

	currentEndpoint, _ := current.RawConfig["endpoint"].(string)
	if currentEndpoint != desiredConf.Endpoint {
		needsUpdate = true
	}

	// JSON numbers unmarshal as float64.
	currentTimeout := int64(10) // API default
	if t, ok := current.RawConfig["timeout_seconds"].(float64); ok {
		currentTimeout = int64(t)
	}
	desiredTimeout := int64(10)
	if desiredConf.TimeoutSeconds != nil {
		desiredTimeout = *desiredConf.TimeoutSeconds
	}
	if currentTimeout != desiredTimeout {
		needsUpdate = true
	}

	// Compare authentication fields (skip password — write-only).
	if desiredConf.Authentication != nil && desiredConf.Authentication.SchemaRegistryAuthenticationBasic != nil {
		desiredAuth := desiredConf.Authentication.SchemaRegistryAuthenticationBasic
		if currentAuth, ok := current.RawConfig["authentication"].(map[string]any); ok {
			currentAuthType, _ := currentAuth["type"].(string)
			if currentAuthType != desiredAuth.GetType() {
				needsUpdate = true
			}

			currentUsername, _ := currentAuth["username"].(string)
			if currentUsername != desiredAuth.Username {
				needsUpdate = true
			}
		} else {
			needsUpdate = true
		}
	} else if desiredConf.Authentication == nil {
		if _, hasAuth := current.RawConfig["authentication"]; hasAuth {
			needsUpdate = true
		}
	}

	// Compare labels
	if !labelsEqual(current.NormalizedLabels, conf.Labels) {
		needsUpdate = true
	}

	if needsUpdate {
		updates = buildSchemaRegistryFields(desired)
	}

	return needsUpdate, updates
}

// labelsEqual returns true iff two label maps are identical.
func labelsEqual(a map[string]string, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// buildSchemaRegistryFields builds the field map used in PlannedChange.Fields.
func buildSchemaRegistryFields(sr resources.EventGatewaySchemaRegistryResource) map[string]any {
	fields := make(map[string]any)

	conf := sr.SchemaRegistryCreate.SchemaRegistryConfluent
	if conf == nil {
		return fields
	}

	fields["name"] = conf.Name
	fields["type"] = conf.GetType()

	if conf.Description != nil {
		fields["description"] = *conf.Description
	}

	fields["config"] = kkComps.SchemaRegistryConfluentConfig{
		SchemaType:     conf.Config.SchemaType,
		Endpoint:       conf.Config.Endpoint,
		TimeoutSeconds: conf.Config.TimeoutSeconds,
		Authentication: conf.Config.Authentication,
	}

	if len(conf.Labels) > 0 {
		fields["labels"] = conf.Labels
	}

	return fields
}

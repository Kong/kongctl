package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewaySchemaRegistryAdapter implements ResourceOperations for Event Gateway Schema Registries.
type EventGatewaySchemaRegistryAdapter struct {
	client *state.Client
}

// NewEventGatewaySchemaRegistryAdapter creates a new adapter for Event Gateway Schema Registries.
func NewEventGatewaySchemaRegistryAdapter(client *state.Client) *EventGatewaySchemaRegistryAdapter {
	return &EventGatewaySchemaRegistryAdapter{client: client}
}

// MapCreateFields maps plan fields to a SchemaRegistryCreate request.
func (a *EventGatewaySchemaRegistryAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.SchemaRegistryCreate,
) error {
	confluent, err := buildConfluentCreate(fields)
	if err != nil {
		return err
	}
	*create = kkComps.CreateSchemaRegistryCreateConfluent(confluent)
	return nil
}

// MapUpdateFields maps plan fields to a SchemaRegistryUpdate request.
func (a *EventGatewaySchemaRegistryAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.SchemaRegistryUpdate,
	_ map[string]string,
) error {
	confluent, err := buildConfluentUpdate(fields)
	if err != nil {
		return err
	}
	*update = kkComps.CreateSchemaRegistryUpdateConfluent(confluent)
	return nil
}

// Create creates a new schema registry.
func (a *EventGatewaySchemaRegistryAdapter) Create(
	ctx context.Context,
	req kkComps.SchemaRegistryCreate,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getGatewayIDFromContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.CreateEventGatewaySchemaRegistry(ctx, gatewayID, req, namespace)
}

// Update updates an existing schema registry.
func (a *EventGatewaySchemaRegistryAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.SchemaRegistryUpdate,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getGatewayIDFromContext(execCtx)
	if err != nil {
		return "", err
	}
	return a.client.UpdateEventGatewaySchemaRegistry(ctx, gatewayID, id, req, namespace)
}

// Delete deletes a schema registry.
func (a *EventGatewaySchemaRegistryAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	gatewayID, err := a.getGatewayIDFromContext(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteEventGatewaySchemaRegistry(ctx, gatewayID, id)
}

// GetByID gets a schema registry by ID.
func (a *EventGatewaySchemaRegistryAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getGatewayIDFromContext(execCtx)
	if err != nil {
		return nil, err
	}

	sr, err := a.client.GetEventGatewaySchemaRegistryByID(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if sr == nil {
		return nil, nil
	}

	return &EventGatewaySchemaRegistryResourceInfo{sr: sr}, nil
}

// GetByName is not directly supported; schema registries are identified by name via list.
func (a *EventGatewaySchemaRegistryAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway schema registries")
}

// ResourceType returns the resource type string.
func (a *EventGatewaySchemaRegistryAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewaySchemaRegistry
}

// RequiredFields returns the list of required fields for this resource.
func (a *EventGatewaySchemaRegistryAdapter) RequiredFields() []string {
	return []string{"name", "type", "config"}
}

// SupportsUpdate indicates whether this resource supports update operations.
func (a *EventGatewaySchemaRegistryAdapter) SupportsUpdate() bool {
	return true
}

// getGatewayIDFromContext extracts the event gateway ID from the execution context.
func (a *EventGatewaySchemaRegistryAdapter) getGatewayIDFromContext(
	execCtx *ExecutionContext,
) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange

	// Priority 1: References (for new parent)
	if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID != "" {
		return gatewayRef.ID, nil
	}

	// Priority 2: Parent field (for existing parent)
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("event gateway ID required for schema registry operations")
}

// buildConfluentCreate converts a field map to a SchemaRegistryConfluent create struct.
func buildConfluentCreate(fields map[string]any) (kkComps.SchemaRegistryConfluent, error) {
	var confluent kkComps.SchemaRegistryConfluent

	name, ok := fields["name"].(string)
	if !ok || name == "" {
		return confluent, fmt.Errorf("name is required for schema registry")
	}
	confluent.Name = name

	if desc, ok := fields["description"].(string); ok {
		confluent.Description = &desc
	}

	config, err := extractConfluentConfig(fields)
	if err != nil {
		return confluent, err
	}
	confluent.Config = config

	if labelsRaw, ok := fields["labels"]; ok {
		switch v := labelsRaw.(type) {
		case map[string]string:
			confluent.Labels = v
		case map[string]any:
			m := make(map[string]string, len(v))
			for k, val := range v {
				if s, ok := val.(string); ok {
					m[k] = s
				}
			}
			confluent.Labels = m
		}
	}

	return confluent, nil
}

// buildConfluentUpdate converts a field map to a SchemaRegistryConfluentSensitiveDataAware struct.
func buildConfluentUpdate(fields map[string]any) (kkComps.SchemaRegistryConfluentSensitiveDataAware, error) {
	var confluent kkComps.SchemaRegistryConfluentSensitiveDataAware

	name, ok := fields["name"].(string)
	if !ok || name == "" {
		return confluent, fmt.Errorf("name is required for schema registry update")
	}
	confluent.Name = name

	if desc, ok := fields["description"].(string); ok {
		confluent.Description = &desc
	}

	configSDA, err := extractConfluentConfigSensitiveDataAware(fields)
	if err != nil {
		return confluent, err
	}
	confluent.Config = configSDA

	if labelsRaw, ok := fields["labels"]; ok {
		switch v := labelsRaw.(type) {
		case map[string]string:
			confluent.Labels = v
		case map[string]any:
			m := make(map[string]string, len(v))
			for k, val := range v {
				if s, ok := val.(string); ok {
					m[k] = s
				}
			}
			confluent.Labels = m
		}
	}

	return confluent, nil
}

// extractConfluentConfig extracts SchemaRegistryConfluentConfig from the fields map.
func extractConfluentConfig(fields map[string]any) (kkComps.SchemaRegistryConfluentConfig, error) {
	var cfg kkComps.SchemaRegistryConfluentConfig

	// Accept pre-built SDK struct
	if sdkCfg, ok := fields["config"].(kkComps.SchemaRegistryConfluentConfig); ok {
		return sdkCfg, nil
	}

	// Accept a map
	cfgRaw, ok := fields["config"]
	if !ok {
		return cfg, fmt.Errorf("config is required for schema registry")
	}

	cfgMap, ok := cfgRaw.(map[string]any)
	if !ok {
		return cfg, fmt.Errorf("config must be an object")
	}

	endpoint, ok := cfgMap["endpoint"].(string)
	if !ok || endpoint == "" {
		return cfg, fmt.Errorf("config.endpoint is required")
	}
	cfg.Endpoint = endpoint

	schemaType, ok := cfgMap["schema_type"].(string)
	if !ok || schemaType == "" {
		return cfg, fmt.Errorf("config.schema_type is required (avro or json)")
	}
	cfg.SchemaType = kkComps.SchemaType(schemaType)

	if timeout, ok := cfgMap["timeout_seconds"].(int64); ok {
		cfg.TimeoutSeconds = &timeout
	} else if timeout, ok := cfgMap["timeout_seconds"].(float64); ok {
		t := int64(timeout)
		cfg.TimeoutSeconds = &t
	}

	// Authentication is optional
	if authRaw, ok := cfgMap["authentication"]; ok {
		auth, err := extractSchemaRegistryAuth(authRaw)
		if err != nil {
			return cfg, fmt.Errorf("config.authentication: %w", err)
		}
		cfg.Authentication = &auth
	}

	return cfg, nil
}

// extractConfluentConfigSensitiveDataAware extracts the sensitive-data-aware config variant.
func extractConfluentConfigSensitiveDataAware(
	fields map[string]any,
) (kkComps.SchemaRegistryConfluentConfigSensitiveDataAware, error) {
	var cfg kkComps.SchemaRegistryConfluentConfigSensitiveDataAware

	if sdkCfg, ok := fields["config"].(kkComps.SchemaRegistryConfluentConfigSensitiveDataAware); ok {
		return sdkCfg, nil
	}

	cfgRaw, ok := fields["config"]
	if !ok {
		return cfg, fmt.Errorf("config is required for schema registry update")
	}

	cfgMap, ok := cfgRaw.(map[string]any)
	if !ok {
		// Accept a pre-built base config and convert it
		if baseCfg, ok := cfgRaw.(kkComps.SchemaRegistryConfluentConfig); ok {
			cfg.Endpoint = baseCfg.Endpoint
			cfg.SchemaType = kkComps.SchemaRegistryConfluentConfigSensitiveDataAwareSchemaType(baseCfg.SchemaType)
			cfg.TimeoutSeconds = baseCfg.TimeoutSeconds
			if baseCfg.Authentication != nil {
				sdaAuth, err := convertAuthToSDA(*baseCfg.Authentication)
				if err != nil {
					return cfg, err
				}
				cfg.Authentication = &sdaAuth
			}
			return cfg, nil
		}
		return cfg, fmt.Errorf("config must be an object")
	}

	endpoint, ok := cfgMap["endpoint"].(string)
	if !ok || endpoint == "" {
		return cfg, fmt.Errorf("config.endpoint is required")
	}
	cfg.Endpoint = endpoint

	schemaType, ok := cfgMap["schema_type"].(string)
	if !ok || schemaType == "" {
		return cfg, fmt.Errorf("config.schema_type is required (avro or json)")
	}
	cfg.SchemaType = kkComps.SchemaRegistryConfluentConfigSensitiveDataAwareSchemaType(schemaType)

	if timeout, ok := cfgMap["timeout_seconds"].(int64); ok {
		cfg.TimeoutSeconds = &timeout
	} else if timeout, ok := cfgMap["timeout_seconds"].(float64); ok {
		t := int64(timeout)
		cfg.TimeoutSeconds = &t
	}

	if authRaw, ok := cfgMap["authentication"]; ok {
		auth, err := extractSchemaRegistryAuth(authRaw)
		if err != nil {
			return cfg, fmt.Errorf("config.authentication: %w", err)
		}
		sdaAuth, err := convertAuthToSDA(auth)
		if err != nil {
			return cfg, err
		}
		cfg.Authentication = &sdaAuth
	}

	return cfg, nil
}

// extractSchemaRegistryAuth builds SchemaRegistryAuthenticationScheme from a raw map or SDK type.
func extractSchemaRegistryAuth(raw any) (kkComps.SchemaRegistryAuthenticationScheme, error) {
	var auth kkComps.SchemaRegistryAuthenticationScheme

	if sdkAuth, ok := raw.(kkComps.SchemaRegistryAuthenticationScheme); ok {
		return sdkAuth, nil
	}

	authMap, ok := raw.(map[string]any)
	if !ok {
		return auth, fmt.Errorf("authentication must be an object")
	}

	authType, _ := authMap["type"].(string)
	if authType == "" {
		return auth, fmt.Errorf("authentication.type is required (currently only 'basic')")
	}

	switch authType {
	case string(kkComps.SchemaRegistryAuthenticationSchemeTypeBasic):
		username, _ := authMap["username"].(string)
		password, _ := authMap["password"].(string)
		basic := kkComps.SchemaRegistryAuthenticationBasic{
			Username: username,
			Password: password,
		}
		auth = kkComps.CreateSchemaRegistryAuthenticationSchemeBasic(basic)
	default:
		return auth, fmt.Errorf("unsupported authentication type %q (currently only 'basic')", authType)
	}

	return auth, nil
}

// convertAuthToSDA converts authentication to the sensitive-data-aware variant.
func convertAuthToSDA(
	auth kkComps.SchemaRegistryAuthenticationScheme,
) (kkComps.SchemaRegistryAuthenticationSensitiveDataAwareScheme, error) {
	var sda kkComps.SchemaRegistryAuthenticationSensitiveDataAwareScheme

	if auth.SchemaRegistryAuthenticationBasic == nil {
		return sda, fmt.Errorf("unsupported authentication type for update")
	}

	basic := auth.SchemaRegistryAuthenticationBasic
	sdaBasic := kkComps.SchemaRegistryAuthenticationBasicSensitiveDataAware{
		Username: basic.Username,
		Password: &basic.Password,
	}
	sda = kkComps.CreateSchemaRegistryAuthenticationSensitiveDataAwareSchemeBasic(sdaBasic)
	return sda, nil
}

// EventGatewaySchemaRegistryResourceInfo wraps an Event Gateway Schema Registry to implement ResourceInfo.
type EventGatewaySchemaRegistryResourceInfo struct {
	sr *state.EventGatewaySchemaRegistry
}

func (e *EventGatewaySchemaRegistryResourceInfo) GetID() string {
	return e.sr.ID
}

func (e *EventGatewaySchemaRegistryResourceInfo) GetName() string {
	return e.sr.Name
}

func (e *EventGatewaySchemaRegistryResourceInfo) GetLabels() map[string]string {
	return e.sr.Labels
}

func (e *EventGatewaySchemaRegistryResourceInfo) GetNormalizedLabels() map[string]string {
	return e.sr.NormalizedLabels
}

package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewayTLSTrustBundleAdapter implements ResourceOperations for Event Gateway TLS Trust Bundle resources.
// TLS trust bundles support update operations.
type EventGatewayTLSTrustBundleAdapter struct {
	client *state.Client
}

// NewEventGatewayTLSTrustBundleAdapter creates a new EventGatewayTLSTrustBundleAdapter.
func NewEventGatewayTLSTrustBundleAdapter(client *state.Client) *EventGatewayTLSTrustBundleAdapter {
	return &EventGatewayTLSTrustBundleAdapter{client: client}
}

// MapCreateFields maps fields to CreateTLSTrustBundleRequest.
func (a *EventGatewayTLSTrustBundleAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateTLSTrustBundleRequest,
) error {
	name, ok := fields[planner.FieldName].(string)
	if !ok || name == "" {
		return fmt.Errorf("name is required")
	}
	create.Name = name

	if desc, ok := fields[planner.FieldDescription].(string); ok {
		create.Description = &desc
	}

	cfg, err := extractTrustBundleConfig(fields)
	if err != nil {
		return err
	}
	create.Config = cfg

	if labelsRaw, ok := fields[planner.FieldLabels].(map[string]any); ok {
		lbls := make(map[string]string, len(labelsRaw))
		for k, v := range labelsRaw {
			if sv, ok := v.(string); ok {
				lbls[k] = sv
			}
		}
		create.Labels = lbls
	} else if lbls, ok := fields[planner.FieldLabels].(map[string]string); ok {
		create.Labels = lbls
	}

	return nil
}

// MapUpdateFields maps fields to UpdateTLSTrustBundleRequest.
func (a *EventGatewayTLSTrustBundleAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateTLSTrustBundleRequest,
	_ map[string]string,
) error {
	if name, ok := fields[planner.FieldName].(string); ok && name != "" {
		update.Name = &name
	}

	if desc, ok := fields[planner.FieldDescription].(string); ok {
		update.Description = &desc
	}

	if _, hasConfig := fields[planner.FieldConfig]; hasConfig {
		cfg, err := extractTrustBundleConfig(fields)
		if err != nil {
			return err
		}
		update.Config = &cfg
	}

	if labelsRaw, ok := fields[planner.FieldLabels].(map[string]any); ok {
		lbls := make(map[string]string, len(labelsRaw))
		for k, v := range labelsRaw {
			if sv, ok := v.(string); ok {
				lbls[k] = sv
			}
		}
		update.Labels = lbls
	} else if lbls, ok := fields[planner.FieldLabels].(map[string]string); ok {
		update.Labels = lbls
	}

	return nil
}

// Create creates a new TLS trust bundle.
func (a *EventGatewayTLSTrustBundleAdapter) Create(
	ctx context.Context,
	req kkComps.CreateTLSTrustBundleRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.CreateEventGatewayTLSTrustBundle(ctx, gatewayID, req, namespace)
}

// Update updates an existing TLS trust bundle.
func (a *EventGatewayTLSTrustBundleAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateTLSTrustBundleRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.UpdateEventGatewayTLSTrustBundle(ctx, gatewayID, id, req, namespace)
}

// Delete deletes a TLS trust bundle.
func (a *EventGatewayTLSTrustBundleAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteEventGatewayTLSTrustBundle(ctx, gatewayID, id)
}

// GetByID retrieves a TLS trust bundle by ID.
func (a *EventGatewayTLSTrustBundleAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	tb, err := a.client.GetEventGatewayTLSTrustBundle(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if tb == nil {
		return nil, nil
	}

	return &EventGatewayTLSTrustBundleResourceInfo{bundle: tb}, nil
}

// GetByName is not supported for TLS trust bundles (no direct name-based lookup).
func (a *EventGatewayTLSTrustBundleAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway TLS trust bundles")
}

// ResourceType returns the resource type string.
func (a *EventGatewayTLSTrustBundleAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayTLSTrustBundle
}

// RequiredFields returns the list of required fields for this resource.
func (a *EventGatewayTLSTrustBundleAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldConfig}
}

// SupportsUpdate returns true – TLS trust bundles support update operations.
func (a *EventGatewayTLSTrustBundleAdapter) SupportsUpdate() bool {
	return true
}

// getEventGatewayIDFromExecutionContext extracts the event gateway ID from the execution context.
func (a *EventGatewayTLSTrustBundleAdapter) getEventGatewayIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange

	// Priority 1: Check References (for new parent)
	if gatewayRef, ok := change.References[planner.FieldEventGatewayID]; ok && gatewayRef.ID != "" {
		return gatewayRef.ID, nil
	}

	// Priority 2: Check Parent field (for existing parent)
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("event gateway ID required for TLS trust bundle operations")
}

// EventGatewayTLSTrustBundleResourceInfo wraps an Event Gateway TLS Trust Bundle to implement ResourceInfo.
type EventGatewayTLSTrustBundleResourceInfo struct {
	bundle *state.EventGatewayTLSTrustBundle
}

func (e *EventGatewayTLSTrustBundleResourceInfo) GetID() string {
	return e.bundle.ID
}

func (e *EventGatewayTLSTrustBundleResourceInfo) GetName() string {
	return e.bundle.Name
}

func (e *EventGatewayTLSTrustBundleResourceInfo) GetLabels() map[string]string {
	return e.bundle.Labels
}

func (e *EventGatewayTLSTrustBundleResourceInfo) GetNormalizedLabels() map[string]string {
	return e.bundle.NormalizedLabels
}

// extractTrustBundleConfig extracts TLSTrustBundleConfig from a fields map.
func extractTrustBundleConfig(fields map[string]any) (kkComps.TLSTrustBundleConfig, error) {
	var cfg kkComps.TLSTrustBundleConfig

	switch v := fields[planner.FieldConfig].(type) {
	case kkComps.TLSTrustBundleConfig:
		cfg = v
	case map[string]any:
		if trustedCA, ok := v["trusted_ca"].(string); ok {
			cfg.TrustedCa = trustedCA
		} else {
			return cfg, fmt.Errorf("config.trusted_ca is required")
		}
	default:
		return cfg, fmt.Errorf("config is required for TLS trust bundle")
	}

	if cfg.TrustedCa == "" {
		return cfg, fmt.Errorf("config.trusted_ca is required")
	}

	return cfg, nil
}

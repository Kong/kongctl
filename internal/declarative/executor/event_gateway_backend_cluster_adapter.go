package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// EventGatewayBackendClusterAdapter implements ResourceOperations for Event Gateway Backend Clusters
type EventGatewayBackendClusterAdapter struct {
	client *state.Client
}

// NewEventGatewayBackendClusterAdapter creates a new adapter for Event Gateway Backend Clusters
func NewEventGatewayBackendClusterAdapter(client *state.Client) *EventGatewayBackendClusterAdapter {
	return &EventGatewayBackendClusterAdapter{client: client}
}

// MapCreateFields maps fields to CreateBackendClusterRequest
func (a *EventGatewayBackendClusterAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateBackendClusterRequest,
) error {
	// Required fields
	name, ok := fields["name"].(string)
	if !ok {
		return fmt.Errorf("name is required")
	}
	create.Name = name

	// Authentication (required)
	if auth, ok := fields["authentication"].(kkComps.BackendClusterAuthenticationScheme); ok {
		create.Authentication = auth
	} else {
		return fmt.Errorf("authentication is required")
	}

	// Bootstrap servers (required)
	if servers, ok := fields["bootstrap_servers"].([]string); ok {
		create.BootstrapServers = servers
	} else {
		return fmt.Errorf("bootstrap_servers is required")
	}

	// TLS (required)
	if tls, ok := fields["tls"].(kkComps.BackendClusterTLS); ok {
		create.TLS = tls
	} else {
		return fmt.Errorf("tls is required")
	}

	// Optional fields
	if desc, ok := fields["description"].(string); ok {
		create.Description = &desc
	}

	if insecure, ok := fields["insecure_allow_anonymous_virtual_cluster_auth"].(bool); ok {
		create.InsecureAllowAnonymousVirtualClusterAuth = &insecure
	}

	if interval, ok := fields["metadata_update_interval_seconds"].(int64); ok {
		create.MetadataUpdateIntervalSeconds = &interval
	}

	// Labels
	if labels, ok := fields["labels"].(map[string]string); ok {
		create.Labels = labels
	}

	return nil
}

// MapUpdateFields maps fields to UpdateBackendClusterRequest
func (a *EventGatewayBackendClusterAdapter) MapUpdateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	update *kkComps.UpdateBackendClusterRequest,
	_ map[string]string,
) error {
	// Only include changed fields
	if name, ok := fields["name"].(string); ok {
		update.Name = name
	}

	if desc, ok := fields["description"].(string); ok {
		update.Description = &desc
	}

	// Note: Authentication type differs between create and update in SDK
	// Convert from BackendClusterAuthenticationScheme to BackendClusterAuthenticationSensitiveDataAwareScheme
	if authField, ok := fields["authentication"]; ok {
		if auth, ok := authField.(kkComps.BackendClusterAuthenticationScheme); ok {
			sensitiveAuth, err := convertToSensitiveDataAwareAuth(auth)
			if err != nil {
				return fmt.Errorf("failed to convert authentication: %w", err)
			}
			update.Authentication = sensitiveAuth
		}
	}

	if servers, ok := fields["bootstrap_servers"].([]string); ok {
		update.BootstrapServers = servers
	}

	if tls, ok := fields["tls"].(kkComps.BackendClusterTLS); ok {
		update.TLS = tls
	}

	if insecure, ok := fields["insecure_allow_anonymous_virtual_cluster_auth"].(bool); ok {
		update.InsecureAllowAnonymousVirtualClusterAuth = &insecure
	}

	if interval, ok := fields["metadata_update_interval_seconds"].(int64); ok {
		update.MetadataUpdateIntervalSeconds = &interval
	}

	if labels, ok := fields["labels"].(map[string]string); ok {
		update.Labels = labels
	}

	return nil
}

// convertToSensitiveDataAwareAuth converts BackendClusterAuthenticationScheme to BackendClusterAuthenticationSensitiveDataAwareScheme
func convertToSensitiveDataAwareAuth(
	auth kkComps.BackendClusterAuthenticationScheme,
) (kkComps.BackendClusterAuthenticationSensitiveDataAwareScheme, error) {
	switch auth.Type {
	case kkComps.BackendClusterAuthenticationSchemeTypeAnonymous:
		return kkComps.CreateBackendClusterAuthenticationSensitiveDataAwareSchemeAnonymous(
			kkComps.BackendClusterAuthenticationAnonymous{},
		), nil

	case kkComps.BackendClusterAuthenticationSchemeTypeSaslPlain:
		if auth.BackendClusterAuthenticationSaslPlain == nil {
			return kkComps.BackendClusterAuthenticationSensitiveDataAwareScheme{},
				fmt.Errorf("SASL Plain authentication data is missing")
		}
		return kkComps.CreateBackendClusterAuthenticationSensitiveDataAwareSchemeSaslPlain(
			kkComps.BackendClusterAuthenticationSaslPlainSensitiveDataAware{
				Username: auth.BackendClusterAuthenticationSaslPlain.Username,
				Password: &auth.BackendClusterAuthenticationSaslPlain.Password,
			},
		), nil

	case kkComps.BackendClusterAuthenticationSchemeTypeSaslScram:
		if auth.BackendClusterAuthenticationSaslScram == nil {
			return kkComps.BackendClusterAuthenticationSensitiveDataAwareScheme{},
				fmt.Errorf("SASL SCRAM authentication data is missing")
		}
		return kkComps.CreateBackendClusterAuthenticationSensitiveDataAwareSchemeSaslScram(
			kkComps.BackendClusterAuthenticationSaslScramSensitiveDataAware{
				Username: auth.BackendClusterAuthenticationSaslScram.Username,
				Password: &auth.BackendClusterAuthenticationSaslScram.Password,
			},
		), nil

	default:
		return kkComps.BackendClusterAuthenticationSensitiveDataAwareScheme{},
			fmt.Errorf("unsupported authentication type: %s", auth.Type)
	}
}

// Create creates a new backend cluster
func (a *EventGatewayBackendClusterAdapter) Create(
	ctx context.Context,
	req kkComps.CreateBackendClusterRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.CreateEventGatewayBackendCluster(ctx, gatewayID, req, namespace)
}

// Update updates an existing backend cluster
func (a *EventGatewayBackendClusterAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.UpdateBackendClusterRequest,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return "", err
	}

	return a.client.UpdateEventGatewayBackendCluster(ctx, gatewayID, id, req, namespace)
}

// Delete deletes a backend cluster
func (a *EventGatewayBackendClusterAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return err
	}

	return a.client.DeleteEventGatewayBackendCluster(ctx, gatewayID, id)
}

// GetByID gets a backend cluster by ID
func (a *EventGatewayBackendClusterAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	// Get event gateway ID from execution context
	gatewayID, err := a.getEventGatewayIDFromExecutionContext(execCtx)
	if err != nil {
		return nil, err
	}

	cluster, err := a.client.GetEventGatewayBackendCluster(ctx, gatewayID, id)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, nil
	}

	return &EventGatewayBackendClusterResourceInfo{backendCluster: cluster}, nil
}

// GetByName is not supported for backend clusters (they are looked up by name within a gateway)
func (a *EventGatewayBackendClusterAdapter) GetByName(
	_ context.Context,
	_ string,
) (ResourceInfo, error) {
	return nil, fmt.Errorf("GetByName not supported for event gateway backend clusters")
}

// ResourceType returns the resource type string
func (a *EventGatewayBackendClusterAdapter) ResourceType() string {
	return planner.ResourceTypeEventGatewayBackendCluster
}

// RequiredFields returns the list of required fields for this resource
func (a *EventGatewayBackendClusterAdapter) RequiredFields() []string {
	return []string{"name", "authentication", "bootstrap_servers", "tls"}
}

// SupportsUpdate indicates whether this resource supports update operations
func (a *EventGatewayBackendClusterAdapter) SupportsUpdate() bool {
	return true
}

// getEventGatewayIDFromExecutionContext extracts the event gateway ID from the execution context
func (a *EventGatewayBackendClusterAdapter) getEventGatewayIDFromExecutionContext(
	execCtx *ExecutionContext,
) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context required")
	}

	change := *execCtx.PlannedChange

	// Priority 1: Check References (for new parent)
	if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID != "" {
		return gatewayRef.ID, nil
	}

	// Priority 2: Check Parent field (for existing parent)
	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("event gateway ID required for backend cluster operations")
}

// EventGatewayBackendClusterResourceInfo wraps an Event Gateway Backend Cluster to implement ResourceInfo
type EventGatewayBackendClusterResourceInfo struct {
	backendCluster *state.EventGatewayBackendCluster
}

func (e *EventGatewayBackendClusterResourceInfo) GetID() string {
	return e.backendCluster.ID
}

func (e *EventGatewayBackendClusterResourceInfo) GetName() string {
	return e.backendCluster.Name
}

func (e *EventGatewayBackendClusterResourceInfo) GetLabels() map[string]string {
	return e.backendCluster.Labels
}

func (e *EventGatewayBackendClusterResourceInfo) GetNormalizedLabels() map[string]string {
	return e.backendCluster.NormalizedLabels
}

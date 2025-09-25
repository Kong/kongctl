package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// ControlPlaneAdapter implements ResourceOperations for control planes
type ControlPlaneAdapter struct {
	client *state.Client
}

// NewControlPlaneAdapter creates a new control plane adapter
func NewControlPlaneAdapter(client *state.Client) *ControlPlaneAdapter {
	return &ControlPlaneAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateControlPlaneRequest
func (a *ControlPlaneAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext,
	fields map[string]any, create *kkComps.CreateControlPlaneRequest,
) error {
	create.Name = common.ExtractResourceName(fields)
	common.MapOptionalStringFieldToPtr(&create.Description, fields, "description")

	if clusterType, ok := extractString(fields["cluster_type"]); ok {
		value := kkComps.CreateControlPlaneRequestClusterType(clusterType)
		create.ClusterType = &value
	}

	if authType, ok := extractString(fields["auth_type"]); ok {
		value := kkComps.AuthType(authType)
		create.AuthType = &value
	}

	if cloudGateway, ok := extractBool(fields["cloud_gateway"]); ok {
		create.CloudGateway = &cloudGateway
	}

	if proxyValues, ok := fields["proxy_urls"]; ok && proxyValues != nil {
		urls, err := convertProxyURLs(proxyValues)
		if err != nil {
			return fmt.Errorf("invalid proxy_urls value: %w", err)
		}
		create.ProxyUrls = urls
	}

	userLabels := labels.ExtractLabelsFromField(fields["labels"])
	create.Labels = labels.BuildCreateLabels(userLabels, execCtx.Namespace, execCtx.Protection)

	return nil
}

// MapUpdateFields maps planner fields to UpdateControlPlaneRequest
func (a *ControlPlaneAdapter) MapUpdateFields(_ context.Context, execCtx *ExecutionContext,
	fields map[string]any, update *kkComps.UpdateControlPlaneRequest, currentLabels map[string]string,
) error {
	for field, value := range fields {
		switch field {
		case "name":
			if name, ok := value.(string); ok {
				update.Name = &name
			}
		case "description":
			if desc, ok := value.(string); ok {
				update.Description = &desc
			}
		case "auth_type":
			if auth, ok := extractString(value); ok {
				typed := kkComps.UpdateControlPlaneRequestAuthType(auth)
				update.AuthType = &typed
			}
		case "proxy_urls":
			urls, err := convertProxyURLs(value)
			if err != nil {
				return fmt.Errorf("invalid proxy_urls value: %w", err)
			}
			update.ProxyUrls = urls
		}
	}

	desiredLabels := labels.ExtractLabelsFromField(fields["labels"])
	if plannerLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels]); plannerLabels != nil {
		currentLabels = plannerLabels
	}

	if desiredLabels != nil {
		update.Labels = labels.BuildUpdateStringLabels(desiredLabels, currentLabels, execCtx.Namespace, execCtx.Protection)
	} else if currentLabels != nil {
		update.Labels = labels.BuildUpdateStringLabels(currentLabels, currentLabels, execCtx.Namespace, execCtx.Protection)
	}

	return nil
}

// Create issues a create call via the state client
func (a *ControlPlaneAdapter) Create(ctx context.Context, req kkComps.CreateControlPlaneRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateControlPlane(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Update issues an update call via the state client
func (a *ControlPlaneAdapter) Update(ctx context.Context, id string, req kkComps.UpdateControlPlaneRequest,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateControlPlane(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Delete removes a control plane
func (a *ControlPlaneAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteControlPlane(ctx, id)
}

// GetByName resolves a control plane by name
func (a *ControlPlaneAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	cp, err := a.client.GetControlPlaneByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, nil
	}
	return &ControlPlaneResourceInfo{controlPlane: cp}, nil
}

// GetByID resolves a control plane by ID
func (a *ControlPlaneAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	cp, err := a.client.GetControlPlaneByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cp == nil {
		return nil, nil
	}
	return &ControlPlaneResourceInfo{controlPlane: cp}, nil
}

// ResourceType returns the adapter resource type
func (a *ControlPlaneAdapter) ResourceType() string {
	return "control_plane"
}

// RequiredFields lists required fields for create
func (a *ControlPlaneAdapter) RequiredFields() []string {
	return []string{"name"}
}

// SupportsUpdate indicates control planes support updates
func (a *ControlPlaneAdapter) SupportsUpdate() bool {
	return true
}

// ControlPlaneResourceInfo implements ResourceInfo for control planes
type ControlPlaneResourceInfo struct {
	controlPlane *state.ControlPlane
}

func (c *ControlPlaneResourceInfo) GetID() string {
	return c.controlPlane.ID
}

func (c *ControlPlaneResourceInfo) GetName() string {
	return c.controlPlane.Name
}

func (c *ControlPlaneResourceInfo) GetLabels() map[string]string {
	return c.controlPlane.Labels
}

func (c *ControlPlaneResourceInfo) GetNormalizedLabels() map[string]string {
	return c.controlPlane.NormalizedLabels
}

func extractString(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	if str, ok := value.(string); ok {
		return str, true
	}
	return "", false
}

func extractBool(value any) (bool, bool) {
	if value == nil {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case *bool:
		if v == nil {
			return false, false
		}
		return *v, true
	default:
		return false, false
	}
}

func convertProxyURLs(value any) ([]kkComps.ProxyURL, error) {
	if value == nil {
		return nil, nil
	}
	if urls, ok := value.([]kkComps.ProxyURL); ok {
		return urls, nil
	}

	switch items := value.(type) {
	case []any:
		return convertProxyInterfaceSlice(items)
	case []map[string]any:
		converted := make([]any, len(items))
		for i := range items {
			converted[i] = items[i]
		}
		return convertProxyInterfaceSlice(converted)
	default:
		return nil, fmt.Errorf("unsupported proxy_urls type %T", value)
	}
}

func convertProxyInterfaceSlice(items []any) ([]kkComps.ProxyURL, error) {
	urls := make([]kkComps.ProxyURL, 0, len(items))
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("proxy url entry must be object, got %T", item)
		}

		host, _ := data["host"].(string)
		protocol, _ := data["protocol"].(string)

		portVal, hasPort := data["port"]
		var port int64
		if hasPort {
			switch pv := portVal.(type) {
			case int64:
				port = pv
			case int:
				port = int64(pv)
			case float64:
				port = int64(pv)
			default:
				return nil, fmt.Errorf("invalid proxy url port type %T", portVal)
			}
		}

		urls = append(urls, kkComps.ProxyURL{
			Host:     host,
			Port:     port,
			Protocol: protocol,
		})
	}

	return urls, nil
}

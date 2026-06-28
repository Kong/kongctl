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

type IdentityDirectoryAdapter struct {
	client *state.Client
}

func NewIdentityDirectoryAdapter(client *state.Client) *IdentityDirectoryAdapter {
	return &IdentityDirectoryAdapter{client: client}
}

func (a *IdentityDirectoryAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.CreateDirectoryBody,
) error {
	create.Name = common.ExtractResourceName(fields)
	if description, ok := fields[planner.FieldDescription].(string); ok {
		create.Description = &description
	}
	if allowedControlPlanes, ok, err := extractIdentityDirectoryStringSliceField(
		fields,
		planner.FieldAllowedControlPlanes,
	); err != nil {
		return err
	} else if ok {
		create.AllowedControlPlanes = allowedControlPlanes
	}
	if allowAllControlPlanes, ok := fields[planner.FieldAllowAllControlPlanes].(bool); ok {
		create.AllowAllControlPlanes = &allowAllControlPlanes
	}
	if ttl, ok, err := extractIdentityDirectoryInt64Field(fields, planner.FieldTTLSecs); err != nil {
		return err
	} else if ok {
		create.TTLSecs = &ttl
	}
	if ttl, ok, err := extractIdentityDirectoryInt64Field(fields, planner.FieldNegativeTTLSecs); err != nil {
		return err
	} else if ok {
		create.NegativeTTLSecs = &ttl
	}

	userLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	create.Labels = labels.BuildCreateLabels(
		userLabels,
		execCtx.Namespace,
		identityDirectoryProtectionValue(execCtx.Protection),
	)

	return nil
}

func (a *IdentityDirectoryAdapter) MapUpdateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	update *kkComps.ReplaceDirectoryBody,
	currentLabels map[string]string,
) error {
	update.Name = common.ExtractResourceName(fields)
	if description, ok := fields[planner.FieldDescription].(string); ok {
		update.Description = &description
	}
	if allowedControlPlanes, ok, err := extractIdentityDirectoryStringSliceField(
		fields,
		planner.FieldAllowedControlPlanes,
	); err != nil {
		return err
	} else if ok {
		update.AllowedControlPlanes = allowedControlPlanes
	}
	if allowAllControlPlanes, ok := fields[planner.FieldAllowAllControlPlanes].(bool); ok {
		update.AllowAllControlPlanes = &allowAllControlPlanes
	}
	if ttl, ok, err := extractIdentityDirectoryInt64Field(fields, planner.FieldTTLSecs); err != nil {
		return err
	} else if ok {
		update.TTLSecs = &ttl
	}
	if ttl, ok, err := extractIdentityDirectoryInt64Field(fields, planner.FieldNegativeTTLSecs); err != nil {
		return err
	} else if ok {
		update.NegativeTTLSecs = &ttl
	}

	userLabels := labels.ExtractLabelsFromField(fields[planner.FieldLabels])
	if _, ok := fields[planner.FieldLabels]; !ok {
		userLabels = currentLabels
	}
	update.Labels = labels.BuildCreateLabels(
		userLabels,
		execCtx.Namespace,
		identityDirectoryProtectionValue(execCtx.Protection),
	)

	return nil
}

func (a *IdentityDirectoryAdapter) Create(
	ctx context.Context,
	req kkComps.CreateDirectoryBody,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	directory, err := a.client.CreateIdentityDirectory(ctx, req, namespace)
	if err != nil {
		return "", err
	}
	return directory.ID, nil
}

func (a *IdentityDirectoryAdapter) Update(
	ctx context.Context,
	id string,
	req kkComps.ReplaceDirectoryBody,
	namespace string,
	_ *ExecutionContext,
) (string, error) {
	directory, err := a.client.ReplaceIdentityDirectory(ctx, id, req, namespace)
	if err != nil {
		return "", err
	}
	return directory.ID, nil
}

func (a *IdentityDirectoryAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteIdentityDirectory(ctx, id)
}

func (a *IdentityDirectoryAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	directory, err := a.client.GetIdentityDirectoryByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if directory == nil {
		return nil, nil
	}
	return &IdentityDirectoryResourceInfo{directory: directory}, nil
}

func (a *IdentityDirectoryAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	directory, err := a.client.GetIdentityDirectoryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if directory == nil {
		return nil, nil
	}
	return &IdentityDirectoryResourceInfo{directory: directory}, nil
}

func (a *IdentityDirectoryAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityDirectory
}

func (a *IdentityDirectoryAdapter) RequiredFields() []string {
	return []string{planner.FieldName}
}

func (a *IdentityDirectoryAdapter) SupportsUpdate() bool {
	return true
}

type IdentityDirectoryResourceInfo struct {
	directory *state.IdentityDirectory
}

func (i *IdentityDirectoryResourceInfo) GetID() string {
	return i.directory.ID
}

func (i *IdentityDirectoryResourceInfo) GetName() string {
	return i.directory.Name
}

func (i *IdentityDirectoryResourceInfo) GetLabels() map[string]string {
	return i.directory.NormalizedLabels
}

func (i *IdentityDirectoryResourceInfo) GetNormalizedLabels() map[string]string {
	return i.directory.NormalizedLabels
}

func extractIdentityDirectoryStringSliceField(fields map[string]any, key string) ([]string, bool, error) {
	value, ok := fields[key]
	if !ok {
		return nil, false, nil
	}

	switch typed := value.(type) {
	case []string:
		return typed, true, nil
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			str, ok := item.(string)
			if !ok {
				return nil, false, fmt.Errorf("%s entries must be strings", key)
			}
			values = append(values, str)
		}
		return values, true, nil
	default:
		return nil, false, fmt.Errorf("%s must be a list of strings", key)
	}
}

func extractIdentityDirectoryInt64Field(fields map[string]any, key string) (int64, bool, error) {
	value, ok := fields[key]
	if !ok {
		return 0, false, nil
	}

	switch typed := value.(type) {
	case int:
		return int64(typed), true, nil
	case int64:
		return typed, true, nil
	case int32:
		return int64(typed), true, nil
	case float64:
		if typed != float64(int64(typed)) {
			return 0, false, fmt.Errorf("%s must be an integer", key)
		}
		return int64(typed), true, nil
	default:
		return 0, false, fmt.Errorf("%s must be an integer", key)
	}
}

func identityDirectoryProtectionValue(protection any) any {
	switch typed := protection.(type) {
	case planner.ProtectionChange:
		return typed.New
	case map[string]any:
		if newValue, ok := typed["new"].(bool); ok {
			return newValue
		}
		if protected, ok := typed["protected"].(bool); ok {
			return protected
		}
	case bool:
		return typed
	}
	return nil
}

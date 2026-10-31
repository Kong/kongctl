package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type IdentityDirectoryAdapter struct{ client *state.Client }

func (a *IdentityDirectoryAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityDirectory
}

func (a *IdentityDirectoryAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldManagedBy}
}
func (a *IdentityDirectoryAdapter) SupportsUpdate() bool { return true }

func mapIdentityDirectoryFields(fields map[string]any, destination any) error {
	for field := range fields {
		switch field {
		case planner.FieldName, planner.FieldDescription, planner.FieldAllowedControlPlanes,
			planner.FieldAllowAllControlPlanes, planner.FieldTTLSecs, planner.FieldNegativeTTLSecs,
			planner.FieldVaultEnabled, planner.FieldLabels, planner.FieldManagedBy:
		default:
			return fmt.Errorf("unsupported directory payload field %q", field)
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid directory payload")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("invalid directory payload fields")
	}
	return nil
}

func (a *IdentityDirectoryAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *kkComps.CreateDirectoryBody,
) error {
	return mapIdentityDirectoryFields(fields, request)
}

func (a *IdentityDirectoryAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *kkComps.ReplaceDirectoryBody, _ map[string]string,
) error {
	return mapIdentityDirectoryFields(fields, request)
}

func (a *IdentityDirectoryAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	value, err := a.client.GetIdentityDirectoryByName(ctx, name)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func (a *IdentityDirectoryAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	value, err := a.client.GetIdentityDirectoryByID(ctx, id)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func (a *IdentityDirectoryAdapter) Create(ctx context.Context, request kkComps.CreateDirectoryBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	if err := validateDirectoryWriteMetadata(request.ManagedBy, namespace, execCtx); err != nil {
		return "", err
	}
	current, err := a.client.GetIdentityDirectoryByName(ctx, request.Name)
	if err != nil {
		return "", err
	}
	if current != nil {
		return "", childCreationCollision("directory", request.Name)
	}
	created, err := a.client.CreateIdentityDirectory(ctx, request)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func validateDirectoryWriteMetadata(metadata map[string]string, namespace string, execCtx *ExecutionContext) error {
	if execCtx == nil || execCtx.PlannedChange == nil || namespace == "" || namespace != execCtx.Namespace {
		return fmt.Errorf("directory execution namespace mismatch")
	}
	if metadata[resources.ManagedByToolKey] != resources.ManagedByTool {
		return fmt.Errorf("directory requires generated kongctl ownership metadata")
	}
	return validateChildPayloadOwnership("directory", metadata, namespace)
}

func (a *IdentityDirectoryAdapter) Update(ctx context.Context, id string, request kkComps.ReplaceDirectoryBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	if err := validateDirectoryWriteMetadata(request.ManagedBy, namespace, execCtx); err != nil {
		return "", err
	}
	current, err := a.client.GetIdentityDirectoryByID(ctx, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("directory no longer exists")
	}
	if current.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool {
		return "", fmt.Errorf("directory is no longer owned by kongctl; explicitly adopt it before managing it")
	}
	full, err := helpers.IdentityDirectoryReplacement(current.KongDirectory)
	if err != nil {
		return "", err
	}
	before, err := ownedChildPayload(full)
	if err != nil {
		return "", err
	}
	mapped, err := ownedChildPayload(request)
	if err != nil {
		return "", err
	}
	after := maps.Clone(before)
	// Only fields actually saved in the plan may override the fresh observation.
	// SDK defaults on omitted fields must not reset current configuration.
	for field, raw := range execCtx.PlannedChange.Fields {
		if value, present := mapped[field]; present {
			after[field] = value
		} else {
			after[field] = raw
		}
	}
	after, err = prepareOwnedMappedChildUpdate("directory", before, after, execCtx)
	if err != nil {
		return "", err
	}
	var replacement kkComps.ReplaceDirectoryBody
	if err := mapIdentityDirectoryFields(after, &replacement); err != nil {
		return "", err
	}
	if err := a.client.ReplaceIdentityDirectory(ctx, id, replacement); err != nil {
		return "", err
	}
	return id, nil
}

func (a *IdentityDirectoryAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	current, err := a.client.GetIdentityDirectoryByID(ctx, id)
	if err != nil || current == nil {
		return err
	}
	if execCtx == nil || execCtx.PlannedChange == nil ||
		current.Name != execCtx.PlannedChange.Fields[planner.FieldName] {
		return fmt.Errorf("directory identity changed; regenerate the plan")
	}
	if current.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool {
		return fmt.Errorf("directory is not owned by kongctl")
	}
	if err := resources.ValidateChildOwnership("directory", current.ManagedBy, execCtx.Namespace); err != nil {
		return err
	}
	if resources.IsChildProtected(current.ManagedBy) {
		return fmt.Errorf("directory is protected")
	}
	return a.client.DeleteIdentityDirectory(ctx, id)
}

func (e *Executor) registerIdentityDirectoryExecutor() {
	e.registerResourceExecutor(crudResourceExecutor(NewBaseExecutor(
		&IdentityDirectoryAdapter{client: e.client}, e.client, e.dryRun,
	)))
}

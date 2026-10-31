package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type IdentityGroupAdapter struct{ client *state.Client }

func (a *IdentityGroupAdapter) ResourceType() string { return planner.ResourceTypeIdentityGroup }
func (a *IdentityGroupAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldManagedBy}
}
func (a *IdentityGroupAdapter) SupportsUpdate() bool { return true }

func mapIdentityGroupFields(fields map[string]any, destination any) error {
	for field, value := range fields {
		switch field {
		case planner.FieldName,
			planner.FieldDescription,
			planner.FieldType,
			planner.FieldMetadata,
			planner.FieldLabels,
			planner.FieldManagedBy:
			if value == nil {
				return fmt.Errorf("group payload field %q cannot be null", field)
			}
		default:
			return fmt.Errorf("unsupported group payload field %q", field)
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid group payload")
	}
	var declaration resources.IdentityGroupResource
	if err := json.Unmarshal(data, &declaration); err != nil {
		return fmt.Errorf("invalid group payload fields")
	}
	declaration.Ref = "group-payload"
	if err := declaration.Validate(); err != nil {
		return err
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("invalid group payload fields")
	}
	return nil
}

func (a *IdentityGroupAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.CreateGroupBody,
) error {
	return mapIdentityGroupFields(fields, request)
}

func (a *IdentityGroupAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.ReplaceGroupBody, _ map[string]string,
) error {
	return mapIdentityGroupFields(fields, request)
}

func identityGroupDirectoryID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil || execCtx.PlannedChange.Parent == nil ||
		unresolvedReferenceID(execCtx.PlannedChange.Parent.ID) {
		return "", fmt.Errorf("group requires a resolved directory parent")
	}
	return execCtx.PlannedChange.Parent.ID, nil
}

func (a *IdentityGroupAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func (a *IdentityGroupAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	directoryID, err := identityGroupDirectoryID(execCtx)
	if err != nil {
		return nil, err
	}
	value, err := a.client.GetIdentityGroup(ctx, directoryID, id)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func (a *IdentityGroupAdapter) validateCurrent(current *state.IdentityGroup, execCtx *ExecutionContext) error {
	if err := helpers.ValidateIdentityGroupManagement(current.IdentityGroup); err != nil {
		return err
	}
	if err := resources.ValidateChildOwnership("group", current.ManagedBy, execCtx.Namespace); err != nil {
		return err
	}
	if name, ok := execCtx.PlannedChange.Fields[planner.FieldName].(string); !ok || name != current.Name {
		return fmt.Errorf("group name changed from the planned identity; regenerate the plan")
	}
	return validateChildPlannedProtection("group", current.ManagedBy, execCtx.PlannedChange)
}

func (a *IdentityGroupAdapter) Create(ctx context.Context, request kkComps.CreateGroupBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	directoryID, err := identityGroupDirectoryID(execCtx)
	if err != nil {
		return "", err
	}
	if err := validateDirectoryWriteMetadata(request.ManagedBy, namespace, execCtx); err != nil {
		return "", err
	}
	values, err := a.client.ListIdentityGroups(ctx, directoryID)
	if err != nil {
		return "", err
	}
	for _, value := range values {
		if value.Name == request.Name {
			return "", childCreationCollision("group", request.Name)
		}
	}
	created, err := a.client.CreateIdentityGroup(ctx, directoryID, request)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func identityGroupPayload(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	err = decoder.Decode(&fields)
	return fields, err
}

func (a *IdentityGroupAdapter) Update(ctx context.Context, id string, request kkComps.ReplaceGroupBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	directoryID, err := identityGroupDirectoryID(execCtx)
	if err != nil {
		return "", err
	}
	if err := validateDirectoryWriteMetadata(request.ManagedBy, namespace, execCtx); err != nil {
		return "", err
	}
	current, err := a.client.GetIdentityGroup(ctx, directoryID, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("group no longer exists")
	}
	if err := a.validateCurrent(current, execCtx); err != nil {
		return "", err
	}
	full, err := helpers.IdentityGroupReplacement(current.IdentityGroup)
	if err != nil {
		return "", err
	}
	before, err := identityGroupPayload(full)
	if err != nil {
		return "", err
	}
	mapped, err := identityGroupPayload(request)
	if err != nil {
		return "", err
	}
	after := maps.Clone(before)
	for field, raw := range execCtx.PlannedChange.Fields {
		if value, present := mapped[field]; present {
			after[field] = value
		} else {
			after[field] = raw
		}
	}
	if resources.IsChildProtected(current.ManagedBy) {
		oldPublic, nextPublic := maps.Clone(before), maps.Clone(after)
		delete(oldPublic, planner.FieldManagedBy)
		delete(nextPublic, planner.FieldManagedBy)
		if !childUnprotectOnly(execCtx.PlannedChange, current.ManagedBy, request.ManagedBy) ||
			!reflect.DeepEqual(oldPublic, nextPublic) {
			return "", fmt.Errorf("group is protected; unprotect it in a separate update")
		}
	}
	after[planner.FieldManagedBy] = resources.BuildChildManagedBy(
		namespace,
		resources.IsChildProtected(request.ManagedBy),
		"",
	)
	var replacement kkComps.ReplaceGroupBody
	if err := mapIdentityGroupFields(after, &replacement); err != nil {
		return "", err
	}
	if err := a.client.ReplaceIdentityGroup(ctx, directoryID, id, replacement); err != nil {
		return "", err
	}
	return id, nil
}

func (a *IdentityGroupAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	directoryID, err := identityGroupDirectoryID(execCtx)
	if err != nil {
		return err
	}
	current, err := a.client.GetIdentityGroup(ctx, directoryID, id)
	if err != nil || current == nil {
		return err
	}
	if err := a.validateCurrent(current, execCtx); err != nil {
		return err
	}
	if resources.IsChildProtected(current.ManagedBy) {
		return fmt.Errorf("group is protected")
	}
	if err := a.client.VerifyIdentityGroupEmpty(ctx, directoryID, id); err != nil {
		return err
	}
	return a.client.DeleteIdentityGroup(ctx, directoryID, id)
}

func (e *Executor) registerIdentityGroupExecutor() {
	e.registerResourceExecutor(
		crudResourceExecutor(NewBaseExecutor(&IdentityGroupAdapter{client: e.client}, e.client, e.dryRun)),
	)
}

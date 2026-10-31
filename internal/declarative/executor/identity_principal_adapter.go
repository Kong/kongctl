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

type IdentityPrincipalAdapter struct{ client *state.Client }

func (a *IdentityPrincipalAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityPrincipal
}

func (a *IdentityPrincipalAdapter) RequiredFields() []string { return []string{planner.FieldManagedBy} }
func (a *IdentityPrincipalAdapter) SupportsUpdate() bool     { return true }

func mapIdentityPrincipalFields(fields map[string]any, destination any) error {
	for field, value := range fields {
		switch field {
		case planner.FieldDisplayName,
			planner.FieldDescription,
			planner.FieldType,
			planner.FieldActive,
			planner.FieldMetadata,
			planner.FieldLabels,
			planner.FieldManagedBy:
			if value == nil {
				return fmt.Errorf("principal payload field %q cannot be null", field)
			}
		default:
			return fmt.Errorf("unsupported principal payload field %q", field)
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid principal payload")
	}
	if _, present := fields[planner.FieldMetadata]; present {
		var payload struct {
			Metadata map[string]any `json:"metadata"`
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			return fmt.Errorf("invalid principal metadata")
		}
		if err := resources.ValidateIdentityMetadata(payload.Metadata); err != nil {
			return err
		}
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("invalid principal payload fields")
	}
	return nil
}

func (a *IdentityPrincipalAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *kkComps.CreatePrincipalBody,
) error {
	return mapIdentityPrincipalFields(fields, request)
}

func (a *IdentityPrincipalAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *kkComps.ReplacePrincipalBody, _ map[string]string,
) error {
	return mapIdentityPrincipalFields(fields, request)
}

func principalDirectoryID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil || execCtx.PlannedChange.Parent == nil ||
		unresolvedReferenceID(execCtx.PlannedChange.Parent.ID) {
		return "", fmt.Errorf("principal requires a resolved directory parent")
	}
	return execCtx.PlannedChange.Parent.ID, nil
}

func (a *IdentityPrincipalAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	// No global name lookup exists; missing ID deletes are already complete.
	return nil, nil
}

func (a *IdentityPrincipalAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	directoryID, err := principalDirectoryID(execCtx)
	if err != nil {
		return nil, err
	}
	value, err := a.client.GetIdentityPrincipal(ctx, directoryID, id)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func validatePrincipalWrite(metadata map[string]string, namespace string, execCtx *ExecutionContext) error {
	if err := validateDirectoryWriteMetadata(metadata, namespace, execCtx); err != nil {
		return err
	}
	if metadata[resources.ManagedByRefKey] != execCtx.PlannedChange.ResourceRef {
		return fmt.Errorf("principal persisted ref does not match the planned identity")
	}
	return resources.ValidateRef(metadata[resources.ManagedByRefKey])
}

func validateObservedPrincipal(current *state.IdentityPrincipal, execCtx *ExecutionContext) error {
	if current.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool ||
		current.GetName() != execCtx.PlannedChange.ResourceRef {
		return fmt.Errorf("principal ownership identity changed; regenerate the plan or explicitly adopt it")
	}
	return resources.ValidateChildOwnership("principal", current.ManagedBy, execCtx.Namespace)
}

func (a *IdentityPrincipalAdapter) Create(ctx context.Context, request kkComps.CreatePrincipalBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	directoryID, err := principalDirectoryID(execCtx)
	if err != nil {
		return "", err
	}
	if err := validatePrincipalWrite(request.ManagedBy, namespace, execCtx); err != nil {
		return "", err
	}
	values, err := a.client.ListIdentityPrincipals(ctx, directoryID)
	if err != nil {
		return "", err
	}
	for _, value := range values {
		if value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
			value.ManagedBy[resources.ChildNamespaceKey] == namespace &&
			value.GetName() == execCtx.PlannedChange.ResourceRef {
			return "", childCreationCollision("principal", value.GetName())
		}
	}
	created, err := a.client.CreateIdentityPrincipal(ctx, directoryID, request)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func (a *IdentityPrincipalAdapter) Update(ctx context.Context, id string, request kkComps.ReplacePrincipalBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	directoryID, err := principalDirectoryID(execCtx)
	if err != nil {
		return "", err
	}
	if err := validatePrincipalWrite(request.ManagedBy, namespace, execCtx); err != nil {
		return "", err
	}
	current, err := a.client.GetIdentityPrincipal(ctx, directoryID, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("principal no longer exists")
	}
	if err := a.validateUniquePrincipal(ctx, directoryID, current, execCtx); err != nil {
		return "", err
	}
	if err := validateChildPlannedProtection("principal", current.ManagedBy, execCtx.PlannedChange); err != nil {
		return "", err
	}
	if err := helpers.ValidatePrincipalActiveUpdate(current.IdentityPrincipal, request.Active); err != nil {
		return "", err
	}
	// Q9: reject saved-plan type changes as well as planner-generated ones.
	if value, present := execCtx.PlannedChange.Fields[planner.FieldType]; present {
		if current.Type == nil || value != string(*current.Type) {
			return "", fmt.Errorf("principal type is create-only pending API question Q9")
		}
	}
	full := helpers.IdentityPrincipalReplacement(current.IdentityPrincipal)
	before, err := principalPayload(full)
	if err != nil {
		return "", err
	}
	mapped, err := principalPayload(request)
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
			return "", fmt.Errorf("principal is protected; unprotect it in a separate update")
		}
	}
	after[planner.FieldManagedBy] = resources.BuildPrincipalManagedBy(
		namespace,
		execCtx.PlannedChange.ResourceRef,
		resources.IsChildProtected(request.ManagedBy),
		"",
	)
	var replacement kkComps.ReplacePrincipalBody
	if err := mapIdentityPrincipalFields(after, &replacement); err != nil {
		return "", err
	}
	if err := a.client.ReplaceIdentityPrincipal(ctx, directoryID, id, replacement); err != nil {
		return "", err
	}
	return id, nil
}

func (a *IdentityPrincipalAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	directoryID, err := principalDirectoryID(execCtx)
	if err != nil {
		return err
	}
	current, err := a.client.GetIdentityPrincipal(ctx, directoryID, id)
	if err != nil || current == nil {
		return err
	}
	if err := a.validateUniquePrincipal(ctx, directoryID, current, execCtx); err != nil {
		return err
	}
	if resources.IsChildProtected(current.ManagedBy) {
		return fmt.Errorf("principal is protected")
	}
	// The API deletes a principal even while identities remain attached. Parent
	// ownership does not authorize deleting independently owned associations.
	identities, err := a.client.ListIdentityPrincipalIdentities(ctx, directoryID, id)
	if err != nil {
		return fmt.Errorf("cannot verify principal identities before deletion: %w", err)
	}
	if len(identities) != 0 {
		return fmt.Errorf(
			"principal still has %d identities; remove them explicitly before deleting it",
			len(identities),
		)
	}
	return a.client.DeleteIdentityPrincipal(ctx, directoryID, id)
}

func (e *Executor) registerIdentityPrincipalExecutor() {
	e.registerResourceExecutor(
		crudResourceExecutor(NewBaseExecutor(&IdentityPrincipalAdapter{client: e.client}, e.client, e.dryRun)),
	)
}

// Complete observation is required again at execution because refs are not unique in the API.
func (a *IdentityPrincipalAdapter) validateUniquePrincipal(ctx context.Context, directoryID string,
	current *state.IdentityPrincipal, execCtx *ExecutionContext,
) error {
	if err := validateObservedPrincipal(current, execCtx); err != nil {
		return err
	}
	values, err := a.client.ListIdentityPrincipals(ctx, directoryID)
	if err != nil {
		return err
	}
	matches := 0
	for _, value := range values {
		if value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
			value.ManagedBy[resources.ChildNamespaceKey] == execCtx.Namespace && value.GetName() == current.GetName() {
			matches++
			if value.ID != current.ID {
				return fmt.Errorf("multiple principals match the planned ownership identity; regenerate the plan")
			}
		}
	}
	if matches != 1 {
		return fmt.Errorf("principal lookup identity changed; regenerate the plan")
	}
	return nil
}

func principalPayload(request any) (map[string]any, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("invalid principal payload")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid principal payload")
	}
	return payload, nil
}

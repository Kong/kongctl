package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/util"
)

type identityAPIKeyAdapter struct {
	client   *state.Client
	executor *Executor
	kind     resources.ResourceType
}

func (a *identityAPIKeyAdapter) ResourceType() string { return string(a.kind) }
func (a *identityAPIKeyAdapter) RequiredFields() []string {
	return []string{planner.FieldType, planner.FieldManagedBy}
}
func (a *identityAPIKeyAdapter) SupportsUpdate() bool { return true }

// A name fallback cannot identify these keys and would hide duplicate or
// authority errors from the scoped UUID/ref lookup.
func (a *identityAPIKeyAdapter) RequiresScopedIDLookup() bool { return true }

func (a *identityAPIKeyAdapter) parentKind() string {
	if a.kind == resources.ResourceTypeIdentityPrincipalAPIKey {
		return planner.ResourceTypeIdentityPrincipal
	}
	return planner.ResourceTypeIdentityGroup
}

func (a *identityAPIKeyAdapter) validateContext(change *planner.PlannedChange) error {
	if change == nil || change.ResourceType != string(a.kind) || resources.ValidateRef(change.ResourceRef) != nil ||
		change.Namespace == "" ||
		change.Parent == nil ||
		resources.ValidateRef(change.Parent.Ref) != nil {
		return fmt.Errorf("API key requires valid resource, namespace and parent context")
	}
	if resources.ValidateRef(change.References[planner.FieldDirectoryID].Ref) != nil {
		return fmt.Errorf("API key requires its directory reference")
	}
	parent := change.ProtectingParent
	if parent == nil || parent.ResourceType != a.parentKind() || parent.ResourceRef != change.Parent.Ref ||
		parent.ResourceName == "" {
		return fmt.Errorf("API key requires its managed protecting parent; regenerate the plan")
	}
	if value, ok := change.Protection.(bool); !ok || value {
		return fmt.Errorf("API keys inherit parent protection and cannot set independent protection")
	}
	if change.Action != planner.ActionCreate && !util.IsValidUUID(change.ResourceID) {
		return fmt.Errorf("API-key update/delete requires its UUID")
	}
	if change.Action == planner.ActionCreate && change.ResourceID != "" {
		return fmt.Errorf("API-key create cannot select an existing UUID")
	}
	return nil
}

func (a *identityAPIKeyAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext, fields map[string]any,
	request *kkComps.CreatePrincipalKeyBodyv1,
) error {
	if execCtx == nil {
		return fmt.Errorf("API-key create requires an execution context")
	}
	for field, value := range fields {
		if field != planner.FieldType && field != planner.FieldManagedBy && field != planner.FieldName &&
			field != planner.FieldExpiresAt {
			return fmt.Errorf("unsupported API-key create field %q", field)
		}
		if value == nil && field != planner.FieldExpiresAt {
			return fmt.Errorf("API-key create field %q cannot be null", field)
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid API-key create payload")
	}
	if err := json.Unmarshal(data, request); err != nil {
		return fmt.Errorf("invalid API-key create payload")
	}
	if fields[planner.FieldType] != "v1" {
		return fmt.Errorf("declarative API keys require type v1")
	}
	ref, owned, err := resources.IdentityAPIKeyRef(request.ManagedBy)
	if err != nil {
		return err
	}
	if !owned || ref != execCtx.PlannedChange.ResourceRef || len(request.ManagedBy) != 4 {
		return fmt.Errorf("API-key create requires its canonical generated identity markers")
	}
	declaration := resources.IdentityAPIKeyFields{Ref: ref, Type: request.GetType(), Name: request.Name}
	return declaration.Validate()
}

func (a *identityAPIKeyAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.UpdatePrincipalKeyBody, _ map[string]string,
) error {
	if len(fields) == 0 {
		return fmt.Errorf("API-key update requires a public-field change")
	}
	for field, value := range fields {
		if field != planner.FieldExpiresAt &&
			(field != planner.FieldName || a.kind == resources.ResourceTypeIdentityGroupAPIKey) {
			return fmt.Errorf("unsupported API-key update field %q", field)
		}
		if field == planner.FieldName && value == nil {
			return fmt.Errorf("null API-key name clearing is unsupported (Q29)")
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid API-key update payload")
	}
	if err := json.Unmarshal(data, request); err != nil {
		return fmt.Errorf("invalid API-key update payload")
	}
	declaration := resources.IdentityAPIKeyFields{
		Ref: "key-payload", Type: resources.IdentityAPIKeyGeneratedType, Name: request.Name,
	}
	return declaration.Validate()
}

func identityAPIKeyPath(execCtx *ExecutionContext) (string, string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil || execCtx.PlannedChange.Parent == nil {
		return "", "", fmt.Errorf("API key requires its directory and parent")
	}
	directory, parent := execCtx.PlannedChange.References[planner.FieldDirectoryID].ID, execCtx.PlannedChange.Parent.ID
	if !util.IsValidUUID(directory) || !util.IsValidUUID(parent) {
		return "", "", fmt.Errorf("API key requires resolved directory and parent UUIDs")
	}
	return directory, parent, nil
}

func (a *identityAPIKeyAdapter) parent(ctx context.Context, execCtx *ExecutionContext) (string, string, error) {
	if execCtx == nil {
		return "", "", fmt.Errorf("API key requires an execution context")
	}
	if err := a.validateContext(execCtx.PlannedChange); err != nil {
		return "", "", err
	}
	directory, parent, err := identityAPIKeyPath(execCtx)
	if err != nil {
		return "", "", err
	}
	protecting := execCtx.PlannedChange.ProtectingParent
	if protecting.ResourceID != "" && protecting.ResourceID != parent {
		return "", "", fmt.Errorf("API-key protecting parent UUID changed; regenerate the plan")
	}
	protected, err := a.client.VerifyIdentityAPIKeyParent(
		ctx,
		a.kind,
		directory,
		parent,
		execCtx.Namespace,
		protecting.ResourceName,
	)
	if err != nil {
		return "", "", err
	}
	if protected && execCtx.PlannedChange.Action != planner.ActionCreate {
		return "", "", fmt.Errorf("API-key parent is protected")
	}
	return directory, parent, nil
}

func (a *identityAPIKeyAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func (a *identityAPIKeyAdapter) current(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (*state.IdentityAPIKey, error) {
	directory, parent, err := a.parent(ctx, execCtx)
	if err != nil {
		return nil, err
	}
	all, err := a.client.ListIdentityAPIKeys(ctx, a.kind, directory, parent)
	if err != nil {
		return nil, err
	}
	value, err := state.FindIdentityAPIKey(all, execCtx.PlannedChange.ResourceRef)
	if err != nil {
		return nil, err
	}
	if value == nil {
		for _, other := range all {
			if other.ID == id {
				return nil, fmt.Errorf("API-key UUID no longer matches the planned generated identity")
			}
		}
		return nil, nil
	}
	if value.ID != id {
		return nil, fmt.Errorf("API-key persisted ref resolves to a different UUID; regenerate the plan")
	}
	fresh, err := a.client.GetIdentityAPIKey(ctx, a.kind, directory, parent, id)
	if err != nil {
		return nil, err
	}
	if fresh == nil {
		return nil, nil
	}
	ref, owned, err := resources.IdentityAPIKeyRef(fresh.ManagedBy)
	if err != nil {
		return nil, err
	}
	if !owned || ref != execCtx.PlannedChange.ResourceRef {
		return nil, fmt.Errorf("API-key identity markers changed; regenerate the plan")
	}
	return fresh, nil
}

func (a *identityAPIKeyAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	value, err := a.current(ctx, id, execCtx)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func (a *identityAPIKeyAdapter) Create(ctx context.Context, request kkComps.CreatePrincipalKeyBodyv1,
	_ string, execCtx *ExecutionContext,
) (string, error) {
	directory, parent, err := a.parent(ctx, execCtx)
	if err != nil {
		return "", err
	}
	all, err := a.client.ListIdentityAPIKeys(ctx, a.kind, directory, parent)
	if err != nil {
		return "", err
	}
	value, err := state.FindIdentityAPIKey(all, execCtx.PlannedChange.ResourceRef)
	if err != nil {
		return "", err
	}
	if value != nil {
		return "", fmt.Errorf("API-key ref already exists; regenerate the plan instead of reissuing material")
	}
	output, err := a.executor.initialClientOutput(execCtx.PlannedChange)
	if err != nil {
		return "", err
	}
	created, err := a.client.CreateIdentityAPIKey(ctx, a.kind, directory, parent, request, output.file, output.metadata)
	if err != nil {
		return "", err
	}
	all, err = a.client.ListIdentityAPIKeys(ctx, a.kind, directory, parent)
	if err != nil {
		return "", err
	}
	value, err = state.FindIdentityAPIKey(all, execCtx.PlannedChange.ResourceRef)
	if err != nil {
		return "", err
	}
	if value == nil || value.ID != created.ID {
		return "", fmt.Errorf(
			"created API-key identity is inconsistent; material was captured; inspect before retrying",
		)
	}
	return created.ID, nil
}

func (a *identityAPIKeyAdapter) Update(ctx context.Context, id string, request kkComps.UpdatePrincipalKeyBody,
	_ string, execCtx *ExecutionContext,
) (string, error) {
	value, err := a.current(ctx, id, execCtx)
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", fmt.Errorf("API key no longer exists")
	}
	directory, parent, err := identityAPIKeyPath(execCtx)
	if err != nil {
		return "", err
	}
	if err := a.client.UpdateIdentityAPIKey(ctx, a.kind, directory, parent, id, request); err != nil {
		return "", err
	}
	return id, nil
}

func (a *identityAPIKeyAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	value, err := a.current(ctx, id, execCtx)
	if err != nil {
		return err
	}
	if value == nil {
		return nil
	}
	directory, parent, err := identityAPIKeyPath(execCtx)
	if err != nil {
		return err
	}
	return a.client.DeleteIdentityAPIKey(ctx, a.kind, directory, parent, id)
}

type identityAPIKeyExecutor struct {
	*BaseExecutor[kkComps.CreatePrincipalKeyBodyv1, kkComps.UpdatePrincipalKeyBody]
	adapter *identityAPIKeyAdapter
}

func (e *identityAPIKeyExecutor) ValidatePayload(ctx context.Context, change planner.PlannedChange) error {
	if err := e.adapter.validateContext(&change); err != nil {
		return err
	}
	if change.Action == planner.ActionDelete && len(change.Fields) != 0 {
		return fmt.Errorf("API-key delete has no payload fields")
	}
	return e.BaseExecutor.ValidatePayload(ctx, change)
}

func (e *Executor) registerIdentityAPIKeyExecutors() {
	kinds := []resources.ResourceType{
		resources.ResourceTypeIdentityPrincipalAPIKey, resources.ResourceTypeIdentityGroupAPIKey,
	}
	for _, kind := range kinds {
		adapter := &identityAPIKeyAdapter{client: e.client, executor: e, kind: kind}
		e.registerResourceExecutor(crudResourceExecutor(&identityAPIKeyExecutor{
			BaseExecutor: NewBaseExecutor(adapter, e.client, e.dryRun), adapter: adapter,
		}))
	}
}

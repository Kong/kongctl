package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

type identityAuthChildAdapter[C, U any] struct {
	client            *state.Client
	kind              string
	allowed, required []string
	list              func(context.Context, string) ([]ResourceInfo, error)
	get               func(context.Context, string, string) (ResourceInfo, error)
	create            func(context.Context, string, C) (string, error)
	update            func(context.Context, string, string, U) error
	remove            func(context.Context, string, string) error
}

func (a *identityAuthChildAdapter[C, U]) ResourceType() string     { return a.kind }
func (a *identityAuthChildAdapter[C, U]) RequiredFields() []string { return a.required }
func (a *identityAuthChildAdapter[C, U]) SupportsUpdate() bool     { return true }
func (a *identityAuthChildAdapter[C, U]) mapFields(fields map[string]any, destination any) error {
	for field, value := range fields {
		if !slices.Contains(a.allowed, field) || value == nil {
			return fmt.Errorf("invalid %s payload field %q", a.kind, field)
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid %s payload", a.kind)
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("invalid %s payload fields", a.kind)
	}
	return nil
}

func (a *identityAuthChildAdapter[C, U]) MapCreateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *C,
) error {
	return a.mapFields(fields, request)
}

func (a *identityAuthChildAdapter[C, U]) MapUpdateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *U, _ map[string]string,
) error {
	return a.mapFields(fields, request)
}

func (a *identityAuthChildAdapter[C, U]) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func authChildParentID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil || execCtx.PlannedChange.Parent == nil ||
		unresolvedReferenceID(execCtx.PlannedChange.Parent.ID) {
		return "", fmt.Errorf("auth-server child requires a resolved parent")
	}
	return execCtx.PlannedChange.Parent.ID, nil
}

func (a *identityAuthChildAdapter[C, U]) GetByID(ctx context.Context, id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	parent, err := authChildParentID(execCtx)
	if err != nil {
		return nil, err
	}
	return a.get(ctx, parent, id)
}

func (a *identityAuthChildAdapter[C, U]) writeParent(ctx context.Context, execCtx *ExecutionContext) (string, error) {
	id, err := authChildParentID(execCtx)
	if err != nil {
		return "", err
	}
	parent, err := a.client.GetIdentityAuthServerByID(ctx, id)
	if err != nil {
		return "", err
	}
	if parent == nil || execCtx.Namespace == "" || parent.Labels[labels.NamespaceKey] != execCtx.Namespace {
		return "", fmt.Errorf("auth-server child requires parent ownership in namespace %q", execCtx.Namespace)
	}
	if protecting := execCtx.PlannedChange.ProtectingParent; protecting == nil ||
		protecting.ResourceType != planner.ResourceTypeIdentityAuthServer || protecting.ResourceName != parent.Name {
		return "", fmt.Errorf("auth-server parent identity changed; regenerate the plan")
	}
	if execCtx.PlannedChange.Action != planner.ActionCreate && labels.IsProtectedResource(parent.Labels) {
		return "", fmt.Errorf("auth server %q is protected", parent.Name)
	}
	return id, nil
}

func (a *identityAuthChildAdapter[C, U]) checkIdentity(ctx context.Context, parent, id, name string) error {
	values, err := a.list(ctx, parent)
	if err != nil {
		return err
	}
	var matched ResourceInfo
	for _, value := range values {
		if value.GetName() != name {
			continue
		}
		if matched != nil {
			return fmt.Errorf("multiple %s records have name %q", a.kind, name)
		}
		matched = value
	}
	if id == "" {
		if matched != nil {
			return childCreationCollision(a.kind, name)
		}
		return nil
	}
	if matched == nil || matched.GetID() != id {
		return fmt.Errorf("auth-server child identity changed; regenerate the plan")
	}
	return nil
}

func (a *identityAuthChildAdapter[C, U]) checkScopeLinks(ctx context.Context, parent string, request any) error {
	if a.kind != planner.ResourceTypeIdentityAuthServerClaim {
		return nil
	}
	fields, err := authServerPayload(request)
	if err != nil {
		return err
	}
	values, _ := fields[planner.FieldIncludeInScopes].([]any)
	if len(values) == 0 {
		return nil
	}
	scopes, err := a.client.ListIdentityAuthServerScopes(ctx, parent)
	if err != nil {
		return err
	}
	ids := make(map[string]bool)
	for _, scope := range scopes {
		ids[scope.ID] = true
	}
	seen := make(map[string]bool)
	for _, value := range values {
		id, ok := value.(string)
		if !ok || !ids[id] || seen[id] {
			return fmt.Errorf("claim scope IDs must be distinct scopes under the same auth server")
		}
		seen[id] = true
	}
	return nil
}

func (a *identityAuthChildAdapter[C, U]) Create(ctx context.Context, request C, namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	parent, err := a.writeParent(ctx, execCtx)
	if err != nil {
		return "", err
	}
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("auth-server child namespace mismatch")
	}
	fields, err := authServerPayload(request)
	if err != nil {
		return "", err
	}
	name, _ := fields[planner.FieldName].(string)
	if err := a.checkIdentity(ctx, parent, "", name); err != nil {
		return "", err
	}
	if err := a.checkScopeLinks(ctx, parent, request); err != nil {
		return "", err
	}
	return a.create(ctx, parent, request)
}

func (a *identityAuthChildAdapter[C, U]) Update(ctx context.Context, id string, request U, namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	parent, err := a.writeParent(ctx, execCtx)
	if err != nil {
		return "", err
	}
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("auth-server child namespace mismatch")
	}
	current, err := a.get(ctx, parent, id)
	if err != nil {
		return "", err
	}
	fields, err := authServerPayload(request)
	if err != nil {
		return "", err
	}
	name, _ := fields[planner.FieldName].(string)
	if current == nil || current.GetName() != name {
		return "", fmt.Errorf("auth-server child identity changed")
	}
	if err := a.checkIdentity(ctx, parent, id, name); err != nil {
		return "", err
	}
	if err := a.checkScopeLinks(ctx, parent, request); err != nil {
		return "", err
	}
	if err := a.update(ctx, parent, id, request); err != nil {
		return "", err
	}
	return id, nil
}

func (a *identityAuthChildAdapter[C, U]) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	parent, err := a.writeParent(ctx, execCtx)
	if err != nil {
		return err
	}
	current, err := a.get(ctx, parent, id)
	if err != nil || current == nil {
		return err
	}
	name, _ := execCtx.PlannedChange.Fields[planner.FieldName].(string)
	if current.GetName() != name {
		return fmt.Errorf("auth-server child identity changed")
	}
	if err := a.checkIdentity(ctx, parent, id, name); err != nil {
		return err
	}
	if a.kind == planner.ResourceTypeIdentityAuthServerScope {
		clients, err := a.client.ListIdentityAuthServerClients(ctx, parent)
		if err != nil {
			return err
		}
		for _, client := range clients {
			if slices.Contains(client.AllowScopes, id) {
				return fmt.Errorf("scope is still referenced by client %q", client.Name)
			}
		}
		claims, err := a.client.ListIdentityAuthServerClaims(ctx, parent)
		if err != nil {
			return err
		}
		for _, claim := range claims {
			if slices.Contains(claim.IncludeInScopes, id) {
				return fmt.Errorf("scope is still referenced by claim %q", claim.Name)
			}
		}
	}
	return a.remove(ctx, parent, id)
}

func (e *Executor) registerIdentityAuthServerChildExecutors() {
	scope := &identityAuthChildAdapter[kkComps.CreateScope, kkComps.UpdateScope]{
		client: e.client, kind: planner.ResourceTypeIdentityAuthServerScope,
		required: []string{planner.FieldName},
		allowed: []string{
			planner.FieldName, planner.FieldDescription, planner.FieldDefault,
			planner.FieldEnabled, planner.FieldIncludeInMetadata,
		},
		list: func(ctx context.Context, parent string) ([]ResourceInfo, error) {
			values, err := e.client.ListIdentityAuthServerScopes(ctx, parent)
			if err != nil {
				return nil, err
			}
			result := make([]ResourceInfo, 0, len(values))
			for _, value := range values {
				result = append(result, value)
			}
			return result, nil
		},
		get: func(ctx context.Context, parent, id string) (ResourceInfo, error) {
			value, err := e.client.GetIdentityAuthServerScope(ctx, parent, id)
			if err != nil || value == nil {
				return nil, err
			}
			return value, nil
		},
		create: func(ctx context.Context, parent string, request kkComps.CreateScope) (string, error) {
			value, err := e.client.CreateIdentityAuthServerScope(ctx, parent, request)
			if err != nil {
				return "", err
			}
			return value.ID, nil
		},
		update: e.client.UpdateIdentityAuthServerScope, remove: e.client.DeleteIdentityAuthServerScope,
	}
	claim := &identityAuthChildAdapter[kkComps.CreateClaim, kkComps.UpdateClaim]{
		client: e.client, kind: planner.ResourceTypeIdentityAuthServerClaim,
		required: []string{planner.FieldName, planner.FieldValue},
		allowed: []string{
			planner.FieldName, planner.FieldValue, planner.FieldIncludeInToken,
			planner.FieldIncludeInAllScopes, planner.FieldIncludeInScopes, planner.FieldEnabled,
		},
		list: func(ctx context.Context, parent string) ([]ResourceInfo, error) {
			values, err := e.client.ListIdentityAuthServerClaims(ctx, parent)
			if err != nil {
				return nil, err
			}
			result := make([]ResourceInfo, 0, len(values))
			for _, value := range values {
				result = append(result, value)
			}
			return result, nil
		},
		get: func(ctx context.Context, parent, id string) (ResourceInfo, error) {
			value, err := e.client.GetIdentityAuthServerClaim(ctx, parent, id)
			if err != nil || value == nil {
				return nil, err
			}
			return value, nil
		},
		create: func(ctx context.Context, parent string, request kkComps.CreateClaim) (string, error) {
			value, err := e.client.CreateIdentityAuthServerClaim(ctx, parent, request)
			if err != nil {
				return "", err
			}
			return value.ID, nil
		},
		update: e.client.UpdateIdentityAuthServerClaim, remove: e.client.DeleteIdentityAuthServerClaim,
	}
	e.registerResourceExecutor(crudResourceExecutor(NewBaseExecutor(scope, e.client, e.dryRun)))
	e.registerResourceExecutor(crudResourceExecutor(NewBaseExecutor(claim, e.client, e.dryRun)))
}

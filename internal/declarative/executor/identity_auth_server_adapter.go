package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

type IdentityAuthServerAdapter struct{ client *state.Client }

func (a *IdentityAuthServerAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityAuthServer
}

func (a *IdentityAuthServerAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldAudience}
}
func (a *IdentityAuthServerAdapter) SupportsUpdate() bool { return true }

func mapIdentityAuthServerFields(fields map[string]any, destination any) error {
	for field := range fields {
		switch field {
		case planner.FieldName, planner.FieldAudience, planner.FieldDescription, planner.FieldSigningAlgorithm,
			planner.FieldTrustedOrigins, planner.FieldDCR, planner.FieldLabels:
		default:
			return fmt.Errorf("unsupported auth-server payload field %q", field)
		}
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid auth-server payload")
	}
	if err := json.Unmarshal(encoded, destination); err != nil {
		return fmt.Errorf("invalid auth-server payload fields")
	}
	return nil
}

func (a *IdentityAuthServerAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext,
	fields map[string]any, request *kkComps.CreateAuthServer,
) error {
	if err := mapIdentityAuthServerFields(fields, request); err != nil {
		return err
	}
	request.Labels = labels.BuildCreateLabels(request.Labels, execCtx.Namespace, execCtx.Protection)
	return nil
}

func (a *IdentityAuthServerAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *kkComps.UpdateAuthServer, _ map[string]string,
) error {
	return mapIdentityAuthServerFields(fields, request)
}

func (a *IdentityAuthServerAdapter) MapUpdateLabels(execCtx *ExecutionContext, request *kkComps.UpdateAuthServer,
	desired, current map[string]string,
) {
	mapPointerUpdateLabels(&request.Labels, execCtx, desired, current)
}

func (a *IdentityAuthServerAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	value, err := a.client.GetIdentityAuthServerByName(ctx, name)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func (a *IdentityAuthServerAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	value, err := a.client.GetIdentityAuthServerByID(ctx, id)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func authServerWriteContext(execCtx *ExecutionContext, namespace string) error {
	if execCtx == nil || execCtx.PlannedChange == nil || namespace == "" || execCtx.Namespace != namespace {
		return fmt.Errorf("auth-server execution namespace mismatch")
	}
	return nil
}

func validateObservedAuthServer(value *state.IdentityAuthServer, execCtx *ExecutionContext) error {
	if execCtx == nil {
		return fmt.Errorf("auth-server execution context required")
	}
	if err := authServerWriteContext(execCtx, execCtx.Namespace); err != nil {
		return err
	}
	if value.Name != execCtx.PlannedChange.Fields[planner.FieldName] {
		return fmt.Errorf("auth-server identity changed; regenerate the plan")
	}
	if value.Labels[labels.NamespaceKey] != execCtx.Namespace {
		return fmt.Errorf("auth-server namespace ownership changed; regenerate the plan or explicitly adopt it")
	}
	return nil
}

func (a *IdentityAuthServerAdapter) Create(ctx context.Context, request kkComps.CreateAuthServer,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	if err := authServerWriteContext(execCtx, namespace); err != nil {
		return "", err
	}
	if request.Labels[labels.NamespaceKey] != namespace {
		return "", fmt.Errorf("auth-server requires generated namespace labels")
	}
	current, err := a.client.GetIdentityAuthServerByName(ctx, request.Name)
	if err != nil {
		return "", err
	}
	if current != nil {
		return "", childCreationCollision("auth server", request.Name)
	}
	value, err := a.client.CreateIdentityAuthServer(ctx, request)
	if err != nil {
		return "", err
	}
	if value.Labels[labels.NamespaceKey] != namespace {
		return "", fmt.Errorf("auth server %s was created but ownership labels were not retained", value.ID)
	}
	return value.ID, nil
}

func (a *IdentityAuthServerAdapter) Update(ctx context.Context, id string, request kkComps.UpdateAuthServer,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	if err := authServerWriteContext(execCtx, namespace); err != nil {
		return "", err
	}
	current, err := a.client.GetIdentityAuthServerByID(ctx, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("auth server no longer exists")
	}
	if err := validateObservedAuthServer(current, execCtx); err != nil {
		return "", err
	}
	change := execCtx.PlannedChange
	isProtected := labels.IsProtectedResource(current.NormalizedLabels)
	if err := common.ValidateResourceProtection(
		a.ResourceType(), current.Name, isProtected, *change, common.IsProtectionChange(change.Protection),
	); err != nil {
		return "", err
	}
	if protection, ok := change.Protection.(planner.ProtectionChange); ok && protection.Old != isProtected {
		return "", fmt.Errorf("auth-server protection changed after planning; regenerate the plan")
	}
	if protection, ok := change.Protection.(map[string]any); ok {
		if old, ok := protection["old"].(bool); ok && old != isProtected {
			return "", fmt.Errorf("auth-server protection changed after planning; regenerate the plan")
		}
	}
	freshLabels := labels.GetUserLabels(current.Labels)
	desiredLabels := freshLabels
	if raw, explicit := change.Fields[planner.FieldLabels]; explicit {
		desiredLabels = labels.ExtractLabelsFromField(raw)
		if desiredLabels == nil {
			desiredLabels = map[string]string{}
		}
	}
	a.MapUpdateLabels(execCtx, &request, desiredLabels, freshLabels)
	if request.Name == nil || *request.Name != current.Name {
		return "", fmt.Errorf("auth-server name cannot change through a saved update")
	}
	if isProtected {
		before, err := authServerPayload(current.AuthServer)
		if err != nil {
			return "", err
		}
		after, err := authServerPayload(request)
		if err != nil {
			return "", err
		}
		for field := range change.Fields {
			if field == planner.FieldLabels {
				continue
			}
			if field == planner.FieldTrustedOrigins && len(current.TrustedOrigins) == 0 {
				before[field] = []any{}
			}
			if !reflect.DeepEqual(before[field], after[field]) {
				return "", fmt.Errorf("auth server is protected; unprotect it in a separate update")
			}
		}
		if !maps.Equal(freshLabels, labels.GetUserLabels(labels.NormalizeLabels(request.Labels))) {
			return "", fmt.Errorf("auth server is protected; unprotect it in a separate update")
		}
	}
	if err := a.client.UpdateIdentityAuthServer(ctx, id, request); err != nil {
		return "", err
	}
	return id, nil
}

func (a *IdentityAuthServerAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return fmt.Errorf("auth-server execution context required")
	}
	current, err := a.client.GetIdentityAuthServerByID(ctx, id)
	if err != nil || current == nil {
		return err
	}
	if err := validateObservedAuthServer(current, execCtx); err != nil {
		return err
	}
	if labels.IsProtectedResource(current.NormalizedLabels) {
		return fmt.Errorf("auth server is protected")
	}
	return a.client.DeleteIdentityAuthServer(ctx, id)
}

func authServerPayload(value any) (map[string]any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("invalid auth-server payload")
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, fmt.Errorf("invalid auth-server payload")
	}
	return payload, nil
}

func (e *Executor) registerIdentityAuthServerExecutor() {
	e.registerResourceExecutor(crudResourceExecutor(NewManagedLabelBaseExecutor(
		&IdentityAuthServerAdapter{client: e.client}, e.client, e.dryRun,
	)))
}

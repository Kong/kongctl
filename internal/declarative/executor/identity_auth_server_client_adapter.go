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
	"github.com/kong/kongctl/internal/util"
)

type identityAuthServerClientAdapter struct {
	client   *state.Client
	executor *Executor
}

func (a *identityAuthServerClientAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityAuthServerClient
}

func (a *identityAuthServerClientAdapter) RequiredFields() []string {
	return []string{planner.FieldName, planner.FieldGrantTypes, planner.FieldResponseTypes}
}
func (a *identityAuthServerClientAdapter) SupportsUpdate() bool { return true }

func mapIdentityClientFields(fields map[string]any, destination any) error {
	public := maps.Clone(fields)
	delete(public, planner.FieldCurrentLabels)
	delete(public, planner.FieldAuthServerID)
	for field, value := range public {
		switch field {
		case planner.FieldName, planner.FieldGrantTypes, planner.FieldResponseTypes, planner.FieldRedirectURIs,
			planner.FieldLoginURI, planner.FieldAccessTokenDuration, planner.FieldIDTokenDuration,
			planner.FieldAllowAllScopes, planner.FieldAllowScopes, planner.FieldTokenEndpointAuthMethod, planner.FieldLabels:
		default:
			return fmt.Errorf("unsupported client payload field %q", field)
		}
		if value == nil {
			return fmt.Errorf("client payload field %q cannot be null", field)
		}
	}
	data, err := json.Marshal(public)
	if err != nil {
		return fmt.Errorf("invalid client payload")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("invalid client payload fields")
	}
	return nil
}

func (a *identityAuthServerClientAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext,
	fields map[string]any, request *kkComps.CreateClient,
) error {
	if err := mapIdentityClientFields(fields, request); err != nil {
		return err
	}
	if request.GrantTypes == nil || request.ResponseTypes == nil {
		return fmt.Errorf("client grant_types and response_types are required")
	}
	request.Labels = labels.BuildCreateLabels(request.Labels, execCtx.Namespace, execCtx.Protection)
	return nil
}

func (a *identityAuthServerClientAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *kkComps.UpdateClient, _ map[string]string,
) error {
	return mapIdentityClientFields(fields, request)
}

func (a *identityAuthServerClientAdapter) MapUpdateLabels(execCtx *ExecutionContext, request *kkComps.UpdateClient,
	desired, current map[string]string,
) {
	mapPointerUpdateLabels(&request.Labels, execCtx, desired, current)
}

func (a *identityAuthServerClientAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func (a *identityAuthServerClientAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	parent, err := authChildParentID(execCtx)
	if err != nil {
		return nil, err
	}
	value, err := a.client.GetIdentityAuthServerClient(ctx, parent, id)
	if value == nil || err != nil {
		return nil, err
	}
	return value, nil
}

func (a *identityAuthServerClientAdapter) parent(ctx context.Context, execCtx *ExecutionContext) (string, error) {
	id, err := authChildParentID(execCtx)
	if err != nil {
		return "", err
	}
	if !util.IsValidUUID(id) || execCtx.Namespace == "" {
		return "", fmt.Errorf("client requires a valid parent and namespace")
	}
	current, err := a.client.GetIdentityAuthServerByID(ctx, id)
	if err != nil {
		return "", err
	}
	reference := execCtx.PlannedChange.References[planner.FieldAuthServerID]
	if current == nil || reference.LookupFields[planner.FieldName] == "" ||
		reference.LookupFields[planner.FieldName] != current.Name {
		return "", fmt.Errorf("client parent identity changed; regenerate the plan")
	}
	return id, nil
}

func validateObservedIdentityClient(current *state.IdentityAuthServerClient, execCtx *ExecutionContext) error {
	if current == nil || execCtx == nil || execCtx.PlannedChange == nil {
		return fmt.Errorf("client no longer exists")
	}
	if current.Name != execCtx.PlannedChange.Fields[planner.FieldName] {
		return fmt.Errorf("client identity changed; regenerate the plan")
	}
	if current.Labels[labels.NamespaceKey] != execCtx.Namespace || execCtx.Namespace == "" {
		return fmt.Errorf("client namespace ownership changed; regenerate the plan or explicitly adopt it")
	}
	if value := current.Labels[labels.ProtectedKey]; value != "" && value != "true" && value != "false" {
		return fmt.Errorf("client has invalid protection labels")
	}
	return nil
}

func (a *identityAuthServerClientAdapter) scopes(ctx context.Context, parent string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	values, err := a.client.ListIdentityAuthServerScopes(ctx, parent)
	if err != nil {
		return err
	}
	available, seen := make(map[string]bool), make(map[string]bool)
	for _, value := range values {
		available[value.ID] = true
	}
	for _, id := range ids {
		if !available[id] || seen[id] {
			return fmt.Errorf("client scope is absent, duplicated or belongs to another auth server")
		}
		seen[id] = true
	}
	return nil
}

func (a *identityAuthServerClientAdapter) uniqueName(ctx context.Context, parent, name, id string) error {
	values, err := a.client.ListIdentityAuthServerClients(ctx, parent)
	if err != nil {
		return err
	}
	count := 0
	for _, value := range values {
		if value.Name == name {
			count++
			if id == "" || value.ID != id {
				return childCreationCollision("auth-server client", name)
			}
		}
	}
	if count > 1 {
		return fmt.Errorf("multiple clients match name %q", name)
	}
	if id != "" && count != 1 {
		return fmt.Errorf("client identity changed; regenerate the plan")
	}
	return nil
}

func (a *identityAuthServerClientAdapter) Create(ctx context.Context, request kkComps.CreateClient,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	parent, err := a.parent(ctx, execCtx)
	if err != nil {
		return "", err
	}
	if namespace != execCtx.Namespace || request.Labels[labels.NamespaceKey] != namespace {
		return "", fmt.Errorf("client namespace mismatch")
	}
	if err := a.uniqueName(ctx, parent, request.Name, ""); err != nil {
		return "", err
	}
	if err := a.scopes(ctx, parent, request.AllowScopes); err != nil {
		return "", err
	}
	output, err := a.executor.initialClientOutput(execCtx.PlannedChange)
	if err != nil {
		return "", err
	}
	value, err := a.client.CreateIdentityAuthServerClient(ctx, parent, request, output.file, output.metadata)
	if err != nil {
		return "", err
	}
	if value.Labels[labels.NamespaceKey] != namespace ||
		labels.IsProtectedResource(value.Labels) != labels.IsProtectedResource(request.Labels) {
		return "", fmt.Errorf(
			"client %q was created and its credential captured, but ownership/protection labels were not retained",
			value.ID,
		)
	}
	return value.ID, nil
}

func (a *identityAuthServerClientAdapter) Update(ctx context.Context, id string, request kkComps.UpdateClient,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	parent, err := a.parent(ctx, execCtx)
	if err != nil {
		return "", err
	}
	current, err := a.client.GetIdentityAuthServerClient(ctx, parent, id)
	if err != nil {
		return "", err
	}
	if err := validateObservedIdentityClient(current, execCtx); err != nil {
		return "", err
	}
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("client namespace mismatch")
	}
	if err := a.uniqueName(ctx, parent, current.Name, id); err != nil {
		return "", err
	}
	change := execCtx.PlannedChange
	protected := labels.IsProtectedResource(current.Labels)
	if err := common.ValidateResourceProtection(a.ResourceType(), current.Name, protected, *change,
		common.IsProtectionChange(change.Protection)); err != nil {
		return "", err
	}
	if request.Name == nil || *request.Name != current.Name || request.ClientSecret != nil {
		return "", fmt.Errorf("client updates cannot rename the selected record or replace secret material")
	}
	if old, _, changing := plannedProtectionTransition(change.Protection); changing && old != protected {
		return "", fmt.Errorf("client protection changed after planning; regenerate the plan")
	}
	if protected {
		payload, err := json.Marshal(current.AuthServerClient)
		if err != nil {
			return "", fmt.Errorf("invalid observed client")
		}
		var before map[string]any
		if err := json.Unmarshal(payload, &before); err != nil {
			return "", err
		}
		for field, next := range change.Fields {
			if field == planner.FieldLabels || field == planner.FieldCurrentLabels ||
				field == planner.FieldAuthServerID {
				continue
			}
			encoded, err := json.Marshal(next)
			if err != nil {
				return "", err
			}
			var normalized any
			if err := json.Unmarshal(encoded, &normalized); err != nil {
				return "", err
			}
			if !reflect.DeepEqual(before[field], normalized) {
				return "", fmt.Errorf("protected client requires metadata-only unprotection")
			}
		}
		if raw, explicit := change.Fields[planner.FieldLabels]; explicit &&
			!maps.Equal(labels.GetUserLabels(current.Labels), labels.ExtractLabelsFromField(raw)) {
			return "", fmt.Errorf("protected client requires unchanged public labels during unprotection")
		}
	}
	if request.TokenEndpointAuthMethod == nil {
		if current.TokenEndpointAuthMethod == nil {
			return "", fmt.Errorf("client authentication method is unavailable; cannot preserve it")
		}
		request.TokenEndpointAuthMethod = current.TokenEndpointAuthMethod
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
	if err := a.scopes(ctx, parent, request.AllowScopes); err != nil {
		return "", err
	}
	value, err := a.client.UpdateIdentityAuthServerClient(ctx, parent, id, request)
	if err != nil {
		return "", err
	}
	expectedProtected := protected
	if _, next, changing := plannedProtectionTransition(execCtx.Protection); changing {
		expectedProtected = next
	} else if next, explicit := execCtx.Protection.(bool); explicit {
		expectedProtected = next
	}
	if value.Labels[labels.NamespaceKey] != namespace ||
		labels.IsProtectedResource(value.Labels) != expectedProtected ||
		value.TokenEndpointAuthMethod == nil ||
		*value.TokenEndpointAuthMethod != *request.TokenEndpointAuthMethod {
		return "", fmt.Errorf("client update did not retain ownership or authentication method; inspect the record")
	}
	return value.ID, nil
}

func (a *identityAuthServerClientAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	parent, err := a.parent(ctx, execCtx)
	if err != nil {
		return err
	}
	current, err := a.client.GetIdentityAuthServerClient(ctx, parent, id)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	if err := validateObservedIdentityClient(current, execCtx); err != nil {
		return err
	}
	if labels.IsProtectedResource(current.Labels) {
		return fmt.Errorf("client is protected and cannot be deleted")
	}
	if err := a.uniqueName(ctx, parent, current.Name, id); err != nil {
		return err
	}
	return a.client.DeleteIdentityAuthServerClient(ctx, parent, id)
}

func (e *Executor) registerIdentityAuthServerClientExecutor() {
	adapter := &identityAuthServerClientAdapter{client: e.client, executor: e}
	e.registerResourceExecutor(crudResourceExecutor(NewManagedLabelBaseExecutor(adapter, e.client, e.dryRun)))
}

func plannedProtectionTransition(raw any) (bool, bool, bool) {
	switch value := raw.(type) {
	case planner.ProtectionChange:
		return value.Old, value.New, true
	case map[string]any:
		old, ok := value["old"].(bool)
		next, exists := value["new"].(bool)
		return old, next, ok && exists
	default:
		return false, false, false
	}
}

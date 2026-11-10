package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
)

type identityVaultProviderCredentialAdapter struct{ client *state.Client }

func (a *identityVaultProviderCredentialAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityVaultProviderCredential
}

func (a *identityVaultProviderCredentialAdapter) RequiredFields() []string {
	return []string{planner.FieldProviderName}
}
func (a *identityVaultProviderCredentialAdapter) SupportsUpdate() bool { return true }

func vaultCredentialPayload(fields map[string]any, secret bool) (kkComps.SetVaultProviderCredentialBody, error) {
	var request kkComps.SetVaultProviderCredentialBody
	public := make(map[string]any)
	for field, value := range fields {
		switch field {
		case planner.FieldProviderName,
			planner.FieldExpectedSelector,
			planner.FieldExpectedPriority,
			planner.FieldDirectoryID:
		case planner.FieldSelector, planner.FieldPriority, planner.FieldSecret:
			if value == nil {
				return request, fmt.Errorf("vault credential request fields cannot be null")
			}
			public[field] = value
		default:
			return request, fmt.Errorf("unsupported vault credential field %q", field)
		}
	}
	data, err := json.Marshal(public)
	if err != nil {
		return request, fmt.Errorf("invalid vault credential payload")
	}
	if json.Unmarshal(data, &request) != nil {
		return request, fmt.Errorf("invalid vault credential field types")
	}
	routing := helpers.IdentityVaultCredentialRouting{Selector: request.Selector, Priority: request.Priority}
	if err := routing.Validate(); err != nil {
		return request, err
	}
	if secret && request.Secret == nil {
		return request, fmt.Errorf("vault credential write requires explicit secret material")
	}
	if !secret && request.Secret != nil {
		return request, fmt.Errorf("credential deletion cannot include secret material")
	}
	return request, nil
}

func (a *identityVaultProviderCredentialAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	request *kkComps.SetVaultProviderCredentialBody,
) error {
	value, err := vaultCredentialPayload(fields, true)
	*request = value
	return err
}

func (a *identityVaultProviderCredentialAdapter) MapUpdateFields(
	ctx context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	request *kkComps.SetVaultProviderCredentialBody,
	_ map[string]string,
) error {
	return a.MapCreateFields(ctx, execCtx, fields, request)
}

func (a *identityVaultProviderCredentialAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func vaultCredentialScope(execCtx *ExecutionContext) (helpers.IdentityStaticVaultCredentialScope, error) {
	var scope helpers.IdentityStaticVaultCredentialScope
	if execCtx == nil || execCtx.PlannedChange == nil || execCtx.PlannedChange.Parent == nil {
		return scope, fmt.Errorf("vault credential requires a resolved provider")
	}
	change := execCtx.PlannedChange
	scope.DirectoryID = change.References[planner.FieldDirectoryID].ID
	scope.ProviderID = change.Parent.ID
	scope.ProviderName, _ = change.Fields[planner.FieldProviderName].(string)
	if field, ok := change.Fields[planner.FieldDirectoryID]; ok && field != scope.DirectoryID {
		return scope, fmt.Errorf("vault credential directory reference mismatch")
	}
	if !util.IsValidUUID(scope.DirectoryID) || !util.IsValidUUID(scope.ProviderID) || scope.ProviderName == "" {
		return scope, fmt.Errorf("vault credential requires directory/provider UUIDs and provider name")
	}
	return scope, nil
}

func (a *identityVaultProviderCredentialAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	scope, err := vaultCredentialScope(execCtx)
	if err != nil {
		return nil, err
	}
	values, err := a.client.ListIdentityVaultProviderCredentials(ctx, scope.DirectoryID, scope.ProviderID)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.ID == id {
			return value, nil
		}
	}
	return nil, nil
}

func (a *identityVaultProviderCredentialAdapter) authorize(
	ctx context.Context,
	execCtx *ExecutionContext,
	write bool,
) (helpers.IdentityStaticVaultCredentialScope, error) {
	scope, err := vaultCredentialScope(execCtx)
	if err != nil {
		return scope, err
	}
	_, err = identityVaultWriteDirectoryByID(ctx, a.client, execCtx, "vault credential", scope.DirectoryID)
	if err != nil {
		return scope, err
	}
	if write {
		change := execCtx.PlannedChange
		if len(change.SecretWrites) != 1 || change.SecretWrites[0].Field != "/secret" {
			return scope, fmt.Errorf("vault credential write requires explicit secret intent")
		}
		keys, err := a.client.ListIdentityVaultKeys(ctx, scope.DirectoryID)
		if err != nil {
			return scope, err
		}
		if len(keys) == 0 {
			return scope, fmt.Errorf("vault credential requires a registered encryption key")
		}
	}
	return scope, nil
}

func expectedVaultCredentialRouting(fields map[string]any) (helpers.IdentityVaultCredentialRouting, error) {
	var expected helpers.IdentityVaultCredentialRouting
	selector, selectorOK := fields[planner.FieldExpectedSelector]
	priority, priorityOK := fields[planner.FieldExpectedPriority]
	if !selectorOK || !priorityOK {
		return expected, fmt.Errorf("vault credential requires an expected routing snapshot")
	}
	data, err := json.Marshal(map[string]any{"selector": selector, "priority": priority})
	if err != nil {
		return expected, fmt.Errorf("invalid expected credential routing")
	}
	var request kkComps.SetVaultProviderCredentialBody
	if json.Unmarshal(data, &request) != nil {
		return expected, fmt.Errorf("invalid expected credential routing types")
	}
	expected.Selector, expected.Priority = request.Selector, request.Priority
	return expected, expected.Validate()
}

func (a *identityVaultProviderCredentialAdapter) Create(
	ctx context.Context,
	request kkComps.SetVaultProviderCredentialBody,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("vault credential namespace mismatch")
	}
	scope, err := a.authorize(ctx, execCtx, true)
	if err != nil {
		return "", err
	}
	if request.Secret == nil {
		return "", fmt.Errorf("vault credential material is missing")
	}
	value, err := a.client.CreateIdentityStaticVaultCredential(
		ctx,
		scope,
		helpers.IdentityVaultCredentialRouting{Selector: request.Selector, Priority: request.Priority},
		*request.Secret,
	)
	if err != nil {
		return "", err
	}
	return value.ID, nil
}

func (a *identityVaultProviderCredentialAdapter) Update(
	ctx context.Context,
	id string,
	request kkComps.SetVaultProviderCredentialBody,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("vault credential namespace mismatch")
	}
	scope, err := a.authorize(ctx, execCtx, true)
	if err != nil {
		return "", err
	}
	expected, err := expectedVaultCredentialRouting(execCtx.PlannedChange.Fields)
	if err != nil {
		return "", err
	}
	if request.Secret == nil {
		return "", fmt.Errorf("vault credential material is missing")
	}
	// Routing priority is the declaration's moniker. Changing it creates another
	// resource; a saved update cannot silently move a credential to another slot.
	if (expected.Priority == nil) != (request.Priority == nil) ||
		expected.Priority != nil && *expected.Priority != *request.Priority {
		return "", fmt.Errorf("credential update cannot change its routing slot")
	}
	value, err := a.client.ReplaceIdentityStaticVaultCredential(
		ctx,
		scope,
		id,
		expected,
		helpers.IdentityVaultCredentialRouting{Selector: request.Selector, Priority: request.Priority},
		*request.Secret,
	)
	if err != nil {
		return "", err
	}
	return value.ID, nil
}

func (a *identityVaultProviderCredentialAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	scope, err := a.authorize(ctx, execCtx, false)
	if err != nil {
		return err
	}
	request, err := vaultCredentialPayload(execCtx.PlannedChange.Fields, false)
	if err != nil {
		return err
	}
	expected, err := expectedVaultCredentialRouting(execCtx.PlannedChange.Fields)
	if err != nil {
		return err
	}
	if (expected.Priority == nil) != (request.Priority == nil) ||
		expected.Priority != nil && *expected.Priority != *request.Priority ||
		(expected.Selector == nil) != (request.Selector == nil) ||
		expected.Selector != nil && *expected.Selector != *request.Selector {
		return fmt.Errorf("credential deletion routing differs from its snapshot")
	}
	return a.client.DeleteIdentityStaticVaultCredential(ctx, scope, id, expected)
}

func (e *Executor) registerIdentityVaultProviderCredentialExecutor() {
	e.registerResourceExecutor(
		crudResourceExecutor(
			NewBaseExecutor(&identityVaultProviderCredentialAdapter{client: e.client}, e.client, e.dryRun),
		),
	)
}

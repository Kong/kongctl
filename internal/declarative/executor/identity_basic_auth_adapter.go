package executor

import (
	"context"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
)

type IdentityBasicAuthAdapter struct{ client *state.Client }

func (a *IdentityBasicAuthAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityBasicAuth
}

func (a *IdentityBasicAuthAdapter) RequiredFields() []string {
	return []string{planner.FieldUsername, planner.FieldManagedBy}
}
func (a *IdentityBasicAuthAdapter) SupportsUpdate() bool { return true }

func mapIdentityBasicAuthFields(fields map[string]any, destination any) error {
	for field, value := range fields {
		if value == nil || field != planner.FieldUsername && field != planner.FieldManagedBy {
			return fmt.Errorf("unsupported or null basic-auth payload field %q", field)
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid basic-auth payload")
	}
	var declaration resources.IdentityBasicAuthResource
	if err := json.Unmarshal(data, &declaration); err != nil {
		return fmt.Errorf("invalid basic-auth payload fields")
	}
	declaration.Ref = "basic-auth-payload"
	if err := declaration.Validate(); err != nil {
		return err
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("invalid basic-auth payload fields")
	}
	return nil
}

func (a *IdentityBasicAuthAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.CreateBasicAuthBody,
) error {
	return mapIdentityBasicAuthFields(fields, request)
}

func (a *IdentityBasicAuthAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.ReplaceBasicAuthBody, _ map[string]string,
) error {
	return mapIdentityBasicAuthFields(fields, request)
}

func identityBasicAuthPath(execCtx *ExecutionContext) (string, string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil || execCtx.PlannedChange.Parent == nil {
		return "", "", fmt.Errorf("basic-auth requires directory and principal parents")
	}
	directory, principal := execCtx.PlannedChange.References[planner.FieldDirectoryID].ID, execCtx.PlannedChange.Parent.ID
	if !util.IsValidUUID(directory) || !util.IsValidUUID(principal) {
		return "", "", fmt.Errorf("basic-auth requires resolved directory and principal UUIDs")
	}
	return directory, principal, nil
}

func (a *IdentityBasicAuthAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func (a *IdentityBasicAuthAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	directory, principal, err := identityBasicAuthPath(execCtx)
	if err != nil {
		return nil, err
	}
	value, err := a.client.GetIdentityBasicAuth(ctx, directory, principal, id)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func (a *IdentityBasicAuthAdapter) validateCurrent(current *state.IdentityBasicAuth, execCtx *ExecutionContext) error {
	if err := resources.ValidateChildOwnership("basic-auth", current.ManagedBy, execCtx.Namespace); err != nil {
		return err
	}
	if username, ok := execCtx.PlannedChange.Fields[planner.FieldUsername].(string); !ok ||
		username != current.Username {
		return fmt.Errorf("basic-auth username changed from the planned identity; regenerate the plan")
	}
	return validateChildPlannedProtection("basic-auth", current.ManagedBy, execCtx.PlannedChange)
}

func (a *IdentityBasicAuthAdapter) Create(ctx context.Context, request kkComps.CreateBasicAuthBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	directory, principal, err := identityBasicAuthPath(execCtx)
	if err != nil {
		return "", err
	}
	if err := validateDirectoryWriteMetadata(request.ManagedBy, namespace, execCtx); err != nil {
		return "", err
	}
	parent, err := a.client.GetIdentityPrincipal(ctx, directory, principal)
	if err != nil {
		return "", err
	}
	if parent == nil {
		return "", fmt.Errorf("basic-auth parent principal no longer exists in the selected directory")
	}
	values, err := a.client.ListIdentityBasicAuths(ctx, directory, principal)
	if err != nil {
		return "", err
	}
	for _, value := range values {
		if value.Username == request.Username {
			return "", childCreationCollision("basic-auth", request.Username)
		}
	}
	created, err := a.client.CreateIdentityBasicAuth(ctx, directory, principal, request)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func (a *IdentityBasicAuthAdapter) Update(ctx context.Context, id string, request kkComps.ReplaceBasicAuthBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	directory, principal, err := identityBasicAuthPath(execCtx)
	if err != nil {
		return "", err
	}
	if err := validateDirectoryWriteMetadata(request.ManagedBy, namespace, execCtx); err != nil {
		return "", err
	}
	current, err := a.client.GetIdentityBasicAuth(ctx, directory, principal, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("basic-auth credential no longer exists")
	}
	if err := a.validateCurrent(current, execCtx); err != nil {
		return "", err
	}
	if request.Username != current.Username {
		return "", fmt.Errorf("basic-auth username is its lookup identity; declare a separate credential to change it")
	}
	if resources.IsChildProtected(current.ManagedBy) &&
		!childUnprotectOnly(execCtx.PlannedChange, current.ManagedBy, request.ManagedBy) {
		return "", fmt.Errorf("basic-auth is protected; unprotect it in a separate update")
	}
	full := helpers.IdentityBasicAuthReplacement(current.IdentityBasicAuth)
	full.ManagedBy = resources.BuildChildManagedBy(namespace, resources.IsChildProtected(request.ManagedBy), "")
	if err := a.client.ReplaceIdentityBasicAuth(ctx, directory, principal, id, full); err != nil {
		return "", err
	}
	return id, nil
}

func (a *IdentityBasicAuthAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	directory, principal, err := identityBasicAuthPath(execCtx)
	if err != nil {
		return err
	}
	current, err := a.client.GetIdentityBasicAuth(ctx, directory, principal, id)
	if err != nil || current == nil {
		return err
	}
	if err := a.validateCurrent(current, execCtx); err != nil {
		return err
	}
	if resources.IsChildProtected(current.ManagedBy) {
		return fmt.Errorf("basic-auth is protected")
	}
	return a.client.DeleteIdentityBasicAuth(ctx, directory, principal, id)
}

func (e *Executor) registerIdentityBasicAuthExecutor() {
	e.registerResourceExecutor(
		crudResourceExecutor(NewBaseExecutor(&IdentityBasicAuthAdapter{client: e.client}, e.client, e.dryRun)),
	)
}

package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
)

type IdentityPrincipalIdentityAdapter struct{ client *state.Client }

func (a *IdentityPrincipalIdentityAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityPrincipalIdentity
}

func (a *IdentityPrincipalIdentityAdapter) RequiredFields() []string {
	return []string{planner.FieldType, planner.FieldManagedBy}
}
func (a *IdentityPrincipalIdentityAdapter) SupportsUpdate() bool { return true }

func mapPrincipalIdentity(fields map[string]any, body *kkComps.KongPrincipalIdentityBody) error {
	for key, value := range fields {
		if value == nil || !slices.Contains([]string{
			planner.FieldType, planner.FieldIssuer, planner.FieldClaim,
			planner.FieldAuthServerID, planner.FieldClientID, planner.FieldControlPlaneID, planner.FieldConsumerID,
			planner.FieldWorkspaceID, planner.FieldKey, planner.FieldValue, planner.FieldLabels, planner.FieldManagedBy,
		}, key) {
			return fmt.Errorf("unsupported or null principal identity field %q", key)
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid principal identity payload")
	}
	var declaration resources.IdentityPrincipalIdentityResource
	if err := json.Unmarshal(data, &declaration); err != nil {
		return fmt.Errorf("invalid principal identity fields")
	}
	declaration.Ref = "payload-validation"
	if err := declaration.Validate(); err != nil {
		return err
	}
	if err := json.Unmarshal(data, body); err != nil {
		return fmt.Errorf("invalid principal identity request")
	}
	return nil
}

func (a *IdentityPrincipalIdentityAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *kkComps.KongPrincipalIdentityBody,
) error {
	return mapPrincipalIdentity(fields, request)
}

func (a *IdentityPrincipalIdentityAdapter) MapUpdateFields(_ context.Context, _ *ExecutionContext,
	fields map[string]any, request *kkComps.KongPrincipalIdentityBody, _ map[string]string,
) error {
	return mapPrincipalIdentity(fields, request)
}

func principalIdentityPath(execCtx *ExecutionContext) (string, string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil || execCtx.PlannedChange.Parent == nil {
		return "", "", fmt.Errorf("principal identity requires directory and principal parents")
	}
	directoryID := execCtx.PlannedChange.References[planner.FieldDirectoryID].ID
	principalID := execCtx.PlannedChange.Parent.ID
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(principalID) {
		return "", "", fmt.Errorf("principal identity requires resolved directory and principal UUIDs")
	}
	return directoryID, principalID, nil
}

func (a *IdentityPrincipalIdentityAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func (a *IdentityPrincipalIdentityAdapter) GetByID(ctx context.Context, id string, execCtx *ExecutionContext,
) (ResourceInfo, error) {
	directory, principal, err := principalIdentityPath(execCtx)
	if err != nil {
		return nil, err
	}
	v, err := a.client.GetIdentityPrincipalIdentity(ctx, directory, principal, id)
	if err != nil || v == nil {
		return nil, err
	}
	return v, nil
}

func (a *IdentityPrincipalIdentityAdapter) parent(ctx context.Context, directoryID, principalID string) error {
	v, err := a.client.GetIdentityPrincipal(ctx, directoryID, principalID)
	if err != nil {
		return err
	}
	if v == nil || v.ID != principalID {
		return fmt.Errorf("identity parent principal no longer exists in the selected directory")
	}
	return nil
}

func identityRequestMetadata(body kkComps.KongPrincipalIdentityBody) (map[string]string, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("invalid principal identity payload")
	}
	var value struct {
		ManagedBy map[string]string `json:"managed_by"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value.ManagedBy, nil
}

func validateIdentityMetadata(metadata map[string]string, namespace string, execCtx *ExecutionContext) error {
	if err := validateDirectoryWriteMetadata(metadata, namespace, execCtx); err != nil {
		return err
	}
	if metadata[resources.ManagedByRefKey] != execCtx.PlannedChange.ResourceRef {
		return fmt.Errorf("principal identity persisted ref does not match the plan")
	}
	return resources.ValidateRef(metadata[resources.ManagedByRefKey])
}

func (a *IdentityPrincipalIdentityAdapter) validateUnique(ctx context.Context, directoryID, principalID string,
	current *state.IdentityPrincipalIdentity, execCtx *ExecutionContext,
) error {
	if current.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool ||
		current.GetName() != execCtx.PlannedChange.ResourceRef {
		return fmt.Errorf("principal identity ownership changed; regenerate the plan or explicitly adopt it")
	}
	if err := resources.ValidateChildOwnership("principal identity", current.ManagedBy, execCtx.Namespace); err != nil {
		return err
	}
	all, err := a.client.ListIdentityPrincipalIdentities(ctx, directoryID, principalID)
	if err != nil {
		return err
	}
	matches := 0
	for _, value := range all {
		if value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
			value.ManagedBy[resources.ChildNamespaceKey] == execCtx.Namespace &&
			value.ManagedBy[resources.ManagedByRefKey] == execCtx.PlannedChange.ResourceRef {
			matches++
			if value.ID != current.ID {
				return fmt.Errorf("multiple identities match the planned ownership identity")
			}
		}
	}
	if matches != 1 {
		return fmt.Errorf("principal identity lookup changed; regenerate the plan")
	}
	return nil
}

func (a *IdentityPrincipalIdentityAdapter) Create(ctx context.Context, body kkComps.KongPrincipalIdentityBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	directory, principal, err := principalIdentityPath(execCtx)
	if err != nil {
		return "", err
	}
	if err := a.parent(ctx, directory, principal); err != nil {
		return "", err
	}
	metadata, err := identityRequestMetadata(body)
	if err != nil {
		return "", err
	}
	if err := validateIdentityMetadata(metadata, namespace, execCtx); err != nil {
		return "", err
	}
	all, err := a.client.ListIdentityPrincipalIdentities(ctx, directory, principal)
	if err != nil {
		return "", err
	}
	for _, value := range all {
		if value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
			value.ManagedBy[resources.ChildNamespaceKey] == namespace &&
			value.ManagedBy[resources.ManagedByRefKey] == execCtx.PlannedChange.ResourceRef {
			return "", childCreationCollision("principal identity", execCtx.PlannedChange.ResourceRef)
		}
	}
	v, err := a.client.CreateIdentityPrincipalIdentity(ctx, directory, principal, body)
	if err != nil {
		return "", err
	}
	return v.ID, nil
}

func (a *IdentityPrincipalIdentityAdapter) Update(
	ctx context.Context,
	id string,
	body kkComps.KongPrincipalIdentityBody,
	namespace string,
	execCtx *ExecutionContext,
) (string, error) {
	directory, principal, err := principalIdentityPath(execCtx)
	if err != nil {
		return "", err
	}
	if err := a.parent(ctx, directory, principal); err != nil {
		return "", err
	}
	metadata, err := identityRequestMetadata(body)
	if err != nil {
		return "", err
	}
	if err := validateIdentityMetadata(metadata, namespace, execCtx); err != nil {
		return "", err
	}
	current, err := a.client.GetIdentityPrincipalIdentity(ctx, directory, principal, id)
	if err != nil {
		return "", err
	}
	if current == nil {
		return "", fmt.Errorf("principal identity no longer exists")
	}
	if err := a.validateUnique(ctx, directory, principal, current, execCtx); err != nil {
		return "", err
	}
	if err := validateChildPlannedProtection("principal identity", current.ManagedBy, execCtx.PlannedChange); err != nil {
		return "", err
	}
	if current.Type != string(body.Type) {
		return "", fmt.Errorf("principal identity type cannot be changed")
	}
	fresh, err := helpers.PrincipalIdentityReplacement(current.PrincipalIdentity)
	if err != nil {
		return "", err
	}
	before, err := principalPayload(fresh)
	if err != nil {
		return "", err
	}
	mapped, err := principalPayload(body)
	if err != nil {
		return "", err
	}
	after := maps.Clone(before)
	for field, raw := range execCtx.PlannedChange.Fields {
		if v, present := mapped[field]; present {
			after[field] = v
		} else {
			after[field] = raw
		}
	}
	if resources.IsChildProtected(current.ManagedBy) {
		// Compare canonical PUT payloads: SDK omission of an empty labels map
		// must not turn a metadata-only unprotection into a public-field change.
		var candidate kkComps.KongPrincipalIdentityBody
		if err := mapPrincipalIdentity(after, &candidate); err != nil {
			return "", err
		}
		normalized, err := principalPayload(candidate)
		if err != nil {
			return "", err
		}
		oldPublic, nextPublic := maps.Clone(before), maps.Clone(normalized)
		delete(oldPublic, planner.FieldManagedBy)
		delete(nextPublic, planner.FieldManagedBy)
		if !childUnprotectOnly(execCtx.PlannedChange, current.ManagedBy, metadata) ||
			!reflect.DeepEqual(oldPublic, nextPublic) {
			return "", fmt.Errorf("principal identity is protected; unprotect it in a separate update")
		}
	}
	after[planner.FieldManagedBy] = resources.BuildPrincipalManagedBy(namespace, execCtx.PlannedChange.ResourceRef,
		resources.IsChildProtected(metadata), "")
	var replacement kkComps.KongPrincipalIdentityBody
	if err := mapPrincipalIdentity(after, &replacement); err != nil {
		return "", err
	}
	if err := a.client.ReplaceIdentityPrincipalIdentity(ctx, directory, principal, id, replacement); err != nil {
		return "", err
	}
	return id, nil
}

func (a *IdentityPrincipalIdentityAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	directory, principal, err := principalIdentityPath(execCtx)
	if err != nil {
		return err
	}
	current, err := a.client.GetIdentityPrincipalIdentity(ctx, directory, principal, id)
	if err != nil || current == nil {
		return err
	}
	if err := a.validateUnique(ctx, directory, principal, current, execCtx); err != nil {
		return err
	}
	if resources.IsChildProtected(current.ManagedBy) {
		return fmt.Errorf("principal identity is protected")
	}
	return a.client.DeleteIdentityPrincipalIdentity(ctx, directory, principal, id)
}

func (e *Executor) registerIdentityPrincipalIdentityExecutor() {
	e.registerResourceExecutor(
		crudResourceExecutor(NewBaseExecutor(&IdentityPrincipalIdentityAdapter{client: e.client}, e.client, e.dryRun)),
	)
}

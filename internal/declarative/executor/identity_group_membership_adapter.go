package executor

import (
	"bytes"
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

type identityGroupMembershipAdapter struct{ client *state.Client }

func (a *identityGroupMembershipAdapter) ResourceType() string {
	return planner.ResourceTypeIdentityGroupMembership
}

func (a *identityGroupMembershipAdapter) RequiredFields() []string {
	return []string{planner.FieldMember}
}
func (a *identityGroupMembershipAdapter) SupportsUpdate() bool { return false }

func mapIdentityMembership(fields map[string]any, request *kkComps.CreateGroupMembershipEntryBody) error {
	if len(fields) != 1 || fields[planner.FieldMember] == nil {
		return fmt.Errorf("membership payload requires only member")
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid membership payload")
	}
	var declaration struct {
		Member resources.IdentityGroupMemberResource `json:"member"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&declaration); err != nil {
		return fmt.Errorf("invalid membership member")
	}
	resource := resources.IdentityGroupMembershipResource{Ref: "membership-payload", Member: declaration.Member}
	if err := resource.Validate(); err != nil {
		return err
	}
	if err := json.Unmarshal(data, request); err != nil {
		return fmt.Errorf("invalid membership payload")
	}
	return nil
}

func (a *identityGroupMembershipAdapter) MapCreateFields(_ context.Context, _ *ExecutionContext, fields map[string]any,
	request *kkComps.CreateGroupMembershipEntryBody,
) error {
	return mapIdentityMembership(fields, request)
}

func (a *identityGroupMembershipAdapter) MapUpdateFields(context.Context, *ExecutionContext, map[string]any,
	*kkComps.CreateGroupMembershipEntryBody, map[string]string,
) error {
	return fmt.Errorf("memberships have no update operation; use add/remove")
}

func (a *identityGroupMembershipAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func identityMembershipPath(execCtx *ExecutionContext) (string, string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil || execCtx.PlannedChange.Parent == nil {
		return "", "", fmt.Errorf("membership requires directory and group")
	}
	directoryID := execCtx.PlannedChange.References[planner.FieldDirectoryID].ID
	groupID := execCtx.PlannedChange.Parent.ID
	if !util.IsValidUUID(directoryID) || !util.IsValidUUID(groupID) {
		return "", "", fmt.Errorf("membership requires resolved directory and group UUIDs")
	}
	return directoryID, groupID, nil
}

func (a *identityGroupMembershipAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	directoryID, groupID, err := identityMembershipPath(execCtx)
	if err != nil {
		return nil, err
	}
	value, err := a.client.GetIdentityGroupMembership(ctx, directoryID, groupID, id)
	if err != nil || value == nil {
		return nil, err
	}
	return value, nil
}

func (a *identityGroupMembershipAdapter) parent(
	ctx context.Context,
	execCtx *ExecutionContext,
) (string, string, error) {
	directoryID, groupID, err := identityMembershipPath(execCtx)
	if err != nil {
		return "", "", err
	}
	group, err := a.client.GetIdentityGroup(ctx, directoryID, groupID)
	if err != nil {
		return "", "", err
	}
	if group == nil {
		return "", "", fmt.Errorf("membership group no longer exists")
	}
	if err := helpers.ValidateIdentityGroupManagement(group.IdentityGroup); err != nil {
		return "", "", err
	}
	if err := resources.ValidateChildOwnership("membership group", group.ManagedBy, execCtx.Namespace); err != nil {
		return "", "", err
	}
	parent := execCtx.PlannedChange.ProtectingParent
	if parent == nil || parent.ResourceType != planner.ResourceTypeIdentityGroup || parent.ResourceName != group.Name ||
		parent.ResourceID != "" && parent.ResourceID != groupID {
		return "", "", fmt.Errorf("membership group identity changed; regenerate the plan")
	}
	if execCtx.PlannedChange.Action != planner.ActionCreate && resources.IsChildProtected(group.ManagedBy) {
		return "", "", fmt.Errorf("group %q is protected", group.Name)
	}
	return directoryID, groupID, nil
}

func (a *identityGroupMembershipAdapter) Create(ctx context.Context, request kkComps.CreateGroupMembershipEntryBody,
	namespace string, execCtx *ExecutionContext,
) (string, error) {
	if namespace != execCtx.Namespace {
		return "", fmt.Errorf("membership namespace mismatch")
	}
	directoryID, groupID, err := a.parent(ctx, execCtx)
	if err != nil {
		return "", err
	}
	created, err := a.client.CreateIdentityGroupMembership(ctx, directoryID, groupID, request)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func (a *identityGroupMembershipAdapter) Update(
	context.Context,
	string,
	kkComps.CreateGroupMembershipEntryBody,
	string,
	*ExecutionContext,
) (string, error) {
	return "", fmt.Errorf("memberships have no update operation")
}

func (a *identityGroupMembershipAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	directoryID, groupID, err := a.parent(ctx, execCtx)
	if err != nil {
		return err
	}
	current, err := a.client.GetIdentityGroupMembership(ctx, directoryID, groupID, id)
	if err != nil || current == nil {
		return err
	}
	var requested kkComps.CreateGroupMembershipEntryBody
	if err := mapIdentityMembership(execCtx.PlannedChange.Fields, &requested); err != nil {
		return err
	}
	if current.Member.ID != requested.Member.ID || current.Member.Type != requested.Member.Type {
		return fmt.Errorf("membership identity changed; regenerate the plan")
	}
	return a.client.DeleteIdentityGroupMembership(ctx, directoryID, groupID, id)
}

func (e *Executor) registerIdentityGroupMembershipExecutor() {
	e.registerResourceExecutor(
		createDeleteResourceExecutor(
			NewBaseExecutor(&identityGroupMembershipAdapter{client: e.client}, e.client, e.dryRun),
		),
	)
}

package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// OrganizationUserTeamMembershipAdapter implements create/delete operations for user team memberships.
type OrganizationUserTeamMembershipAdapter struct {
	client *state.Client
}

func NewOrganizationUserTeamMembershipAdapter(client *state.Client) *OrganizationUserTeamMembershipAdapter {
	return &OrganizationUserTeamMembershipAdapter{client: client}
}

func (o *OrganizationUserTeamMembershipAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *state.OrganizationUserTeamMembership,
) error {
	userID, _ := fields[planner.FieldUserID].(string)
	teamID, _ := fields[planner.FieldTeamID].(string)
	create.UserID = userID
	create.TeamID = teamID
	return nil
}

func (o *OrganizationUserTeamMembershipAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	_ map[string]any,
	_ *state.OrganizationUserTeamMembership,
	_ map[string]string,
) error {
	return nil
}

func (o *OrganizationUserTeamMembershipAdapter) Create(
	ctx context.Context,
	req state.OrganizationUserTeamMembership,
	_ string,
	execCtx *ExecutionContext,
) (string, error) {
	userID, teamID, err := o.getUserAndTeamIDs(req, execCtx)
	if err != nil {
		return "", err
	}
	return teamID, o.client.AddOrganizationUserToTeam(ctx, userID, teamID)
}

func (o *OrganizationUserTeamMembershipAdapter) Update(
	_ context.Context,
	_ string,
	_ state.OrganizationUserTeamMembership,
	_ string,
	_ *ExecutionContext,
) (string, error) {
	return "", fmt.Errorf("organization user team memberships do not support update operations")
}

func (o *OrganizationUserTeamMembershipAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	userID, teamID, err := o.getUserAndTeamIDs(state.OrganizationUserTeamMembership{TeamID: id}, execCtx)
	if err != nil {
		return err
	}
	return o.client.RemoveOrganizationUserFromTeam(ctx, userID, teamID)
}

func (o *OrganizationUserTeamMembershipAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

func (o *OrganizationUserTeamMembershipAdapter) GetByID(
	_ context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	userID, teamID, err := o.getUserAndTeamIDs(state.OrganizationUserTeamMembership{TeamID: id}, execCtx)
	if err != nil {
		return nil, err
	}
	return &OrganizationUserTeamMembershipResourceInfo{
		membership: state.OrganizationUserTeamMembership{UserID: userID, TeamID: teamID},
	}, nil
}

func (o *OrganizationUserTeamMembershipAdapter) ResourceType() string {
	return planner.ResourceTypeOrganizationUserTeamMembership
}

func (o *OrganizationUserTeamMembershipAdapter) RequiredFields() []string {
	return []string{planner.FieldUserID}
}

func (o *OrganizationUserTeamMembershipAdapter) SupportsUpdate() bool {
	return false
}

func (o *OrganizationUserTeamMembershipAdapter) getUserAndTeamIDs(
	req state.OrganizationUserTeamMembership,
	execCtx *ExecutionContext,
) (string, string, error) {
	userID := req.UserID
	teamID := req.TeamID
	if execCtx != nil && execCtx.PlannedChange != nil {
		if ref, ok := execCtx.PlannedChange.References[planner.FieldUserID]; ok && ref.ID != "" {
			userID = ref.ID
		}
		if ref, ok := execCtx.PlannedChange.References[planner.FieldTeamID]; ok && ref.ID != "" {
			teamID = ref.ID
		}
	}
	if userID == "" {
		return "", "", fmt.Errorf("user ID is required for organization user team membership operations")
	}
	if teamID == "" {
		return "", "", fmt.Errorf("team ID is required for organization user team membership operations")
	}
	return userID, teamID, nil
}

type OrganizationUserTeamMembershipResourceInfo struct {
	membership state.OrganizationUserTeamMembership
}

func (o *OrganizationUserTeamMembershipResourceInfo) GetID() string {
	return o.membership.TeamID
}

func (o *OrganizationUserTeamMembershipResourceInfo) GetName() string {
	return o.membership.UserID + ":" + o.membership.TeamID
}

func (o *OrganizationUserTeamMembershipResourceInfo) GetLabels() map[string]string {
	return make(map[string]string)
}

func (o *OrganizationUserTeamMembershipResourceInfo) GetNormalizedLabels() map[string]string {
	return make(map[string]string)
}

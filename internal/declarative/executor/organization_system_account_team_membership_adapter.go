package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// OrganizationSystemAccountTeamMembershipAdapter handles system account team memberships.
type OrganizationSystemAccountTeamMembershipAdapter struct {
	client *state.Client
}

func NewOrganizationSystemAccountTeamMembershipAdapter(
	client *state.Client,
) *OrganizationSystemAccountTeamMembershipAdapter {
	return &OrganizationSystemAccountTeamMembershipAdapter{client: client}
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) MapCreateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	create *state.OrganizationSystemAccountTeamMembership,
) error {
	accountID, _ := fields[planner.FieldSystemAccountID].(string)
	teamID, _ := fields[planner.FieldTeamID].(string)
	create.SystemAccountID = accountID
	create.TeamID = teamID
	return nil
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	_ map[string]any,
	_ *state.OrganizationSystemAccountTeamMembership,
	_ map[string]string,
) error {
	return nil
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) Create(
	ctx context.Context,
	req state.OrganizationSystemAccountTeamMembership,
	_ string,
	execCtx *ExecutionContext,
) (string, error) {
	accountID, teamID, err := o.getAccountAndTeamIDs(req, execCtx)
	if err != nil {
		return "", err
	}
	return teamID, o.client.AddOrganizationSystemAccountToTeam(ctx, accountID, teamID)
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) Update(
	_ context.Context,
	_ string,
	_ state.OrganizationSystemAccountTeamMembership,
	_ string,
	_ *ExecutionContext,
) (string, error) {
	return "", fmt.Errorf("organization system account team memberships do not support update operations")
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	accountID, teamID, err := o.getAccountAndTeamIDs(
		state.OrganizationSystemAccountTeamMembership{TeamID: id},
		execCtx,
	)
	if err != nil {
		return err
	}
	return o.client.RemoveOrganizationSystemAccountFromTeam(ctx, accountID, teamID)
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) GetByID(
	_ context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	accountID, teamID, err := o.getAccountAndTeamIDs(
		state.OrganizationSystemAccountTeamMembership{TeamID: id},
		execCtx,
	)
	if err != nil {
		return nil, err
	}
	return &OrganizationSystemAccountTeamMembershipResourceInfo{
		membership: state.OrganizationSystemAccountTeamMembership{
			SystemAccountID: accountID,
			TeamID:          teamID,
		},
	}, nil
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) ResourceType() string {
	return planner.ResourceTypeOrganizationSystemAccountTeamMembership
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) RequiredFields() []string {
	return []string{planner.FieldSystemAccountID}
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) SupportsUpdate() bool {
	return false
}

func (o *OrganizationSystemAccountTeamMembershipAdapter) getAccountAndTeamIDs(
	req state.OrganizationSystemAccountTeamMembership,
	execCtx *ExecutionContext,
) (string, string, error) {
	accountID := req.SystemAccountID
	teamID := req.TeamID
	if execCtx != nil && execCtx.PlannedChange != nil {
		if ref, ok := execCtx.PlannedChange.References[planner.FieldSystemAccountID]; ok && ref.ID != "" {
			accountID = ref.ID
		}
		if ref, ok := execCtx.PlannedChange.References[planner.FieldTeamID]; ok && ref.ID != "" {
			teamID = ref.ID
		}
	}
	if accountID == "" {
		return "", "", fmt.Errorf(
			"system account ID is required for organization system account team membership operations",
		)
	}
	if teamID == "" {
		return "", "", fmt.Errorf("team ID is required for organization system account team membership operations")
	}
	return accountID, teamID, nil
}

type OrganizationSystemAccountTeamMembershipResourceInfo struct {
	membership state.OrganizationSystemAccountTeamMembership
}

func (o *OrganizationSystemAccountTeamMembershipResourceInfo) GetID() string {
	return o.membership.TeamID
}

func (o *OrganizationSystemAccountTeamMembershipResourceInfo) GetName() string {
	return o.membership.SystemAccountID + ":" + o.membership.TeamID
}

func (o *OrganizationSystemAccountTeamMembershipResourceInfo) GetLabels() map[string]string {
	return map[string]string{}
}

func (o *OrganizationSystemAccountTeamMembershipResourceInfo) GetNormalizedLabels() map[string]string {
	return map[string]string{}
}

package executor

import (
	"context"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// OrganizationTeamAdapter implements ResourceOperations for teams
type OrganizationTeamAdapter struct {
	client *state.Client
}

// NewOrganizationTeamAdapter creates a new team adapter
func NewOrganizationTeamAdapter(client *state.Client) *OrganizationTeamAdapter {
	return &OrganizationTeamAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateTeam request
func (a *OrganizationTeamAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext,
	fields map[string]any, create *kkComps.CreateTeam,
) error {
	create.Name = common.ExtractResourceName(fields)
	common.MapOptionalStringFieldToPtr(&create.Description, fields, "description")

	userLabels := labels.ExtractLabelsFromField(fields["labels"])
	create.Labels = labels.BuildCreateLabels(userLabels, execCtx.Namespace, execCtx.Protection)

	return nil
}

// MapUpdateFields maps planner fields to UpdateTeam request
func (a *OrganizationTeamAdapter) MapUpdateFields(_ context.Context, execCtx *ExecutionContext,
	fields map[string]any, update *kkComps.UpdateTeam, currentLabels map[string]string,
) error {
	for field, value := range fields {
		switch field {
		case "name":
			if name, ok := value.(string); ok {
				update.Name = &name
			}
		case "description":
			if desc, ok := value.(string); ok {
				update.Description = &desc
			}
		}
	}

	desiredLabels := labels.ExtractLabelsFromField(fields["labels"])
	if plannerLabels := labels.ExtractLabelsFromField(fields[planner.FieldCurrentLabels]); plannerLabels != nil {
		currentLabels = plannerLabels
	}

	if desiredLabels != nil {
		update.Labels = labels.BuildUpdateLabels(
			desiredLabels,
			currentLabels,
			execCtx.Namespace,
			execCtx.Protection,
		)
	} else if currentLabels != nil {
		update.Labels = labels.BuildUpdateLabels(
			currentLabels,
			currentLabels,
			execCtx.Namespace,
			execCtx.Protection,
		)
	}

	return nil
}

// Create issues a create call via the state client
func (a *OrganizationTeamAdapter) Create(ctx context.Context, req kkComps.CreateTeam,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateOrganizationTeam(ctx, &req, namespace)
	if err != nil {
		return "", err
	}
	return resp, nil
}

// Update issues an update call via the state client
func (a *OrganizationTeamAdapter) Update(ctx context.Context, id string, req kkComps.UpdateTeam,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateOrganizationTeam(ctx, id, &req, namespace)
	if err != nil {
		return "", err
	}
	return resp, nil
}

// Delete issues a delete call via the state client
func (a *OrganizationTeamAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteOrganizationTeam(ctx, id)
}

// GetByName resolves a organization_team by name
func (a *OrganizationTeamAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	team, err := a.client.GetOrganizationTeamByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, nil
	}
	return &OrganizationTeamResourceInfo{team: team}, nil
}

// GetByID resolves a organization_team by ID
func (a *OrganizationTeamAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	team, err := a.client.GetOrganizationTeamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, nil
	}
	return &OrganizationTeamResourceInfo{team: team}, nil
}

// ResourceType returns the adapter resource type
func (a *OrganizationTeamAdapter) ResourceType() string {
	return "organization_team"
}

// RequiredFields lists required fields for create
func (a *OrganizationTeamAdapter) RequiredFields() []string {
	return []string{"name"}
}

// SupportsUpdate indicates team support updates
func (a *OrganizationTeamAdapter) SupportsUpdate() bool {
	return true
}

// OrganizationTeamResourceInfo implements ResourceInfo for organization_teams
type OrganizationTeamResourceInfo struct {
	team *state.OrganizationTeam
}

func (c *OrganizationTeamResourceInfo) GetID() string {
	return *c.team.ID
}

func (c *OrganizationTeamResourceInfo) GetName() string {
	return *c.team.Name
}

func (c *OrganizationTeamResourceInfo) GetLabels() map[string]string {
	return c.team.Labels
}

func (c *OrganizationTeamResourceInfo) GetNormalizedLabels() map[string]string {
	return c.team.NormalizedLabels
}

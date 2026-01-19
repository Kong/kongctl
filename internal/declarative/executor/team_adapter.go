package executor

import (
	"context"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// TeamAdapter implements ResourceOperations for teams
type TeamAdapter struct {
	client *state.Client
}

// NewTeamAdapter creates a new team adapter
func NewTeamAdapter(client *state.Client) *TeamAdapter {
	return &TeamAdapter{client: client}
}

// MapCreateFields maps planner fields to CreateTeam request
func (a *TeamAdapter) MapCreateFields(_ context.Context, execCtx *ExecutionContext,
	fields map[string]any, create *kkComps.CreateTeam,
) error {
	create.Name = common.ExtractResourceName(fields)
	common.MapOptionalStringFieldToPtr(&create.Description, fields, "description")

	userLabels := labels.ExtractLabelsFromField(fields["labels"])
	create.Labels = labels.BuildCreateLabels(userLabels, execCtx.Namespace, execCtx.Protection)

	return nil
}

// MapUpdateFields maps planner fields to UpdateTeam request
func (a *TeamAdapter) MapUpdateFields(_ context.Context, execCtx *ExecutionContext,
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
func (a *TeamAdapter) Create(ctx context.Context, req kkComps.CreateTeam,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := a.client.CreateTeam(ctx, &req, namespace)
	if err != nil {
		return "", err
	}
	return resp, nil
}

// Update issues an update call via the state client
func (a *TeamAdapter) Update(ctx context.Context, id string, req kkComps.UpdateTeam,
	namespace string, _ *ExecutionContext,
) (string, error) {
	resp, err := a.client.UpdateTeam(ctx, id, &req, namespace)
	if err != nil {
		return "", err
	}
	return resp, nil
}

// Delete issues an delete call via the state client
func (a *TeamAdapter) Delete(ctx context.Context, id string, _ *ExecutionContext) error {
	return a.client.DeleteTeam(ctx, id)
}

// GetByName resolves a team by name
func (a *TeamAdapter) GetByName(ctx context.Context, name string) (ResourceInfo, error) {
	team, err := a.client.GetTeamByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, nil
	}
	return &TeamResourceInfo{team: team}, nil
}

// GetByID resolves a team by ID
func (a *TeamAdapter) GetByID(ctx context.Context, id string, _ *ExecutionContext) (ResourceInfo, error) {
	team, err := a.client.GetTeamByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, nil
	}
	return &TeamResourceInfo{team: team}, nil
}

// ResourceType returns the adapter resource type
func (a *TeamAdapter) ResourceType() string {
	return "team"
}

// RequiredFields lists required fields for create
func (a *TeamAdapter) RequiredFields() []string {
	return []string{"name"}
}

// SupportsUpdate indicates team support updates
func (a *TeamAdapter) SupportsUpdate() bool {
	return true
}

// TeamResourceInfo implements ResourceInfo for teams
type TeamResourceInfo struct {
	team *state.Team
}

func (c *TeamResourceInfo) GetID() string {
	return *c.team.ID
}

func (c *TeamResourceInfo) GetName() string {
	return *c.team.Name
}

func (c *TeamResourceInfo) GetLabels() map[string]string {
	return c.team.Labels
}

func (c *TeamResourceInfo) GetNormalizedLabels() map[string]string {
	return c.team.NormalizedLabels
}

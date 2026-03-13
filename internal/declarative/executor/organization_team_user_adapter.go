package executor

import (
	"context"
	"fmt"
	"log/slog"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
)

// OrganizationTeamUserAdapter implements CreateDeleteOperations for organization team user memberships.
// User memberships only support add and remove operations, not updates.
type OrganizationTeamUserAdapter struct {
	client *state.Client
}

// NewOrganizationTeamUserAdapter creates a new organization team user adapter.
func NewOrganizationTeamUserAdapter(client *state.Client) *OrganizationTeamUserAdapter {
	return &OrganizationTeamUserAdapter{client: client}
}

// MapCreateFields maps the planned change fields to an AddUserToTeam request.
func (a *OrganizationTeamUserAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.AddUserToTeam,
) error {
	// Prefer the resolved user_id from References over raw fields
	if execCtx != nil && execCtx.PlannedChange != nil {
		if ref, ok := execCtx.PlannedChange.References["user_id"]; ok && ref.ID != "" {
			create.UserID = ref.ID
			return nil
		}
	}

	userID, _ := fields["user_id"].(string)
	if userID == "" {
		return fmt.Errorf("user_id is required for organization_team_user membership")
	}

	create.UserID = userID
	return nil
}

// Create adds a user to an organization team.
func (a *OrganizationTeamUserAdapter) Create(
	ctx context.Context,
	req kkComps.AddUserToTeam,
	_ string,
	execCtx *ExecutionContext,
) (string, error) {
	teamID, err := a.getTeamID(execCtx)
	if err != nil {
		return "", err
	}

	if req.UserID == "" {
		return "", fmt.Errorf("user_id is required to add user to team")
	}

	logger := orgTeamUserLogger(ctx)
	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Adding user to organization team",
			slog.String("team_id", teamID),
			slog.String("user_id", req.UserID))
	}

	if err := a.client.AddUserToTeam(ctx, teamID, req.UserID); err != nil {
		return "", err
	}

	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Added user to organization team",
			slog.String("team_id", teamID),
			slog.String("user_id", req.UserID))
	}

	// Membership has no separate ID; use the user's Konnect ID as the resource ID.
	return req.UserID, nil
}

// Delete removes a user from an organization team.
func (a *OrganizationTeamUserAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	teamID, err := a.getTeamID(execCtx)
	if err != nil {
		return err
	}

	userID := id
	if userID == "" {
		// Fall back to fields
		if execCtx != nil && execCtx.PlannedChange != nil {
			if uid, ok := execCtx.PlannedChange.Fields["user_id"].(string); ok {
				userID = uid
			}
		}
	}

	if userID == "" {
		return fmt.Errorf("user_id is required to remove user from team")
	}

	logger := orgTeamUserLogger(ctx)
	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Removing user from organization team",
			slog.String("team_id", teamID),
			slog.String("user_id", userID))
	}

	if err := a.client.RemoveUserFromTeam(ctx, userID, teamID); err != nil {
		return err
	}

	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Removed user from organization team",
			slog.String("team_id", teamID),
			slog.String("user_id", userID))
	}

	return nil
}

// GetByName is not supported for team user memberships.
func (a *OrganizationTeamUserAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

// ResourceType returns the resource type name.
func (a *OrganizationTeamUserAdapter) ResourceType() string {
	return "organization_team_user"
}

// RequiredFields returns the required fields for creation.
func (a *OrganizationTeamUserAdapter) RequiredFields() []string {
	return []string{"user_id"}
}

// getTeamID extracts the team Konnect ID from the execution context.
func (a *OrganizationTeamUserAdapter) getTeamID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf("execution context is required for organization_team_user operations")
	}

	change := *execCtx.PlannedChange

	if ref, ok := change.References["team_id"]; ok && ref.ID != "" {
		return ref.ID, nil
	}

	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("team ID is required for organization_team_user operations")
}

func orgTeamUserLogger(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	logger, _ := ctx.Value(log.LoggerKey).(*slog.Logger)
	return logger
}

package executor

import (
	"context"
	"fmt"
	"log/slog"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
)

// OrganizationTeamSystemAccountAdapter implements CreateDeleteOperations for organization team
// system account memberships. Memberships only support add and remove operations, not updates.
type OrganizationTeamSystemAccountAdapter struct {
	client *state.Client
}

// NewOrganizationTeamSystemAccountAdapter creates a new organization team system account adapter.
func NewOrganizationTeamSystemAccountAdapter(client *state.Client) *OrganizationTeamSystemAccountAdapter {
	return &OrganizationTeamSystemAccountAdapter{client: client}
}

// MapCreateFields maps the planned change fields to an AddSystemAccountToTeam request.
func (a *OrganizationTeamSystemAccountAdapter) MapCreateFields(
	_ context.Context,
	execCtx *ExecutionContext,
	fields map[string]any,
	create *kkComps.AddSystemAccountToTeam,
) error {
	// Prefer the resolved account_id from References over raw fields
	if execCtx != nil && execCtx.PlannedChange != nil {
		if ref, ok := execCtx.PlannedChange.References["account_id"]; ok && ref.ID != "" {
			create.AccountID = &ref.ID
			return nil
		}
	}

	accountID, _ := fields["account_id"].(string)
	if accountID == "" {
		return fmt.Errorf("account_id is required for organization_team_system_account membership")
	}

	create.AccountID = &accountID
	return nil
}

// Create adds a system account to an organization team.
func (a *OrganizationTeamSystemAccountAdapter) Create(
	ctx context.Context,
	req kkComps.AddSystemAccountToTeam,
	_ string,
	execCtx *ExecutionContext,
) (string, error) {
	teamID, err := a.getTeamID(execCtx)
	if err != nil {
		return "", err
	}

	if req.AccountID == nil || *req.AccountID == "" {
		return "", fmt.Errorf("account_id is required to add system account to team")
	}

	accountID := *req.AccountID

	logger := orgTeamSALogger(ctx)
	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Adding system account to organization team",
			slog.String("team_id", teamID),
			slog.String("account_id", accountID))
	}

	if err := a.client.AddSystemAccountToTeam(ctx, teamID, accountID); err != nil {
		return "", err
	}

	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Added system account to organization team",
			slog.String("team_id", teamID),
			slog.String("account_id", accountID))
	}

	// Membership has no separate ID; use the system account's Konnect ID as the resource ID.
	return accountID, nil
}

// Delete removes a system account from an organization team.
func (a *OrganizationTeamSystemAccountAdapter) Delete(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) error {
	teamID, err := a.getTeamID(execCtx)
	if err != nil {
		return err
	}

	accountID := id
	if accountID == "" {
		if execCtx != nil && execCtx.PlannedChange != nil {
			if aid, ok := execCtx.PlannedChange.Fields["account_id"].(string); ok {
				accountID = aid
			}
		}
	}

	if accountID == "" {
		return fmt.Errorf("account_id is required to remove system account from team")
	}

	logger := orgTeamSALogger(ctx)
	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Removing system account from organization team",
			slog.String("team_id", teamID),
			slog.String("account_id", accountID))
	}

	if err := a.client.RemoveSystemAccountFromTeam(ctx, teamID, accountID); err != nil {
		return err
	}

	if logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, "Removed system account from organization team",
			slog.String("team_id", teamID),
			slog.String("account_id", accountID))
	}

	return nil
}

// GetByName is not supported for team system account memberships.
func (a *OrganizationTeamSystemAccountAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
	return nil, nil
}

// ResourceType returns the resource type name.
func (a *OrganizationTeamSystemAccountAdapter) ResourceType() string {
	return "organization_team_system_account"
}

// RequiredFields returns the required fields for creation.
func (a *OrganizationTeamSystemAccountAdapter) RequiredFields() []string {
	return []string{"account_id"}
}

// getTeamID extracts the team Konnect ID from the execution context.
func (a *OrganizationTeamSystemAccountAdapter) getTeamID(execCtx *ExecutionContext) (string, error) {
	if execCtx == nil || execCtx.PlannedChange == nil {
		return "", fmt.Errorf(
			"execution context is required for organization_team_system_account operations",
		)
	}

	change := *execCtx.PlannedChange

	if ref, ok := change.References["team_id"]; ok && ref.ID != "" {
		return ref.ID, nil
	}

	if change.Parent != nil && change.Parent.ID != "" {
		return change.Parent.ID, nil
	}

	return "", fmt.Errorf("team ID is required for organization_team_system_account operations")
}

func orgTeamSALogger(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	logger, _ := ctx.Value(log.LoggerKey).(*slog.Logger)
	return logger
}

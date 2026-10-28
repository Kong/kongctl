package executor

import (
	"context"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

func validateCurrentAIGatewayChildDeletion(
	ctx context.Context, client *state.Client, execCtx *ExecutionContext,
	gatewayID, id, name string,
) error {
	change := *execCtx.PlannedChange
	change.Parent = &planner.ParentInfo{ID: gatewayID}
	change.ResourceID = id
	change.Fields = map[string]any{planner.FieldName: name}
	return planner.NewPlanner(client, slog.Default()).ValidateAIGatewayChildDeletion(ctx, change)
}

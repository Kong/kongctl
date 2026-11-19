package executor

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAICostCreateCannotTargetExistingChannel(t *testing.T) {
	handler := &aiCostExecutor{kind: resources.ResourceTypeAICostChannel}
	change := planner.PlannedChange{
		Action: planner.ActionCreate, ResourceID: "4078f559-063b-4f10-9098-9e6553b21d9b",
	}
	require.ErrorContains(t, handler.ValidatePayload(t.Context(), change), "cannot select an existing ID")
}

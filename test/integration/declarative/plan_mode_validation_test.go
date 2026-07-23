//go:build integration

package declarative_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/declarative"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionCommandsRejectMismatchedPlanModes(t *testing.T) {
	tests := []struct {
		name         string
		command      verbs.VerbValue
		actualMode   planner.PlanMode
		requiredMode planner.PlanMode
	}{
		{
			name:         "apply rejects sync plan",
			command:      "apply",
			actualMode:   planner.PlanModeSync,
			requiredMode: planner.PlanModeApply,
		},
		{
			name:         "apply rejects delete plan",
			command:      "apply",
			actualMode:   planner.PlanModeDelete,
			requiredMode: planner.PlanModeApply,
		},
		{
			name:         "sync rejects apply plan",
			command:      "sync",
			actualMode:   planner.PlanModeApply,
			requiredMode: planner.PlanModeSync,
		},
		{
			name:         "sync rejects delete plan",
			command:      "sync",
			actualMode:   planner.PlanModeDelete,
			requiredMode: planner.PlanModeSync,
		},
		{
			name:         "delete rejects apply plan",
			command:      "delete",
			actualMode:   planner.PlanModeApply,
			requiredMode: planner.PlanModeDelete,
		},
		{
			name:         "delete rejects sync plan",
			command:      "delete",
			actualMode:   planner.PlanModeSync,
			requiredMode: planner.PlanModeDelete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planFile := writePlanArtifact(t, tt.actualMode)
			cmd, err := declarative.NewDeclarativeCmd(tt.command)
			require.NoError(t, err)
			cmd.SetContext(SetupTestContext(t))

			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"--plan", planFile, "--auto-approve"})

			err = cmd.Execute()

			require.Error(t, err)
			assert.Contains(t, err.Error(), `plan "`+planFile+`" was generated in "`+string(tt.actualMode)+`" mode`)
			assert.Contains(
				t,
				err.Error(),
				tt.command.String()+` command, which requires "`+string(tt.requiredMode)+`" mode`,
			)
			assert.Contains(
				t,
				err.Error(),
				"kongctl plan --mode "+string(tt.requiredMode)+" -f <files> --output-file <plan-file>",
			)
			assert.Contains(t, err.Error(), "kongctl "+string(tt.actualMode)+" --plan <plan-file>")
		})
	}
}

func TestDiffAcceptsSavedPlansFromEveryMode(t *testing.T) {
	for _, mode := range []planner.PlanMode{
		planner.PlanModeApply,
		planner.PlanModeSync,
		planner.PlanModeDelete,
	} {
		t.Run(string(mode), func(t *testing.T) {
			planFile := writePlanArtifact(t, mode)
			cmd, err := declarative.NewDeclarativeCmd("diff")
			require.NoError(t, err)
			cmd.SetContext(SetupTestContext(t))

			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"--plan", planFile})

			err = cmd.Execute()

			require.NoError(t, err)
			assert.Contains(t, output.String(), "No changes detected")
		})
	}
}

func writePlanArtifact(t *testing.T, mode planner.PlanMode) string {
	t.Helper()

	plan := planner.NewPlan("1.0", "kongctl/integration-test", mode)
	planData, err := json.Marshal(plan)
	require.NoError(t, err)

	planFile := filepath.Join(t.TempDir(), string(mode)+"-plan.json")
	require.NoError(t, os.WriteFile(planFile, planData, 0o600))
	return planFile
}

package declarative

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffColorModes(t *testing.T) {
	// An unset NO_COLOR differs from an empty but present NO_COLOR.
	original, present := os.LookupEnv("NO_COLOR")
	t.Cleanup(func() {
		if present {
			require.NoError(t, os.Setenv("NO_COLOR", original))
		} else {
			require.NoError(t, os.Unsetenv("NO_COLOR"))
		}
	})
	for _, noColor := range []bool{false, true} {
		if noColor {
			require.NoError(t, os.Setenv("NO_COLOR", ""))
		} else {
			require.NoError(t, os.Unsetenv("NO_COLOR"))
		}
		for _, term := range []string{"xterm-256color", "dumb"} {
			t.Setenv("TERM", term)
			for _, terminal := range []bool{false, true} {
				label := fmt.Sprintf("NO_COLOR=%t TERM=%s terminal=%t", noColor, term, terminal)
				assert.True(t, diffColorEnabled(cmdcommon.ColorModeAlways, terminal), label)
				assert.False(t, diffColorEnabled(cmdcommon.ColorModeNever, terminal), label)
				assert.Equal(t, terminal && !noColor && term != "dumb",
					diffColorEnabled(cmdcommon.ColorModeAuto, terminal), label)
			}
		}
	}
}

func TestDiffColorFlag(t *testing.T) {
	command := newDeclarativeDiffCmd()
	mode, err := diffColorMode(command)
	require.NoError(t, err)
	assert.Equal(t, cmdcommon.ColorModeAuto, mode)
	for _, value := range []string{"always", "never", "auto"} {
		require.NoError(t, command.Flags().Set(cmdcommon.ColorFlagName, value))
		mode, err = diffColorMode(command)
		require.NoError(t, err)
		assert.Equal(t, value, mode.String())
	}
	require.NoError(t, command.Flags().Set(cmdcommon.ColorFlagName, "invalid"))
	require.ErrorContains(t, runDiff(command, nil), "invalid color mode")
}

func TestDiffDeferredWriteLabel(t *testing.T) {
	plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
	plan.AddChange(planner.PlannedChange{
		ID: "write", ResourceType: planner.ResourceTypeAIGatewayAuthStrategy,
		ResourceRef: "auth", Action: planner.ActionUpdate,
		SecretWrites: []planner.SecretWriteIntent{{Field: "/config/client_secret"}},
	})
	plan.SetExecutionOrder([]string{"write"})
	command := newDeclarativeDiffCmd()
	var out bytes.Buffer
	command.SetOut(&out)
	require.NoError(t, displayTextDiff(command, plan, false))
	assert.Contains(t, out.String(), "  config.client_secret: (write deferred)\n")
	assert.NotContains(t, out.String(), "sensitive value changed")
}

func TestDiffColorPreservesPlainOutput(t *testing.T) {
	plan := planner.NewPlan("1.0", "test", planner.PlanModeSync)
	plan.AddChange(planner.PlannedChange{
		ID: "update", ResourceType: planner.ResourceTypeAIGatewayAuthStrategy,
		ResourceRef: "auth", Namespace: "demo", Action: planner.ActionUpdate,
		ChangedFields: map[string]planner.FieldChange{
			planner.FieldConfig: {
				Old: map[string]any{
					"client_secret": "hidden-old", "removed": true, "type": 80,
					"array":       []any{map[string]any{"port": 80, "unchanged": true}},
					"replacement": false,
				},
				New: map[string]any{
					"client_secret": "hidden-new", "added": true, "type": "80",
					"array":       []any{map[string]any{"port": 443, "unchanged": true}},
					"replacement": map[string]any{"label": "visible-nested", "long": strings.Repeat("x", 90)},
				},
			},
		},
	})
	plan.AddChange(planner.PlannedChange{
		ID: "create", ResourceType: planner.ResourceTypePortal, ResourceRef: "new",
		Action: planner.ActionCreate, Fields: map[string]any{planner.FieldName: "new"},
	})
	plan.AddChange(planner.PlannedChange{
		ID: "delete", ResourceType: planner.ResourceTypePortal, ResourceRef: "old", Action: planner.ActionDelete,
	})
	plan.SetExecutionOrder([]string{"update", "create", "delete"})
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	var baseline string
	var colored []string
	for _, paletteName := range []string{theme.DefaultName, theme.DarkName, "dracula"} {
		palette, ok := theme.Get(paletteName)
		require.True(t, ok)
		for _, mode := range []string{"never", "auto", "always"} {
			command := newDeclarativeDiffCmd()
			command.SetContext(theme.ContextWithPalette(t.Context(), palette))
			require.NoError(t, command.Flags().Set(cmdcommon.ColorFlagName, mode))
			var out bytes.Buffer
			command.SetOut(&out)
			require.NoError(t, displayTextDiff(command, plan, true))
			if baseline == "" {
				baseline = out.String()
			}
			if mode == "always" {
				assert.Contains(t, out.String(), "\x1b[")
				assert.Equal(t, baseline, ansi.Strip(out.String()))
				assert.Contains(t, out.String(), palette.ForegroundStyle(theme.ColorDiffRemoved).Render("80"))
				assert.Contains(t, out.String(), palette.ForegroundStyle(theme.ColorDiffAdded).Render("443"))
				colored = append(colored, out.String())
			} else {
				assert.Equal(t, baseline, out.String())
				assert.NotContains(t, out.String(), "\x1b[")
			}
			assert.NotContains(t, out.String(), "hidden-")
		}
	}
	assert.NotEqual(t, colored[0], colored[1])
	assert.NotEqual(t, colored[1], colored[2])
}

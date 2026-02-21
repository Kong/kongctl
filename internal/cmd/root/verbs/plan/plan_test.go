package plan

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPlanCmd(t *testing.T) {
	cmd, err := NewPlanCmd()
	if err != nil {
		t.Fatalf("NewPlanCmd should not return an error: %v", err)
	}
	if cmd == nil {
		t.Fatal("NewPlanCmd should return a command")
		return
	}

	// Test basic command properties
	assert.Equal(t, "plan", cmd.Use, "Command use should be 'plan'")
	assert.Contains(t, cmd.Short, "Preview changes to Kong Konnect resources",
		"Short description should mention preview changes")
	assert.Contains(t, cmd.Long, "execution plan", "Long description should mention execution plan")
	assert.Contains(t, cmd.Example, meta.CLIName, "Examples should include CLI name")

	// Test that konnect subcommand is added
	subcommands := cmd.Commands()
	if len(subcommands) != 1 {
		t.Fatalf("Should have exactly one subcommand, got %d", len(subcommands))
	}
	assert.Equal(t, "konnect", subcommands[0].Name(), "Subcommand should be 'konnect'")
}

func TestPlanCmdVerb(t *testing.T) {
	assert.Equal(t, verbs.Plan, Verb, "Verb constant should be verbs.Plan")
	assert.Equal(t, "plan", Verb.String(), "Verb string should be 'plan'")
}

func TestPlanCmdHelpText(t *testing.T) {
	cmd, err := NewPlanCmd()
	if err != nil {
		t.Fatalf("NewPlanCmd should not return an error: %v", err)
	}

	// Test that help text contains expected content
	assert.Contains(t, cmd.Short, "Preview changes", "Short should mention preview changes")
	assert.Contains(t, cmd.Long, "execution plan", "Long should mention execution plan")
	assert.Contains(t, cmd.Example, "-f", "Examples should show -f flag usage")
	assert.Contains(t, cmd.Example, "help plan", "Examples should mention extended help")
}

func TestPlanCmdSubcommandAccess(t *testing.T) {
	cmd, err := NewPlanCmd()
	if err != nil {
		t.Fatalf("NewPlanCmd should not return an error: %v", err)
	}

	// Find konnect subcommand
	var konnectCmd *cobra.Command
	for _, subcmd := range cmd.Commands() {
		if subcmd.Name() == "konnect" {
			konnectCmd = subcmd
			break
		}
	}

	if konnectCmd == nil {
		t.Fatal("Should have konnect subcommand")
	}
	assert.Equal(t, "konnect", konnectCmd.Name(), "Subcommand name should be konnect")
}

func TestPlanCmd_RejectsPositionalArgs(t *testing.T) {
	cmd, err := NewPlanCmd()
	require.NoError(t, err)

	// Positional arguments should be rejected with a helpful error
	err = cmd.Args(cmd, []string{"./some-file.yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-f/--filename", "Error should suggest using -f/--filename flag")
	assert.Contains(t, err.Error(), "./some-file.yaml", "Error should include the unexpected argument")
}

func TestPlanCmd_AcceptsNoArgs(t *testing.T) {
	cmd, err := NewPlanCmd()
	require.NoError(t, err)

	// No positional arguments should be accepted
	err = cmd.Args(cmd, []string{})
	assert.NoError(t, err)
}

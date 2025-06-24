package plan

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestNewPlanCmd(t *testing.T) {
	cmd, err := NewPlanCmd()
	if err != nil {
		t.Fatalf("NewPlanCmd should not return an error: %v", err)
	}
	if cmd == nil {
		t.Fatal("NewPlanCmd should return a command")
	}

	// Test basic command properties
	assert.Equal(t, "plan", cmd.Use, "Command use should be 'plan'")
	assert.Contains(t, cmd.Short, "Generate a declarative configuration plan artifact",
		"Short description should mention plan artifact")
	assert.Contains(t, cmd.Long, "plan artifact", "Long description should mention plan artifact")
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
	assert.Contains(t, cmd.Short, "declarative configuration", "Short should mention declarative configuration")
	assert.Contains(t, cmd.Long, "desired state", "Long should mention desired state")
	assert.Contains(t, cmd.Example, "--dir", "Examples should show --dir flag usage")
	assert.Contains(t, cmd.Example, "--output-file", "Examples should show --output-file flag usage")
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
package declarative

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
)

// NewDeclarativeCmd creates the appropriate declarative command based on the verb
func NewDeclarativeCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	switch verb {
	case verbs.Plan:
		return newDeclarativePlanCmd(), nil
	case verbs.Sync:
		return newDeclarativeSyncCmd(), nil
	case verbs.Diff:
		return newDeclarativeDiffCmd(), nil
	case verbs.Export:
		return newDeclarativeExportCmd(), nil
	case verbs.Apply:
		return newDeclarativeApplyCmd(), nil
	case verbs.Add, verbs.Get, verbs.Create, verbs.Dump, verbs.Update,
		verbs.Delete, verbs.Help, verbs.List, verbs.Login:
		return nil, fmt.Errorf("verb %s does not support declarative configuration", verb)
	}
	return nil, fmt.Errorf("unexpected verb %s", verb)
}

func newDeclarativePlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "konnect",
		Short: "Generate a plan artifact for Konnect resources",
		Long: `Generate a plan artifact from declarative configuration files for Konnect.

The plan artifact represents the desired state of Konnect resources and can be used
for review, approval workflows, or as input to sync operations.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("plan command not yet implemented")
		},
	}

	// Add declarative config flags
	cmd.Flags().StringP("dir", "d", ".", "Directory containing configuration files")
	cmd.Flags().String("output-file", "", "Save plan artifact to file")

	return cmd
}

func newDeclarativeSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "konnect",
		Short: "Synchronize declarative configuration to Konnect",
		Long: `Synchronize declarative configuration files to Konnect.

Sync analyzes the current state of Konnect resources, compares it with the desired
state defined in the configuration files, and applies the necessary changes to
achieve the desired state.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("sync command not yet implemented")
		},
	}

	// Add declarative config flags
	cmd.Flags().StringP("dir", "d", ".", "Directory containing configuration files")
	cmd.Flags().Bool("dry-run", false, "Preview changes without applying them")

	return cmd
}

func newDeclarativeDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "konnect",
		Short: "Display differences between current and desired Konnect state",
		Long: `Compare the current state of Konnect resources with the desired state
defined in declarative configuration files and display the differences.

The diff output shows what changes would be made without actually applying them,
useful for reviewing changes before synchronization.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("diff command not yet implemented")
		},
	}

	// Add declarative config flags
	cmd.Flags().StringP("dir", "d", ".", "Directory containing configuration files")
	cmd.Flags().Bool("detailed", false, "Show detailed diff output")

	return cmd
}

func newDeclarativeExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "konnect",
		Short: "Export current Konnect state as declarative configuration",
		Long: `Export the current state of Konnect resources as declarative configuration files.

This command retrieves the current configuration from Konnect and generates
declarative configuration files that can be version controlled, modified,
and applied to other environments.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("export command not yet implemented")
		},
	}

	// Add declarative config flags
	cmd.Flags().StringP("dir", "d", "./exported-config", "Directory to export configuration files")
	cmd.Flags().String("resources", "", "Comma-separated list of resource types to export")

	return cmd
}

func newDeclarativeApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "konnect",
		Short: "Apply declarative configuration to Konnect",
		Long: `Apply declarative configuration files to Konnect.

Apply reads the configuration files and makes the necessary API calls to create,
update, or delete resources to match the desired state.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("apply command not yet implemented")
		},
	}

	// Add declarative config flags
	cmd.Flags().StringP("dir", "d", ".", "Directory containing configuration files")
	cmd.Flags().Bool("force", false, "Force apply without confirmation")

	return cmd
}
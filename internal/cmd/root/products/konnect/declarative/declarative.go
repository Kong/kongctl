package declarative

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
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
		RunE: runPlan,
	}

	// Add declarative config flags
	cmd.Flags().StringSliceP("filename", "f", []string{}, 
		"Filename or directory to files to use to create the resource (can specify multiple)")
	cmd.Flags().BoolP("recursive", "R", false, 
		"Process the directory used in -f, --filename recursively")
	cmd.Flags().String("output-file", "", "Save plan artifact to file")

	return cmd
}

func runPlan(command *cobra.Command, args []string) error {
	ctx := command.Context()
	filenames, _ := command.Flags().GetStringSlice("filename")
	recursive, _ := command.Flags().GetBool("recursive")
	
	// Build helper
	helper := cmd.BuildHelper(command, args)
	
	// Get configuration
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	
	// Get logger
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	
	// Get Konnect SDK
	kkClient, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize Konnect client: %w", err)
	}
	
	// Parse sources from filenames
	sources, err := loader.ParseSources(filenames)
	if err != nil {
		return fmt.Errorf("failed to parse sources: %w", err)
	}
	
	// Load configuration
	ldr := loader.New()
	resourceSet, err := ldr.LoadFromSources(sources, recursive)
	if err != nil {
		// Provide more helpful error message for common cases
		if len(filenames) == 0 && strings.Contains(err.Error(), "no YAML files found") {
			return fmt.Errorf("no configuration files found in current directory. Use -f to specify files or directories")
		}
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	
	// Check if configuration is empty
	totalResources := len(resourceSet.Portals) + len(resourceSet.ApplicationAuthStrategies) +
		len(resourceSet.ControlPlanes) + len(resourceSet.APIs)
	
	if totalResources == 0 {
		// Check if we're using default directory (no explicit sources)
		if len(filenames) == 0 {
			return fmt.Errorf("no configuration files found in current directory. Use -f to specify files or directories")
		}
		return fmt.Errorf("no resources found in configuration files")
	}
	
	// Create planner
	portalAPI := kkClient.GetPortalAPI()
	stateClient := state.NewClient(portalAPI)
	p := planner.NewPlanner(stateClient)
	
	// Generate plan
	plan, err := p.GeneratePlan(ctx, resourceSet)
	if err != nil {
		return fmt.Errorf("failed to generate plan: %w", err)
	}
	
	// Handle output
	outputFile, _ := command.Flags().GetString("output-file")
	
	if outputFile != "" {
		// Save to file
		planJSON, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal plan: %w", err)
		}
		
		if err := os.WriteFile(outputFile, planJSON, 0600); err != nil {
			return fmt.Errorf("failed to write plan file: %w", err)
		}
		
		fmt.Fprintf(command.OutOrStdout(), "Plan saved to: %s\n", outputFile)
	} else {
		// Display plan to stdout
		planJSON, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal plan: %w", err)
		}
		fmt.Fprintln(command.OutOrStdout(), string(planJSON))
	}
	
	// Display summary
	fmt.Fprintf(command.OutOrStdout(), "\nPlan Summary:\n")
	fmt.Fprintf(command.OutOrStdout(), "Total changes: %d\n", plan.Summary.TotalChanges)
	
	if plan.Summary.TotalChanges > 0 {
		for action, count := range plan.Summary.ByAction {
			if count > 0 {
				fmt.Fprintf(command.OutOrStdout(), "  %s: %d\n", action, count)
			}
		}
		
		fmt.Fprintln(command.OutOrStdout(), "\nResources by type:")
		for resourceType, count := range plan.Summary.ByResource {
			if count > 0 {
				fmt.Fprintf(command.OutOrStdout(), "  %s: %d\n", resourceType, count)
			}
		}
		
		if plan.Summary.ProtectionChanges != nil && 
		   (plan.Summary.ProtectionChanges.Protecting > 0 || plan.Summary.ProtectionChanges.Unprotecting > 0) {
			fmt.Fprintln(command.OutOrStdout(), "\nProtection changes:")
			if plan.Summary.ProtectionChanges.Protecting > 0 {
				fmt.Fprintf(command.OutOrStdout(), "  Protecting: %d\n", plan.Summary.ProtectionChanges.Protecting)
			}
			if plan.Summary.ProtectionChanges.Unprotecting > 0 {
				fmt.Fprintf(command.OutOrStdout(), "  Unprotecting: %d\n", plan.Summary.ProtectionChanges.Unprotecting)
			}
		}
		
		if len(plan.Warnings) > 0 {
			fmt.Fprintf(command.OutOrStdout(), "\nWarnings: %d\n", len(plan.Warnings))
			for _, warning := range plan.Warnings {
				fmt.Fprintf(command.OutOrStdout(), "  - [%s] %s\n", warning.ChangeID, warning.Message)
			}
		}
	}
	
	if plan.IsEmpty() {
		fmt.Fprintln(command.OutOrStdout(), "\nNo changes detected. Infrastructure is up to date.")
	} else {
		if outputFile != "" {
			fmt.Fprintf(command.OutOrStdout(), "\nRun 'kongctl diff --plan %s' to review changes.\n", outputFile)
		} else {
			fmt.Fprintln(command.OutOrStdout(), "\nSave this plan with --output-file to review changes later.")
		}
	}
	
	return nil
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
	cmd.Flags().StringSliceP("filename", "f", []string{}, 
		"Filename or directory to files to use to create the resource (can specify multiple)")
	cmd.Flags().BoolP("recursive", "R", false, 
		"Process the directory used in -f, --filename recursively")
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
	cmd.Flags().StringSliceP("filename", "f", []string{}, 
		"Filename or directory to files to use to create the resource (can specify multiple)")
	cmd.Flags().BoolP("recursive", "R", false, 
		"Process the directory used in -f, --filename recursively")
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
	cmd.Flags().StringP("output", "o", "./exported-config", "Directory to export configuration files")
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
	cmd.Flags().StringSliceP("filename", "f", []string{}, 
		"Filename or directory to files to use to create the resource (can specify multiple)")
	cmd.Flags().BoolP("recursive", "R", false, 
		"Process the directory used in -f, --filename recursively")
	cmd.Flags().Bool("force", false, "Force apply without confirmation")

	return cmd
}
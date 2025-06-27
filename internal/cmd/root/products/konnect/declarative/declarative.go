package declarative

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
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
	
	// Marshal plan to JSON
	planJSON, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}
	
	// Handle output
	outputFile, _ := command.Flags().GetString("output-file")
	
	if outputFile != "" {
		// Save to file
		if err := os.WriteFile(outputFile, planJSON, 0600); err != nil {
			return fmt.Errorf("failed to write plan file: %w", err)
		}
	} else {
		// Output to stdout
		fmt.Fprintln(command.OutOrStdout(), string(planJSON))
	}
	
	return nil
}

func runDiff(command *cobra.Command, args []string) error {
	ctx := command.Context()
	var plan *planner.Plan
	
	// Check if plan file is provided
	planFile, _ := command.Flags().GetString("plan")
	
	if planFile != "" {
		// Load existing plan
		var planData []byte
		var err error
		
		if planFile == "-" {
			// Read from stdin
			planData, err = io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read plan from stdin: %w", err)
			}
		} else {
			// Read from file
			planData, err = os.ReadFile(planFile)
			if err != nil {
				return fmt.Errorf("failed to read plan file: %w", err)
			}
		}
		
		plan = &planner.Plan{}
		if err := json.Unmarshal(planData, plan); err != nil {
			return fmt.Errorf("failed to parse plan: %w", err)
		}
	} else {
		// Generate new plan from configuration files
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
				return fmt.Errorf("no configuration files found. Use -f to specify files or --plan to use existing plan")
			}
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		
		// Check if configuration is empty
		totalResources := len(resourceSet.Portals) + len(resourceSet.ApplicationAuthStrategies) +
			len(resourceSet.ControlPlanes) + len(resourceSet.APIs)
		
		if totalResources == 0 {
			// Check if we're using default directory (no explicit sources)
			if len(filenames) == 0 {
				return fmt.Errorf("no configuration files found. Use -f to specify files or --plan to use existing plan")
			}
			return fmt.Errorf("no resources found in configuration files")
		}
		
		// Create planner
		portalAPI := kkClient.GetPortalAPI()
		stateClient := state.NewClient(portalAPI)
		p := planner.NewPlanner(stateClient)
		
		// Generate plan
		plan, err = p.GeneratePlan(ctx, resourceSet)
		if err != nil {
			return fmt.Errorf("failed to generate plan: %w", err)
		}
	}
	
	// Display diff based on output format
	outputFormat, _ := command.Flags().GetString("output")
	
	switch outputFormat {
	case "json":
		// JSON output
		encoder := json.NewEncoder(command.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
		
	case "yaml":
		// YAML output
		yamlData, err := yaml.Marshal(plan)
		if err != nil {
			return fmt.Errorf("failed to marshal plan to YAML: %w", err)
		}
		fmt.Fprintln(command.OutOrStdout(), string(yamlData))
		return nil
		
	case "text":
		// Human-readable text output
		return displayTextDiff(command, plan)
		
	default:
		return fmt.Errorf("unsupported output format: %s (use text, json, or yaml)", outputFormat)
	}
}

func displayTextDiff(command *cobra.Command, plan *planner.Plan) error {
	out := command.OutOrStdout()
	
	// Handle empty plan
	if plan.IsEmpty() {
		fmt.Fprintln(out, "No changes detected. Infrastructure is up to date.")
		return nil
	}
	
	// Display summary
	createCount := plan.Summary.ByAction[planner.ActionCreate]
	updateCount := plan.Summary.ByAction[planner.ActionUpdate]
	deleteCount := plan.Summary.ByAction[planner.ActionDelete]
	
	if deleteCount > 0 {
		fmt.Fprintf(out, "Plan: %d to add, %d to change, %d to destroy\n\n",
			createCount, updateCount, deleteCount)
	} else {
		fmt.Fprintf(out, "Plan: %d to add, %d to change\n\n",
			createCount, updateCount)
	}
	
	// Display warnings if any
	if len(plan.Warnings) > 0 {
		fmt.Fprintln(out, "Warnings:")
		for _, warning := range plan.Warnings {
			fmt.Fprintf(out, "  ⚠ [%s] %s\n", warning.ChangeID, warning.Message)
		}
		fmt.Fprintln(out)
	}
	
	// Display each change in execution order
	for _, changeID := range plan.ExecutionOrder {
		// Find the change
		var change *planner.PlannedChange
		for i := range plan.Changes {
			if plan.Changes[i].ID == changeID {
				change = &plan.Changes[i]
				break
			}
		}
		if change == nil {
			continue
		}
		
		switch change.Action {
		case planner.ActionCreate:
			fmt.Fprintf(out, "+ [%s] %s %q will be created\n",
				change.ID, change.ResourceType, change.ResourceRef)
			
			// Show key fields
			for field, value := range change.Fields {
				displayField(out, field, value, "  ")
			}
			
			// Show protection status
			if prot, ok := change.Protection.(bool); ok && prot {
				fmt.Fprintln(out, "  protection: enabled")
			}
			
		case planner.ActionUpdate:
			fmt.Fprintf(out, "~ [%s] %s %q will be updated\n",
				change.ID, change.ResourceType, change.ResourceRef)
			
			// Check if this is a protection change
			if pc, ok := change.Protection.(planner.ProtectionChange); ok {
				if pc.Old && !pc.New {
					fmt.Fprintln(out, "  protection: enabled → disabled")
				} else if !pc.Old && pc.New {
					fmt.Fprintln(out, "  protection: disabled → enabled")
				}
			} else if prot, ok := change.Protection.(bool); ok && prot {
				fmt.Fprintln(out, "  protection: enabled (no change)")
			}
			
			// Show field changes
			for field, value := range change.Fields {
				if fc, ok := value.(planner.FieldChange); ok {
					fmt.Fprintf(out, "  %s: %v → %v\n", field, fc.Old, fc.New)
				} else if fc, ok := value.(map[string]interface{}); ok {
					// Handle FieldChange that was unmarshaled from JSON
					if oldVal, hasOld := fc["old"]; hasOld {
						if newVal, hasNew := fc["new"]; hasNew {
							fmt.Fprintf(out, "  %s: %v → %v\n", field, oldVal, newVal)
							continue
						}
					}
					// Fallback for other map types
					displayField(out, field, value, "  ")
				} else {
					displayField(out, field, value, "  ")
				}
			}
			
		case planner.ActionDelete:
			// DELETE action (future implementation)
			fmt.Fprintf(out, "- [%s] %s %q will be deleted\n",
				change.ID, change.ResourceType, change.ResourceRef)
		}
		
		// Show dependencies
		if len(change.DependsOn) > 0 {
			fmt.Fprintf(out, "  depends on: %v\n", change.DependsOn)
		}
		
		// Show references
		if len(change.References) > 0 {
			fmt.Fprintln(out, "  references:")
			for field, ref := range change.References {
				if ref.ID == "<unknown>" {
					fmt.Fprintf(out, "    %s: %s (to be resolved)\n", field, ref.Ref)
				} else {
					fmt.Fprintf(out, "    %s: %s → %s\n", field, ref.Ref, ref.ID)
				}
			}
		}
		
		fmt.Fprintln(out)
	}
	
	// Display protection changes summary if any
	if plan.Summary.ProtectionChanges != nil && 
	   (plan.Summary.ProtectionChanges.Protecting > 0 || plan.Summary.ProtectionChanges.Unprotecting > 0) {
		fmt.Fprintln(out, "Protection changes summary:")
		if plan.Summary.ProtectionChanges.Protecting > 0 {
			fmt.Fprintf(out, "  Resources being protected: %d\n", plan.Summary.ProtectionChanges.Protecting)
		}
		if plan.Summary.ProtectionChanges.Unprotecting > 0 {
			fmt.Fprintf(out, "  Resources being unprotected: %d\n", plan.Summary.ProtectionChanges.Unprotecting)
		}
	}
	
	return nil
}

func displayField(out io.Writer, field string, value interface{}, indent string) {
	switch v := value.(type) {
	case string:
		if v != "" {
			fmt.Fprintf(out, "%s%s: %q\n", indent, field, v)
		}
	case bool:
		fmt.Fprintf(out, "%s%s: %t\n", indent, field, v)
	case float64:
		fmt.Fprintf(out, "%s%s: %g\n", indent, field, v)
	case map[string]interface{}:
		// Skip empty maps
		if len(v) == 0 {
			return
		}
		fmt.Fprintf(out, "%s%s:\n", indent, field)
		for k, val := range v {
			displayField(out, k, val, indent+"  ")
		}
	case []interface{}:
		// Skip empty slices
		if len(v) == 0 {
			return
		}
		fmt.Fprintf(out, "%s%s:\n", indent, field)
		for i, item := range v {
			displayField(out, fmt.Sprintf("[%d]", i), item, indent+"  ")
		}
	default:
		if v != nil {
			fmt.Fprintf(out, "%s%s: %v\n", indent, field, v)
		}
	}
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
		RunE: runDiff,
	}

	// Add declarative config flags
	cmd.Flags().StringSliceP("filename", "f", []string{}, 
		"Filename or directory to files to use to create the resource (can specify multiple)")
	cmd.Flags().BoolP("recursive", "R", false, 
		"Process the directory used in -f, --filename recursively")
	cmd.Flags().String("plan", "", "Path to existing plan file to display")
	cmd.Flags().StringP("output", "o", "text", "Output format (text, json, or yaml)")

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
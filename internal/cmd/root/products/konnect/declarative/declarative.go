package declarative

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/executor"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/kong/kongctl/internal/konnect/helpers"
	applog "github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// contextKey is used for storing values in context
type contextKey string

const (
	// currentPlanKey is the context key for storing the current plan
	currentPlanKey contextKey = "current_plan"
	// planFileKey is the context key for storing the plan file path
	planFileKey contextKey = "plan_file"
	// textOutputFormat is the string constant for text output format
	textOutputFormat = "text"
	// requireNamespaceFlagName is the CLI flag for specific namespace enforcement
	requireNamespaceFlagName = "require-namespace"
	// requireNamespaceConfigPath is the config path backing the namespace flag
	requireNamespaceConfigPath = "konnect.declarative." + requireNamespaceFlagName
	// baseDirFlagName is the CLI flag for the !file base directory boundary
	baseDirFlagName = "base-dir"
	// baseDirConfigPath is the config path backing the base-dir flag
	baseDirConfigPath = "konnect.declarative." + baseDirFlagName
	// requireAnyNamespaceFlagName is the CLI flag for requiring any namespace
	requireAnyNamespaceFlagName = "require-any-namespace"
	// requireAnyNamespaceConfigPath is the config path backing the any namespace flag
	requireAnyNamespaceConfigPath = "konnect.declarative." + requireAnyNamespaceFlagName
)

const diffFieldRedactedValue = "[REDACTED]"

var diffSensitiveExactFieldKeys = map[string]struct{}{
	"access_token":        {},
	"refresh_token":       {},
	"id_token":            {},
	"token":               {},
	"api_key":             {},
	"apikey":              {},
	"x_api_key":           {},
	"secret":              {},
	"password":            {},
	"authorization":       {},
	"cookie":              {},
	"credential":          {},
	"private_key":         {},
	"passphrase":          {},
	"client_secret":       {},
	"set_cookie":          {},
	"konnectaccesstoken":  {},
	"konnectrefreshtoken": {},
}

var diffNonSensitiveTokenFieldKeys = map[string]struct{}{
	"token_count": {},
	"token_type":  {},
}

func addBaseDirFlag(cmd *cobra.Command) {
	cmd.Flags().String(baseDirFlagName, "",
		fmt.Sprintf(`Base directory boundary for !file resolution.
Defaults to each -f source root (file: its parent dir, dir: the directory itself). For stdin, defaults to CWD.
- Config path: [ %s ]`, baseDirConfigPath))
}

func addRequireNamespaceFlags(cmd *cobra.Command) {
	// Add require-any-namespace flag (bool)
	cmd.Flags().Bool(requireAnyNamespaceFlagName, false,
		fmt.Sprintf(`Require explicit namespace on all resources (via kongctl.namespace or _defaults.kongctl.namespace).
Cannot be used with --require-namespace.
- Config path: [ %s ]`, requireAnyNamespaceConfigPath))

	// Add require-namespace flag (StringSlice)
	cmd.Flags().StringSlice(requireNamespaceFlagName, nil,
		fmt.Sprintf(`Require specific namespaces. Accepts comma-separated list or repeated flags.
Cannot be used with --require-any-namespace.
Examples:
  --require-namespace=foo                          # Allow only 'foo' namespace
  --require-namespace=foo,bar                      # Allow 'foo' or 'bar' (comma-separated)
  --require-namespace=foo --require-namespace=bar  # Allow 'foo' or 'bar' (repeated flags)
- Config path: [ %s ]`, requireNamespaceConfigPath))
}

func validateNonEmpty(value, flagName string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("--%s cannot be empty", flagName)
	}
	return nil
}

func resolveBaseDir(command *cobra.Command, cfg config.Hook) (string, error) {
	if command.Flags().Changed(baseDirFlagName) {
		value, err := command.Flags().GetString(baseDirFlagName)
		if err != nil {
			return "", err
		}
		if err := validateNonEmpty(value, baseDirFlagName); err != nil {
			return "", err
		}
		return value, nil
	}

	if cfg == nil {
		return "", nil
	}

	value := strings.TrimSpace(cfg.GetString(baseDirConfigPath))
	if value == "" {
		return "", nil
	}

	return value, nil
}

func normalizeBaseDir(baseDir string) (string, error) {
	if err := validateNonEmpty(baseDir, baseDirFlagName); err != nil {
		return "", err
	}

	baseDir = filepath.Clean(baseDir)
	if !filepath.IsAbs(baseDir) {
		absDir, err := filepath.Abs(baseDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve base dir %q: %w", baseDir, err)
		}
		baseDir = absDir
	}

	info, err := os.Stat(baseDir)
	if err != nil {
		return "", fmt.Errorf("base dir %q not found: %w", baseDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("base dir %q is not a directory", baseDir)
	}

	return baseDir, nil
}

func newDeclarativeLoader(command *cobra.Command, cfg config.Hook) (*loader.Loader, error) {
	baseDir, err := resolveBaseDir(command, cfg)
	if err != nil {
		return nil, err
	}
	if baseDir == "" {
		return loader.New(), nil
	}
	baseDir, err = normalizeBaseDir(baseDir)
	if err != nil {
		return nil, err
	}
	return loader.NewWithBaseDir(baseDir), nil
}

func parseNamespaceRequirement(
	command *cobra.Command,
	cfg config.Hook,
	nsValidator *validator.NamespaceValidator,
) (validator.NamespaceRequirement, error) {
	// Check for mutual exclusivity
	anyNamespaceSet := command.Flags().Changed(requireAnyNamespaceFlagName)
	specificNamespacesSet := command.Flags().Changed(requireNamespaceFlagName)

	if anyNamespaceSet && specificNamespacesSet {
		return validator.NamespaceRequirement{}, fmt.Errorf(
			"--%s and --%s are mutually exclusive",
			requireAnyNamespaceFlagName, requireNamespaceFlagName)
	}

	// Check config for mutual exclusivity as well
	configAnyNamespace := cfg.GetBool(requireAnyNamespaceConfigPath)
	configSpecificNamespaces := cfg.GetStringSlice(requireNamespaceConfigPath)

	if !anyNamespaceSet && !specificNamespacesSet {
		// No command-line flags, check config
		if configAnyNamespace && len(configSpecificNamespaces) > 0 {
			return validator.NamespaceRequirement{}, fmt.Errorf(
				"config has both %s and %s set, but they are mutually exclusive",
				requireAnyNamespaceConfigPath, requireNamespaceConfigPath)
		}
	}

	// Handle --require-any-namespace flag
	if anyNamespaceSet {
		anyNamespace, _ := command.Flags().GetBool(requireAnyNamespaceFlagName)
		if anyNamespace {
			return validator.NamespaceRequirement{
				Mode:              validator.NamespaceRequirementAny,
				AllowedNamespaces: []string{},
			}, nil
		}
	} else if configAnyNamespace && !specificNamespacesSet {
		// Use config value for any-namespace
		return validator.NamespaceRequirement{
			Mode:              validator.NamespaceRequirementAny,
			AllowedNamespaces: []string{},
		}, nil
	}

	// Handle --require-namespace flag (specific namespaces)
	if specificNamespacesSet {
		namespaces, err := command.Flags().GetStringSlice(requireNamespaceFlagName)
		if err != nil {
			return validator.NamespaceRequirement{}, err
		}
		if len(namespaces) == 0 {
			return validator.NamespaceRequirement{Mode: validator.NamespaceRequirementNone}, nil
		}
		return nsValidator.ParseNamespaceRequirementSlice(namespaces)
	} else if len(configSpecificNamespaces) > 0 && !anyNamespaceSet {
		// Use config value for specific namespaces
		return nsValidator.ParseNamespaceRequirementSlice(configSpecificNamespaces)
	}

	// Neither flag set - no namespace requirement
	return validator.NamespaceRequirement{Mode: validator.NamespaceRequirementNone}, nil
}

func resolveNamespaceRequirement(
	command *cobra.Command,
	cfg config.Hook,
) (*validator.NamespaceValidator, validator.NamespaceRequirement, error) {
	nsValidator := validator.NewNamespaceValidator()
	requirement, err := parseNamespaceRequirement(command, cfg, nsValidator)
	if err != nil {
		return nil, validator.NamespaceRequirement{}, err
	}
	return nsValidator, requirement, nil
}

func planGenerator(helper cmd.Helper) string {
	buildInfo, err := helper.GetBuildInfo()
	if err != nil || buildInfo == nil {
		return fmt.Sprintf("%s/dev", meta.CLIName)
	}

	version := strings.TrimSpace(buildInfo.Version)
	if version == "" {
		version = "dev"
	}

	return fmt.Sprintf("%s/%s", meta.CLIName, version)
}

func withDeclarativeHTTPLogContext(
	ctx context.Context,
	command *cobra.Command,
	verb verbs.VerbValue,
	mode planner.PlanMode,
) context.Context {
	commandPath := ""
	if command != nil {
		commandPath = strings.TrimSpace(command.CommandPath())
	}

	return applog.WithHTTPLogContext(ctx, applog.HTTPLogContext{
		CommandPath:    commandPath,
		CommandVerb:    verb.String(),
		CommandMode:    string(mode),
		CommandProduct: "konnect",
		Workflow:       "declarative",
		WorkflowMode:   string(mode),
	})
}

// NewDeclarativeCmd creates the appropriate declarative command based on the verb
func NewDeclarativeCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	// Handle supported declarative verbs
	if verb == verbs.Plan {
		return newDeclarativePlanCmd(), nil
	}
	if verb == verbs.Sync {
		return newDeclarativeSyncCmd(), nil
	}
	if verb == verbs.Diff {
		return newDeclarativeDiffCmd(), nil
	}
	if verb == verbs.Export {
		return newDeclarativeExportCmd(), nil
	}
	if verb == verbs.Apply {
		return newDeclarativeApplyCmd(), nil
	}
	if verb == verbs.Create {
		return newDeclarativeCreateCmd(), nil
	}
	if verb == verbs.Delete {
		return newDeclarativeDeleteCmd(), nil
	}

	// Unsupported verbs
	return nil, fmt.Errorf("verb %s does not support declarative configuration", verb)
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
	addBaseDirFlag(cmd)
	cmd.Flags().String("output-file", "", "Save plan artifact to file")
	cmd.Flags().String("mode", "sync", "Plan generation mode (create|sync|apply|delete)")
	addRequireNamespaceFlags(cmd)

	return cmd
}

func parsePlanMode(mode string) (planner.PlanMode, error) {
	switch mode {
	case string(planner.PlanModeCreate):
		return planner.PlanModeCreate, nil
	case string(planner.PlanModeSync):
		return planner.PlanModeSync, nil
	case string(planner.PlanModeApply):
		return planner.PlanModeApply, nil
	case string(planner.PlanModeDelete):
		return planner.PlanModeDelete, nil
	default:
		return "", fmt.Errorf("invalid mode %q: must be 'create', 'sync', 'apply', or 'delete'", mode)
	}
}

func runPlan(command *cobra.Command, args []string) error {
	// Silence usage for all runtime errors (command syntax is already valid at this point)
	command.SilenceUsage = true

	// Reject --output/-o flag: plan always outputs JSON; use --output-file to save to a file
	if outputFlag := command.Flag(cmdcommon.OutputFlagName); outputFlag != nil && outputFlag.Changed {
		return fmt.Errorf(
			"flags -o/--%s are not supported for the plan command; use --output-file to save the plan to a file",
			cmdcommon.OutputFlagName,
		)
	}

	ctx := command.Context()
	filenames, _ := command.Flags().GetStringSlice("filename")
	recursive, _ := command.Flags().GetBool("recursive")
	mode, _ := command.Flags().GetString("mode")
	outputFile, _ := command.Flags().GetString("output-file")

	planMode, err := parsePlanMode(mode)
	if err != nil {
		return err
	}

	ctx = withDeclarativeHTTPLogContext(ctx, command, verbs.Plan, planMode)
	command.SetContext(ctx)

	// Build helper
	helper := cmd.BuildHelper(command, args)
	generator := planGenerator(helper)

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

	nsValidator, requirement, err := resolveNamespaceRequirement(command, cfg)
	if err != nil {
		return err
	}

	// Parse sources from filenames
	sources, err := loader.ParseSources(filenames)
	if err != nil {
		return fmt.Errorf("failed to parse sources: %w", err)
	}

	// Load configuration
	ldr, err := newDeclarativeLoader(command, cfg)
	if err != nil {
		return err
	}
	resourceSet, err := ldr.LoadFromSources(sources, recursive)
	if err != nil {
		// Provide more helpful error message for common cases
		if len(filenames) == 0 && strings.Contains(err.Error(), "no YAML files found") {
			return fmt.Errorf(
				"no configuration files found in current directory. Use -f to specify files or directories",
			)
		}
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Check if configuration is empty
	totalResources := resourceSet.ResourceCount()

	if err := nsValidator.ValidateNamespaceRequirement(resourceSet, requirement); err != nil {
		return err
	}

	if totalResources == 0 {
		// Check if we're using default directory (no explicit sources)
		if len(filenames) == 0 {
			return fmt.Errorf(
				"no configuration files found in current directory. Use -f to specify files or directories",
			)
		}

		// In sync mode, empty config is valid - it means delete all managed resources.
		// In create, apply, and delete modes, we need at least one resource.
		if planMode == planner.PlanModeCreate ||
			planMode == planner.PlanModeApply ||
			planMode == planner.PlanModeDelete {
			return fmt.Errorf("no resources found in configuration files")
		}

		// For sync mode, log that we're checking for deletions
		if outputFile == "" {
			namespaces := resourceSet.DefaultNamespaces
			if len(namespaces) == 0 && resourceSet.DefaultNamespace != "" {
				namespaces = []string{resourceSet.DefaultNamespace}
			}
			if len(namespaces) > 0 {
				fmt.Fprintf(command.OutOrStderr(),
					"No resources defined in configuration. Using namespace(s) '%s' from _defaults.\n"+
						"Checking for managed resources to remove...\n",
					strings.Join(namespaces, ", "))
			} else {
				fmt.Fprintln(command.OutOrStderr(),
					"No resources defined in configuration. Checking 'default' namespace for managed resources to remove...")
			}
		}
	}

	// Create planner
	stateClient := createStateClient(kkClient)
	p := planner.NewPlanner(stateClient, logger)

	// Show namespace processing info if outputting to file
	if outputFile != "" && totalResources > 0 {
		// Count namespaces in resources
		namespaces := make(map[string]bool)
		for _, portal := range resourceSet.Portals {
			ns := "default"
			if portal.Kongctl != nil && portal.Kongctl.Namespace != nil {
				ns = *portal.Kongctl.Namespace
			}
			namespaces[ns] = true
		}
		for _, api := range resourceSet.APIs {
			ns := "default"
			if api.Kongctl != nil && api.Kongctl.Namespace != nil {
				ns = *api.Kongctl.Namespace
			}
			namespaces[ns] = true
		}
		for _, catalogService := range resourceSet.CatalogServices {
			ns := "default"
			if catalogService.Kongctl != nil && catalogService.Kongctl.Namespace != nil {
				ns = *catalogService.Kongctl.Namespace
			}
			namespaces[ns] = true
		}
		for _, authStrategy := range resourceSet.ApplicationAuthStrategies {
			ns := "default"
			if authStrategy.Kongctl != nil && authStrategy.Kongctl.Namespace != nil {
				ns = *authStrategy.Kongctl.Namespace
			}
			namespaces[ns] = true
		}
		for _, egwControlPlane := range resourceSet.EventGatewayControlPlanes {
			ns := "default"
			if egwControlPlane.Kongctl != nil && egwControlPlane.Kongctl.Namespace != nil {
				ns = *egwControlPlane.Kongctl.Namespace
			}
			namespaces[ns] = true
		}

		if len(namespaces) > 1 {
			fmt.Fprintf(command.OutOrStderr(), "Processing %d namespaces...\n", len(namespaces))
		}
	}

	deckOpts, err := deckPlanOptions(resourceSet, cfg, logger)
	if err != nil {
		return err
	}

	// Generate plan
	opts := planner.Options{
		Mode:      planMode,
		Generator: generator,
		Deck:      deckOpts,
	}
	plan, err := p.GeneratePlan(ctx, resourceSet, opts)
	if err != nil {
		return fmt.Errorf("failed to generate plan: %w", err)
	}

	if err := normalizeDeckBaseDirs(plan, outputFile); err != nil {
		return err
	}

	// Marshal plan to JSON
	planJSON, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	// Handle output
	if outputFile != "" {
		// Save to file
		if err := os.WriteFile(outputFile, planJSON, 0o600); err != nil {
			return fmt.Errorf("failed to write plan file: %w", err)
		}
	} else {
		// Output to stdout
		fmt.Fprintln(command.OutOrStdout(), string(planJSON))
	}

	return nil
}

func normalizeDeckBaseDirs(plan *planner.Plan, outputFile string) error {
	if plan == nil {
		return nil
	}

	var planDir string
	if strings.TrimSpace(outputFile) != "" {
		planDir = filepath.Dir(outputFile)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to resolve current working directory: %w", err)
		}
		planDir = cwd
	}

	planDirAbs, err := filepath.Abs(planDir)
	if err != nil {
		return fmt.Errorf("failed to resolve plan directory %q: %w", planDir, err)
	}

	updated := false
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType != planner.ResourceTypeDeck {
			continue
		}
		raw, ok := change.Fields["deck_base_dir"].(string)
		if !ok {
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" || !filepath.IsAbs(raw) {
			continue
		}
		rel, err := filepath.Rel(planDirAbs, raw)
		if err != nil {
			return fmt.Errorf("failed to resolve deck base dir for %s: %w", change.ResourceRef, err)
		}
		change.Fields["deck_base_dir"] = rel
		updated = true
	}

	if updated {
		plan.UpdateSummary()
	}

	return nil
}

func deckPlanOptions(
	resourceSet *resources.ResourceSet,
	cfg config.Hook,
	logger *slog.Logger,
) (planner.DeckOptions, error) {
	if !resourceSetHasDeckConfig(resourceSet) {
		return planner.DeckOptions{}, nil
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return planner.DeckOptions{}, err
	}

	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return planner.DeckOptions{}, err
	}

	return planner.DeckOptions{
		KonnectToken:   token,
		KonnectAddress: baseURL,
	}, nil
}

func resourceSetHasDeckConfig(resourceSet *resources.ResourceSet) bool {
	if resourceSet == nil {
		return false
	}
	for i := range resourceSet.ControlPlanes {
		if resourceSet.ControlPlanes[i].HasDeckConfig() {
			return true
		}
	}
	return false
}

func resolvePlanBaseDir(planFile string) string {
	planFile = strings.TrimSpace(planFile)
	if planFile == "" {
		return ""
	}
	if planFile == "-" {
		cwd, err := os.Getwd()
		if err != nil {
			return ""
		}
		return cwd
	}
	dir := filepath.Dir(planFile)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

// checkStdinApprovalConflict returns an error if stdin is being used as input
// while interactive confirmation is required (no auto-approve or dry-run).
// planFile is the --plan flag value; filenames are the --filename flag values.
func checkStdinApprovalConflict(planFile string, filenames []string) error {
	usingStdin := planFile == "-" || (planFile == "" && slices.Contains(filenames, "-"))
	if !usingStdin {
		return nil
	}
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return fmt.Errorf("cannot use stdin for input without --auto-approve flag " +
			"(no terminal available for interactive confirmation). " +
			"Use --auto-approve to skip confirmation when piping commands")
	}
	defer tty.Close()
	return nil
}

func runDiff(command *cobra.Command, args []string) error {
	// Silence usage for all runtime errors (command syntax is already valid at this point)
	command.SilenceUsage = true

	mode, _ := command.Flags().GetString("mode")
	planMode, err := parsePlanMode(mode)
	if err != nil {
		return err
	}

	ctx := command.Context()
	ctx = withDeclarativeHTTPLogContext(ctx, command, verbs.Diff, planMode)
	command.SetContext(ctx)

	helper := cmd.BuildHelper(command, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	nsValidator, requirement, err := resolveNamespaceRequirement(command, cfg)
	if err != nil {
		return err
	}

	planFile, _ := command.Flags().GetString("plan")
	if command.Flags().Changed("mode") && planFile != "" {
		return fmt.Errorf("--mode cannot be used together with --plan; plan mode is read from the plan artifact")
	}
	if requirement.Mode != validator.NamespaceRequirementNone && planFile != "" {
		return fmt.Errorf(
			"--%s cannot be used together with --plan; generate the plan with namespace enforcement enabled instead",
			requireNamespaceFlagName,
		)
	}

	var plan *planner.Plan

	if planFile != "" {
		plan, err = common.LoadPlan(planFile, command.InOrStdin())
		if err != nil {
			return err
		}
	} else {
		filenames, _ := command.Flags().GetStringSlice("filename")
		recursive, _ := command.Flags().GetBool("recursive")
		generator := planGenerator(helper)

		logger, err := helper.GetLogger()
		if err != nil {
			return err
		}

		kkClient, err := helper.GetKonnectSDK(cfg, logger)
		if err != nil {
			return fmt.Errorf("failed to initialize Konnect client: %w", err)
		}

		sources, err := loader.ParseSources(filenames)
		if err != nil {
			return fmt.Errorf("failed to parse sources: %w", err)
		}

		ldr, err := newDeclarativeLoader(command, cfg)
		if err != nil {
			return err
		}
		resourceSet, err := ldr.LoadFromSources(sources, recursive)
		if err != nil {
			if len(filenames) == 0 && strings.Contains(err.Error(), "no YAML files found") {
				return fmt.Errorf("no configuration files found. Use -f to specify files or --plan to use existing plan")
			}
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		if err := nsValidator.ValidateNamespaceRequirement(resourceSet, requirement); err != nil {
			return err
		}

		totalResources := resourceSet.ResourceCount()
		if totalResources == 0 {
			if len(filenames) == 0 {
				return fmt.Errorf("no configuration files found. Use -f to specify files or --plan to use existing plan")
			}
			return fmt.Errorf("no resources found in configuration files")
		}

		stateClient := createStateClient(kkClient)
		p := planner.NewPlanner(stateClient, logger)
		deckOpts, err := deckPlanOptions(resourceSet, cfg, logger)
		if err != nil {
			return err
		}
		opts := planner.Options{
			Mode:      planMode,
			Generator: generator,
			Deck:      deckOpts,
		}
		plan, err = p.GeneratePlan(ctx, resourceSet, opts)
		if err != nil {
			return fmt.Errorf("failed to generate plan: %w", err)
		}
	}

	// Display diff based on output format
	outputFormat, _ := command.Flags().GetString("output")
	fullContent, _ := command.Flags().GetBool("full-content")

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

	case textOutputFormat:
		// Human-readable text output
		return displayTextDiff(command, plan, fullContent)

	default:
		return fmt.Errorf("unsupported output format: %s (use text, json, or yaml)", outputFormat)
	}
}

func displayTextDiff(command *cobra.Command, plan *planner.Plan, fullContent bool) error {
	out := command.OutOrStdout()

	// Handle empty plan
	if plan.IsEmpty() {
		fmt.Fprintln(out, "No changes detected. Konnect is up to date.")
		return nil
	}

	// Display summary
	createCount := plan.Summary.ByAction[planner.ActionCreate]
	updateCount := plan.Summary.ByAction[planner.ActionUpdate]
	deleteCount := plan.Summary.ByAction[planner.ActionDelete]
	externalToolCount := plan.Summary.ByAction[planner.ActionExternalTool]

	summaryParts := []string{
		fmt.Sprintf("%d to add", createCount),
		fmt.Sprintf("%d to change", updateCount),
	}
	if deleteCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d to destroy", deleteCount))
	}
	if externalToolCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d external tool step", externalToolCount))
	}
	fmt.Fprintf(out, "Plan: %s\n\n", strings.Join(summaryParts, ", "))

	// Display warnings if any
	if len(plan.Warnings) > 0 {
		fmt.Fprintln(out, "Warnings:")
		for _, warning := range plan.Warnings {
			fmt.Fprintf(out, "  ⚠ [%s] %s\n", warning.ChangeID, warning.Message)
		}
		fmt.Fprintln(out)
	}

	// Group changes by namespace
	changesByNamespace := make(map[string][]*planner.PlannedChange)
	namespaces := make([]string, 0)
	namespaceSeen := make(map[string]bool)

	// Build namespace groups following execution order
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

		namespace := change.Namespace
		if namespace == "" {
			namespace = "default"
		}

		if !namespaceSeen[namespace] {
			namespaceSeen[namespace] = true
			namespaces = append(namespaces, namespace)
		}

		changesByNamespace[namespace] = append(changesByNamespace[namespace], change)
	}

	// Sort namespaces for consistent output
	sort.Strings(namespaces)

	// Display changes grouped by namespace
	for nsIdx, namespace := range namespaces {
		// Show namespace header
		fmt.Fprintf(out, "=== Namespace: %s ===\n", namespace)

		// Display each change in this namespace
		for _, change := range changesByNamespace[namespace] {

			switch change.Action {
			case planner.ActionCreate:
				fmt.Fprintf(out, "+ [%s] %s %q will be created\n",
					change.ID, change.ResourceType, change.ResourceRef)

				// Show key fields
				for field, value := range change.Fields {
					displayField(out, field, value, "  ", fullContent)
				}

				// Show protection status
				if prot, ok := change.Protection.(bool); ok {
					if prot {
						fmt.Fprintln(out, "  protection: enabled")
					} else {
						fmt.Fprintln(out, "  protection: disabled")
					}
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
				} else if prot, ok := change.Protection.(bool); ok {
					if prot {
						fmt.Fprintln(out, "  protection: enabled (no change)")
					} else {
						fmt.Fprintln(out, "  protection: disabled (no change)")
					}
				}

				if len(change.ChangedFields) > 0 {
					displayFieldChanges(out, change.ChangedFields, "  ", fullContent)
				} else {
					// Backward compatibility with plans that only have raw update fields.
					for field, value := range change.Fields {
						if fc, ok := value.(planner.FieldChange); ok {
							displayFieldChange(out, field, fc.Old, fc.New, "  ", fullContent)
						} else if fc, ok := value.(map[string]any); ok {
							// Handle FieldChange that was unmarshaled into map[string]any.
							oldVal, hasOld := fc["old"]
							newVal, hasNew := fc["new"]
							if hasOld && hasNew {
								displayFieldChange(out, field, oldVal, newVal, "  ", fullContent)
								continue
							}
							displayField(out, field, value, "  ", fullContent)
						} else {
							displayField(out, field, value, "  ", fullContent)
						}
					}
				}

			case planner.ActionDelete:
				// DELETE action (future implementation)
				fmt.Fprintf(out, "- [%s] %s %q will be deleted\n",
					change.ID, change.ResourceType, change.ResourceRef)
			case planner.ActionExternalTool:
				fmt.Fprintf(out, "> [%s] %s %q will run external tool steps\n",
					change.ID, change.ResourceType, change.ResourceRef)

				for field, value := range change.Fields {
					displayField(out, field, value, "  ", fullContent)
				}
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

		// Add spacing between namespaces
		if nsIdx < len(namespaces)-1 {
			fmt.Fprintln(out)
		}
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

func displayFieldChanges(
	out io.Writer,
	changes map[string]planner.FieldChange,
	indent string,
	fullContent bool,
) {
	keys := make([]string, 0, len(changes))
	for field := range changes {
		keys = append(keys, field)
	}
	sort.Strings(keys)

	for _, field := range keys {
		fc := changes[field]
		displayFieldChange(out, field, fc.Old, fc.New, indent, fullContent)
	}
}

func displayFieldChange(
	out io.Writer,
	field string,
	oldValue any,
	newValue any,
	indent string,
	fullContent bool,
) {
	oldText := formatFieldValueForField(field, oldValue, fullContent)
	newText := formatFieldValueForField(field, newValue, fullContent)
	fmt.Fprintf(out, "%s%s: %s → %s\n", indent, field, oldText, newText)
}

func formatFieldValue(value any, fullContent bool) string {
	const maxDisplayLength = 500
	value = dereferenceFieldValue(value)

	switch v := value.(type) {
	case string:
		if !fullContent && len(v) > maxDisplayLength {
			lines := strings.Count(v, "\n") + 1
			return fmt.Sprintf("<%d bytes, %d lines>", len(v), lines)
		}
		return fmt.Sprintf("%q", v)
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatFieldValueForField(field string, value any, fullContent bool) string {
	resolved := dereferenceFieldValue(value)
	if isSensitiveDiffField(field) {
		if resolved == nil {
			return "null"
		}
		return diffFieldRedactedValue
	}
	return formatFieldValue(resolved, fullContent)
}

func isSensitiveDiffField(field string) bool {
	normalized := normalizeFieldKey(field)
	if normalized == "" {
		return false
	}

	if _, ok := diffSensitiveExactFieldKeys[normalized]; ok {
		return true
	}

	if _, ok := diffNonSensitiveTokenFieldKeys[normalized]; ok {
		return false
	}

	if containsSegment(normalized, "secret") ||
		containsSegment(normalized, "password") ||
		containsSegment(normalized, "credential") ||
		containsSegment(normalized, "passphrase") ||
		hasSegmentPair(normalized, "private", "key") ||
		hasSegmentPair(normalized, "api", "key") ||
		hasSegmentPair(normalized, "client", "secret") {
		return true
	}

	if strings.Contains(normalized, "access_token") || strings.Contains(normalized, "refresh_token") {
		return true
	}
	if strings.HasSuffix(normalized, "_token") {
		return true
	}

	return false
}

func containsSegment(normalized, segment string) bool {
	return slices.Contains(strings.Split(normalized, "_"), segment)
}

func hasSegmentPair(normalized, first, second string) bool {
	parts := strings.Split(normalized, "_")
	for idx := 0; idx < len(parts)-1; idx++ {
		if parts[idx] == first && parts[idx+1] == second {
			return true
		}
	}
	return false
}

func normalizeFieldKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	runes := []rune(key)
	out := make([]rune, 0, len(runes))
	appendUnderscore := func() {
		if len(out) > 0 && out[len(out)-1] != '_' {
			out = append(out, '_')
		}
	}

	for idx, current := range runes {
		switch {
		case current == '_' || current == '-' || current == '.' ||
			current == '[' || current == ']' || current == '/' || unicode.IsSpace(current):
			appendUnderscore()
		case unicode.IsUpper(current):
			if idx > 0 {
				prev := runes[idx-1]
				nextIsLower := idx+1 < len(runes) && unicode.IsLower(runes[idx+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextIsLower) {
					appendUnderscore()
				}
			}
			out = append(out, unicode.ToLower(current))
		default:
			out = append(out, unicode.ToLower(current))
		}
	}

	return strings.Trim(string(out), "_")
}

func dereferenceFieldValue(value any) any {
	for {
		rv := reflect.ValueOf(value)
		if !rv.IsValid() {
			return nil
		}
		if rv.Kind() != reflect.Pointer {
			return value
		}
		if rv.IsNil() {
			return nil
		}
		value = rv.Elem().Interface()
	}
}

func displayField(out io.Writer, field string, value any, indent string, fullContent bool) {
	if isSensitiveDiffField(field) {
		if dereferenceFieldValue(value) != nil {
			fmt.Fprintf(out, "%s%s: %s\n", indent, field, diffFieldRedactedValue)
		}
		return
	}

	switch v := value.(type) {
	case string:
		if v != "" {
			// Check if string is large and should be summarized
			const maxDisplayLength = 500
			if !fullContent && len(v) > maxDisplayLength {
				// Count lines in the string
				lines := strings.Count(v, "\n") + 1
				fmt.Fprintf(out, "%s%s: <%d bytes, %d lines>\n", indent, field, len(v), lines)
			} else {
				fmt.Fprintf(out, "%s%s: %q\n", indent, field, v)
			}
		}
	case bool:
		fmt.Fprintf(out, "%s%s: %t\n", indent, field, v)
	case float64:
		fmt.Fprintf(out, "%s%s: %g\n", indent, field, v)
	case map[string]any:
		// Skip empty maps
		if len(v) == 0 {
			return
		}
		fmt.Fprintf(out, "%s%s:\n", indent, field)
		for k, val := range v {
			displayField(out, k, val, indent+"  ", fullContent)
		}
	case []any:
		// Skip empty slices
		if len(v) == 0 {
			return
		}
		fmt.Fprintf(out, "%s%s:\n", indent, field)
		for i, item := range v {
			displayField(out, fmt.Sprintf("[%d]", i), item, indent+"  ", fullContent)
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
		RunE: runSync,
	}

	// Add declarative config flags (matching apply command pattern)
	cmd.Flags().StringSliceP("filename", "f", []string{},
		"Filename or directory to files to use to create the resource (can specify multiple)")
	cmd.Flags().BoolP("recursive", "R", false,
		"Process the directory used in -f, --filename recursively")
	addBaseDirFlag(cmd)
	cmd.Flags().String("plan", "", "Path to existing plan file")
	cmd.Flags().Bool("dry-run", false, "Preview changes without applying them")
	cmd.Flags().Bool("auto-approve", false, "Skip confirmation prompt")
	cmd.Flags().StringP("output", "o", textOutputFormat, "Output format (text|json|yaml)")
	cmd.Flags().String("execution-report-file", "", "Save execution report as JSON to file")
	addRequireNamespaceFlags(cmd)

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
	addBaseDirFlag(cmd)
	cmd.Flags().String("plan", "", "Path to existing plan file to display")
	cmd.Flags().String("mode", "sync", "Diff mode (create|sync|apply|delete)")
	cmd.Flags().StringP("output", "o", textOutputFormat, "Output format (text, json, or yaml)")
	cmd.Flags().Bool("full-content", false, "Display full content for large fields instead of summary")
	addRequireNamespaceFlags(cmd)

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

func runCreate(command *cobra.Command, args []string) error {
	command.SilenceUsage = true

	ctx := command.Context()
	ctx = withDeclarativeHTTPLogContext(ctx, command, verbs.Create, planner.PlanModeCreate)
	command.SetContext(ctx)

	planFile, _ := command.Flags().GetString("plan")
	dryRun, _ := command.Flags().GetBool("dry-run")
	autoApprove, _ := command.Flags().GetBool("auto-approve")
	outputFormat, _ := command.Flags().GetString("output")
	filenames, _ := command.Flags().GetStringSlice("filename")

	if !dryRun && !autoApprove && outputFormat != textOutputFormat {
		return fmt.Errorf("cannot use %s output format without --auto-approve or --dry-run flag "+
			"(interactive confirmation not available with structured output)", outputFormat)
	}

	var usingStdinForInput bool
	if !dryRun && !autoApprove {
		if err := checkStdinApprovalConflict(planFile, filenames); err != nil {
			return err
		}
		usingStdinForInput = planFile == "-" || (planFile == "" && slices.Contains(filenames, "-"))
	}

	helper := cmd.BuildHelper(command, args)
	generator := planGenerator(helper)

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	kkClient, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize Konnect client: %w", err)
	}

	nsValidator, requirement, err := resolveNamespaceRequirement(command, cfg)
	if err != nil {
		return err
	}

	var plan *planner.Plan
	if requirement.Mode != validator.NamespaceRequirementNone && planFile != "" {
		return fmt.Errorf(
			"--%s cannot be used together with --plan; generate the plan with namespace enforcement enabled instead",
			requireNamespaceFlagName,
		)
	}
	if planFile != "" {
		if outputFormat == textOutputFormat {
			if planFile == "-" {
				fmt.Fprintf(command.OutOrStderr(), "Using plan from: stdin\n")
			} else {
				fmt.Fprintf(command.OutOrStderr(), "Using plan from: %s\n", planFile)
			}
		}

		plan, err = common.LoadPlan(planFile, command.InOrStdin())
		if err != nil {
			return err
		}
	} else {
		recursive, _ := command.Flags().GetBool("recursive")

		sources, err := loader.ParseSources(filenames)
		if err != nil {
			return fmt.Errorf("failed to parse sources: %w", err)
		}

		ldr, err := newDeclarativeLoader(command, cfg)
		if err != nil {
			return err
		}
		resourceSet, err := ldr.LoadFromSources(sources, recursive)
		if err != nil {
			if len(filenames) == 0 && strings.Contains(err.Error(), "no YAML files found") {
				return fmt.Errorf("no configuration files found in current directory. Use -f to specify files or directories")
			}
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		if err := nsValidator.ValidateNamespaceRequirement(resourceSet, requirement); err != nil {
			return err
		}

		totalResources := resourceSet.ResourceCount()
		if totalResources == 0 {
			if len(filenames) == 0 {
				return fmt.Errorf("no configuration files found in current directory. Use -f to specify files or directories")
			}
			return fmt.Errorf("no resources found in configuration files")
		}

		stateClient := createStateClient(kkClient)
		p := planner.NewPlanner(stateClient, logger)
		deckOpts, err := deckPlanOptions(resourceSet, cfg, logger)
		if err != nil {
			return err
		}
		opts := planner.Options{
			Mode:      planner.PlanModeCreate,
			Generator: generator,
			Deck:      deckOpts,
		}
		plan, err = p.GeneratePlan(ctx, resourceSet, opts)
		if err != nil {
			return fmt.Errorf("failed to generate plan: %w", err)
		}
	}

	ctx = context.WithValue(ctx, currentPlanKey, plan)
	if planFile != "" {
		ctx = context.WithValue(ctx, planFileKey, planFile)
	}
	command.SetContext(ctx)

	if err := validateCreatePlan(plan); err != nil {
		return err
	}

	if plan.IsEmpty() {
		if outputFormat == textOutputFormat {
			fmt.Fprintln(command.OutOrStderr(), "No changes needed. No creatable resources found in the input configuration.")
			return nil
		}

		emptyResult := &executor.ExecutionResult{
			SuccessCount:   0,
			FailureCount:   0,
			SkippedCount:   0,
			DryRun:         dryRun,
			ChangesApplied: []executor.AppliedChange{},
		}
		return outputExecutionResult(command, emptyResult, outputFormat)
	}

	if outputFormat == textOutputFormat {
		common.DisplayPlanSummary(plan, command.OutOrStderr())

		if !dryRun && !autoApprove {
			inputReader := command.InOrStdin()
			if usingStdinForInput {
				tty, err := os.Open("/dev/tty")
				if err != nil {
					return fmt.Errorf("cannot open terminal for confirmation: %w", err)
				}
				defer tty.Close()
				inputReader = tty
			}

			if !common.ConfirmExecution(plan, command.OutOrStdout(), command.OutOrStderr(), inputReader) {
				return fmt.Errorf("create cancelled")
			}
		}

		fmt.Fprintln(command.OutOrStderr())
	}

	stateClient := createStateClient(kkClient)

	var reporter executor.ProgressReporter
	if outputFormat == textOutputFormat {
		reporter = executor.NewConsoleReporterWithOptions(command.OutOrStderr(), dryRun)
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return err
	}
	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return err
	}

	exec := executor.NewWithOptions(stateClient, reporter, dryRun, executor.Options{
		KonnectToken:   token,
		KonnectBaseURL: baseURL,
		Mode:           planner.PlanModeCreate,
		PlanBaseDir:    resolvePlanBaseDir(planFile),
	})

	result := exec.Execute(ctx, plan)

	if err := outputExecutionResult(command, result, outputFormat); err != nil {
		return err
	}

	if result.HasErrors() && result.SuccessCount == 0 {
		return fmt.Errorf("execution completed with %d errors", result.FailureCount)
	}

	return nil
}

func runApply(command *cobra.Command, args []string) error {
	// Silence usage for all runtime errors (command syntax is already valid at this point)
	command.SilenceUsage = true

	ctx := command.Context()
	ctx = withDeclarativeHTTPLogContext(ctx, command, verbs.Apply, planner.PlanModeApply)
	command.SetContext(ctx)

	planFile, _ := command.Flags().GetString("plan")
	dryRun, _ := command.Flags().GetBool("dry-run")
	autoApprove, _ := command.Flags().GetBool("auto-approve")
	outputFormat, _ := command.Flags().GetString("output")
	filenames, _ := command.Flags().GetStringSlice("filename")

	// Early check for non-text output without auto-approve
	if !dryRun && !autoApprove && outputFormat != textOutputFormat {
		return fmt.Errorf("cannot use %s output format without --auto-approve or --dry-run flag "+
			"(interactive confirmation not available with structured output)", outputFormat)
	}

	var usingStdinForInput bool
	if !dryRun && !autoApprove {
		if err := checkStdinApprovalConflict(planFile, filenames); err != nil {
			return err
		}
		usingStdinForInput = planFile == "-" || (planFile == "" && slices.Contains(filenames, "-"))
	}

	// Build helper
	helper := cmd.BuildHelper(command, args)
	generator := planGenerator(helper)

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

	nsValidator, requirement, err := resolveNamespaceRequirement(command, cfg)
	if err != nil {
		return err
	}

	// Load or generate plan
	var plan *planner.Plan
	if requirement.Mode != validator.NamespaceRequirementNone && planFile != "" {
		return fmt.Errorf(
			"--%s cannot be used together with --plan; generate the plan with namespace enforcement enabled instead",
			requireNamespaceFlagName,
		)
	}
	if planFile != "" {
		// Show plan source information early
		if outputFormat == textOutputFormat {
			if planFile == "-" {
				fmt.Fprintf(command.OutOrStderr(), "Using plan from: stdin\n")
			} else {
				fmt.Fprintf(command.OutOrStderr(), "Using plan from: %s\n", planFile)
			}
		}

		// Load existing plan
		plan, err = common.LoadPlan(planFile, command.InOrStdin())
		if err != nil {
			return err
		}
	} else {

		// Generate plan from configuration files
		recursive, _ := command.Flags().GetBool("recursive")

		// Parse sources from filenames
		sources, err := loader.ParseSources(filenames)
		if err != nil {
			return fmt.Errorf("failed to parse sources: %w", err)
		}

		// Load configuration
		ldr, err := newDeclarativeLoader(command, cfg)
		if err != nil {
			return err
		}
		resourceSet, err := ldr.LoadFromSources(sources, recursive)
		if err != nil {
			// Provide more helpful error message for common cases
			if len(filenames) == 0 && strings.Contains(err.Error(), "no YAML files found") {
				return fmt.Errorf("no configuration files found in current directory. Use -f to specify files or directories")
			}
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		if err := nsValidator.ValidateNamespaceRequirement(resourceSet, requirement); err != nil {
			return err
		}

		// Check if configuration is empty
		totalResources := resourceSet.ResourceCount()

		if totalResources == 0 {
			// Check if we're using default directory (no explicit sources)
			if len(filenames) == 0 {
				return fmt.Errorf("no configuration files found in current directory. Use -f to specify files or directories")
			}
			return fmt.Errorf("no resources found in configuration files")
		}

		// Create planner
		stateClient := createStateClient(kkClient)
		p := planner.NewPlanner(stateClient, logger)
		deckOpts, err := deckPlanOptions(resourceSet, cfg, logger)
		if err != nil {
			return err
		}

		// Generate plan in apply mode
		opts := planner.Options{
			Mode:      planner.PlanModeApply,
			Generator: generator,
			Deck:      deckOpts,
		}
		plan, err = p.GeneratePlan(ctx, resourceSet, opts)
		if err != nil {
			return fmt.Errorf("failed to generate plan: %w", err)
		}
	}

	// Store plan in context for output formatting
	ctx = context.WithValue(ctx, currentPlanKey, plan)
	// Store plan file path if provided
	if planFile != "" {
		ctx = context.WithValue(ctx, planFileKey, planFile)
	}
	command.SetContext(ctx)

	// Validate plan for apply
	if err := validateApplyPlan(plan, command); err != nil {
		return err
	}

	// Check if plan is empty (no changes needed)
	if plan.IsEmpty() {
		if outputFormat == textOutputFormat {
			fmt.Fprintln(command.OutOrStderr(), "No changes needed. Resources match configuration.")
			return nil
		}
		// Use consistent output format with empty result
		emptyResult := &executor.ExecutionResult{
			SuccessCount:   0,
			FailureCount:   0,
			SkippedCount:   0,
			DryRun:         dryRun,
			ChangesApplied: []executor.AppliedChange{},
		}
		return outputExecutionResult(command, emptyResult, outputFormat)
	}

	// Show plan summary for text format (both regular and dry-run)
	if outputFormat == textOutputFormat {
		common.DisplayPlanSummary(plan, command.OutOrStderr())

		// Show confirmation prompt for non-dry-run, non-auto-approve
		if !dryRun && !autoApprove {
			// If we're using stdin for input, use /dev/tty for confirmation
			inputReader := command.InOrStdin()
			if usingStdinForInput {
				tty, err := os.Open("/dev/tty")
				if err != nil {
					// This shouldn't happen as we checked earlier
					return fmt.Errorf("cannot open terminal for confirmation: %w", err)
				}
				defer tty.Close()
				inputReader = tty
			}

			if !common.ConfirmExecution(plan, command.OutOrStdout(), command.OutOrStderr(), inputReader) {
				return fmt.Errorf("apply cancelled")
			}
		}

		// Add spacing before execution output
		fmt.Fprintln(command.OutOrStderr())
	}

	// Create executor
	stateClient := createStateClient(kkClient)

	var reporter executor.ProgressReporter
	if outputFormat == textOutputFormat {
		reporter = executor.NewConsoleReporterWithOptions(command.OutOrStderr(), dryRun)
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return err
	}
	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return err
	}

	exec := executor.NewWithOptions(stateClient, reporter, dryRun, executor.Options{
		KonnectToken:   token,
		KonnectBaseURL: baseURL,
		Mode:           planner.PlanModeApply,
		PlanBaseDir:    resolvePlanBaseDir(planFile),
	})

	// Execute plan
	result := exec.Execute(ctx, plan)

	// Output results based on format
	outputErr := outputExecutionResult(command, result, outputFormat)
	if outputErr != nil {
		return outputErr
	}

	if result.HasErrors() {
		return fmt.Errorf("execution completed with %d errors", result.FailureCount)
	}

	return nil
}

func validateDeletePlan(plan *planner.Plan) error {
	if plan.Metadata.Mode != planner.PlanModeDelete {
		return fmt.Errorf(
			"delete command requires a plan generated in delete mode, got %q mode. "+
				"Generate a delete plan with: kongctl plan --mode delete -f <files>",
			plan.Metadata.Mode,
		)
	}
	return nil
}

func validateApplyPlan(plan *planner.Plan, command *cobra.Command) error {
	// Check if plan contains DELETE operations
	for _, change := range plan.Changes {
		if change.Action == planner.ActionDelete {
			return fmt.Errorf("apply command cannot execute plans with DELETE operations. Use 'sync' command instead")
		}
	}

	// Warn if plan was generated in sync mode
	if plan.Metadata.Mode == planner.PlanModeSync {
		fmt.Fprintf(
			command.OutOrStderr(),
			"Warning: Plan was generated in sync mode but apply will skip DELETE operations\n",
		)
	}

	return nil
}

// Displays an output for the execution of an apply or sync command.
// The returned error indicates if the function itself succeeded or not, not if the execution result had errors
func outputExecutionResult(command *cobra.Command,
	result *executor.ExecutionResult, format string,
) error {
	// Human-readable output already handled by progress reporter
	if format == "text" {
		return nil
	}

	// Get the full plan from context
	var plan *planner.Plan
	ctx := command.Context()
	if ctx != nil {
		if p, ok := ctx.Value(currentPlanKey).(*planner.Plan); ok {
			plan = p
		}
	}

	// Build the execution section
	execution := map[string]any{
		"dry_run": result.DryRun,
	}

	// Add appropriate execution details based on mode
	if result.DryRun {
		if len(result.ValidationResults) > 0 {
			execution["validation_results"] = result.ValidationResults
		}
	} else {
		if len(result.ChangesApplied) > 0 {
			execution["applied_changes"] = result.ChangesApplied
		}
		if len(result.ExistingChanges) > 0 {
			execution["existing_changes"] = result.ExistingChanges
		}
	}

	// Always include errors if present
	if len(result.Errors) > 0 {
		execution["errors"] = result.Errors
	}

	// Build the summary section
	summary := map[string]any{
		"total_changes": result.TotalChanges(),
		"applied":       result.SuccessCount,
		"existing":      result.ExistingCount,
		"failed":        result.FailureCount,
		"skipped":       result.SkippedCount,
		"status":        "success",
	}

	if result.TotalChanges() == 0 {
		if plan != nil && plan.Metadata.Mode == planner.PlanModeCreate {
			summary["message"] = "No changes needed. No creatable resources found in the input configuration."
		} else if plan != nil && plan.Metadata.Mode == planner.PlanModeDelete {
			summary["message"] = "No changes needed. No matching resources found to delete."
		} else {
			summary["message"] = "No changes needed. All resources match the desired configuration."
		}
	} else if result.FailureCount > 0 {
		if result.SuccessCount < 1 {
			summary["status"] = "error"
			summary["message"] = fmt.Sprintf("Execution failed with %d errors", result.FailureCount)
		} else {
			summary["status"] = "partial_success"
			summary["message"] = fmt.Sprintf("Execution partially succeeded with %d errors", result.FailureCount)
		}
	} else if result.SuccessCount > 0 && result.ExistingCount > 0 {
		summary["message"] = fmt.Sprintf(
			"Execution succeeded with %d changes; %d resources already existed",
			result.SuccessCount,
			result.ExistingCount,
		)
	} else if result.SuccessCount > 0 {
		summary["message"] = fmt.Sprintf("Execution succeeded with %d changes", result.SuccessCount)
	} else if result.ExistingCount > 0 {
		summary["message"] = fmt.Sprintf("Execution succeeded. %d resources already existed.", result.ExistingCount)
	} else if result.SkippedCount > 0 && result.DryRun {
		summary["message"] = fmt.Sprintf("Dry-run complete. %d changes would be executed.", result.SkippedCount)
	}

	// Check if we need to save execution report to file
	executionReportFile, _ := command.Flags().GetString("execution-report-file")
	if executionReportFile != "" {
		// Build the complete execution report
		report := make(map[string]any)
		if plan != nil {
			report["plan"] = plan
		}
		report["execution"] = execution
		report["summary"] = summary

		// Marshal to JSON with indentation
		reportJSON, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal execution report: %w", err)
		}

		// Write to file
		if err := os.WriteFile(executionReportFile, reportJSON, 0o600); err != nil {
			return fmt.Errorf("failed to write execution report file: %w", err)
		}
	}

	switch format {
	case "json":
		// Use custom JSON encoding to preserve field order
		out := command.OutOrStdout()
		fmt.Fprintln(out, "{")

		// Output full plan first if present
		if plan != nil {
			planJSON, _ := json.MarshalIndent(plan, "  ", "  ")
			fmt.Fprintf(out, "  \"plan\": %s,\n", planJSON)
		}

		// Output execution second
		execJSON, _ := json.MarshalIndent(execution, "  ", "  ")
		fmt.Fprintf(out, "  \"execution\": %s,\n", execJSON)

		// Output summary last
		summaryJSON, _ := json.MarshalIndent(summary, "  ", "  ")
		fmt.Fprintf(out, "  \"summary\": %s\n", summaryJSON)

		fmt.Fprintln(out, "}")
		return nil

	case "yaml":
		// Build YAML content manually to preserve order
		out := command.OutOrStdout()

		// Output full plan first if present
		if plan != nil {
			fmt.Fprintln(out, "plan:")
			planYAML, _ := yaml.Marshal(plan)
			planLines := strings.SplitSeq(strings.TrimSpace(string(planYAML)), "\n")
			for line := range planLines {
				fmt.Fprintf(out, "  %s\n", line)
			}
		}

		// Output execution second
		fmt.Fprintln(out, "execution:")
		execYAML, _ := yaml.Marshal(execution)
		execLines := strings.Split(strings.TrimSpace(string(execYAML)), "\n")
		for _, line := range execLines {
			fmt.Fprintf(out, "  %s\n", line)
		}

		// Output summary last
		fmt.Fprintln(out, "summary:")
		summaryYAML, _ := yaml.Marshal(summary)
		summaryLines := strings.Split(strings.TrimSpace(string(summaryYAML)), "\n")
		for _, line := range summaryLines {
			fmt.Fprintf(out, "  %s\n", line)
		}

		return nil

	default: // text
		// Human-readable output already handled by progress reporter
		// If there were errors during execution, return a non-nil error to signal failure
		return nil
	}
}

func newDeclarativeCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "konnect",
		Short: "Best-effort creation of declarative resources",
		Long: `Execute a plan to create resources from declarative configuration without pre-checking current state.

The create command emits CREATE operations only. It does not update existing resources,
and it continues attempting dependent child resources even when parent CREATE requests fail.`,
		RunE: runCreate,
	}

	cmd.Flags().StringSliceP("filename", "f", []string{},
		"Filename or directory to files to use to create the resource (can specify multiple)")
	cmd.Flags().BoolP("recursive", "R", false,
		"Process the directory used in -f, --filename recursively")
	addBaseDirFlag(cmd)
	cmd.Flags().String("plan", "", "Path to existing create plan file")
	cmd.Flags().Bool("dry-run", false, "Preview creates without applying")
	cmd.Flags().Bool("auto-approve", false, "Skip confirmation prompt")
	cmd.Flags().StringP("output", "o", textOutputFormat, "Output format (text|json|yaml)")
	cmd.Flags().String("execution-report-file", "", "Save execution report as JSON to file")
	addRequireNamespaceFlags(cmd)

	return cmd
}

func newDeclarativeApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "konnect",
		Short: "Apply configuration changes (create/update only)",
		Long: `Execute a plan to create new resources and update existing ones. Never deletes resources.

The apply command provides a safe way to apply configuration changes by only
performing CREATE and UPDATE operations. Use the sync command if you need to
delete resources.`,
		RunE: runApply,
	}

	// Add declarative config flags
	cmd.Flags().StringSliceP("filename", "f", []string{},
		"Filename or directory to files to use to create the resource (can specify multiple)")
	cmd.Flags().BoolP("recursive", "R", false,
		"Process the directory used in -f, --filename recursively")
	addBaseDirFlag(cmd)
	cmd.Flags().String("plan", "", "Path to existing plan file")
	cmd.Flags().Bool("dry-run", false, "Preview changes without applying")
	cmd.Flags().Bool("auto-approve", false, "Skip confirmation prompt")
	cmd.Flags().StringP("output", "o", textOutputFormat, "Output format (text|json|yaml)")
	cmd.Flags().String("execution-report-file", "", "Save execution report as JSON to file")
	addRequireNamespaceFlags(cmd)

	return cmd
}

func validateCreatePlan(plan *planner.Plan) error {
	if plan.Metadata.Mode != planner.PlanModeCreate {
		return fmt.Errorf(
			"create command requires a plan generated in create mode, got %q mode. "+
				"Generate a create plan with: kongctl plan --mode create -f <files>",
			plan.Metadata.Mode,
		)
	}

	for _, change := range plan.Changes {
		if change.Action != planner.ActionCreate && change.Action != planner.ActionExternalTool {
			return fmt.Errorf(
				"create command cannot execute %s actions in create plans; found %s for %s %q",
				change.Action,
				change.Action,
				change.ResourceType,
				change.ResourceRef,
			)
		}
	}

	return nil
}

func newDeclarativeDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "konnect",
		Short: "Delete resources defined in declarative configuration",
		Long: `Delete Konnect resources defined in declarative configuration files.

The delete command generates a delete plan from the configuration files and
executes it, removing matching resources from Konnect. Resources not found
in Konnect are skipped with a warning. Child resources are removed via
cascade deletion.

This is equivalent to running:
  kongctl plan --mode delete -f <files> | kongctl sync --plan -`,
		RunE: runDelete,
	}

	// Add declarative config flags
	cmd.Flags().StringSliceP("filename", "f", []string{},
		"Filename or directory to files to use to create the resource (can specify multiple)")
	cmd.Flags().BoolP("recursive", "R", false,
		"Process the directory used in -f, --filename recursively")
	addBaseDirFlag(cmd)
	cmd.Flags().String("plan", "", "Path to existing delete plan file")
	cmd.Flags().Bool("dry-run", false, "Preview deletions without executing them")
	cmd.Flags().Bool("auto-approve", false, "Skip confirmation prompt")
	cmd.Flags().StringP("output", "o", textOutputFormat, "Output format (text|json|yaml)")
	cmd.Flags().String("execution-report-file", "", "Save execution report as JSON to file")
	addRequireNamespaceFlags(cmd)

	return cmd
}

func runDelete(command *cobra.Command, args []string) error {
	// Silence usage for all runtime errors (command syntax is already valid at this point)
	command.SilenceUsage = true

	ctx := command.Context()
	planFile, _ := command.Flags().GetString("plan")
	dryRun, _ := command.Flags().GetBool("dry-run")
	autoApprove, _ := command.Flags().GetBool("auto-approve")
	outputFormat, _ := command.Flags().GetString("output")
	filenames, _ := command.Flags().GetStringSlice("filename")

	// Early check for non-text output without auto-approve
	if !dryRun && !autoApprove && outputFormat != textOutputFormat {
		return fmt.Errorf("cannot use %s output format without --auto-approve or --dry-run flag "+
			"(interactive confirmation not available with structured output)", outputFormat)
	}

	var usingStdinForInput bool
	if !dryRun && !autoApprove {
		if err := checkStdinApprovalConflict(planFile, filenames); err != nil {
			return err
		}
		usingStdinForInput = planFile == "-" || (planFile == "" && slices.Contains(filenames, "-"))
	}

	// Build helper
	helper := cmd.BuildHelper(command, args)
	generator := planGenerator(helper)

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

	nsValidator, requirement, err := resolveNamespaceRequirement(command, cfg)
	if err != nil {
		return err
	}

	// Load or generate plan
	var plan *planner.Plan
	if requirement.Mode != validator.NamespaceRequirementNone && planFile != "" {
		return fmt.Errorf(
			"--%s cannot be used together with --plan; "+
				"generate the plan with namespace enforcement enabled instead",
			requireNamespaceFlagName,
		)
	}
	if planFile != "" {
		if outputFormat == textOutputFormat {
			if planFile == "-" {
				fmt.Fprintf(command.OutOrStderr(), "Using plan from: stdin\n")
			} else {
				fmt.Fprintf(command.OutOrStderr(), "Using plan from: %s\n", planFile)
			}
		}

		plan, err = common.LoadPlan(planFile, command.InOrStdin())
		if err != nil {
			return err
		}
	} else {
		// Generate plan from configuration files
		recursive, _ := command.Flags().GetBool("recursive")

		sources, err := loader.ParseSources(filenames)
		if err != nil {
			return fmt.Errorf("failed to parse sources: %w", err)
		}

		ldr, err := newDeclarativeLoader(command, cfg)
		if err != nil {
			return err
		}
		resourceSet, err := ldr.LoadFromSources(sources, recursive)
		if err != nil {
			if len(filenames) == 0 && strings.Contains(err.Error(), "no YAML files found") {
				return fmt.Errorf(
					"no configuration files found in current directory. " +
						"Use -f to specify files or directories")
			}
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		if err := nsValidator.ValidateNamespaceRequirement(resourceSet, requirement); err != nil {
			return err
		}

		totalResources := resourceSet.ResourceCount()
		if totalResources == 0 {
			if len(filenames) == 0 {
				return fmt.Errorf(
					"no configuration files found in current directory. " +
						"Use -f to specify files or directories")
			}
			return fmt.Errorf("no resources found in configuration files")
		}

		// Create planner
		stateClient := createStateClient(kkClient)
		p := planner.NewPlanner(stateClient, logger)
		deckOpts, err := deckPlanOptions(resourceSet, cfg, logger)
		if err != nil {
			return err
		}

		// Generate plan in delete mode
		opts := planner.Options{
			Mode:      planner.PlanModeDelete,
			Generator: generator,
			Deck:      deckOpts,
		}
		plan, err = p.GeneratePlan(ctx, resourceSet, opts)
		if err != nil {
			return fmt.Errorf("failed to generate plan: %w", err)
		}
	}

	// Store plan in context for output formatting
	ctx = context.WithValue(ctx, currentPlanKey, plan)
	if planFile != "" {
		ctx = context.WithValue(ctx, planFileKey, planFile)
	}
	command.SetContext(ctx)

	// Validate that the plan was generated in delete mode
	if err := validateDeletePlan(plan); err != nil {
		return err
	}

	// Check if plan is empty (no changes needed)
	if plan.IsEmpty() {
		if outputFormat == textOutputFormat {
			fmt.Fprintln(command.OutOrStderr(),
				"No changes needed. No matching resources found to delete.")
			return nil
		}
		emptyResult := &executor.ExecutionResult{
			SuccessCount:   0,
			FailureCount:   0,
			SkippedCount:   0,
			DryRun:         dryRun,
			ChangesApplied: []executor.AppliedChange{},
		}
		return outputExecutionResult(command, emptyResult, outputFormat)
	}

	// Show plan summary for text format
	if outputFormat == textOutputFormat {
		common.DisplayPlanSummary(plan, command.OutOrStderr())

		if !dryRun && !autoApprove {
			inputReader := command.InOrStdin()
			if usingStdinForInput {
				tty, err := os.Open("/dev/tty")
				if err != nil {
					return fmt.Errorf("cannot open terminal for confirmation: %w", err)
				}
				defer tty.Close()
				inputReader = tty
			}

			if !common.ConfirmExecution(plan, command.OutOrStdout(), command.OutOrStderr(), inputReader) {
				return fmt.Errorf("delete cancelled")
			}
		}

		fmt.Fprintln(command.OutOrStderr())
	}

	// Create executor
	stateClient := createStateClient(kkClient)

	var reporter executor.ProgressReporter
	if outputFormat == textOutputFormat {
		reporter = executor.NewConsoleReporterWithOptions(command.OutOrStderr(), dryRun)
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return err
	}
	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return err
	}

	exec := executor.NewWithOptions(stateClient, reporter, dryRun, executor.Options{
		KonnectToken:   token,
		KonnectBaseURL: baseURL,
		Mode:           planner.PlanModeDelete,
		PlanBaseDir:    resolvePlanBaseDir(planFile),
	})

	// Execute plan
	result := exec.Execute(ctx, plan)

	// Output results based on format
	outputErr := outputExecutionResult(command, result, outputFormat)
	if outputErr != nil {
		return outputErr
	}
	if result.HasErrors() {
		return fmt.Errorf("execution completed with %d errors", result.FailureCount)
	}

	return nil
}

func runSync(command *cobra.Command, args []string) error {
	// Silence usage for all runtime errors (command syntax is already valid at this point)
	command.SilenceUsage = true

	ctx := command.Context()
	ctx = withDeclarativeHTTPLogContext(ctx, command, verbs.Sync, planner.PlanModeSync)
	command.SetContext(ctx)

	planFile, _ := command.Flags().GetString("plan")
	dryRun, _ := command.Flags().GetBool("dry-run")
	autoApprove, _ := command.Flags().GetBool("auto-approve")
	outputFormat, _ := command.Flags().GetString("output")
	filenames, _ := command.Flags().GetStringSlice("filename")

	// Early check for non-text output without auto-approve
	if !dryRun && !autoApprove && outputFormat != textOutputFormat {
		return fmt.Errorf("cannot use %s output format without --auto-approve or --dry-run flag "+
			"(interactive confirmation not available with structured output)", outputFormat)
	}

	var usingStdinForInput bool
	if !dryRun && !autoApprove {
		if err := checkStdinApprovalConflict(planFile, filenames); err != nil {
			return err
		}
		usingStdinForInput = planFile == "-" || (planFile == "" && slices.Contains(filenames, "-"))
	}

	// Build helper
	helper := cmd.BuildHelper(command, args)
	generator := planGenerator(helper)

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

	nsValidator, requirement, err := resolveNamespaceRequirement(command, cfg)
	if err != nil {
		return err
	}

	// Load or generate plan
	var plan *planner.Plan
	if requirement.Mode != validator.NamespaceRequirementNone && planFile != "" {
		return fmt.Errorf(
			"--%s cannot be used together with --plan; generate the plan with namespace enforcement enabled instead",
			requireNamespaceFlagName,
		)
	}
	if planFile != "" {
		// Show plan source information early
		if outputFormat == textOutputFormat {
			if planFile == "-" {
				fmt.Fprintf(command.OutOrStderr(), "Using plan from: stdin\n")
			} else {
				fmt.Fprintf(command.OutOrStderr(), "Using plan from: %s\n", planFile)
			}
		}

		// Load existing plan
		plan, err = common.LoadPlan(planFile, command.InOrStdin())
		if err != nil {
			return err
		}
	} else {

		// Generate plan from configuration files
		recursive, _ := command.Flags().GetBool("recursive")

		// Parse sources from filenames
		sources, err := loader.ParseSources(filenames)
		if err != nil {
			return fmt.Errorf("failed to parse sources: %w", err)
		}

		// Load configuration
		ldr, err := newDeclarativeLoader(command, cfg)
		if err != nil {
			return err
		}
		resourceSet, err := ldr.LoadFromSources(sources, recursive)
		if err != nil {
			// Provide more helpful error message for common cases
			if len(filenames) == 0 && strings.Contains(err.Error(), "no YAML files found") {
				return fmt.Errorf("no configuration files found in current directory. Use -f to specify files or directories")
			}
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		if err := nsValidator.ValidateNamespaceRequirement(resourceSet, requirement); err != nil {
			return err
		}

		// Check if configuration is empty
		totalResources := resourceSet.ResourceCount()

		// In sync mode, allow empty configuration to detect resources to delete
		if totalResources == 0 {
			// Check if we're using default directory (no explicit sources)
			if len(filenames) == 0 {
				return fmt.Errorf("no configuration files found in current directory. Use -f to specify files or directories")
			}

			// In sync mode, empty config is valid - it means delete all managed resources
			if outputFormat == textOutputFormat {
				namespaces := resourceSet.DefaultNamespaces
				if len(namespaces) == 0 && resourceSet.DefaultNamespace != "" {
					namespaces = []string{resourceSet.DefaultNamespace}
				}
				if len(namespaces) > 0 {
					fmt.Fprintf(command.OutOrStderr(),
						"No resources defined in configuration. Using namespace(s) '%s' from _defaults.\n"+
							"Checking for managed resources to remove...\n",
						strings.Join(namespaces, ", "))
				} else {
					fmt.Fprintln(command.OutOrStderr(),
						"No resources defined in configuration. Checking 'default' namespace for managed resources to remove...")
				}
			}
		}

		// Create planner
		stateClient := createStateClient(kkClient)
		p := planner.NewPlanner(stateClient, logger)
		deckOpts, err := deckPlanOptions(resourceSet, cfg, logger)
		if err != nil {
			return err
		}

		// Generate plan in sync mode
		opts := planner.Options{
			Mode:      planner.PlanModeSync,
			Generator: generator,
			Deck:      deckOpts,
		}
		plan, err = p.GeneratePlan(ctx, resourceSet, opts)
		if err != nil {
			return fmt.Errorf("failed to generate plan: %w", err)
		}
	}

	// Store plan in context for output formatting
	ctx = context.WithValue(ctx, currentPlanKey, plan)
	// Store plan file path if provided
	if planFile != "" {
		ctx = context.WithValue(ctx, planFileKey, planFile)
	}
	command.SetContext(ctx)

	// Check if plan is empty (no changes needed)
	if plan.IsEmpty() {
		if outputFormat == textOutputFormat {
			fmt.Fprintln(command.OutOrStderr(), "No changes needed. Resources match configuration.")
			return nil
		}
		// Use consistent output format with empty result
		emptyResult := &executor.ExecutionResult{
			SuccessCount:   0,
			FailureCount:   0,
			SkippedCount:   0,
			DryRun:         dryRun,
			ChangesApplied: []executor.AppliedChange{},
		}
		return outputExecutionResult(command, emptyResult, outputFormat)
	}

	// Show plan summary for text format (both regular and dry-run)
	if outputFormat == textOutputFormat {
		common.DisplayPlanSummary(plan, command.OutOrStderr())

		// Show confirmation prompt for non-dry-run, non-auto-approve
		if !dryRun && !autoApprove {
			// If we're using stdin for input, use /dev/tty for confirmation
			inputReader := command.InOrStdin()
			if usingStdinForInput {
				tty, err := os.Open("/dev/tty")
				if err != nil {
					// This shouldn't happen as we checked earlier
					return fmt.Errorf("cannot open terminal for confirmation: %w", err)
				}
				defer tty.Close()
				inputReader = tty
			}

			if !common.ConfirmExecution(plan, command.OutOrStdout(), command.OutOrStderr(), inputReader) {
				return fmt.Errorf("sync cancelled")
			}
		}

		// Add spacing before execution output
		fmt.Fprintln(command.OutOrStderr())
	}

	// Create executor
	stateClient := createStateClient(kkClient)

	var reporter executor.ProgressReporter
	if outputFormat == textOutputFormat {
		reporter = executor.NewConsoleReporterWithOptions(command.OutOrStderr(), dryRun)
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return err
	}
	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return err
	}

	exec := executor.NewWithOptions(stateClient, reporter, dryRun, executor.Options{
		KonnectToken:   token,
		KonnectBaseURL: baseURL,
		Mode:           planner.PlanModeSync,
		PlanBaseDir:    resolvePlanBaseDir(planFile),
	})

	// Execute plan
	result := exec.Execute(ctx, plan)

	// Output results based on format
	outputErr := outputExecutionResult(command, result, outputFormat)
	if outputErr != nil {
		return outputErr
	}
	if result.HasErrors() {
		return fmt.Errorf("execution completed with %d errors", result.FailureCount)
	}

	return nil
}

// createStateClient creates a new state client with all necessary APIs
func createStateClient(kkClient helpers.SDKAPI) *state.Client {
	return state.NewClient(state.ClientConfig{
		// Core APIs
		PortalAPI:             kkClient.GetPortalAPI(),
		APIAPI:                kkClient.GetAPIAPI(),
		AppAuthAPI:            kkClient.GetAppAuthStrategiesAPI(),
		ControlPlaneAPI:       kkClient.GetControlPlaneAPI(),
		ControlPlaneGroupsAPI: kkClient.GetControlPlaneGroupsAPI(),
		GatewayServiceAPI:     kkClient.GetGatewayServiceAPI(),
		CatalogServiceAPI:     kkClient.GetCatalogServicesAPI(),

		// Portal child resource APIs
		PortalPageAPI:          kkClient.GetPortalPageAPI(),
		PortalAuthSettingsAPI:  kkClient.GetPortalAuthSettingsAPI(),
		PortalCustomizationAPI: kkClient.GetPortalCustomizationAPI(),
		PortalCustomDomainAPI:  kkClient.GetPortalCustomDomainAPI(),
		PortalSnippetAPI:       kkClient.GetPortalSnippetAPI(),
		PortalTeamAPI:          kkClient.GetPortalTeamAPI(),
		PortalTeamRolesAPI:     kkClient.GetPortalTeamRolesAPI(),
		PortalEmailsAPI:        kkClient.GetPortalEmailsAPI(),
		AssetsAPI:              kkClient.GetAssetsAPI(),

		// API child resource APIs
		APIVersionAPI:        kkClient.GetAPIVersionAPI(),
		APIPublicationAPI:    kkClient.GetAPIPublicationAPI(),
		APIImplementationAPI: kkClient.GetAPIImplementationAPI(),
		APIDocumentAPI:       kkClient.GetAPIDocumentAPI(),

		// Event Gateway APIs
		EGWControlPlaneAPI:                  kkClient.GetEventGatewayControlPlaneAPI(),
		EventGatewayBackendClusterAPI:       kkClient.GetEventGatewayBackendClusterAPI(),
		EventGatewayVirtualClusterAPI:       kkClient.GetEventGatewayVirtualClusterAPI(),
		EventGatewayListenerAPI:             kkClient.GetEventGatewayListenerAPI(),
		EventGatewayListenerPolicyAPI:       kkClient.GetEventGatewayListenerPolicyAPI(),
		EventGatewayDataPlaneCertificateAPI: kkClient.GetEventGatewayDataPlaneCertificateAPI(),

		// Organization APIs
		OrganizationTeamAPI: kkClient.GetOrganizationTeamAPI(),
	})
}

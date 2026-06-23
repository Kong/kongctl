package root

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/build"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs/adopt"
	"github.com/kong/kongctl/internal/cmd/root/verbs/api"
	"github.com/kong/kongctl/internal/cmd/root/verbs/apply"
	"github.com/kong/kongctl/internal/cmd/root/verbs/create"
	"github.com/kong/kongctl/internal/cmd/root/verbs/del"
	"github.com/kong/kongctl/internal/cmd/root/verbs/diff"
	"github.com/kong/kongctl/internal/cmd/root/verbs/dump"
	"github.com/kong/kongctl/internal/cmd/root/verbs/explain"
	extensioncmd "github.com/kong/kongctl/internal/cmd/root/verbs/extensions"
	"github.com/kong/kongctl/internal/cmd/root/verbs/get"
	"github.com/kong/kongctl/internal/cmd/root/verbs/help"
	"github.com/kong/kongctl/internal/cmd/root/verbs/install"
	"github.com/kong/kongctl/internal/cmd/root/verbs/lint"
	"github.com/kong/kongctl/internal/cmd/root/verbs/list"
	"github.com/kong/kongctl/internal/cmd/root/verbs/listen"
	"github.com/kong/kongctl/internal/cmd/root/verbs/login"
	"github.com/kong/kongctl/internal/cmd/root/verbs/logout"
	"github.com/kong/kongctl/internal/cmd/root/verbs/patch"
	"github.com/kong/kongctl/internal/cmd/root/verbs/plan"
	"github.com/kong/kongctl/internal/cmd/root/verbs/ps"
	"github.com/kong/kongctl/internal/cmd/root/verbs/scaffold"
	"github.com/kong/kongctl/internal/cmd/root/verbs/sync"
	"github.com/kong/kongctl/internal/cmd/root/verbs/view"
	"github.com/kong/kongctl/internal/cmd/root/version"
	"github.com/kong/kongctl/internal/config"
	extensioncore "github.com/kong/kongctl/internal/extensions"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/profile"
	"github.com/kong/kongctl/internal/telemetry"
	"github.com/kong/kongctl/internal/theme"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	rootLong = normalizers.LongDesc(i18n.T("root.rootLong", `
  kongctl is the official command line tool for the Kong Konnect API platform.

Find more information at:
   https://developer.konghq.com/kongctl/`))

	rootShort = i18n.T("root/rootShort", fmt.Sprintf("%s controls Kong", meta.CLIName))

	rootCmd *cobra.Command

	// Stores the default configuration file path, loaded on init
	defaultConfigFilePath = ""
	// Stores the global runtime value for the configured configuration file path,
	configFilePath = ""
	// Stores the global runtime value for the configured profile
	currProfile = profile.DefaultProfile

	currConfig config.Hook
	streams    *iostreams.IOStreams
	pMgr       profile.Manager

	outputFormat = cmdpkg.NewDeferredEnum([]string{
		common.JSON.String(),
		common.YAML.String(),
		common.TEXT.String(),
	},
		common.TEXT.String())

	logLevel = cmdpkg.NewEnum([]string{
		common.TRACE.String(),
		common.DEBUG.String(),
		common.INFO.String(),
		common.WARN.String(),
		common.ERROR.String(),
	},
		common.ERROR.String())

	buildInfo *build.Info

	logger      *slog.Logger
	logFilePath string
	logFile     *os.File

	telemetryRecorder *telemetry.Recorder

	// noTelemetry is set by the persistent --no-telemetry flag. It is the
	// highest-priority disable signal in telemetry.resolveEnabled; see the
	// precedence note there.
	noTelemetry bool

	// konnectEnv switches Konnect target defaults for this invocation. It is
	// intentionally hidden for Kong-internal use.
	konnectEnv string
)

// NoTelemetryFlagName is the persistent root flag that disables telemetry
// for a single command invocation.
const NoTelemetryFlagName = "no-telemetry"

const mergedFlagsUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}
{{if or .HasAvailableLocalFlags .HasAvailableInheritedFlags}}

Flags:
{{mergedFlagUsages . | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

const logFilePIDToken = "%PID%"

func mergedFlagUsages(cmd *cobra.Command) string {
	flags := pflag.NewFlagSet(cmd.DisplayName(), pflag.ContinueOnError)
	flags.SortFlags = true
	hideOutput := common.IsOutputFormatValidationSkipped(cmd)
	addFlagSetCopies(flags, cmd.LocalFlags(), hideOutput)
	addFlagSetCopies(flags, cmd.InheritedFlags(), hideOutput)

	if f := flags.Lookup(common.OutputFlagName); f != nil {
		f.Usage = outputFlagUsage(common.AllowedOutputFormats(cmd))
	}

	return strings.TrimRight(flags.FlagUsages(), "\n")
}

func addFlagSetCopies(dst, src *pflag.FlagSet, hideOutput bool) {
	if dst == nil || src == nil {
		return
	}
	src.VisitAll(func(flag *pflag.Flag) {
		if hideOutput && flag.Name == common.OutputFlagName {
			return
		}
		flagCopy := *flag
		if flag.Annotations != nil {
			flagCopy.Annotations = make(map[string][]string, len(flag.Annotations))
			for k, v := range flag.Annotations {
				flagCopy.Annotations[k] = slices.Clone(v)
			}
		}
		dst.AddFlag(&flagCopy)
	})
}

func outputFlagUsage(allowed []string) string {
	return fmt.Sprintf(`Configures the format of data written to STDOUT.
- Config path: [ %s ]
- Allowed    : [ %s ]`,
		common.OutputConfigPath, strings.Join(allowed, "|"))
}

func validateOutputFormat(cmd *cobra.Command) error {
	value := strings.TrimSpace(outputFormat.Value)
	if currConfig != nil {
		configured := strings.TrimSpace(currConfig.GetString(common.OutputConfigPath))
		if configured != "" {
			value = configured
		}
	}
	if value == "" {
		value = common.DefaultOutputFormat
	}
	return common.ValidateOutputFormat(cmd, value)
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           meta.CLIName,
		Short:         rootShort,
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return cmdpkg.UnknownSubcommandError(cmd, args[0])
			}
			renderRootOverview(cmd.OutOrStdout(), cmd)
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := applyKonnectEnvironmentDefaults(cmd.Root(), currConfig); err != nil {
				return &cmdpkg.ConfigurationError{Err: err}
			}
			if err := validateOutputFormat(cmd); err != nil {
				return &cmdpkg.ConfigurationError{Err: err}
			}
			ctx := context.WithValue(cmd.Context(), config.ConfigKey, currConfig)
			ctx = context.WithValue(ctx, iostreams.StreamsKey, streams)
			ctx = context.WithValue(ctx, profile.ProfileManagerKey, pMgr)
			ctx = context.WithValue(ctx, build.InfoKey, buildInfo)
			ctx = context.WithValue(ctx, log.LoggerKey, logger)
			ctx = theme.ContextWithPalette(ctx, theme.Current())

			if telemetryRecorder == nil {
				telemetryRecorder = telemetry.NewRecorder(
					ctx, currConfig, buildInfo, streams, logger, noTelemetry,
				)
			}
			telemetryRecorder.SetCommand(telemetry.CommandInfo{
				Path: cmd.CommandPath(),
			})
			ctx = telemetry.ContextWithRecorder(ctx, telemetryRecorder)

			cmd.SetContext(ctx)
			return nil
		},
	}

	// Disable Cobra's automatic help command since we have our own custom help command
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})
	cobra.AddTemplateFunc("mergedFlagUsages", mergedFlagUsages)
	rootCmd.SetUsageTemplate(mergedFlagsUsageTemplate)
	rootCmd.SetFlagErrorFunc(rootFlagError)

	// parses all flags not just the target command
	rootCmd.TraverseChildren = true

	rootCmd.PersistentFlags().StringVar(&configFilePath, common.ConfigFilePathFlagName,
		defaultConfigFilePath,
		i18n.T("root."+common.ConfigFilePathFlagName, "Path to the configuration file to load."))

	rootCmd.PersistentFlags().StringVarP(&currProfile, common.ProfileFlagName, common.ProfileFlagShort,
		profile.DefaultProfile,
		"Specify the profile to use for this command.")

	// -------------------------------------------------------------------------
	// These require some extra gymnastics to ensure that the output flag is
	// from a valid set of values. There may be a way to do this more elegantly
	// in the pFlag library
	rootCmd.PersistentFlags().VarP(outputFormat, common.OutputFlagName, common.OutputFlagShort,
		outputFlagUsage(outputFormat.Allowed))

	rootCmd.PersistentFlags().Var(logLevel, common.LogLevelFlagName,
		fmt.Sprintf(`Configures the logging level. Execution logs are written to STDERR.
- Config path: [ %s ]
- Allowed    : [ %s ]`,
			common.LogLevelConfigPath, strings.Join(logLevel.Allowed, "|")))

	rootCmd.PersistentFlags().StringVar(&logFilePath, common.LogFileFlagName, "",
		fmt.Sprintf(`Write execution logs to the specified file instead of STDERR.
- Config path: [ %s ]`,
			common.LogFileConfigPath))

	rootCmd.PersistentFlags().BoolVar(&noTelemetry, NoTelemetryFlagName, false,
		fmt.Sprintf(`Disable telemetry for this command invocation. Overrides config and env.
- Config path: [ %s ]
- Env var    : [ %s ]
- Default    : [ false ]`,
			telemetry.ConfigKeyEnabled, telemetry.EnvNoTelemetry))

	rootCmd.PersistentFlags().StringVar(&konnectEnv, konnectcommon.KonnectEnvFlagName,
		"",
		fmt.Sprintf("Konnect environment for this command invocation. Allowed values: %s, %s.",
			konnectcommon.EnvironmentCom, konnectcommon.EnvironmentTech))
	util.CheckError(rootCmd.PersistentFlags().MarkHidden(konnectcommon.KonnectEnvFlagName))

	themeFlag := theme.NewFlag(common.DefaultColorTheme)
	rootCmd.PersistentFlags().Var(themeFlag, common.ColorThemeFlagName,
		fmt.Sprintf(`Configures the CLI UI/theme (prompt, tables, TUI elements).
- Config path: [ %s ]
- Examples   : [ %s ]
- Reference  : [ https://github.com/lrstanley/bubbletint/blob/master/DEFAULT_TINTS.md ]`,
			common.ColorThemeConfigPath, strings.Join(sampleThemeNames(), ", ")))

	// -------------------------------------------------------------------------

	return rootCmd
}

// addCommands adds the root subcommands to the command.
func addCommands() error {
	rootCmd.AddCommand(version.NewVersionCmd())

	command, err := api.NewAPICmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = get.NewGetCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = create.NewCreateCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = explain.NewExplainCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = scaffold.NewScaffoldCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = listen.NewListenCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = listen.NewTailCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = extensioncmd.NewLinkCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = extensioncmd.NewUninstallCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = extensioncmd.NewUpgradeCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = view.NewViewCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = list.NewListCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = del.NewDeleteCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = ps.NewPSCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = login.NewLoginCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = lint.NewLintCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = install.NewInstallCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = logout.NewLogoutCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = dump.NewDumpCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = patch.NewPatchCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = plan.NewPlanCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = sync.NewSyncCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = diff.NewDiffCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	//command, err = export.NewExportCmd()
	//if err != nil {
	//	return err
	//}
	//rootCmd.AddCommand(command)

	command, err = apply.NewApplyCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = adopt.NewAdoptCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	// Add help command
	rootCmd.AddCommand(help.NewHelpCmd())
	installUsageErrorFallbacks(rootCmd)

	return nil
}

func installUsageErrorFallbacks(command *cobra.Command) {
	if command == nil {
		return
	}
	if !command.Hidden && command.HasAvailableSubCommands() && !command.Runnable() {
		cmdpkg.ConfigureRequiresSubcommand(command)
	}
	for _, child := range command.Commands() {
		installUsageErrorFallbacks(child)
	}
}

func renderRootOverview(w io.Writer, command *cobra.Command) {
	if w == nil || command == nil {
		return
	}

	if long := strings.TrimSpace(command.Long); long != "" {
		fmt.Fprintln(w, long)
		fmt.Fprintln(w)
	}

	if command.HasAvailableSubCommands() {
		fmt.Fprintln(w, "Available Commands:")
		for _, child := range command.Commands() {
			if child.IsAvailableCommand() || child.Name() == "help" {
				fmt.Fprintf(w, "  %-*s %s\n", child.NamePadding(), child.Name(), child.Short)
			}
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, `Use "%s [command] --help" for more information about a command.`, command.CommandPath())
		fmt.Fprintln(w)
	}
}

func rootFlagError(command *cobra.Command, err error) error {
	if command == nil || command.Root() != command {
		return err
	}

	var flagErr *pflag.NotExistError
	if !errors.As(err, &flagErr) {
		return err
	}

	for _, arg := range command.Flags().Args() {
		arg = strings.TrimSpace(arg)
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		if rootCommandHasSubcommand(command, arg) {
			return err
		}
		return cmdpkg.UnknownSubcommandError(command, arg)
	}
	return err
}

func rootCommandHasSubcommand(command *cobra.Command, name string) bool {
	if command == nil || name == "" {
		return false
	}
	for _, child := range command.Commands() {
		if child.IsAvailableCommand() && (child.Name() == name || child.HasAlias(name)) {
			return true
		}
	}
	return false
}

func sampleThemeNames() []string {
	const maxSamples = 5

	names := theme.Available()
	if len(names) == 0 {
		return []string{common.DefaultColorTheme}
	}

	samples := make([]string, 0, maxSamples)
	defaultIncluded := false
	for _, name := range names {
		if name == common.DefaultColorTheme {
			defaultIncluded = true
		}
		if len(samples) < maxSamples {
			samples = append(samples, name)
		}
	}

	if !defaultIncluded {
		samples = append([]string{common.DefaultColorTheme}, samples...)
	}
	if len(samples) > maxSamples {
		samples = samples[:maxSamples]
	}
	return samples
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd = newRootCmd()
	err := addCommands()
	util.CheckError(err)

	// Because the profile is not part of the configuration, we can't use viper
	// to read it following it's built in priorities.  So here we look for a well known
	// profile variable and set our package level variable if it's set before
	// continuing to process the command run.  This creates a ENV_VAR < CLI_FLAG priority
	profileEnvVar, found := os.LookupEnv(fmt.Sprintf("%s_PROFILE", strings.ToUpper(meta.CLIName)))
	if found {
		currProfile = profileEnvVar
	}
}

func bindFlags(config config.Hook) {
	f := rootCmd.Flags().Lookup(common.OutputFlagName)
	util.CheckError(config.BindFlag(common.OutputConfigPath, f))

	f = rootCmd.Flags().Lookup(common.LogLevelFlagName)
	util.CheckError(config.BindFlag(common.LogLevelConfigPath, f))

	f = rootCmd.Flags().Lookup(common.LogFileFlagName)
	util.CheckError(config.BindFlag(common.LogFileConfigPath, f))

	f = rootCmd.Flags().Lookup(common.ColorThemeFlagName)
	util.CheckError(config.BindFlag(common.ColorThemeConfigPath, f))
}

func applyExtensionRuntimeDefaultsBeforeConfig(runtimeCtx *extensioncore.RuntimeContext) {
	if runtimeCtx == nil {
		return
	}
	if value := strings.TrimSpace(runtimeCtx.Resolved.ConfigFile); value != "" &&
		!commandTreeFlagChanged(rootCmd, common.ConfigFilePathFlagName) {
		configFilePath = value
	}
	if value := strings.TrimSpace(runtimeCtx.Resolved.Profile); value != "" &&
		!commandTreeFlagChanged(rootCmd, common.ProfileFlagName) {
		currProfile = value
	}
}

func applyExtensionRuntimeDefaults(runtimeCtx *extensioncore.RuntimeContext, cfg config.Hook) {
	if runtimeCtx == nil || cfg == nil {
		return
	}
	if value := strings.TrimSpace(runtimeCtx.Resolved.Output); value != "" &&
		!commandTreeFlagChanged(rootCmd, common.OutputFlagName) {
		util.CheckError(outputFormat.Set(value))
		cfg.SetString(common.OutputConfigPath, value)
	}
	if value := strings.TrimSpace(runtimeCtx.Resolved.LogLevel); value != "" &&
		!commandTreeFlagChanged(rootCmd, common.LogLevelFlagName) {
		util.CheckError(logLevel.Set(value))
		cfg.SetString(common.LogLevelConfigPath, value)
	}
	if value := strings.TrimSpace(runtimeCtx.Resolved.ColorTheme); value != "" &&
		!commandTreeFlagChanged(rootCmd, common.ColorThemeFlagName) {
		cfg.SetString(common.ColorThemeConfigPath, value)
	}
	if value := strings.TrimSpace(runtimeCtx.Resolved.BaseURL); value != "" &&
		!commandTreeFlagChanged(rootCmd, konnectcommon.BaseURLFlagName) &&
		!commandTreeFlagChanged(rootCmd, konnectcommon.RegionFlagName) {
		cfg.SetString(konnectcommon.BaseURLConfigPath, value)
	}
	if value := strings.TrimSpace(os.Getenv(extensioncore.KonnectPATEnvName)); value != "" &&
		!commandTreeFlagChanged(rootCmd, konnectcommon.PATFlagName) {
		cfg.SetString(konnectcommon.PATConfigPath, value)
	}
}

func applyKonnectEnvironmentDefaults(command *cobra.Command, cfg config.Hook) error {
	if command == nil || cfg == nil {
		return nil
	}

	defaults, selected, err := selectedKonnectEnvironmentDefaults(command)
	if err != nil || !selected {
		return err
	}

	if !commandTreeFlagChanged(command, konnectcommon.BaseURLFlagName) {
		baseURL := defaults.BaseURL
		if region, ok := commandTreeChangedFlagString(command, konnectcommon.RegionFlagName); ok {
			resolved, err := konnectcommon.BuildBaseURLFromRegionForEnvironment(region, defaults.Name)
			if err != nil {
				return err
			}
			baseURL = resolved
		} else if region := strings.TrimSpace(cfg.GetString(konnectcommon.RegionConfigPath)); region != "" {
			resolved, err := konnectcommon.BuildBaseURLFromRegionForEnvironment(region, defaults.Name)
			if err != nil {
				return err
			}
			baseURL = resolved
		}
		cfg.SetString(konnectcommon.BaseURLConfigPath, baseURL)
		if err := setCommandTreeFlagValue(command, konnectcommon.BaseURLFlagName, baseURL); err != nil {
			return err
		}
	}
	if !commandTreeFlagChanged(command, konnectcommon.AuthBaseURLFlagName) {
		cfg.SetString(konnectcommon.AuthBaseURLConfigPath, defaults.AuthBaseURL)
		if err := setCommandTreeFlagValue(command, konnectcommon.AuthBaseURLFlagName, defaults.AuthBaseURL); err != nil {
			return err
		}
	}
	if !commandTreeFlagChanged(command, konnectcommon.MachineClientIDFlagName) {
		cfg.SetString(konnectcommon.MachineClientIDConfigPath, defaults.MachineClientID)
		if err := setCommandTreeFlagValue(command, konnectcommon.MachineClientIDFlagName, defaults.MachineClientID); err != nil {
			return err
		}
	}
	return nil
}

func selectedKonnectEnvironmentDefaults(command *cobra.Command) (konnectcommon.EnvironmentDefaults, bool, error) {
	if value, ok := commandTreeChangedFlagString(command, konnectcommon.KonnectEnvFlagName); ok {
		defaults, err := konnectEnvironmentDefaultsForSelector(value)
		return defaults, true, err
	}

	if value := strings.TrimSpace(konnectEnv); value != "" {
		defaults, err := konnectEnvironmentDefaultsForSelector(value)
		return defaults, true, err
	}

	value, ok := os.LookupEnv(konnectcommon.KonnectEnvEnvName)
	if !ok || strings.TrimSpace(value) == "" {
		return konnectcommon.EnvironmentDefaults{}, false, nil
	}

	defaults, err := konnectEnvironmentDefaultsForSelector(value)
	return defaults, true, err
}

func konnectEnvironmentDefaultsForSelector(value string) (konnectcommon.EnvironmentDefaults, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case konnectcommon.EnvironmentCom, konnectcommon.EnvironmentTech:
		return konnectcommon.EnvironmentDefaultsFor(normalized)
	default:
		return konnectcommon.EnvironmentDefaults{},
			fmt.Errorf("unsupported konnect environment %q (allowed: %s, %s)",
				value, konnectcommon.EnvironmentCom, konnectcommon.EnvironmentTech)
	}
}

func setCommandTreeFlagValue(command *cobra.Command, name, value string) error {
	if command == nil {
		return nil
	}
	for _, flags := range []*pflag.FlagSet{
		command.Flags(),
		command.PersistentFlags(),
		command.LocalNonPersistentFlags(),
		command.InheritedFlags(),
	} {
		if flags == nil {
			continue
		}
		if flag := flags.Lookup(name); flag != nil && !flag.Changed {
			if err := flag.Value.Set(value); err != nil {
				return fmt.Errorf("set --%s default: %w", name, err)
			}
			flag.DefValue = value
		}
	}
	for _, child := range command.Commands() {
		if err := setCommandTreeFlagValue(child, name, value); err != nil {
			return err
		}
	}
	return nil
}

func commandTreeFlagChanged(command *cobra.Command, name string) bool {
	return commandTreeChangedFlag(command, name) != nil
}

func commandTreeChangedFlagString(command *cobra.Command, name string) (string, bool) {
	flag := commandTreeChangedFlag(command, name)
	if flag == nil {
		return "", false
	}
	return flag.Value.String(), true
}

func commandTreeChangedFlag(command *cobra.Command, name string) *pflag.Flag {
	if command == nil {
		return nil
	}
	for _, flags := range []*pflag.FlagSet{
		command.Flags(),
		command.PersistentFlags(),
		command.LocalNonPersistentFlags(),
		command.InheritedFlags(),
	} {
		if flags == nil {
			continue
		}
		if flag := flags.Lookup(name); flag != nil && flag.Changed {
			return flag
		}
	}
	for _, child := range command.Commands() {
		if flag := commandTreeChangedFlag(child, name); flag != nil {
			return flag
		}
	}
	return nil
}

func initConfig() {
	runtimeCtx, runtimeCtxErr := extensioncore.LoadRuntimeContextFromEnv()
	util.CheckError(runtimeCtxErr)
	applyExtensionRuntimeDefaultsBeforeConfig(runtimeCtx)

	if configFilePath == "" {
		configFilePath = defaultConfigFilePath
	}
	cfg, e1 := config.GetConfig(configFilePath, currProfile, defaultConfigFilePath)
	util.CheckError(e1)
	currConfig = cfg

	pMgr = profile.NewManager(cfg.Viper)

	bindFlags(currConfig)
	applyExtensionRuntimeDefaults(runtimeCtx, currConfig)

	themeName := strings.TrimSpace(currConfig.GetString(common.ColorThemeConfigPath))
	if themeName == "" {
		themeName = common.DefaultColorTheme
	}
	if err := theme.SetCurrent(themeName); err != nil {
		if streams != nil && streams.ErrOut != nil {
			fmt.Fprintf(streams.ErrOut,
				"warning: %v; falling back to %q theme\n",
				err, common.DefaultColorTheme)
		}
		_ = theme.SetCurrent(common.DefaultColorTheme)
		currConfig.SetString(common.ColorThemeConfigPath, common.DefaultColorTheme)
		themeName = common.DefaultColorTheme
	}
	// Show the hint when automatic theme selection is active. Users who have
	// chosen a fixed theme have already discovered the feature.
	theme.SetConfiguredExplicitly(themeName != common.DefaultColorTheme)

	loggerOpts := &slog.HandlerOptions{
		Level: log.ConfigLevelStringToSlogLevel(cfg.GetString(common.LogLevelConfigPath)),
	}

	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}

	logPath := strings.TrimSpace(cfg.GetString(common.LogFileConfigPath))
	if logPath == "" {
		configPath := cfg.GetPath()
		configDir := filepath.Dir(configPath)
		defaultLogPath := filepath.Join(configDir, "logs", meta.CLIName+".log")
		logPath = defaultLogPath
		cfg.SetString(common.LogFileConfigPath, logPath)
	}
	if strings.Contains(logPath, logFilePIDToken) {
		logPath = strings.ReplaceAll(logPath, logFilePIDToken, fmt.Sprintf("%d", os.Getpid()))
		cfg.SetString(common.LogFileConfigPath, logPath)
	}

	var handler slog.Handler
	if logPath != "" {
		if dir := filepath.Dir(logPath); dir != "" && dir != "." {
			err := os.MkdirAll(dir, 0o755)
			util.CheckError(err)
		}
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		util.CheckError(err)
		logFile = file
		fileHandler := slog.NewTextHandler(file, loggerOpts)
		errorHandler := log.NewFriendlyErrorHandler(streams.ErrOut)

		handler = log.NewDualHandler(fileHandler, errorHandler)
	} else {
		handler = log.NewFriendlyErrorHandler(streams.ErrOut)
	}

	logger = slog.New(handler)
}

func Execute(ctx context.Context, s *iostreams.IOStreams, bi *build.Info) {
	var err error
	executedCmd := rootCmd
	defer func() {
		if panicValue := recover(); panicValue != nil {
			cleanupTelemetryRecorder(ctx)
			closeLogFile()
			panic(panicValue)
		}
	}()
	buildInfo = bi
	version := meta.DefaultCLIVersion
	if bi != nil {
		version = bi.Version
	}
	meta.SetCLIVersion(version)
	cobra.EnableTraverseRunHooks = true
	streams = s
	defaultConfigFilePath, err = config.GetDefaultConfigFilePath()
	if err == nil {
		if f := rootCmd.PersistentFlags().Lookup(common.ConfigFilePathFlagName); f != nil {
			displayDefault := fmt.Sprintf("$XDG_CONFIG_HOME/%s/config.yaml", meta.CLIName)
			f.Usage = fmt.Sprintf(`Path to the configuration file to load.
- Default: [ %s ]`, displayDefault)
			f.DefValue = ""
		}
	}
	if err == nil {
		err = registerExtensions()
	}
	if err == nil {
		executedCmd, err = rootCmd.ExecuteContextC(ctx)
	}
	cleanupTelemetryRecorder(ctx)
	if err != nil {
		// If there was an execution error, use the logger to write it out and exit
		var executionError *cmdpkg.ExecutionError
		if errors.Is(err, context.Canceled) {
			logger.Info("Operation canceled")
		} else if errors.As(err, &executionError) {
			if executionError.Msg != "" && executionError.Attrs != nil && len(executionError.Attrs) > 0 {
				logger.Error(executionError.Msg, executionError.Attrs...)
			} else {
				logger.Error(executionError.Err.Error(), executionError.Attrs...)
			}
		} else {
			renderCommandError(streams.ErrOut, executedCmd, err)
		}
		closeLogFile()
		os.Exit(1)
	}
	closeLogFile()
}

func cleanupTelemetryRecorder(ctx context.Context) {
	if telemetryRecorder == nil {
		return
	}
	telemetryRecorder.Finalize(time.Now())
	_ = telemetryRecorder.Close(ctx)
	telemetryRecorder = nil
}

func registerExtensions() error {
	store, err := extensioncore.DefaultStore()
	if err != nil {
		return err
	}
	return extensioncore.RegisterInstalledCommands(rootCmd, store)
}

func closeLogFile() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

func renderCommandError(w io.Writer, command *cobra.Command, err error) {
	if cmdpkg.IsUsageError(err) {
		renderCommandUsageError(w, command, err)
		return
	}
	renderPlainCommandError(w, err)
}

func renderPlainCommandError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	errorText := strings.TrimSpace(err.Error())
	if errorText == "" {
		errorText = "an unknown error occurred"
	}
	fmt.Fprintf(w, "Error: %s\n", errorText)
}

func renderCommandUsageError(w io.Writer, command *cobra.Command, err error) {
	if w == nil || err == nil {
		return
	}

	errorText := strings.TrimSpace(stripCobraSuggestion(err.Error()))
	if errorText == "" {
		errorText = "invalid command usage"
	}

	fmt.Fprintf(w, "Error: %s\n", errorText)

	suggestion := cmdpkg.SuggestionForError(command, err)
	if len(suggestion.Values) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, suggestionHeader(suggestion.Kind, len(suggestion.Values)))
		for _, value := range suggestion.Values {
			fmt.Fprintf(w, "  %s\n", value)
		}
		fmt.Fprintln(w)
	} else {
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "Run '%s --help' for usage\n", commandPath(command))
}

func stripCobraSuggestion(message string) string {
	message, _, _ = strings.Cut(message, "\n\nDid you mean")
	return message
}

func commandPath(command *cobra.Command) string {
	if command == nil {
		return meta.CLIName
	}
	return command.CommandPath()
}

func suggestionHeader(kind string, count int) string {
	if kind == "" {
		kind = "suggestion"
	}
	if kind == "subcommand" {
		return "Available subcommands:"
	}
	if count == 1 {
		return fmt.Sprintf("Did you mean this %s?", kind)
	}
	return fmt.Sprintf("Did you mean one of these %ss?", kind)
}

package root

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs/adopt"
	"github.com/kong/kongctl/internal/cmd/root/verbs/api"
	"github.com/kong/kongctl/internal/cmd/root/verbs/apply"
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
	"github.com/kong/kongctl/internal/theme"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	rootLong = normalizers.LongDesc(i18n.T("root.rootLong", `
  Kong CLI is the official command line tool for Kong projects and products.

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

	outputFormat = cmd.NewEnum([]string{
		common.JSON.String(),
		common.YAML.String(),
		common.TEXT.String(),
	},
		common.TEXT.String())

	logLevel = cmd.NewEnum([]string{
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
)

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
	flags.AddFlagSet(cmd.LocalFlags())
	flags.AddFlagSet(cmd.InheritedFlags())

	return strings.TrimRight(flags.FlagUsages(), "\n")
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   meta.CLIName,
		Short: rootShort,
		Long:  rootLong,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			ctx := context.WithValue(cmd.Context(), config.ConfigKey, currConfig)
			ctx = context.WithValue(ctx, iostreams.StreamsKey, streams)
			ctx = context.WithValue(ctx, profile.ProfileManagerKey, pMgr)
			ctx = context.WithValue(ctx, build.InfoKey, buildInfo)
			ctx = context.WithValue(ctx, log.LoggerKey, logger)
			ctx = theme.ContextWithPalette(ctx, theme.Current())
			cmd.SetContext(ctx)
		},
	}

	// Disable Cobra's automatic help command since we have our own custom help command
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})
	cobra.AddTemplateFunc("mergedFlagUsages", mergedFlagUsages)
	rootCmd.SetUsageTemplate(mergedFlagsUsageTemplate)

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
		fmt.Sprintf(`Configures the format of data written to STDOUT.
- Config path: [ %s ]
- Allowed    : [ %s ]`,
			common.OutputConfigPath, strings.Join(outputFormat.Allowed, "|")))

	rootCmd.PersistentFlags().Var(logLevel, common.LogLevelFlagName,
		fmt.Sprintf(`Configures the logging level. Execution logs are written to STDERR.
- Config path: [ %s ]
- Allowed    : [ %s ]`,
			common.LogLevelConfigPath, strings.Join(logLevel.Allowed, "|")))

	rootCmd.PersistentFlags().StringVar(&logFilePath, common.LogFileFlagName, "",
		fmt.Sprintf(`Write execution logs to the specified file instead of STDERR.
- Config path: [ %s ]`,
			common.LogFileConfigPath))

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

	return nil
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

func commandTreeFlagChanged(command *cobra.Command, name string) bool {
	if command == nil {
		return false
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
			return true
		}
	}
	for _, child := range command.Commands() {
		if commandTreeFlagChanged(child, name) {
			return true
		}
	}
	return false
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
	// Show the hint whenever the active theme is the built-in default.
	// Users who have chosen any other theme have already discovered the feature.
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
		err = rootCmd.ExecuteContext(ctx)
	}
	if err != nil {
		// If there was an execution error, use the logger to write it out and exit
		// If it was a configuration error, we want the cobra framework to also
		// show the usage information, so we don't also print the error here
		var executionError *cmd.ExecutionError
		if errors.Is(err, context.Canceled) {
			logger.Info("Operation canceled")
		} else if errors.As(err, &executionError) {
			if executionError.Msg != "" && executionError.Attrs != nil && len(executionError.Attrs) > 0 {
				logger.Error(executionError.Msg, executionError.Attrs...)
			} else {
				logger.Error(executionError.Err.Error(), executionError.Attrs...)
			}
		}
		closeLogFile()
		os.Exit(1)
	}
	closeLogFile()
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

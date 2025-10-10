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
	"github.com/kong/kongctl/internal/cmd/root/verbs/adopt"
	"github.com/kong/kongctl/internal/cmd/root/verbs/api"
	"github.com/kong/kongctl/internal/cmd/root/verbs/apply"
	"github.com/kong/kongctl/internal/cmd/root/verbs/del"
	"github.com/kong/kongctl/internal/cmd/root/verbs/diff"
	"github.com/kong/kongctl/internal/cmd/root/verbs/dump"
	"github.com/kong/kongctl/internal/cmd/root/verbs/get"
	"github.com/kong/kongctl/internal/cmd/root/verbs/help"
	"github.com/kong/kongctl/internal/cmd/root/verbs/kai"
	"github.com/kong/kongctl/internal/cmd/root/verbs/list"
	"github.com/kong/kongctl/internal/cmd/root/verbs/login"
	"github.com/kong/kongctl/internal/cmd/root/verbs/plan"
	"github.com/kong/kongctl/internal/cmd/root/verbs/sync"
	"github.com/kong/kongctl/internal/cmd/root/version"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/profile"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	rootLong = normalizers.LongDesc(i18n.T("root.rootLong", `
  Kong CLI is the official command line tool for Kong projects and products.

  Find more information at:
   https://github.com/Kong/kongctl`))

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

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   meta.CLIName,
		Short: rootShort,
		Long:  rootLong,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			if isKaiCommand(cmd) {
				log.DisableErrorMirroring()
			} else {
				log.EnableErrorMirroring()
			}

			ctx := context.WithValue(cmd.Context(), config.ConfigKey, currConfig)
			ctx = context.WithValue(ctx, iostreams.StreamsKey, streams)
			ctx = context.WithValue(ctx, profile.ProfileManagerKey, pMgr)
			ctx = context.WithValue(ctx, build.InfoKey, buildInfo)
			ctx = context.WithValue(ctx, log.LoggerKey, logger)
			cmd.SetContext(ctx)
		},
	}

	// Disable Cobra's automatic help command since we have our own custom help command
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})

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

	command, err = kai.NewKaiCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = get.NewGetCmd()
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

	command, err = login.NewLoginCmd()
	if err != nil {
		return err
	}
	rootCmd.AddCommand(command)

	command, err = dump.NewDumpCmd()
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

func isKaiCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}

	for current := cmd; current != nil; current = current.Parent() {
		if current.Name() == kai.Verb.String() {
			return true
		}
	}

	return false
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
}

func initConfig() {
	if configFilePath == "" {
		configFilePath = defaultConfigFilePath
	}
	config, e1 := config.GetConfig(configFilePath, currProfile, defaultConfigFilePath)
	util.CheckError(e1)
	currConfig = config

	pMgr = profile.NewManager(config.Viper)

	bindFlags(currConfig)

	loggerOpts := &slog.HandlerOptions{
		Level: log.ConfigLevelStringToSlogLevel(config.GetString(common.LogLevelConfigPath)),
	}

	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}

	logPath := strings.TrimSpace(config.GetString(common.LogFileConfigPath))
	if logPath == "" {
		configPath := config.GetPath()
		configDir := filepath.Dir(configPath)
		defaultLogPath := filepath.Join(configDir, "logs", meta.CLIName+".log")
		logPath = defaultLogPath
		config.SetString(common.LogFileConfigPath, logPath)
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

		errorOpts := *loggerOpts
		errorOpts.Level = slog.LevelError
		errorHandler := slog.NewTextHandler(streams.ErrOut, &errorOpts)

		handler = log.NewDualHandler(fileHandler, errorHandler)
	} else {
		handler = slog.NewTextHandler(streams.ErrOut, loggerOpts)
	}

	logger = slog.New(handler)
}

func Execute(ctx context.Context, s *iostreams.IOStreams, bi *build.Info) {
	var err error
	buildInfo = bi
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

func closeLogFile() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

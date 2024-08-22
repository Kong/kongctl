package root

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/kong/kong-cli/internal/build"
	"github.com/kong/kong-cli/internal/cmd"
	"github.com/kong/kong-cli/internal/cmd/common"
	"github.com/kong/kong-cli/internal/cmd/root/verbs/create"
	"github.com/kong/kong-cli/internal/cmd/root/verbs/del"
	"github.com/kong/kong-cli/internal/cmd/root/verbs/get"
	"github.com/kong/kong-cli/internal/cmd/root/verbs/list"
	"github.com/kong/kong-cli/internal/cmd/root/verbs/login"
	"github.com/kong/kong-cli/internal/cmd/root/version"
	"github.com/kong/kong-cli/internal/config"
	"github.com/kong/kong-cli/internal/iostreams"
	"github.com/kong/kong-cli/internal/log"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/profile"
	"github.com/kong/kong-cli/internal/util"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	rootLong = normalizers.LongDesc(i18n.T("root.rootLong", `
  Kong CLI is the official command line tool for Kong projects and products.

  Find more information at:
   https://github.com/Kong/kongctl`))

	rootShort = i18n.T("root/rootShort", fmt.Sprintf("%s controls Kong", meta.CLIName))

	rootCmd *cobra.Command

	// Stores the global runtime value for the Configuration file path,
	configFilePath = config.ExpandDefaultConfigFilePath()
	currProfile    = profile.DefaultProfile

	currConfig   config.Hook
	streams      *iostreams.IOStreams
	pMgr         profile.Manager
	outputFormat = cmd.NewEnum([]string{"json", "yaml", "text"}, "text")
	logLevel     = cmd.NewEnum([]string{"debug", "info", "warn", "error"}, "error")

	buildInfo *build.Info

	logger *slog.Logger
)

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
			cmd.SetContext(ctx)
		},
		PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
			// return streams.ErrOut.Flush()
			return nil
		},
	}

	// parses all flags not just the target command
	rootCmd.TraverseChildren = true

	rootCmd.PersistentFlags().StringVar(&configFilePath, common.ConfigFilePathFlagName,
		config.ExpandDefaultConfigFilePath(),
		i18n.T("root."+common.ConfigFilePathFlagName, "Path to the configuration file to load."))

	rootCmd.PersistentFlags().StringVarP(&currProfile, common.ProfileFlagName, common.ProfileFlagShort,
		profile.DefaultProfile,
		"Specify the profile to use for this command.")

	// -------------------------------------------------------------------------
	// These require some extra gymnastics to ensure that the output flag is
	// from a valid set of values. There may be a way to do this more elegantly
	// in the pFlag library
	rootCmd.PersistentFlags().VarP(outputFormat, common.OutputFlagName, common.OutputFlagShort,
		fmt.Sprintf(`Configures the output format.
- Config path: [ %s ]
- Allowed    : [ %s ]`,
			common.OutputConfigPath, strings.Join(outputFormat.Allowed, "|")))

	rootCmd.PersistentFlags().Var(logLevel, common.LogLevelFlagName,
		fmt.Sprintf(`Configures the logging level. Execution logs are written to STDERR.
- Config path: [ %s ]
- Allowed    : [ %s ]`,
			common.LogLevelConfigPath, strings.Join(logLevel.Allowed, "|")))
	// -------------------------------------------------------------------------

	return rootCmd
}

// addCommands adds the root subcommands to the command.
func addCommands() error {
	rootCmd.AddCommand(version.NewVersionCmd())
	c, e := get.NewGetCmd()
	if e != nil {
		return e
	}
	rootCmd.AddCommand(c)

	c, e = list.NewListCmd()
	if e != nil {
		return e
	}
	rootCmd.AddCommand(c)

	c, e = create.NewCreateCmd()
	if e != nil {
		return e
	}
	rootCmd.AddCommand(c)

	c, e = del.NewDeleteCmd()
	if e != nil {
		return e
	}
	rootCmd.AddCommand(c)

	c, e = login.NewLoginCmd()
	if e != nil {
		return e
	}
	rootCmd.AddCommand(c)

	return nil
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
}

func initConfig() {
	config, e1 := config.GetConfig(configFilePath, currProfile)
	util.CheckError(e1)
	currConfig = config

	pMgr = profile.NewManager(config.Viper)

	bindFlags(currConfig)

	loggerOpts := &slog.HandlerOptions{
		Level: log.ConfigLevelStringToSlogLevel(config.GetString(common.LogLevelConfigPath)),
	}

	logger = slog.New(slog.NewTextHandler(streams.ErrOut, loggerOpts))
}

func Execute(ctx context.Context, s *iostreams.IOStreams, bi *build.Info) {
	buildInfo = bi
	cobra.EnableTraverseRunHooks = true
	streams = s
	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		// If there was an execution error, use the logger to write it out and exit
		// If it was a configuration error, we want the cobra framework to also
		// show the usage information, so we don't also print the error here
		var executionError *cmd.ExecutionError
		if errors.Is(err, context.Canceled) {
			fmt.Println("Canceled...")
		} else if errors.As(err, &executionError) {
			if executionError.Msg != "" && executionError.Attrs != nil && len(executionError.Attrs) > 0 {
				logger.Error(executionError.Msg, executionError.Attrs...)
			} else {
				logger.Error(executionError.Err.Error(), executionError.Attrs...)
			}
		}
		os.Exit(1)
	}
}

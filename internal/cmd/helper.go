package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/err"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/spf13/cobra"
)

type Common interface {
	bindFlags(cmd *cobra.Command, args []string) error
	validate(helper Helper) error
	run(helper Helper) error
}

type Helper interface {
	GetCmd() *cobra.Command
	GetArgs() []string
	GetVerb() (verbs.VerbValue, error)
	GetProduct() (products.ProductValue, error)
	GetStreams() *iostreams.IOStreams
	GetConfig() (config.Hook, error)
	GetOutputFormat() (common.OutputFormat, error)
	GetLogger() (*slog.Logger, error)
	GetBuildInfo() (*build.Info, error)
	GetContext() context.Context
	GetKonnectSDK(cfg config.Hook, logger *slog.Logger) (helpers.SDKAPI, error)
}

type CommandHelper struct {
	// Cmd is a pointer to the command that is being executed
	Cmd *cobra.Command
	// Args are the arguments (not flags) passed to the command
	Args []string
}

func (r *CommandHelper) GetCmd() *cobra.Command {
	return r.Cmd
}

func (r *CommandHelper) GetArgs() []string {
	return r.Args
}

func (r *CommandHelper) GetBuildInfo() (*build.Info, error) {
	return r.Cmd.Context().Value(build.InfoKey).(*build.Info), nil
}

func (r *CommandHelper) GetLogger() (*slog.Logger, error) {
	rv := r.Cmd.Context().Value(log.LoggerKey).(*slog.Logger)
	if rv == nil {
		return nil, &err.ConfigurationError{
			Err: fmt.Errorf("no logger configured"),
		}
	}
	return rv, nil
}

func (r *CommandHelper) GetVerb() (verbs.VerbValue, error) {
	verbVal := r.Cmd.Context().Value(verbs.Verb)
	if verbVal == nil {
		return "", &err.ExecutionError{
			Err: fmt.Errorf("no verb found in context"),
		}
	}
	return verbVal.(verbs.VerbValue), nil
}

func (r *CommandHelper) GetProduct() (products.ProductValue, error) {
	prdVal := r.Cmd.Context().Value(products.Product)
	if prdVal == nil {
		return "", &err.ExecutionError{
			Err: fmt.Errorf("no product found in context"),
		}
	}
	return prdVal.(products.ProductValue), nil
}

func (r *CommandHelper) GetStreams() *iostreams.IOStreams {
	return r.Cmd.Context().Value(iostreams.StreamsKey).(*iostreams.IOStreams)
}

func (r *CommandHelper) GetConfig() (config.Hook, error) {
	cfgVal := r.Cmd.Context().Value(config.ConfigKey)
	if cfgVal == nil {
		return nil, &err.ExecutionError{
			Err: fmt.Errorf("no config found in context"),
		}
	}
	return cfgVal.(config.Hook), nil
}

func (r *CommandHelper) GetOutputFormat() (common.OutputFormat, error) {
	c, e := r.GetConfig()
	if e != nil {
		return common.TEXT, e
	}
	s := c.GetString(common.OutputConfigPath)
	rv, e := common.OutputFormatStringToIota(s)
	if e != nil {
		return common.TEXT, e
	}
	return rv, nil
}

func (r *CommandHelper) GetContext() context.Context {
	return r.Cmd.Context()
}

func (r *CommandHelper) GetKonnectSDK(cfg config.Hook, logger *slog.Logger) (helpers.SDKAPI, error) {
	return r.Cmd.Context().Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)(cfg, logger)
}

func BuildHelper(cmd *cobra.Command, args []string) Helper {
	return &CommandHelper{
		Cmd:  cmd,
		Args: args,
	}
}

// This will construct an execution error AND turn off error and usage output for the command
func PrepareExecutionError(msg string, e error, cmd *cobra.Command, attrs ...any) *err.ExecutionError {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	return &err.ExecutionError{
		Msg:   msg,
		Err:   e,
		Attrs: attrs,
	}
}

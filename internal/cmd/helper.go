package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
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
	IsInteractive() (bool, error)
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
	val := r.Cmd.Context().Value(build.InfoKey)
	if val == nil {
		return nil, &ConfigurationError{
			Err: fmt.Errorf("no build info configured"),
		}
	}

	info, ok := val.(*build.Info)
	if !ok || info == nil {
		return nil, &ConfigurationError{
			Err: fmt.Errorf("invalid build info configured"),
		}
	}

	return info, nil
}

func (r *CommandHelper) GetLogger() (*slog.Logger, error) {
	rv := r.Cmd.Context().Value(log.LoggerKey).(*slog.Logger)
	if rv == nil {
		return nil, &ConfigurationError{
			Err: fmt.Errorf("no logger configured"),
		}
	}
	return rv, nil
}

func (r *CommandHelper) GetVerb() (verbs.VerbValue, error) {
	verbVal := r.Cmd.Context().Value(verbs.Verb)
	if verbVal == nil {
		return "", PrepareExecutionErrorMsg(r, "no verb found in context")
	}
	return verbVal.(verbs.VerbValue), nil
}

func (r *CommandHelper) GetProduct() (products.ProductValue, error) {
	prdVal := r.Cmd.Context().Value(products.Product)
	if prdVal == nil {
		return "", PrepareExecutionErrorMsg(r, "no product found in context")
	}
	return prdVal.(products.ProductValue), nil
}

func (r *CommandHelper) GetStreams() *iostreams.IOStreams {
	return r.Cmd.Context().Value(iostreams.StreamsKey).(*iostreams.IOStreams)
}

func (r *CommandHelper) GetConfig() (config.Hook, error) {
	cfgVal := r.Cmd.Context().Value(config.ConfigKey)
	if cfgVal == nil {
		return nil, PrepareExecutionErrorMsg(r, "no config found in context")
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

func (r *CommandHelper) IsInteractive() (bool, error) {
	flag := r.Cmd.Flags().Lookup(common.InteractiveFlagName)
	if flag == nil {
		flag = r.Cmd.InheritedFlags().Lookup(common.InteractiveFlagName)
	}
	if flag == nil {
		return false, nil
	}

	val := flag.Value.String()
	if val == "" {
		return false, nil
	}

	interactive, err := strconv.ParseBool(val)
	if err != nil {
		return false, &ConfigurationError{
			Err: fmt.Errorf("invalid value %q for --%s flag", val, common.InteractiveFlagName),
		}
	}
	return interactive, nil
}

func (r *CommandHelper) GetContext() context.Context {
	return r.Cmd.Context()
}

func (r *CommandHelper) GetKonnectSDK(cfg config.Hook, logger *slog.Logger) (helpers.SDKAPI, error) {
	factory := r.Cmd.Context().Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	sdk, err := factory(cfg, logger)
	if err != nil {
		return nil, PrepareExecutionErrorFromErr(r, err)
	}
	return sdk, nil
}

func BuildHelper(cmd *cobra.Command, args []string) Helper {
	return &CommandHelper{
		Cmd:  cmd,
		Args: args,
	}
}

// ConfigurationError represents errors that are a result of bad flags, combinations of
// flags, configuration settings, environment values, or other command usage issues.
type ConfigurationError struct {
	Err error
}

// ExecutionError represents errors that occur after a command has been validated and an
// unsuccessful result occurs.  Network errors, server side errors, invalid credentials or responses
// are examples of RunttimeError types.
type ExecutionError struct {
	// friendly error message to display to the user
	Msg string
	// Err is the error that occurred during execution
	Err error
	// Optional attributes that can be used to provide additional context to the error
	Attrs []any
}

func (e *ConfigurationError) Error() string {
	return e.Err.Error()
}

func (e *ExecutionError) Error() string {
	return e.Err.Error()
}

// Will try and json unmarshal an error string into a slice of interfaces
// that match the slog algorithm for varadic parameters (alternating key value pairs)
func TryConvertErrorToAttrs(err error) []any {
	var result map[string]any
	umError := json.Unmarshal([]byte(err.Error()), &result)
	if umError != nil {
		return nil
	}
	attrs := make([]any, 0, len(result)*2)
	for k, v := range result {
		attrs = append(attrs, k, v)
	}
	return attrs
}

// PrepareExecutionErrorWithHelper mirrors PrepareExecutionError but accepts a Helper.
// It ensures command usage/error output is silenced for runtime failures.
func PrepareExecutionErrorWithHelper(helper Helper, msg string, err error, attrs ...any) *ExecutionError {
	if helper == nil {
		return PrepareExecutionError(msg, err, nil, attrs...)
	}
	return PrepareExecutionError(msg, err, helper.GetCmd(), attrs...)
}

// PrepareExecutionErrorFromErr converts an arbitrary error into an ExecutionError while
// silencing usage/error output on the associated command. The friendly message defaults
// to the underlying error string when msg is empty.
func PrepareExecutionErrorFromErr(helper Helper, err error, attrs ...any) *ExecutionError {
	if err == nil {
		return nil
	}
	msg := err.Error()
	return PrepareExecutionErrorWithHelper(helper, msg, err, attrs...)
}

// PrepareExecutionErrorMsg builds an ExecutionError from a message when a backing error
// is not already available.
func PrepareExecutionErrorMsg(helper Helper, msg string, attrs ...any) *ExecutionError {
	if msg == "" {
		return PrepareExecutionErrorWithHelper(helper, msg, errors.New("an unknown error occurred"), attrs...)
	}
	return PrepareExecutionErrorWithHelper(helper, msg, errors.New(msg), attrs...)
}

// This will construct an execution error AND turn off error and usage output for the command
func PrepareExecutionError(msg string, err error, cmd *cobra.Command, attrs ...any) *ExecutionError {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	return &ExecutionError{
		Msg:   msg,
		Err:   err,
		Attrs: attrs,
	}
}

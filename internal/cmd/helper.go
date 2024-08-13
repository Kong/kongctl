package cmd

import (
	"fmt"

	"github.com/kong/kong-cli/internal/cmd/common"
	"github.com/kong/kong-cli/internal/cmd/root/products"
	"github.com/kong/kong-cli/internal/cmd/root/verbs"
	"github.com/kong/kong-cli/internal/config"
	"github.com/kong/kong-cli/internal/iostreams"
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
	GetOutputFormat() (string, error)
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

func (r *CommandHelper) GetVerb() (verbs.VerbValue, error) {
	verbVal := r.Cmd.Context().Value(verbs.Verb)
	if verbVal == nil {
		return "", &ExecutionError{
			Err: fmt.Errorf("no verb found in context"),
		}
	}
	return verbVal.(verbs.VerbValue), nil
}

func (r *CommandHelper) GetProduct() (products.ProductValue, error) {
	prdVal := r.Cmd.Context().Value(products.Product)
	if prdVal == nil {
		return "", &ExecutionError{
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
		return nil, &ExecutionError{
			Err: fmt.Errorf("no config found in context"),
		}
	}
	return cfgVal.(config.Hook), nil
}

func (r *CommandHelper) GetOutputFormat() (string, error) {
	c, e := r.GetConfig()
	if e != nil {
		return "", e
	}
	rv := c.GetString(common.OutputFlagName)
	return rv, nil
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
	Err error
}

func (e *ConfigurationError) Error() string {
	return e.Err.Error()
}

func (e *ExecutionError) Error() string {
	return e.Err.Error()
}
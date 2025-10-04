package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	apiutil "github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.API
)

var (
	apiUse = fmt.Sprintf("%s <endpoint>", Verb.String())

	apiShort = i18n.T("root.verbs.api.apiShort", "Call the Konnect API directly")

	apiLong = normalizers.LongDesc(i18n.T("root.verbs.api.apiLong",
		"Send an authenticated GET request to a Konnect API endpoint."))

	apiExamples = normalizers.Examples(i18n.T("root.verbs.api.apiExamples",
		fmt.Sprintf(`
	# Get the current user
	%[1]s api /v1/me

	# List declarative sessions
	%[1]s api /v1/sessions`, meta.CLIName)))
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().String(konnectcommon.BaseURLFlagName, konnectcommon.BaseURLDefault,
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`, konnectcommon.BaseURLConfigPath))

	cmd.Flags().String(konnectcommon.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`, konnectcommon.PATConfigPath))
}

func bindFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	f := c.Flags().Lookup(konnectcommon.BaseURLFlagName)
	if err = cfg.BindFlag(konnectcommon.BaseURLConfigPath, f); err != nil {
		return err
	}

	f = c.Flags().Lookup(konnectcommon.PATFlagName)
	if f != nil {
		if err = cfg.BindFlag(konnectcommon.PATConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}

func NewAPICmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     apiUse,
		Short:   apiShort,
		Long:    apiLong,
		Example: apiExamples,
		Args:    cobra.MinimumNArgs(1),
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = context.WithValue(ctx, verbs.Verb, Verb)
			c.SetContext(ctx)
			return bindFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(c, args)
			return run(helper)
		},
	}

	addFlags(cmd)

	return cmd, nil
}

func run(helper cmd.Helper) error {
	args := helper.GetArgs()
	endpoint := strings.TrimSpace(args[0])
	if endpoint == "" {
		return cmd.PrepareExecutionError("endpoint is required", errors.New("endpoint cannot be empty"), helper.GetCmd())
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	baseURL := cfg.GetString(konnectcommon.BaseURLConfigPath)
	if baseURL == "" {
		baseURL = konnectcommon.BaseURLDefault
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return cmd.PrepareExecutionError("failed to resolve Konnect access token", err, helper.GetCmd())
	}

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}

	client := httpclient.NewLoggingHTTPClient(logger)
	result, err := apiutil.Request(ctx, client, http.MethodGet, baseURL, endpoint, token, nil, nil)
	if err != nil {
		return cmd.PrepareExecutionError("failed to call Konnect API", err, helper.GetCmd())
	}

	if result.StatusCode >= 400 {
		return cmd.PrepareExecutionError(
			fmt.Sprintf("request failed with status %d", result.StatusCode),
			errors.New(string(result.Body)),
			helper.GetCmd(),
		)
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	streams := helper.GetStreams()

	switch outType {
	case cmdcommon.TEXT:
		_, err = fmt.Fprintln(streams.Out, strings.TrimRight(string(result.Body), "\n"))
		return err
	case cmdcommon.JSON, cmdcommon.YAML:
		var payload any
		if len(result.Body) > 0 {
			if err := json.Unmarshal(result.Body, &payload); err != nil {
				payload = string(result.Body)
			}
		}
		printer, err := cli.Format(outType.String(), streams.Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(payload)
		return nil
	}

	return nil
}

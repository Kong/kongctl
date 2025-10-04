package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

	cmd.Flags().String("jq", "", "Filter JSON responses using a limited jq-style expression (dot notation, array indexes)")
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

	jqFilter, err := helper.GetCmd().Flags().GetString("jq")
	if err != nil {
		return err
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

	var bodyToRender []byte
	bodyToRender = result.Body

	if jqFilter != "" {
		filtered, err := applyJQFilter(bodyToRender, jqFilter)
		if err != nil {
			return cmd.PrepareExecutionError("jq filter failed", err, helper.GetCmd())
		}
		bodyToRender = filtered
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	streams := helper.GetStreams()

	switch outType {
	case cmdcommon.TEXT:
		_, err = fmt.Fprintln(streams.Out, strings.TrimRight(bodyToPrintable(bodyToRender), "\n"))
		return err
	case cmdcommon.JSON, cmdcommon.YAML:
		var payload any
		if len(bodyToRender) > 0 {
			if err := json.Unmarshal(bodyToRender, &payload); err != nil {
				payload = strings.TrimRight(bodyToPrintable(bodyToRender), "\n")
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

func applyJQFilter(body []byte, filter string) ([]byte, error) {
	if len(body) == 0 {
		return nil, errors.New("response body is empty, cannot apply jq filter")
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("response is not valid JSON: %w", err)
	}

	result, err := evaluateSimpleJQ(payload, strings.TrimSpace(filter))
	if err != nil {
		return nil, err
	}

	filtered, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to encode filtered result: %w", err)
	}

	return filtered, nil
}

func evaluateSimpleJQ(data any, filter string) (any, error) {
	if filter == "" || filter == "." {
		return data, nil
	}

	filter = strings.TrimPrefix(filter, ".")

	if filter == "" {
		return data, nil
	}

	segments := strings.Split(filter, ".")
	current := data

	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		var key string
		var index *int

		if strings.Contains(segment, "[") {
			if !strings.HasSuffix(segment, "]") {
				return nil, fmt.Errorf("unsupported jq segment %q", segment)
			}
			open := strings.Index(segment, "[")
			key = segment[:open]
			idxStr := segment[open+1 : len(segment)-1]
			if idxStr == "" {
				return nil, fmt.Errorf("empty array index in segment %q", segment)
			}
			i, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index in segment %q", segment)
			}
			index = &i
		} else {
			key = segment
		}

		if key != "" {
			obj, ok := current.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("segment %q expects object but found %T", key, current)
			}
			var exists bool
			current, exists = obj[key]
			if !exists {
				return nil, fmt.Errorf("key %q not found", key)
			}
		}

		if index != nil {
			arr, ok := current.([]any)
			if !ok {
				return nil, fmt.Errorf("segment %q expects array but found %T", segment, current)
			}
			if *index < 0 || *index >= len(arr) {
				return nil, fmt.Errorf("index %d out of range for segment %q", *index, segment)
			}
			current = arr[*index]
		}
	}

	return current, nil
}

func bodyToPrintable(body []byte) string {
	var js any
	if err := json.Unmarshal(body, &js); err != nil {
		return string(body)
	}
	formatted, err := json.MarshalIndent(js, "", "  ")
	if err != nil {
		return string(body)
	}
	return string(formatted)
}

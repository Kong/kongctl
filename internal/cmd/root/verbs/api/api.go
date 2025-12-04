package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/itchyny/gojq"
	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/iostreams"
	apiutil "github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/mattn/go-isatty"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	Verb                     = verbs.API
	jqColorFlagName          = "jq-color"
	jqColorThemeFlagName     = "jq-color-theme"
	jqColorEnabledConfigPath = "jq.color.enabled"
	jqColorThemeConfigPath   = "jq.color.theme"
	jqColorDefaultThemeValue = "friendly"
	responseHeadersFlagName  = "include-response-headers"
)

var (
	apiUse = fmt.Sprintf("%s <endpoint> [field=value ...]", Verb.String())

	apiShort = i18n.T("root.verbs.api.apiShort", "Call the Konnect API directly")

	apiLong = normalizers.LongDesc(i18n.T("root.verbs.api.apiLong",
		"Send authenticated requests to Konnect API endpoints using common HTTP verbs."))

	apiExamples = normalizers.Examples(i18n.T("root.verbs.api.apiExamples",
		fmt.Sprintf(`
	# Get the current user
	%[1]s api /v3/users/me

	# Explicit GET
	%[1]s api get /v3/users/me

	# Create a resource with JSON fields
	%[1]s api post /v3/apis name=my-api config:={"enabled":true}

	# Update a resource
	%[1]s api put /v3/apis/123 name="my-updated-api"

	# Partially update a resource
	%[1]s api patch /v3/apis/123 config:={"enabled":false}

	# Delete a resource
	%[1]s api delete /v3/apis/123`, meta.CLIName)))

	jqQueryCache sync.Map
)

var requestFn = apiutil.Request

func addFlags(command *cobra.Command) {
	command.PersistentFlags().String(konnectcommon.BaseURLFlagName, "",
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`, konnectcommon.BaseURLConfigPath, konnectcommon.BaseURLDefault))

	command.PersistentFlags().String(konnectcommon.RegionFlagName, "",
		fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`, konnectcommon.BaseURLFlagName, konnectcommon.RegionConfigPath))

	command.PersistentFlags().String(konnectcommon.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`, konnectcommon.PATConfigPath))

	command.PersistentFlags().String(
		"jq",
		"",
		"Filter JSON responses using jq expressions (powered by gojq for full jq compatibility)",
	)

	var bodyFileFlagValue string
	command.PersistentFlags().VarP(
		newSingleBodyFileValue(&bodyFileFlagValue),
		"body-file",
		"f",
		"Read request body from file ('-' to read from standard input)",
	)

	jqColor := cmd.NewEnum([]string{
		cmdcommon.ColorModeAuto.String(),
		cmdcommon.ColorModeAlways.String(),
		cmdcommon.ColorModeNever.String(),
	}, cmdcommon.DefaultColorMode)
	command.PersistentFlags().Var(
		jqColor,
		jqColorFlagName,
		fmt.Sprintf(`Controls colorized output for jq filter results.
- Config path: [ %s ]
- Allowed    : [ auto|always|never ]`, jqColorEnabledConfigPath),
	)

	command.PersistentFlags().String(
		jqColorThemeFlagName,
		jqColorDefaultThemeValue,
		fmt.Sprintf(`Select the color theme used for jq filter results.
- Config path: [ %s ]
- Examples   : [ friendly, github-dark, dracula ]
- Reference  : [ https://xyproto.github.io/splash/docs/ ]`, jqColorThemeConfigPath),
	)

	command.PersistentFlags().Bool(
		responseHeadersFlagName,
		false,
		"Include response headers in error output",
	)

	// Ensure persistent flags are accessible via Flags() for commands without subcommands (tests)
	command.Flags().AddFlagSet(command.PersistentFlags())
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

	f = c.Flags().Lookup(konnectcommon.RegionFlagName)
	if f != nil {
		if err = cfg.BindFlag(konnectcommon.RegionConfigPath, f); err != nil {
			return err
		}
	}

	f = c.Flags().Lookup(konnectcommon.PATFlagName)
	if f != nil {
		if err = cfg.BindFlag(konnectcommon.PATConfigPath, f); err != nil {
			return err
		}
	}

	f = c.Flags().Lookup(jqColorFlagName)
	if f != nil {
		if err = cfg.BindFlag(jqColorEnabledConfigPath, f); err != nil {
			return err
		}
	}

	f = c.Flags().Lookup(jqColorThemeFlagName)
	if f != nil {
		if err = cfg.BindFlag(jqColorThemeConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}

func NewAPICmd() (*cobra.Command, error) {
	rootCmd := &cobra.Command{
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
	}

	addFlags(rootCmd)

	rootCmd.RunE = func(c *cobra.Command, args []string) error {
		helper := cmd.BuildHelper(c, args)
		return run(helper, http.MethodGet, false)
	}

	rootCmd.AddCommand(newMethodCmd("get", http.MethodGet, false))
	rootCmd.AddCommand(newMethodCmd("post", http.MethodPost, true))
	rootCmd.AddCommand(newMethodCmd("put", http.MethodPut, true))
	rootCmd.AddCommand(newMethodCmd("patch", http.MethodPatch, true))
	rootCmd.AddCommand(newMethodCmd("delete", http.MethodDelete, false))

	return rootCmd, nil
}

func newMethodCmd(name, httpMethod string, allowBody bool) *cobra.Command {
	use := fmt.Sprintf("%s <endpoint>", name)
	if allowBody {
		use += " [field=value ...]"
	}

	return &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("Send an HTTP %s request to a Konnect endpoint", strings.ToUpper(httpMethod)),
		Args:  cobra.MinimumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(c, args)
			return run(helper, httpMethod, allowBody)
		},
	}
}

func run(helper cmd.Helper, method string, allowBody bool) error {
	args := helper.GetArgs()
	endpoint := strings.TrimSpace(args[0])
	if endpoint == "" {
		return cmd.PrepareExecutionError(
			"endpoint is required",
			errors.New("endpoint cannot be empty"),
			helper.GetCmd(),
		)
	}

	jw := helper.GetCmd().Flags()
	jqFilter, err := jw.GetString("jq")
	if err != nil {
		return err
	}
	jFlagChanged := jw.Changed("jq")
	jqFilter = strings.TrimSpace(jqFilter)
	if jFlagChanged && jqFilter == "" {
		jqFilter = "."
	}

	bodyFilePath, err := helper.GetCmd().Flags().GetString("body-file")
	if err != nil {
		return err
	}

	includeResponseHeaders, err := helper.GetCmd().Flags().GetBool(responseHeadersFlagName)
	if err != nil {
		return err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	jqColorValue := cfg.GetString(jqColorEnabledConfigPath)
	jColorMode, err := cmdcommon.ColorModeStringToIota(strings.ToLower(jqColorValue))
	if err != nil {
		return err
	}
	jqThemeValue := strings.TrimSpace(cfg.GetString(jqColorThemeConfigPath))
	if jqThemeValue == "" {
		jqThemeValue = jqColorDefaultThemeValue
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return cmd.PrepareExecutionError("failed to resolve Konnect base URL", err, helper.GetCmd())
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return cmd.PrepareExecutionError("failed to resolve Konnect access token", err, helper.GetCmd())
	}

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}

	streams := helper.GetStreams()

	useJQColor := false
	if jqFilter != "" {
		useJQColor = shouldUseJQColor(jColorMode, streams.Out)
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	interactive, err := helper.IsInteractive()
	if err != nil {
		return err
	}
	if interactive || outType == cmdcommon.TEXT {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("%s command supports only json or yaml output formats (received %q)",
				helper.GetCmd().CommandPath(), outType.String()),
		}
	}

	dataArgs := args[1:]
	var bodyReader io.Reader
	headers := map[string]string{}

	if bodyFilePath != "" {
		if !allowBody {
			return cmd.PrepareExecutionError(
				"unexpected request body",
				fmt.Errorf("request body is not allowed for %s", strings.ToUpper(method)),
				helper.GetCmd(),
			)
		}
		if len(dataArgs) > 0 {
			return cmd.PrepareExecutionError(
				"conflicting request data",
				fmt.Errorf("cannot combine --body-file with inline field assignments"),
				helper.GetCmd(),
			)
		}
		payload, err := loadRequestBody(bodyFilePath, streams)
		if err != nil {
			return cmd.PrepareExecutionError("failed to load request body", err, helper.GetCmd())
		}
		bodyReader = bytes.NewReader(payload)
		headers["Content-Type"] = "application/json"
	} else if len(dataArgs) > 0 {
		if !allowBody {
			return cmd.PrepareExecutionError(
				"unexpected data arguments",
				fmt.Errorf("data fields may only be supplied with POST, PUT, or PATCH"),
				helper.GetCmd(),
			)
		}
		payload, err := parseAssignments(dataArgs)
		if err != nil {
			return cmd.PrepareExecutionError("invalid request data", err, helper.GetCmd())
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return cmd.PrepareExecutionError("failed to encode request body", err, helper.GetCmd())
		}
		bodyReader = bytes.NewReader(encoded)
		headers["Content-Type"] = "application/json"
	} else if !allowBody {
		headers = nil
	}

	client := httpclient.NewLoggingHTTPClient(logger)
	result, err := requestFn(ctx, client, method, baseURL, endpoint, token, headers, bodyReader)
	if err != nil {
		return cmd.PrepareExecutionError("failed to call Konnect API", err, helper.GetCmd())
	}

	if result.StatusCode >= 400 {
		statusText := http.StatusText(result.StatusCode)
		summary := fmt.Sprintf("request failed with status %d", result.StatusCode)
		if statusText != "" {
			summary = fmt.Sprintf("%s %s", summary, statusText)
		}

		body := strings.TrimSpace(string(result.Body))
		respErr := errors.New(summary)
		if body != "" {
			respErr = fmt.Errorf("%s: %s", summary, body)
		}

		attrs := []any{
			"status", result.StatusCode,
			"method", method,
			"endpoint", endpoint,
		}
		if statusText != "" {
			attrs = append(attrs, "status_text", statusText)
		}
		if body != "" {
			attrs = append(attrs, "response", body)
		}
		if includeResponseHeaders && len(result.Header) > 0 {
			attrs = append(attrs, "headers", result.Header)
		}

		return cmd.PrepareExecutionError(summary, respErr, helper.GetCmd(), attrs...)
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

	switch outType {
	case cmdcommon.TEXT:
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("unsupported output format %q for %s command", outType.String(),
				helper.GetCmd().CommandPath()),
		}
	case cmdcommon.JSON:
		if len(bodyToRender) == 0 {
			return nil
		}
		printable := bodyToPrintable(bodyToRender)
		if useJQColor {
			printable = maybeColorizeJQOutput(bodyToRender, printable, jqThemeValue)
		}
		_, err = fmt.Fprintln(streams.Out, strings.TrimRight(printable, "\n"))
		return err
	case cmdcommon.YAML:
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
	default:
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("unsupported output format %q for %s command", outType.String(),
				helper.GetCmd().CommandPath()),
		}
	}
}

type singleBodyFileValue struct {
	target *string
	set    bool
}

func newSingleBodyFileValue(target *string) *singleBodyFileValue {
	return &singleBodyFileValue{target: target}
}

func (s *singleBodyFileValue) String() string {
	if s == nil || s.target == nil {
		return ""
	}
	return *s.target
}

func (s *singleBodyFileValue) Set(value string) error {
	if s == nil {
		return fmt.Errorf("body file flag not initialized")
	}
	if s.set {
		return fmt.Errorf("--body-file may only be provided once")
	}
	if s.target == nil {
		return fmt.Errorf("body file target not configured")
	}
	if value == "" {
		return fmt.Errorf("--body-file requires a filename or '-' for stdin")
	}
	*s.target = value
	s.set = true
	return nil
}

func (s *singleBodyFileValue) Type() string { return "string" }

func (s *singleBodyFileValue) Get() any {
	if s == nil || s.target == nil {
		return ""
	}
	return *s.target
}

var isTerminalFile = func(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func loadRequestBody(path string, streams *iostreams.IOStreams) ([]byte, error) {
	if path == "-" {
		if streams == nil || streams.In == nil {
			return nil, fmt.Errorf("standard input is not available")
		}
		reader := streams.In
		if isInteractiveInput(reader) {
			return nil, fmt.Errorf("standard input is a terminal; pipe data or provide a file path")
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body from stdin: %w", err)
		}
		closeIfPossible(reader)
		return data, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open request body file %q: %w", path, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body file %q: %w", path, err)
	}

	return data, nil
}

type fdProvider interface {
	Fd() uintptr
}

func isInteractiveInput(r io.Reader) bool {
	if provider, ok := r.(fdProvider); ok {
		fd := provider.Fd()
		if fd != ^uintptr(0) && isTerminalFile(fd) {
			return true
		}
	}
	return false
}

func closeIfPossible(r io.Reader) {
	if closer, ok := r.(io.Closer); ok {
		_ = closer.Close()
	}
}

var jqTerminalDetector = func(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func shouldUseJQColor(mode cmdcommon.ColorMode, out io.Writer) bool {
	switch mode {
	case cmdcommon.ColorModeAlways:
		return true
	case cmdcommon.ColorModeNever:
		return false
	case cmdcommon.ColorModeAuto:
		if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
			return false
		}
		return isJQTerminal(out)
	default:
		if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
			return false
		}
		return isJQTerminal(out)
	}
}

func isJQTerminal(out io.Writer) bool {
	type fdWriter interface {
		Fd() uintptr
	}
	if fw, ok := out.(fdWriter); ok {
		return jqTerminalDetector(fw.Fd())
	}
	return false
}

func maybeColorizeJQOutput(raw []byte, formatted, theme string) string {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return formatted
	}
	switch payload.(type) {
	case map[string]any, []any:
		// acceptable for colorization
	default:
		return formatted
	}

	lexer := lexers.Get("json")
	if lexer == nil {
		return formatted
	}
	iterator, err := lexer.Tokenise(nil, formatted)
	if err != nil {
		return formatted
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Get("terminal")
	}
	if formatter == nil {
		return formatted
	}

	style := styles.Get(theme)
	if style == nil {
		style = styles.Get(jqColorDefaultThemeValue)
	}
	if style == nil {
		style = styles.Fallback
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return formatted
	}

	return buf.String()
}

func parseAssignments(fields []string) (map[string]any, error) {
	payload := make(map[string]any, len(fields))
	for _, field := range fields {
		if field == "" {
			return nil, fmt.Errorf("empty assignment is not allowed")
		}

		var (
			key       string
			value     string
			jsonTyped bool
		)

		if strings.Contains(field, ":=") {
			parts := strings.SplitN(field, ":=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("malformed assignment %q", field)
			}
			key = strings.TrimSpace(parts[0])
			value = parts[1]
			jsonTyped = true
		} else if strings.Contains(field, "=") {
			parts := strings.SplitN(field, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("malformed assignment %q", field)
			}
			key = strings.TrimSpace(parts[0])
			value = parts[1]
		} else {
			return nil, fmt.Errorf("expected key=value or key:=value, got %q", field)
		}

		if key == "" {
			return nil, fmt.Errorf("assignment %q is missing a key", field)
		}

		tokens, err := parseAssignmentPath(key)
		if err != nil {
			return nil, err
		}

		var decoded any
		if jsonTyped {
			if err := json.Unmarshal([]byte(value), &decoded); err != nil {
				return nil, fmt.Errorf("failed to parse %s as JSON: %w", key, err)
			}
		} else {
			decoded = value
		}

		if err := setNestedValue(payload, tokens, key, decoded); err != nil {
			return nil, err
		}
	}

	return payload, nil
}

type assignmentPathToken struct {
	key     string
	isIndex bool
	index   int
}

func parseAssignmentPath(path string) ([]assignmentPathToken, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("assignment path cannot be empty")
	}

	var tokens []assignmentPathToken
	for i := 0; i < len(trimmed); {
		switch trimmed[i] {
		case '.':
			i++
			if i >= len(trimmed) {
				return nil, fmt.Errorf("path %q has trailing dot", path)
			}
			continue
		case '[':
			end := strings.IndexByte(trimmed[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("path %q has unterminated array segment", path)
			}
			end = i + end
			idxStr := strings.TrimSpace(trimmed[i+1 : end])
			if idxStr == "" {
				return nil, fmt.Errorf("path %q has empty array index", path)
			}
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil, fmt.Errorf("path %q has invalid array index %q", path, idxStr)
			}
			tokens = append(tokens, assignmentPathToken{isIndex: true, index: idx})
			i = end + 1
			continue
		default:
			start := i
			for i < len(trimmed) && trimmed[i] != '.' && trimmed[i] != '[' {
				i++
			}
			segment := strings.TrimSpace(trimmed[start:i])
			if segment == "" {
				return nil, fmt.Errorf("path %q has empty object key", path)
			}
			tokens = append(tokens, assignmentPathToken{key: segment})
		}
	}

	if len(tokens) == 0 {
		return nil, fmt.Errorf("assignment path %q produced no segments", path)
	}

	if tokens[0].isIndex {
		return nil, fmt.Errorf("assignment path %q must start with an object key", path)
	}

	return tokens, nil
}

func setNestedValue(root map[string]any, tokens []assignmentPathToken, path string, value any) error {
	first := tokens[0]
	if len(tokens) == 1 {
		root[first.key] = value
		return nil
	}

	current, exists := root[first.key]
	if !exists || current == nil {
		if tokens[1].isIndex {
			current = []any{}
		} else {
			current = map[string]any{}
		}
	}

	if err := assignNestedValue(&current, tokens[1:], path, value); err != nil {
		return err
	}

	root[first.key] = current
	return nil
}

func assignNestedValue(current *any, tokens []assignmentPathToken, path string, value any) error {
	if len(tokens) == 0 {
		*current = value
		return nil
	}

	segment := tokens[0]
	if segment.isIndex {
		if segment.index < 0 {
			return fmt.Errorf("path %q has negative array index %d", path, segment.index)
		}
		var arr []any
		switch typed := (*current).(type) {
		case nil:
			arr = []any{}
		case []any:
			arr = typed
		default:
			return fmt.Errorf("path %q segment [%d] expects array but found %T", path, segment.index, *current)
		}
		if segment.index >= len(arr) {
			arr = append(arr, make([]any, segment.index-len(arr)+1)...)
		}
		next := arr[segment.index]
		if err := assignNestedValue(&next, tokens[1:], path, value); err != nil {
			return err
		}
		arr[segment.index] = next
		*current = arr
		return nil
	}

	var obj map[string]any
	switch typed := (*current).(type) {
	case nil:
		obj = map[string]any{}
	case map[string]any:
		obj = typed
	default:
		return fmt.Errorf("path %q segment %q expects object but found %T", path, segment.key, *current)
	}

	next := obj[segment.key]
	if err := assignNestedValue(&next, tokens[1:], path, value); err != nil {
		return err
	}
	obj[segment.key] = next
	*current = obj
	return nil
}

func applyJQFilter(body []byte, filter string) ([]byte, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		filter = "."
	}

	if len(body) == 0 {
		return nil, errors.New("response body is empty, cannot apply jq filter")
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("response is not valid JSON: %w", err)
	}

	query, err := getCachedJQQuery(filter)
	if err != nil {
		return nil, err
	}

	iter := query.Run(payload)
	var results []any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			return nil, fmt.Errorf("jq filter failed: %w", err)
		}
		results = append(results, normalizeGoJQValue(v))
	}

	if len(results) == 0 {
		return []byte("null"), nil
	}

	if len(results) == 1 {
		filtered, err := json.Marshal(results[0])
		if err != nil {
			return nil, fmt.Errorf("failed to encode filtered result: %w", err)
		}
		return filtered, nil
	}

	filtered, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("failed to encode filtered result: %w", err)
	}

	return filtered, nil
}

func getCachedJQQuery(filter string) (*gojq.Code, error) {
	if code, ok := jqQueryCache.Load(filter); ok {
		return code.(*gojq.Code), nil
	}

	parsed, err := gojq.Parse(filter)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression: %w", err)
	}

	code, err := gojq.Compile(parsed)
	if err != nil {
		return nil, fmt.Errorf("failed to compile jq expression: %w", err)
	}

	jqQueryCache.Store(filter, code)
	return code, nil
}

func normalizeGoJQValue(v any) any {
	switch value := v.(type) {
	case map[any]any:
		converted := make(map[string]any, len(value))
		for k, val := range value {
			converted[fmt.Sprint(k)] = normalizeGoJQValue(val)
		}
		return converted
	case []any:
		for i := range value {
			value[i] = normalizeGoJQValue(value[i])
		}
		return value
	default:
		return value
	}
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

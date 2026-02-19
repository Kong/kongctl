package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/cmd/root/products"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	configpkg "github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	apiutil "github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/helpers"
	cmdtest "github.com/kong/kongctl/test/cmd"
	configtest "github.com/kong/kongctl/test/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestApplyJQFilter(t *testing.T) {
	body := []byte(`{"foo":{"bar":1},"list":[1,2,3],"str":"value"}`)

	out, err := applyJQFilter(body, ".foo.bar")
	require.NoError(t, err)
	require.JSONEq(t, "1", string(out))

	out, err = applyJQFilter(body, ".list[1]")
	require.NoError(t, err)
	require.JSONEq(t, "2", string(out))

	out, err = applyJQFilter(body, ".str")
	require.NoError(t, err)
	require.JSONEq(t, `"value"`, string(out))

	_, err = applyJQFilter([]byte("not-json"), ".foo")
	require.Error(t, err)

	_, err = applyJQFilter(body, ".list[")
	require.Error(t, err)

	out, err = applyJQFilter(body, ".missing")
	require.NoError(t, err)
	require.JSONEq(t, "null", string(out))

	out, err = applyJQFilter(body, ".list[] | select(. > 1)")
	require.NoError(t, err)
	require.JSONEq(t, "[2,3]", string(out))
}

func TestParseAssignmentsStrings(t *testing.T) {
	payload, err := parseAssignments([]string{"foo=bar", "empty="})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"foo": "bar", "empty": ""}, payload)
}

func newMockAPIConfig() *configtest.MockConfigHook {
	return &configtest.MockConfigHook{
		GetStringMock: func(key string) string {
			switch key {
			case konnectcommon.BaseURLConfigPath:
				return "https://api.example.com"
			case konnectcommon.PATConfigPath:
				return "test-token"
			case konnectcommon.RefreshPathConfigPath:
				return "/refresh"
			case cmdcommon.OutputConfigPath:
				return "text"
			case jqoutput.ColorEnabledConfigPath:
				return cmdcommon.ColorModeAuto.String()
			case jqoutput.ColorThemeConfigPath:
				return jqoutput.DefaultTheme
			default:
				return ""
			}
		},
		GetBoolMock:        func(string) bool { return false },
		GetIntMock:         func(string) int { return 0 },
		SaveMock:           func() error { return nil },
		BindFlagMock:       func(string, *pflag.Flag) error { return nil },
		GetProfileMock:     func() string { return "default" },
		GetStringSlickMock: func(string) []string { return nil },
		SetStringMock:      func(string, string) {},
		SetMock:            func(string, any) {},
		GetMock:            func(string) any { return nil },
		GetPathMock:        func() string { return "" },
	}
}

func TestParseAssignmentsTyped(t *testing.T) {
	payload, err := parseAssignments([]string{"count:=2", "enabled:=true", "meta:={\"name\":\"test\"}"})
	require.NoError(t, err)
	require.Equal(t, float64(2), payload["count"])
	require.Equal(t, true, payload["enabled"])
	require.Equal(t, map[string]any{"name": "test"}, payload["meta"])
}

func TestParseAssignmentsNested(t *testing.T) {
	payload, err := parseAssignments([]string{
		"config.plugins[0].name=rate-limiting",
		"config.plugins[0].enabled:=true",
		"config.plugins[1].name=request-size-limiting",
		"config.plugins[1].config:={\"limit\":10}",
	})
	require.NoError(t, err)

	config, ok := payload["config"].(map[string]any)
	require.True(t, ok)

	plugins, ok := config["plugins"].([]any)
	require.True(t, ok)
	require.Len(t, plugins, 2)

	plugin0, ok := plugins[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "rate-limiting", plugin0["name"])
	require.Equal(t, true, plugin0["enabled"])

	plugin1, ok := plugins[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "request-size-limiting", plugin1["name"])

	plugin1Config, ok := plugin1["config"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(10), plugin1Config["limit"])
}

func TestParseAssignmentsNestedConflicts(t *testing.T) {
	_, err := parseAssignments([]string{"config=flat", "config.name=value"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "expects object")
}

func TestResolveMethodAndArgs(t *testing.T) {
	testCases := []struct {
		name       string
		args       []string
		expectMeth string
		allowBody  bool
		remaining  []string
	}{
		{
			name:       "default to get when no args",
			args:       nil,
			expectMeth: http.MethodGet,
			allowBody:  false,
			remaining:  nil,
		},
		{
			name:       "default to get when endpoint only",
			args:       []string{"/v1/resources"},
			expectMeth: http.MethodGet,
			allowBody:  false,
			remaining:  []string{"/v1/resources"},
		},
		{
			name:       "post mixed case",
			args:       []string{"PoSt", "/v1/resources"},
			expectMeth: http.MethodPost,
			allowBody:  true,
			remaining:  []string{"/v1/resources"},
		},
		{
			name:       "put lowercase",
			args:       []string{"put", "/v1/resources/1"},
			expectMeth: http.MethodPut,
			allowBody:  true,
			remaining:  []string{"/v1/resources/1"},
		},
		{
			name:       "patch uppercase",
			args:       []string{"PATCH", "/v1/resources/1"},
			expectMeth: http.MethodPatch,
			allowBody:  true,
			remaining:  []string{"/v1/resources/1"},
		},
		{
			name:       "delete mixed case",
			args:       []string{"DeLeTe", "/v1/resources/1"},
			expectMeth: http.MethodDelete,
			allowBody:  false,
			remaining:  []string{"/v1/resources/1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			method, allowBody, remaining := resolveMethodAndArgs(tc.args)
			require.Equal(t, tc.expectMeth, method)
			require.Equal(t, tc.allowBody, allowBody)
			require.Equal(t, tc.remaining, remaining)
		})
	}
}

func TestParseAssignmentsInvalid(t *testing.T) {
	_, err := parseAssignments([]string{"novalue"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected key=value")

	_, err = parseAssignments([]string{":=true"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing a key")
}

func TestRunPostBuildsJSONBody(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	var (
		capturedMethod   string
		capturedEndpoint string
		capturedToken    string
		capturedBody     string
		capturedHeaders  map[string]string
	)

	requestFn = func(
		_ context.Context,
		client apiutil.Doer,
		method string,
		baseURL string,
		endpoint string,
		token string,
		headers map[string]string,
		body io.Reader,
	) (*apiutil.Result, error) {
		require.Equal(t, "https://api.example.com", baseURL)
		require.NotNil(t, client)
		capturedMethod = method
		capturedEndpoint = endpoint
		capturedToken = token
		capturedHeaders = headers
		if body != nil {
			bytes, err := io.ReadAll(body)
			require.NoError(t, err)
			capturedBody = string(bytes)
		}
		return &apiutil.Result{StatusCode: http.StatusOK, Body: []byte(`{"ok":true}`)}, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)

	cfg := newMockAPIConfig()

	args := []string{"/v1/resources", "foo=bar", "count:=2"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock:    func() context.Context { return context.Background() },
		GetVerbMock:       func() (verbs.VerbValue, error) { return verbs.API, nil },
		GetProductMock:    func() (products.ProductValue, error) { return "", nil },
		GetKonnectSDKMock: func(configpkg.Hook, *slog.Logger) (helpers.SDKAPI, error) { return nil, nil },
	}

	err := run(helper, http.MethodPost, true)
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, capturedMethod)
	require.Equal(t, "/v1/resources", capturedEndpoint)
	require.Equal(t, "test-token", capturedToken)
	require.Equal(t, "application/json", capturedHeaders["Content-Type"])
	require.JSONEq(t, `{"foo":"bar","count":2}`, capturedBody)

	outBuf := streams.Out.(*bytes.Buffer)
	require.Contains(t, strings.TrimSpace(outBuf.String()), "\"ok\"")
}

func TestRunPatchBuildsJSONBody(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	var (
		capturedMethod   string
		capturedEndpoint string
		capturedBody     string
	)

	requestFn = func(
		_ context.Context,
		client apiutil.Doer,
		method string,
		baseURL string,
		endpoint string,
		token string,
		headers map[string]string,
		body io.Reader,
	) (*apiutil.Result, error) {
		require.Equal(t, "https://api.example.com", baseURL)
		require.NotNil(t, client)
		capturedMethod = method
		capturedEndpoint = endpoint
		require.Equal(t, "application/json", headers["Content-Type"])
		bytes, err := io.ReadAll(body)
		require.NoError(t, err)
		capturedBody = string(bytes)
		require.Equal(t, "test-token", token)
		return &apiutil.Result{StatusCode: http.StatusOK, Body: []byte(`{"ok":true}`)}, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)

	cfg := newMockAPIConfig()

	args := []string{"/v1/resources/123", "name=patched"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock:    func() context.Context { return context.Background() },
		GetVerbMock:       func() (verbs.VerbValue, error) { return verbs.API, nil },
		GetProductMock:    func() (products.ProductValue, error) { return "", nil },
		GetKonnectSDKMock: func(configpkg.Hook, *slog.Logger) (helpers.SDKAPI, error) { return nil, nil },
	}

	err := run(helper, http.MethodPatch, true)
	require.NoError(t, err)
	require.Equal(t, http.MethodPatch, capturedMethod)
	require.Equal(t, "/v1/resources/123", capturedEndpoint)
	require.JSONEq(t, `{"name":"patched"}`, capturedBody)

	outBuf := streams.Out.(*bytes.Buffer)
	require.Contains(t, strings.TrimSpace(outBuf.String()), "\"ok\"")
}

func TestRunPostReadsBodyFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	bodyFile, err := os.CreateTemp(tmpDir, "body-*.json")
	require.NoError(t, err)

	content := `{"foo":"file","count":3}`
	_, err = bodyFile.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, bodyFile.Close())

	original := requestFn
	t.Cleanup(func() { requestFn = original })

	var capturedBody string
	requestFn = func(
		_ context.Context,
		client apiutil.Doer,
		method string,
		baseURL string,
		endpoint string,
		token string,
		headers map[string]string,
		body io.Reader,
	) (*apiutil.Result, error) {
		require.Equal(t, http.MethodPost, method)
		require.NotNil(t, client)
		require.Equal(t, "https://api.example.com", baseURL)
		require.Equal(t, "/v1/resources", endpoint)
		require.Equal(t, "test-token", token)
		bytes, err := io.ReadAll(body)
		require.NoError(t, err)
		capturedBody = string(bytes)
		require.Equal(t, "application/json", headers["Content-Type"])
		return &apiutil.Result{StatusCode: http.StatusCreated, Body: []byte(`{"ok":true}`)}, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)
	require.NoError(t, cmdObj.Flags().Set("body-file", bodyFile.Name()))

	cfg := newMockAPIConfig()

	args := []string{"/v1/resources"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock:    func() context.Context { return context.Background() },
		GetVerbMock:       func() (verbs.VerbValue, error) { return verbs.API, nil },
		GetProductMock:    func() (products.ProductValue, error) { return "", nil },
		GetKonnectSDKMock: func(configpkg.Hook, *slog.Logger) (helpers.SDKAPI, error) { return nil, nil },
	}

	require.NoError(t, run(helper, http.MethodPost, true))
	require.JSONEq(t, content, capturedBody)
}

func TestRunAppliesJQColorToJSONOutput(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	requestFn = func(
		_ context.Context,
		_ apiutil.Doer,
		_ string,
		_ string,
		_ string,
		_ string,
		_ map[string]string,
		_ io.Reader,
	) (*apiutil.Result, error) {
		return &apiutil.Result{StatusCode: http.StatusOK, Body: []byte(`{"foo":{"bar":1}}`)}, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)
	require.NoError(t, cmdObj.Flags().Set("jq", ".foo"))
	require.NoError(t, cmdObj.Flags().Set("jq-color", "always"))
	require.NoError(t, cmdObj.Flags().Set("jq-color-theme", "github"))

	cfg := &configtest.MockConfigHook{
		GetStringMock: func(key string) string {
			switch key {
			case konnectcommon.BaseURLConfigPath:
				return "https://api.example.com"
			case konnectcommon.PATConfigPath:
				return "test-token"
			case cmdcommon.OutputConfigPath:
				return "json"
			case jqoutput.ColorEnabledConfigPath:
				return cmdcommon.ColorModeAlways.String()
			case jqoutput.ColorThemeConfigPath:
				return jqoutput.DefaultTheme
			default:
				return ""
			}
		},
		GetBoolMock:        func(string) bool { return false },
		GetIntMock:         func(string) int { return 0 },
		SaveMock:           func() error { return nil },
		BindFlagMock:       func(string, *pflag.Flag) error { return nil },
		GetProfileMock:     func() string { return "default" },
		GetStringSlickMock: func(string) []string { return nil },
		SetStringMock:      func(string, string) {},
		SetMock:            func(string, any) {},
		GetMock:            func(string) any { return nil },
		GetPathMock:        func() string { return "" },
	}

	args := []string{"/v1/resources"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	require.NoError(t, run(helper, http.MethodGet, false))
	output := streams.Out.(*bytes.Buffer).String()
	require.Contains(t, output, "\x1b[")
}

func TestRunLoadsJQColorSettingsFromConfig(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	requestFn = func(
		_ context.Context,
		_ apiutil.Doer,
		_ string,
		_ string,
		_ string,
		_ string,
		_ map[string]string,
		_ io.Reader,
	) (*apiutil.Result, error) {
		return &apiutil.Result{StatusCode: http.StatusOK, Body: []byte(`{"foo":{"bar":1}}`)}, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)
	require.NoError(t, cmdObj.Flags().Set("jq", ".foo"))

	cfg := &configtest.MockConfigHook{
		GetStringMock: func(key string) string {
			switch key {
			case konnectcommon.BaseURLConfigPath:
				return "https://api.example.com"
			case konnectcommon.PATConfigPath:
				return "test-token"
			case cmdcommon.OutputConfigPath:
				return "json"
			case jqoutput.ColorEnabledConfigPath:
				return cmdcommon.ColorModeAlways.String()
			case jqoutput.ColorThemeConfigPath:
				return "monokai"
			default:
				return ""
			}
		},
		GetBoolMock:        func(string) bool { return false },
		GetIntMock:         func(string) int { return 0 },
		SaveMock:           func() error { return nil },
		BindFlagMock:       func(string, *pflag.Flag) error { return nil },
		GetProfileMock:     func() string { return "default" },
		GetStringSlickMock: func(string) []string { return nil },
		SetStringMock:      func(string, string) {},
		SetMock:            func(string, any) {},
		GetMock:            func(string) any { return nil },
		GetPathMock:        func() string { return "" },
	}

	args := []string{"/v1/resources"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	require.NoError(t, run(helper, http.MethodGet, false))
	output := streams.Out.(*bytes.Buffer).String()
	require.Contains(t, output, "\x1b[")
}

func TestRunUsesJQDefaultExpressionFromConfig(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	requestFn = func(
		_ context.Context,
		_ apiutil.Doer,
		_ string,
		_ string,
		_ string,
		_ string,
		_ map[string]string,
		_ io.Reader,
	) (*apiutil.Result, error) {
		return &apiutil.Result{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"foo":{"bar":1},"other":{"name":"keep-out"}}`),
		}, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)

	cfg := &configtest.MockConfigHook{
		GetStringMock: func(key string) string {
			switch key {
			case konnectcommon.BaseURLConfigPath:
				return "https://api.example.com"
			case konnectcommon.PATConfigPath:
				return "test-token"
			case cmdcommon.OutputConfigPath:
				return "json"
			case jqoutput.ColorEnabledConfigPath:
				return cmdcommon.ColorModeNever.String()
			case jqoutput.ColorThemeConfigPath:
				return jqoutput.DefaultTheme
			case jqoutput.DefaultExpressionConfigPath:
				return ".foo"
			default:
				return ""
			}
		},
		GetBoolMock:        func(string) bool { return false },
		GetIntMock:         func(string) int { return 0 },
		SaveMock:           func() error { return nil },
		BindFlagMock:       func(string, *pflag.Flag) error { return nil },
		GetProfileMock:     func() string { return "default" },
		GetStringSlickMock: func(string) []string { return nil },
		SetStringMock:      func(string, string) {},
		SetMock:            func(string, any) {},
		GetMock:            func(string) any { return nil },
		GetPathMock:        func() string { return "" },
	}

	args := []string{"/v1/resources"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	require.NoError(t, run(helper, http.MethodGet, false))
	output := streams.Out.(*bytes.Buffer).String()
	require.Contains(t, output, "\"bar\": 1")
	require.NotContains(t, output, "keep-out")
}

func TestAddFlagsSupportsJQRawOutputShortFlag(t *testing.T) {
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)

	require.NoError(t, cmdObj.Flags().Parse([]string{"-r"}))
	rawEnabled, err := cmdObj.Flags().GetBool(jqoutput.RawOutputFlagName)
	require.NoError(t, err)
	require.True(t, rawEnabled)
}

func TestRunAppliesJQRawOutput(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	requestFn = func(
		_ context.Context,
		_ apiutil.Doer,
		_ string,
		_ string,
		_ string,
		_ string,
		_ map[string]string,
		_ io.Reader,
	) (*apiutil.Result, error) {
		return &apiutil.Result{StatusCode: http.StatusOK, Body: []byte(`{"foo":"example-api"}`)}, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)
	require.NoError(t, cmdObj.Flags().Set("jq", ".foo"))
	require.NoError(t, cmdObj.Flags().Parse([]string{"-r"}))

	cfg := &configtest.MockConfigHook{
		GetStringMock: func(key string) string {
			switch key {
			case konnectcommon.BaseURLConfigPath:
				return "https://api.example.com"
			case konnectcommon.PATConfigPath:
				return "test-token"
			case cmdcommon.OutputConfigPath:
				return "json"
			case jqoutput.ColorEnabledConfigPath:
				return cmdcommon.ColorModeNever.String()
			case jqoutput.ColorThemeConfigPath:
				return jqoutput.DefaultTheme
			default:
				return ""
			}
		},
		GetBoolMock: func(key string) bool {
			return key == jqoutput.RawOutputConfigPath
		},
		GetIntMock:         func(string) int { return 0 },
		SaveMock:           func() error { return nil },
		BindFlagMock:       func(string, *pflag.Flag) error { return nil },
		GetProfileMock:     func() string { return "default" },
		GetStringSlickMock: func(string) []string { return nil },
		SetStringMock:      func(string, string) {},
		SetMock:            func(string, any) {},
		GetMock:            func(string) any { return nil },
		GetPathMock:        func() string { return "" },
	}

	args := []string{"/v1/resources"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	require.NoError(t, run(helper, http.MethodGet, false))
	require.Equal(t, "example-api\n", streams.Out.(*bytes.Buffer).String())
}

func TestRunRejectsJQRawOutputWithoutJQ(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	requestFn = func(
		_ context.Context,
		_ apiutil.Doer,
		_ string,
		_ string,
		_ string,
		_ string,
		_ map[string]string,
		_ io.Reader,
	) (*apiutil.Result, error) {
		require.Fail(t, "requestFn should not be called when jq raw output is invalid")
		return nil, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)
	require.NoError(t, cmdObj.Flags().Parse([]string{"-r"}))

	cfg := &configtest.MockConfigHook{
		GetStringMock: func(key string) string {
			switch key {
			case konnectcommon.BaseURLConfigPath:
				return "https://api.example.com"
			case konnectcommon.PATConfigPath:
				return "test-token"
			case cmdcommon.OutputConfigPath:
				return "json"
			case jqoutput.ColorEnabledConfigPath:
				return cmdcommon.ColorModeNever.String()
			case jqoutput.ColorThemeConfigPath:
				return jqoutput.DefaultTheme
			default:
				return ""
			}
		},
		GetBoolMock: func(key string) bool {
			return key == jqoutput.RawOutputConfigPath
		},
		GetIntMock:         func(string) int { return 0 },
		SaveMock:           func() error { return nil },
		BindFlagMock:       func(string, *pflag.Flag) error { return nil },
		GetProfileMock:     func() string { return "default" },
		GetStringSlickMock: func(string) []string { return nil },
		SetStringMock:      func(string, string) {},
		SetMock:            func(string, any) {},
		GetMock:            func(string) any { return nil },
		GetPathMock:        func() string { return "" },
	}

	args := []string{"/v1/resources"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	err := run(helper, http.MethodGet, false)
	require.Error(t, err)
	var cfgErr *cmdpkg.ConfigurationError
	require.True(t, errors.As(err, &cfgErr))
	require.Contains(t, err.Error(), "--jq")
}

func TestRunRejectsTextOutputFormat(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })
	requestFn = func(
		context.Context,
		apiutil.Doer,
		string,
		string,
		string,
		string,
		map[string]string,
		io.Reader,
	) (*apiutil.Result, error) {
		require.Fail(t, "requestFn should not be called when output format is rejected")
		return nil, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)

	cfg := newMockAPIConfig()

	args := []string{"/v1/resources"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.TEXT, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	err := run(helper, http.MethodGet, false)
	require.Error(t, err)
	var cfgErr *cmdpkg.ConfigurationError
	require.True(t, errors.As(err, &cfgErr))
	require.Contains(t, err.Error(), "json or yaml")
}

func TestShouldUseJQColorModes(t *testing.T) {
	origNoColor, hadNoColor := os.LookupEnv("NO_COLOR")
	_ = os.Unsetenv("NO_COLOR")
	t.Cleanup(func() {
		if hadNoColor {
			_ = os.Setenv("NO_COLOR", origNoColor)
			return
		}
		_ = os.Unsetenv("NO_COLOR")
	})

	tty := &ttyBuffer{}
	require.True(t, shouldUseJQColor(cmdcommon.ColorModeAlways, tty))
	require.False(t, shouldUseJQColor(cmdcommon.ColorModeNever, tty))

	originalDetector := jqTerminalDetector
	jqTerminalDetector = func(uintptr) bool { return true }
	t.Cleanup(func() { jqTerminalDetector = originalDetector })

	require.True(t, shouldUseJQColor(cmdcommon.ColorModeAuto, tty))
}

func TestShouldUseJQColorHonorsNoColor(t *testing.T) {
	originalDetector := jqTerminalDetector
	jqTerminalDetector = func(uintptr) bool { return true }
	t.Cleanup(func() { jqTerminalDetector = originalDetector })
	t.Setenv("NO_COLOR", "1")

	tty := &ttyBuffer{}
	require.False(t, shouldUseJQColor(cmdcommon.ColorModeAuto, tty))
}

func TestMaybeColorizeJQOutputUsesFallbackPalette(t *testing.T) {
	raw := []byte(`{"foo":{"bar":1}}`)
	formatted := bodyToPrintable(raw)
	colored := maybeColorizeJQOutput(raw, formatted, "not-a-style")
	require.Contains(t, colored, "\x1b[")
}

func TestRunPostBodyFileConflictsWithFields(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	requestFn = func(
		_ context.Context,
		_ apiutil.Doer,
		_ string,
		_ string,
		_ string,
		_ string,
		_ map[string]string,
		_ io.Reader,
	) (*apiutil.Result, error) {
		require.Fail(t, "requestFn should not be invoked when inputs conflict")
		return nil, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)
	require.NoError(t, cmdObj.Flags().Set("body-file", "payload.json"))

	cfg := newMockAPIConfig()

	args := []string{"/v1/resources", "foo=bar"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:     func() *cobra.Command { return cmdObj },
		GetArgsMock:    func() []string { return args },
		GetStreamsMock: func() *iostreams.IOStreams { return streams },
		GetConfigMock: func() (configpkg.Hook, error) {
			return cfg, nil
		},
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	err := run(helper, http.MethodPost, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot combine --body-file with inline field assignments")
}

func TestRunPostReadsBodyFromStdin(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	var captured string
	requestFn = func(
		_ context.Context,
		_ apiutil.Doer,
		_ string,
		_ string,
		_ string,
		_ string,
		_ map[string]string,
		body io.Reader,
	) (*apiutil.Result, error) {
		bytes, err := io.ReadAll(body)
		require.NoError(t, err)
		captured = string(bytes)
		return &apiutil.Result{StatusCode: http.StatusOK, Body: []byte(`{"ok":true}`)}, nil
	}

	stdin := &closableBuffer{Buffer: bytes.NewBufferString(`{"foo":"stdin"}`)}
	streams := &iostreams.IOStreams{In: stdin, Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)
	require.NoError(t, cmdObj.Flags().Set("body-file", "-"))

	cfg := newMockAPIConfig()

	args := []string{"/v1/resources"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	require.NoError(t, run(helper, http.MethodPost, true))
	require.JSONEq(t, `{"foo":"stdin"}`, captured)
	require.True(t, stdin.closed)
}

func TestRunReturnsStatusDetailsOnError(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	requestFn = func(
		context.Context,
		apiutil.Doer,
		string,
		string,
		string,
		string,
		map[string]string,
		io.Reader,
	) (*apiutil.Result, error) {
		return &apiutil.Result{
			StatusCode: http.StatusNotFound,
			Body:       []byte(`{"message":"Cannot POST"}`),
		}, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)

	cfg := newMockAPIConfig()
	args := []string{"/v1/resources"}

	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	err := run(helper, http.MethodGet, false)
	require.Error(t, err)
	var execErr *cmdpkg.ExecutionError
	require.True(t, errors.As(err, &execErr))
	require.Contains(t, execErr.Msg, "status 404")
	require.Contains(t, execErr.Err.Error(), "Cannot POST")

	attrMap := attrsToMap(t, execErr.Attrs)
	require.Equal(t, http.StatusNotFound, attrMap["status"])
	require.Equal(t, http.MethodGet, attrMap["method"])
	require.Equal(t, "/v1/resources", attrMap["endpoint"])
	require.Contains(t, attrMap["response"], "Cannot POST")
}

func TestRunIncludesHeadersWhenRequested(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	requestFn = func(
		context.Context,
		apiutil.Doer,
		string,
		string,
		string,
		string,
		map[string]string,
		io.Reader,
	) (*apiutil.Result, error) {
		return &apiutil.Result{
			StatusCode: http.StatusBadRequest,
			Body:       []byte(`{"message":"bad request"}`),
			Header: http.Header{
				"X-Request-Id": {"abc123"},
			},
		}, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)
	require.NoError(t, cmdObj.Flags().Set(responseHeadersFlagName, "true"))

	cfg := newMockAPIConfig()
	args := []string{"/v1/resources"}

	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	err := run(helper, http.MethodGet, false)
	require.Error(t, err)
	var execErr *cmdpkg.ExecutionError
	require.True(t, errors.As(err, &execErr))

	attrMap := attrsToMap(t, execErr.Attrs)
	require.Equal(t, http.StatusBadRequest, attrMap["status"])
	require.Equal(t, "/v1/resources", attrMap["endpoint"])
	require.Equal(t, http.Header{"X-Request-Id": {"abc123"}}, attrMap["headers"])
}

func TestRunPostBodyFileFromInteractiveStdinFails(t *testing.T) {
	originalRequest := requestFn
	t.Cleanup(func() { requestFn = originalRequest })

	requestFn = func(
		_ context.Context,
		_ apiutil.Doer,
		_ string,
		_ string,
		_ string,
		_ string,
		_ map[string]string,
		_ io.Reader,
	) (*apiutil.Result, error) {
		require.Fail(t, "requestFn should not be called when stdin is interactive")
		return nil, nil
	}

	originalIsTerminal := isTerminalFile
	isTerminalFile = func(uintptr) bool { return true }
	t.Cleanup(func() { isTerminalFile = originalIsTerminal })

	stdin := &fakeTTYReader{Reader: bytes.NewBuffer(nil)}
	streams := &iostreams.IOStreams{In: stdin, Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)
	require.NoError(t, cmdObj.Flags().Set("body-file", "-"))

	cfg := newMockAPIConfig()

	args := []string{"/v1/resources"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	err := run(helper, http.MethodPost, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "standard input is a terminal")
}

func TestRunBodyFileRejectedForGet(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	requestFn = func(
		_ context.Context,
		_ apiutil.Doer,
		_ string,
		_ string,
		_ string,
		_ string,
		_ map[string]string,
		_ io.Reader,
	) (*apiutil.Result, error) {
		require.Fail(t, "requestFn should not be called for GET body")
		return nil, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)
	require.NoError(t, cmdObj.Flags().Set("body-file", "payload.json"))

	cfg := newMockAPIConfig()

	args := []string{"/v1/resources"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock: func() context.Context { return context.Background() },
	}

	err := run(helper, http.MethodGet, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "request body is not allowed")
}

func TestSingleBodyFileValueRejectsMultiple(t *testing.T) {
	var target string
	val := newSingleBodyFileValue(&target)
	require.NoError(t, val.Set("first.json"))
	require.Equal(t, "first.json", target)
	require.EqualError(t, val.Set("second.json"), "--body-file may only be provided once")
}

type closableBuffer struct {
	*bytes.Buffer
	closed bool
}

func (c *closableBuffer) Close() error {
	c.closed = true
	return nil
}

type ttyBuffer struct {
	bytes.Buffer
}

func (t *ttyBuffer) Fd() uintptr { return 1 }

type fakeTTYReader struct {
	io.Reader
	closed bool
}

func (f *fakeTTYReader) Close() error {
	f.closed = true
	return nil
}

func (f *fakeTTYReader) Fd() uintptr { return 0 }

func attrsToMap(t *testing.T, attrs []any) map[string]any {
	t.Helper()
	require.Zero(t, len(attrs)%2, "attrs slice must contain key/value pairs")
	result := make(map[string]any, len(attrs)/2)
	for i := 0; i < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		require.True(t, ok, "attr keys must be strings")
		result[key] = attrs[i+1]
	}
	return result
}

func TestRunRejectsUnexpectedPayload(t *testing.T) {
	original := requestFn
	t.Cleanup(func() { requestFn = original })

	requestFn = func(
		context.Context,
		apiutil.Doer,
		string,
		string,
		string,
		string,
		map[string]string,
		io.Reader,
	) (*apiutil.Result, error) {
		require.Fail(t, "requestFn should not be called for invalid payloads")
		return nil, nil
	}

	streams := iostreams.NewTestIOStreamsOnly()
	cmdObj := &cobra.Command{Use: "test"}
	addFlags(cmdObj)

	cfg := newMockAPIConfig()

	args := []string{"/v3/users/me", "foo=bar"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.JSON, nil },
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})), nil
		},
		GetContextMock:    func() context.Context { return context.Background() },
		GetVerbMock:       func() (verbs.VerbValue, error) { return verbs.API, nil },
		GetProductMock:    func() (products.ProductValue, error) { return "", nil },
		GetKonnectSDKMock: func(configpkg.Hook, *slog.Logger) (helpers.SDKAPI, error) { return nil, nil },
	}

	err := run(helper, http.MethodGet, false)
	require.Error(t, err)
	var execErr *cmdpkg.ExecutionError
	require.True(t, errors.As(err, &execErr))
	require.Contains(t, err.Error(), "data fields may only be supplied with POST, PUT, or PATCH")
}

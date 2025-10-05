package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
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

	_, err = applyJQFilter(body, ".missing")
	require.Error(t, err)
}

func TestParseAssignmentsStrings(t *testing.T) {
	payload, err := parseAssignments([]string{"foo=bar", "empty="})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"foo": "bar", "empty": ""}, payload)
}

func TestParseAssignmentsTyped(t *testing.T) {
	payload, err := parseAssignments([]string{"count:=2", "enabled:=true", "meta:={\"name\":\"test\"}"})
	require.NoError(t, err)
	require.Equal(t, float64(2), payload["count"])
	require.Equal(t, true, payload["enabled"])
	require.Equal(t, map[string]any{"name": "test"}, payload["meta"])
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
	cmdObj.Flags().String("jq", "", "")

	cfg := &configtest.MockConfigHook{
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

	args := []string{"/v1/resources", "foo=bar", "count:=2"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.TEXT, nil },
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
	cmdObj.Flags().String("jq", "", "")

	cfg := &configtest.MockConfigHook{
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

	args := []string{"/v1/me", "foo=bar"}
	helper := &cmdtest.MockHelper{
		GetCmdMock:          func() *cobra.Command { return cmdObj },
		GetArgsMock:         func() []string { return args },
		GetStreamsMock:      func() *iostreams.IOStreams { return streams },
		GetConfigMock:       func() (configpkg.Hook, error) { return cfg, nil },
		GetOutputFormatMock: func() (cmdcommon.OutputFormat, error) { return cmdcommon.TEXT, nil },
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
	require.Contains(t, err.Error(), "data fields may only be supplied with POST or PUT")
}

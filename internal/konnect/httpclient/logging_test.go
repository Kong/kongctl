package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestLoggingHTTPClient_DebugLogsRequestAndResponseWithoutBodies(t *testing.T) {
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body:    io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Request: req,
			}, nil
		}),
	}

	loggingClient := NewLoggingHTTPClientWithClient(client, logger)
	req, err := http.NewRequest(http.MethodGet, "https://us.api.konghq.com/v2/control-planes?page=2&token=secret", nil)
	require.NoError(t, err)

	resp, err := loggingClient.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	logs := parseJSONLogs(t, logOutput.String())
	require.Len(t, logs, 2)

	requestLog := mustFindLogByType(t, logs, logTypeRequest)
	responseLog := mustFindLogByType(t, logs, logTypeResponse)

	assert.Equal(t, "GET", requestLog["method"])
	assert.Equal(t, "/v2/control-planes", requestLog["route"])

	queryValues, ok := requestLog["query_params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2", queryValues["page"])
	assert.Equal(t, redactedValue, queryValues["token"])

	assert.NotContains(t, requestLog, "request_body")
	assert.NotContains(t, responseLog, "response_body")
	assert.Equal(t, requestLog["request_id"], responseLog["request_id"])
	assert.EqualValues(t, 200, int(responseLog["status_code"].(float64)))
}

func TestLoggingHTTPClient_TraceLogsBodiesAndRedactsSensitiveFields(t *testing.T) {
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, &slog.HandlerOptions{
		Level: log.LevelTrace,
	}))

	requestBody := `{"name":"demo","password":"super-secret","nested":{"api_key":"key-value"}}`
	responseBody := `{"id":"123","token":"response-secret"}`

	var requestBodySeenByTransport string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			bodyBytes, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			requestBodySeenByTransport = string(bodyBytes)

			return &http.Response{
				StatusCode: http.StatusCreated,
				Status:     "201 Created",
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"Set-Cookie":   []string{"session=abc123"},
				},
				Body:    io.NopCloser(strings.NewReader(responseBody)),
				Request: req,
			}, nil
		}),
	}

	loggingClient := NewLoggingHTTPClientWithClient(client, logger)
	req, err := http.NewRequest(
		http.MethodPost,
		"https://us.api.konghq.com/v2/control-planes",
		strings.NewReader(requestBody),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer definitely-secret")

	resp, err := loggingClient.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, requestBody, requestBodySeenByTransport)

	responseBodyRead, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, responseBody, string(responseBodyRead))

	logs := parseJSONLogs(t, logOutput.String())
	require.Len(t, logs, 2)

	requestLog := mustFindLogByType(t, logs, logTypeRequest)
	responseLog := mustFindLogByType(t, logs, logTypeResponse)

	requestLoggedBody, ok := requestLog["request_body"].(string)
	require.True(t, ok)
	assert.Contains(t, requestLoggedBody, `"password":"`+redactedValue+`"`)
	assert.Contains(t, requestLoggedBody, `"api_key":"`+redactedValue+`"`)
	assert.NotContains(t, requestLoggedBody, "super-secret")

	requestHeaders, ok := requestLog["request_headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, redactedValue, requestHeaders["Authorization"])

	responseLoggedBody, ok := responseLog["response_body"].(string)
	require.True(t, ok)
	assert.Contains(t, responseLoggedBody, `"token":"`+redactedValue+`"`)
	assert.NotContains(t, responseLoggedBody, "response-secret")

	responseHeaders, ok := responseLog["response_headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, redactedValue, responseHeaders["Set-Cookie"])
}

func TestLoggingHTTPClient_NoRequestResponseLogsBelowDebug(t *testing.T) {
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Status:     "204 No Content",
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		}),
	}

	loggingClient := NewLoggingHTTPClientWithClient(client, logger)
	req, err := http.NewRequest(http.MethodGet, "https://us.api.konghq.com/v2/control-planes", nil)
	require.NoError(t, err)

	resp, err := loggingClient.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, strings.TrimSpace(logOutput.String()))
}

func TestLoggingHTTPClient_IncludesWorkflowContextFields(t *testing.T) {
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Request:    req,
			}, nil
		}),
	}

	loggingClient := NewLoggingHTTPClientWithClient(client, logger)
	ctx := log.WithHTTPLogContext(context.Background(), log.HTTPLogContext{
		CommandPath:       "kongctl apply",
		CommandVerb:       "apply",
		CommandMode:       "apply",
		CommandProduct:    "konnect",
		Workflow:          "declarative",
		WorkflowPhase:     "executor",
		WorkflowComponent: "portal",
		WorkflowMode:      "apply",
		WorkflowNamespace: "default",
		WorkflowAction:    "create",
		WorkflowChangeID:  "1:c:portal:example",
		WorkflowResource:  "portal",
		WorkflowRef:       "example",
		SDKOperationID:    "listPortals",
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://us.api.konghq.com/v2/portals", nil)
	require.NoError(t, err)

	resp, err := loggingClient.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	logs := parseJSONLogs(t, logOutput.String())
	require.Len(t, logs, 2)

	requestLog := mustFindLogByType(t, logs, logTypeRequest)
	responseLog := mustFindLogByType(t, logs, logTypeResponse)

	assert.Equal(t, "kongctl apply", requestLog["command_path"])
	assert.Equal(t, "apply", requestLog["command_mode"])
	assert.Equal(t, "declarative", requestLog["workflow"])
	assert.Equal(t, "executor", requestLog["workflow_phase"])
	assert.Equal(t, "portal", requestLog["workflow_component"])
	assert.Equal(t, "default", requestLog["workflow_namespace"])
	assert.Equal(t, "create", requestLog["workflow_action"])
	assert.Equal(t, "1:c:portal:example", requestLog["workflow_change_id"])
	assert.Equal(t, "portal", requestLog["workflow_resource"])
	assert.Equal(t, "example", requestLog["workflow_ref"])
	assert.Equal(t, "listPortals", requestLog["sdk_operation_id"])

	assert.Equal(t, requestLog["workflow_phase"], responseLog["workflow_phase"])
	assert.Equal(t, requestLog["workflow_component"], responseLog["workflow_component"])
}

func TestLoggingHTTPClient_LogsRequestError(t *testing.T) {
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial tcp: i/o timeout")
		}),
	}

	loggingClient := NewLoggingHTTPClientWithClient(client, logger)
	req, err := http.NewRequest(http.MethodGet, "https://us.api.konghq.com/v2/portals", nil)
	require.NoError(t, err)

	resp, err := loggingClient.Do(req)
	require.Error(t, err)
	require.Nil(t, resp)

	logs := parseJSONLogs(t, logOutput.String())
	require.Len(t, logs, 2)

	requestLog := mustFindLogByType(t, logs, logTypeRequest)
	errorLog := mustFindLogByType(t, logs, logTypeError)

	assert.Equal(t, requestLog["request_id"], errorLog["request_id"])
	errorValue, ok := errorLog["error"].(string)
	require.True(t, ok)
	assert.Contains(t, errorValue, "dial tcp: i/o timeout")
	assert.Equal(t, "/v2/portals", errorLog["route"])
	assert.Contains(t, errorLog, "duration")
}

func TestLoggingHTTPClient_TraceTruncatesLargeBody(t *testing.T) {
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, &slog.HandlerOptions{
		Level: log.LevelTrace,
	}))

	largeBody := strings.Repeat("a", maxBodyLogChars+128)
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    req,
			}, nil
		}),
	}

	loggingClient := NewLoggingHTTPClientWithClient(client, logger)
	req, err := http.NewRequest(
		http.MethodPost,
		"https://us.api.konghq.com/v2/control-planes",
		strings.NewReader(largeBody),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := loggingClient.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	logs := parseJSONLogs(t, logOutput.String())
	requestLog := mustFindLogByType(t, logs, logTypeRequest)
	requestBody, ok := requestLog["request_body"].(string)
	require.True(t, ok)
	assert.Contains(t, requestBody, "[truncated, total")
}

func TestLoggingHTTPClient_NilLoggerBypassesLogging(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    req,
			}, nil
		}),
	}

	loggingClient := NewLoggingHTTPClientWithClient(client, nil)
	req, err := http.NewRequest(http.MethodGet, "https://us.api.konghq.com/v2/portals", nil)
	require.NoError(t, err)

	resp, err := loggingClient.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestLoggingHTTPClient_TraceSetsGetBodyForReplay(t *testing.T) {
	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, &slog.HandlerOptions{
		Level: log.LevelTrace,
	}))

	payload := `{"name":"example"}`
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.NotNil(t, req.GetBody, "GetBody should be populated for replay/retries")

			replayReader, err := req.GetBody()
			require.NoError(t, err)
			replayBytes, err := io.ReadAll(replayReader)
			require.NoError(t, err)
			assert.Equal(t, payload, string(replayBytes))

			bodyBytes, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Equal(t, payload, string(bodyBytes))

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    req,
			}, nil
		}),
	}

	loggingClient := NewLoggingHTTPClientWithClient(client, logger)
	req, err := http.NewRequest(http.MethodPost, "https://us.api.konghq.com/v2/portals", strings.NewReader(payload))
	require.NoError(t, err)
	req.GetBody = nil // Ensure logging layer is the one that restores this.
	req.Header.Set("Content-Type", "application/json")

	resp, err := loggingClient.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestRedactBody_NonTextualBody(t *testing.T) {
	redacted := redactBody([]byte{0x00, 0x01, 0xff, 0x10}, "application/octet-stream")
	assert.Equal(t, fmt.Sprintf("%s (%d bytes)", omittedBinaryMessage, 4), redacted)
}

func TestRedactBody_FormURLEncoded(t *testing.T) {
	redacted := redactBody(
		[]byte("username=demo&password=secret&token_count=10&token_type=Bearer"),
		"application/x-www-form-urlencoded",
	)

	assert.Contains(t, redacted, "username=demo")
	assert.Contains(t, redacted, "password=%5BREDACTED%5D")
	assert.Contains(t, redacted, "token_count=10")
	assert.Contains(t, redacted, "token_type=Bearer")
}

func TestRedactBody_MalformedJSONFallsBackToPlainTextRedaction(t *testing.T) {
	redacted := redactBody([]byte(`{"token":"secret-token"`), "application/json")
	assert.Contains(t, redacted, `"token":"`+redactedValue+`"`)
	assert.NotContains(t, redacted, "secret-token")
}

func TestSanitizeQuery_DoesNotRedactTokenMetadataFields(t *testing.T) {
	query := url.Values{
		"access_token": {"secret"},
		"token_count":  {"12"},
		"token_type":   {"Bearer"},
	}

	sanitized := sanitizeQuery(query)
	assert.Equal(t, redactedValue, sanitized["access_token"])
	assert.Equal(t, "12", sanitized["token_count"])
	assert.Equal(t, "Bearer", sanitized["token_type"])
}

func TestRedactPlainTextBody_BearerPatternRequiresMinimumLength(t *testing.T) {
	short := redactPlainTextBody("bearer abc")
	assert.Equal(t, "bearer abc", short)

	long := redactPlainTextBody("bearer abcdefghijklmnop")
	assert.Equal(t, "bearer "+redactedValue, long)
	assert.NotContains(t, long, "abcdefghijklmnop")
}

func TestIsSensitiveFieldKey_CamelCaseTokenFields(t *testing.T) {
	assert.True(t, isSensitiveFieldKey("accessToken"))
	assert.True(t, isSensitiveFieldKey("refreshToken"))
	assert.False(t, isSensitiveFieldKey("tokenType"))
	assert.False(t, isSensitiveFieldKey("tokenCount"))
}

func parseJSONLogs(t *testing.T, raw string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	results := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &payload))
		results = append(results, payload)
	}
	return results
}

func mustFindLogByType(t *testing.T, logs []map[string]any, logType string) map[string]any {
	t.Helper()
	for _, entry := range logs {
		if entry["log_type"] == logType {
			return entry
		}
	}
	t.Fatalf("log type %q not found", logType)
	return nil
}

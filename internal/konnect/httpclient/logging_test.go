package httpclient

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
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

package ask

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestChatAggregatesResponse(t *testing.T) {
	require := require.New(t)

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(http.MethodPost, req.Method)
			require.Equal("/v1/chat", req.URL.Path)
			require.Equal("Bearer test-token", req.Header.Get("Authorization"))
			require.Equal("text/event-stream", req.Header.Get("Accept"))

			body := strings.Join([]string{
				"event: llm-response",
				"data: {\"data\": \"Hello\"}",
				"",
				"event: tool-response",
				"data: {\"data\": {\"foo\": \"bar\"}}",
				"",
				"event: llm-response",
				"data: {\"data\": \" World\"}",
				"",
				"event: end",
				"data: {\"data\": \"EOF\"}",
				"",
			}, "\n")

			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}
			resp.Header.Set("Content-Type", "text/event-stream")
			return resp, nil
		}),
	}

	result, err := Chat(context.Background(), client, "https://example.com", "test-token", "hello")
	require.NoError(err)
	require.Equal("Hello World", result.Response)
	require.Len(result.Events, 4)
}

func TestChatReturnsStreamErrorEvent(t *testing.T) {
	require := require.New(t)

	client := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			body := strings.Join([]string{
				"event: error",
				"data: {\"error\": \"bad things\"}",
				"",
				"event: end",
				"",
			}, "\n")
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}
			resp.Header.Set("Content-Type", "text/event-stream")
			return resp, nil
		}),
	}

	_, err := Chat(context.Background(), client, "https://example.com", "tok", "hi")
	require.Error(err)
	require.Contains(err.Error(), "bad things")
}

func TestChatHandlesHTTPError(t *testing.T) {
	require := require.New(t)

	client := &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader("{\"detail\": \"nope\"}")),
			}, nil
		}),
	}

	_, err := Chat(context.Background(), client, "https://example.com", "tok", "hi")
	require.Error(err)
	require.Contains(err.Error(), "401")
	require.Contains(err.Error(), "nope")
}

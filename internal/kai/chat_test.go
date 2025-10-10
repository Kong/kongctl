package kai

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

func TestChatStreamSessionRequiresID(t *testing.T) {
	require := require.New(t)

	_, err := ChatStreamSession(context.Background(), nil, "https://example.com", "tok", "", "hello", nil)
	require.Error(err)
}

func TestChatStreamSessionIncludesContext(t *testing.T) {
	require := require.New(t)

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(http.MethodPost, req.Method)
			require.Equal("/v1/sessions/session-id/chat", req.URL.Path)

			payload, err := io.ReadAll(req.Body)
			require.NoError(err)
			req.Body.Close()

			require.Contains(string(payload), `"prompt":"hello"`)
			require.Contains(string(payload), `"context"`)
			require.Contains(string(payload), `"control_plane"`)
			require.Contains(string(payload), `"123"`)

			body := strings.Join([]string{
				`event: llm-response`,
				`data: {"data": "chunk"}`,
				"",
				`event: end`,
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

	stream, err := ChatStreamSession(
		context.Background(),
		client,
		"https://example.com",
		"tok",
		"session-id",
		"hello",
		[]ChatContext{{Type: ContextTypeControlPlane, ID: "123"}},
	)
	require.NoError(err)

	for evt := range stream.Events {
		_ = evt
	}
	require.NoError(stream.Err())
}

func TestListSessionsDecodesResponse(t *testing.T) {
	require := require.New(t)

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(http.MethodGet, req.Method)
			require.Equal("/v1/sessions", req.URL.Path)

			body := `[{"id":"1","name":"test","created_at":"2024-09-01T10:00:00Z"}]`
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}),
	}

	sessions, err := ListSessions(context.Background(), client, "https://example.com", "tok")
	require.NoError(err)
	require.Len(sessions, 1)
	require.Equal("test", sessions[0].Name)
}

func TestChatStreamEmitsEvents(t *testing.T) {
	require := require.New(t)

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(http.MethodPost, req.Method)
			require.Equal("/v1/chat", req.URL.Path)

			body := strings.Join([]string{
				`event: llm-response`,
				`data: {"data": "chunk"}`,
				"",
				`event: end`,
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

	stream, err := ChatStream(context.Background(), client, "https://example.com", "tok", "prompt")
	require.NoError(err)

	var events []ChatEvent
	for evt := range stream.Events {
		events = append(events, evt)
	}

	require.NoError(stream.Err())
	require.Len(events, 2)
	require.Equal("llm-response", events[0].Event)
	require.Equal("end", events[1].Event)
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

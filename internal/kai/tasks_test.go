package kai

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStreamTaskStatusMarksTransientError(t *testing.T) {
	require := require.New(t)

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(http.MethodGet, req.Method)
			require.Equal("/v1/sessions/session-123/tasks/task-abc/status", req.URL.Path)
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(errReader{}),
			}
			resp.Header.Set("Content-Type", "text/event-stream")
			return resp, nil
		}),
	}

	stream, err := StreamTaskStatus(
		context.Background(),
		client,
		"https://example.com",
		"tok",
		"session-123",
		"task-abc",
	)
	require.NoError(err)

	_, ok := <-stream.Events
	require.False(ok)

	streamErr := stream.Err()
	require.Error(streamErr)
	var transient *TransientError
	require.ErrorAs(streamErr, &transient)
}

package auditlogs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeLogFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      string
		expectErr bool
	}{
		{
			name:  "empty uses default",
			input: "",
			want:  defaultLogFormatValue,
		},
		{
			name:  "lowercase accepted",
			input: "cef",
			want:  "cef",
		},
		{
			name:  "uppercase normalized",
			input: "JSON",
			want:  "json",
		},
		{
			name:      "invalid rejected",
			input:     "invalid",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeLogFormat(tt.input)
			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultDestinationName(t *testing.T) {
	t.Parallel()

	name := defaultDestinationName()
	require.True(t, strings.HasPrefix(name, "kongctl-"))
	require.Contains(t, name, fmt.Sprintf("-%d", os.Getpid()))
}

func TestSanitizeDestinationNameComponent(t *testing.T) {
	t.Parallel()

	require.Equal(t, "my-host.local", sanitizeDestinationNameComponent("My Host.local"))
	require.Equal(t, "unknown-host", sanitizeDestinationNameComponent("  "))
	require.Equal(t, "a-b-c", sanitizeDestinationNameComponent("a/b\\c"))
}

func TestPayloadContainsDestinationName(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"data": []any{
			map[string]any{
				"id":       "dest_1",
				"name":     "kongctl-dev-123",
				"endpoint": "https://example.com/audit-logs",
			},
			map[string]any{
				"id":       "dest_2",
				"name":     "other",
				"endpoint": "https://example.com/other",
			},
		},
	}

	require.True(t, payloadContainsDestinationName(payload, "kongctl-dev-123"))
	require.False(t, payloadContainsDestinationName(payload, "missing"))
	require.False(t, payloadContainsDestinationName(map[string]any{"name": "kongctl-dev-123"}, "kongctl-dev-123"))
}

func TestEnsureNoActiveRegionalWebhookAllowsWhenDisabled(t *testing.T) {
	t.Parallel()

	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, webhookPathV2, r.URL.Path)
		return httpResponse(http.StatusOK, `{"enabled":false,"endpoint":"unconfigured"}`), nil
	})

	err := ensureNoActiveRegionalWebhook(context.Background(), client, "https://region.example.com", "token", nil)
	require.NoError(t, err)
}

func TestEnsureNoActiveRegionalWebhookBlocksWhenConnected(t *testing.T) {
	t.Parallel()

	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, webhookPathV2, r.URL.Path)
		return httpResponse(http.StatusOK, `{"enabled":true,"endpoint":"https://example.com/audit-logs"}`), nil
	})

	err := ensureNoActiveRegionalWebhook(context.Background(), client, "https://region.example.com", "token", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "regional audit-log webhook is already configured")
	require.Contains(t, err.Error(), "enabled=true")
}

func TestEnsureNoActiveRegionalWebhookBlocksWhenDisabledButConfigured(t *testing.T) {
	t.Parallel()

	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, webhookPathV2, r.URL.Path)
		return httpResponse(http.StatusOK, `{"enabled":false,"endpoint":"https://example.com/audit-logs"}`), nil
	})

	err := ensureNoActiveRegionalWebhook(context.Background(), client, "https://region.example.com", "token", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "regional audit-log webhook is already configured")
	require.Contains(t, err.Error(), "enabled=false")
}

func TestEnsureNoActiveRegionalWebhookBlocksWhenStateIsUnknown(t *testing.T) {
	t.Parallel()

	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, webhookPathV2, r.URL.Path)
		return httpResponse(http.StatusOK, `{"enabled":false}`), nil
	})

	err := ensureNoActiveRegionalWebhook(context.Background(), client, "https://region.example.com", "token", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "regional audit-log webhook is already configured")
}

func TestDeleteDestinationWithRetryRetriesDestinationInUseConflict(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v2/audit-log-destinations/dest-123", r.URL.Path)

		attempt := attempts.Add(1)
		if attempt < 3 {
			return httpResponse(http.StatusConflict, `{"detail":"Destination is in use"}`), nil
		}

		return httpResponse(http.StatusNoContent, ""), nil
	})

	err := deleteDestinationWithRetry(
		context.Background(),
		client,
		"https://region.example.com",
		"/v2/audit-log-destinations/dest-123",
		"token",
		"dest-123",
		nil,
	)
	require.NoError(t, err)
	require.EqualValues(t, 3, attempts.Load())
}

func TestDeleteDestinationWithRetryDoesNotRetryOtherConflict(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v2/audit-log-destinations/dest-123", r.URL.Path)

		attempts.Add(1)
		return httpResponse(http.StatusConflict, `{"detail":"some other conflict"}`), nil
	})

	err := deleteDestinationWithRetry(
		context.Background(),
		client,
		"https://region.example.com",
		"/v2/audit-log-destinations/dest-123",
		"token",
		"dest-123",
		nil,
	)
	require.Error(t, err)
	require.EqualValues(t, 1, attempts.Load())
}

func TestReleaseRegionalWebhookDisablesAndDeletes(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		attempt := attempts.Add(1)

		switch attempt {
		case 1:
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, webhookPathV2, r.URL.Path)
			return httpResponse(http.StatusOK, `{"enabled":false}`), nil
		case 2:
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, deleteWebhookPathV3, r.URL.Path)
			return httpResponse(http.StatusNoContent, ""), nil
		default:
			t.Fatalf("unexpected request attempt: %d", attempt)
			return nil, nil
		}
	})

	err := releaseRegionalWebhook(context.Background(), client, "https://region.example.com", "token", nil)
	require.NoError(t, err)
	require.EqualValues(t, 2, attempts.Load())
}

func TestReleaseRegionalWebhookTreatsNotFoundAsSuccess(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		attempt := attempts.Add(1)

		switch attempt {
		case 1:
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, webhookPathV2, r.URL.Path)
			return httpResponse(http.StatusNotFound, ""), nil
		case 2:
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, deleteWebhookPathV3, r.URL.Path)
			return httpResponse(http.StatusNotFound, ""), nil
		default:
			t.Fatalf("unexpected request attempt: %d", attempt)
			return nil, nil
		}
	})

	err := releaseRegionalWebhook(context.Background(), client, "https://region.example.com", "token", nil)
	require.NoError(t, err)
	require.EqualValues(t, 2, attempts.Load())
}

func TestReleaseRegionalWebhookReturnsErrorOnUnexpectedStatus(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		attempt := attempts.Add(1)

		switch attempt {
		case 1:
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, webhookPathV2, r.URL.Path)
			return httpResponse(http.StatusOK, `{"enabled":false}`), nil
		case 2:
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, deleteWebhookPathV3, r.URL.Path)
			return httpResponse(http.StatusConflict, `{"detail":"conflict"}`), nil
		default:
			t.Fatalf("unexpected request attempt: %d", attempt)
			return nil, nil
		}
	})

	err := releaseRegionalWebhook(context.Background(), client, "https://region.example.com", "token", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "delete audit-log webhook failed")
	require.EqualValues(t, 2, attempts.Load())
}

func TestDeleteDestinationWithRetryStopsOnContextTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	t.Cleanup(cancel)

	var attempts atomic.Int32
	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, "/v2/audit-log-destinations/dest-123", r.URL.Path)

		attempts.Add(1)
		return httpResponse(http.StatusConflict, `{"detail":"Destination is in use"}`), nil
	})

	err := deleteDestinationWithRetry(
		ctx,
		client,
		"https://region.example.com",
		"/v2/audit-log-destinations/dest-123",
		"token",
		"dest-123",
		nil,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "wait for destination release")
	require.GreaterOrEqual(t, attempts.Load(), int32(1))
}

func TestFindFirstBool(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"items": []any{
			map[string]any{
				"nested": map[string]any{
					"enabled": true,
				},
			},
		},
	}

	require.True(t, findFirstBool(payload, "enabled"))
	require.False(t, findFirstBool(payload, "missing"))
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

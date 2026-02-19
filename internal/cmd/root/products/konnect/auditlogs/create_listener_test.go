package auditlogs

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	auditlogstore "github.com/kong/kongctl/internal/auditlogs"
	"github.com/stretchr/testify/require"
)

func TestHasGzipContentEncoding(t *testing.T) {
	t.Parallel()

	require.True(t, hasGzipContentEncoding("gzip"))
	require.True(t, hasGzipContentEncoding("gzip, br"))
	require.True(t, hasGzipContentEncoding("br, gzip"))
	require.False(t, hasGzipContentEncoding("br"))
	require.False(t, hasGzipContentEncoding(""))
}

func TestMaybeDecodeRequestBody(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"ok":true}`)
	compressed := mustGzipBytes(t, raw)

	decoded, decodedGzip, err := maybeDecodeRequestBody("gzip", compressed, 1024)
	require.NoError(t, err)
	require.True(t, decodedGzip)
	require.Equal(t, raw, decoded)

	passthrough, decodedGzip, err := maybeDecodeRequestBody("", raw, 1024)
	require.NoError(t, err)
	require.False(t, decodedGzip)
	require.Equal(t, raw, passthrough)
}

func TestDecodeGzipBodyTooLarge(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"key":"value"}`)
	compressed := mustGzipBytes(t, raw)

	_, err := decodeGzipBody(compressed, 4)
	require.ErrorIs(t, err, errDecodedBodyTooLarge)
}

func TestListenerAuthorizationValidation(t *testing.T) {
	t.Parallel()

	t.Run("missing authorization is rejected", func(t *testing.T) {
		t.Parallel()

		handler, eventsFile := newTestListenerHandler(t, "Bearer secret")
		req := httptest.NewRequest(http.MethodPost, "/audit-logs", strings.NewReader(`{"id":"evt-1"}`))
		res := httptest.NewRecorder()

		handler(res, req)

		require.Equal(t, http.StatusUnauthorized, res.Code)
		_, err := os.Stat(eventsFile)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("mismatched authorization is rejected", func(t *testing.T) {
		t.Parallel()

		handler, eventsFile := newTestListenerHandler(t, "Bearer secret")
		req := httptest.NewRequest(http.MethodPost, "/audit-logs", strings.NewReader(`{"id":"evt-2"}`))
		req.Header.Set("Authorization", "Bearer wrong")
		res := httptest.NewRecorder()

		handler(res, req)

		require.Equal(t, http.StatusUnauthorized, res.Code)
		_, err := os.Stat(eventsFile)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("matching authorization is accepted", func(t *testing.T) {
		t.Parallel()

		handler, eventsFile := newTestListenerHandler(t, "Bearer secret")
		req := httptest.NewRequest(http.MethodPost, "/audit-logs", strings.NewReader(`{"id":"evt-3"}`))
		req.Header.Set("Authorization", "Bearer secret")
		res := httptest.NewRecorder()

		handler(res, req)

		require.Equal(t, http.StatusAccepted, res.Code)
		raw, err := os.ReadFile(eventsFile)
		require.NoError(t, err)
		require.Equal(t, "{\"id\":\"evt-3\"}\n", string(raw))
	})

	t.Run("no configured authorization allows request", func(t *testing.T) {
		t.Parallel()

		handler, eventsFile := newTestListenerHandler(t, "")
		req := httptest.NewRequest(http.MethodPost, "/audit-logs", strings.NewReader(`{"id":"evt-4"}`))
		res := httptest.NewRecorder()

		handler(res, req)

		require.Equal(t, http.StatusAccepted, res.Code)
		raw, err := os.ReadFile(eventsFile)
		require.NoError(t, err)
		require.Equal(t, "{\"id\":\"evt-4\"}\n", string(raw))
	})
}

func newTestListenerHandler(t *testing.T, expectedAuthorization string) (http.HandlerFunc, string) {
	t.Helper()

	eventsFile := filepath.Join(t.TempDir(), "events.jsonl")
	store := auditlogstore.NewStore(eventsFile)
	cmdObj := &createListenerCmd{maxBodyBytes: defaultMaxBodyBytes}

	handler := cmdObj.newListenerHandler("/audit-logs", store, strings.TrimSpace(expectedAuthorization), nil)
	return handler, eventsFile
}

func mustGzipBytes(t *testing.T, payload []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(payload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return buf.Bytes()
}

package auditlogs

import (
	"bytes"
	"compress/gzip"
	"testing"

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

func mustGzipBytes(t *testing.T, payload []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(payload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return buf.Bytes()
}

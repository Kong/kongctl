package auditlogs

import (
	"bytes"
	"testing"

	"github.com/kong/kongctl/internal/iostreams"
	"github.com/stretchr/testify/require"
)

func TestNewTailEventEmitterInvalidJQ(t *testing.T) {
	t.Parallel()

	streams := &iostreams.IOStreams{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
	_, err := newTailEventEmitter(streams, ".records[")
	require.Error(t, err)
}

func TestTailEventEmitterEmitRecordsRaw(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var errOut bytes.Buffer
	streams := &iostreams.IOStreams{Out: &out, ErrOut: &errOut}

	emitter, err := newTailEventEmitter(streams, "")
	require.NoError(t, err)

	err = emitter.EmitRecords([][]byte{
		[]byte(`{"a":1}`),
		[]byte(`{"b":2}`),
	})
	require.NoError(t, err)
	require.Equal(t, "{\"a\":1}\n{\"b\":2}\n", out.String())
	require.Empty(t, errOut.String())
}

func TestTailEventEmitterEmitRecordsWithJQ(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var errOut bytes.Buffer
	streams := &iostreams.IOStreams{Out: &out, ErrOut: &errOut}

	emitter, err := newTailEventEmitter(streams, ".request.id")
	require.NoError(t, err)

	err = emitter.EmitRecords([][]byte{
		[]byte(`{"request":{"id":"abc"}}`),
	})
	require.NoError(t, err)
	require.Equal(t, "\"abc\"\n", out.String())
	require.Empty(t, errOut.String())
}

func TestTailEventEmitterEmitRecordsSkipsInvalidJSONForJQ(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	var errOut bytes.Buffer
	streams := &iostreams.IOStreams{Out: &out, ErrOut: &errOut}

	emitter, err := newTailEventEmitter(streams, ".request")
	require.NoError(t, err)

	err = emitter.EmitRecords([][]byte{
		[]byte(`not-json`),
		[]byte(`{"request":{"ok":true}}`),
	})
	require.NoError(t, err)
	require.Equal(t, "{\"ok\":true}\n", out.String())
	require.Contains(t, errOut.String(), "skipping non-JSON or invalid jq record")
}

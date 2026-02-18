package auditlogs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolvePathsUsesXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	paths, err := ResolvePaths("dev profile")
	require.NoError(t, err)

	expected := filepath.Join(tmp, "kongctl", "audit-logs", "dev_profile")
	require.Equal(t, expected, paths.BaseDir)
	require.Equal(t, filepath.Join(expected, "events.jsonl"), paths.EventsFile)
	require.Equal(t, filepath.Join(expected, "listener.json"), paths.ListenerStateFile)
	require.Equal(t, filepath.Join(expected, "destination.json"), paths.DestinationStateFile)
}

func TestSplitPayloadRecords(t *testing.T) {
	t.Parallel()

	records := SplitPayloadRecords([]byte("\n  {\"a\":1}\r\n\r\n{\"b\":2}\n"))
	require.Len(t, records, 2)
	require.Equal(t, `{"a":1}`, string(records[0]))
	require.Equal(t, `{"b":2}`, string(records[1]))
}

func TestStoreAppendWritesJSONL(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "events.jsonl")
	store := NewStore(path)

	written, err := store.Append([]byte("{\"id\":\"evt-1\"}\n{\"id\":\"evt-2\"}\n"))
	require.NoError(t, err)
	require.Equal(t, 2, written)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	require.Len(t, lines, 2)
	require.Equal(t, `{"id":"evt-1"}`, lines[0])
	require.Equal(t, `{"id":"evt-2"}`, lines[1])
}

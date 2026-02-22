package processes

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWriteListAndRemoveRecord(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	recordPath, err := ResolvePathForPID(4242)
	require.NoError(t, err)

	record := Record{
		PID:       4242,
		Kind:      "konnect.audit-logs.listen",
		Profile:   "default",
		CreatedAt: time.Now().UTC(),
		LogFile:   filepath.Join(tmpDir, "logs", "kongctl-listener-4242.log"),
		Args:      []string{"listen", "--authorization", "secret"},
	}
	require.NoError(t, WriteRecord(recordPath, record))

	records, err := ListRecords()
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, 4242, records[0].PID)
	require.Equal(t, "konnect.audit-logs.listen", records[0].Kind)
	require.Equal(t, []string{"listen", "--authorization", "<redacted>"}, records[0].Args)

	require.NoError(t, RemoveRecordByPID(4242))
	_, err = os.Stat(recordPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestResolvePathFromTemplate(t *testing.T) {
	t.Parallel()

	got := ResolvePathFromTemplate("/tmp/%PID%.json", 123)
	require.Equal(t, "/tmp/123.json", got)
}

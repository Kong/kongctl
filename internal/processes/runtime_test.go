//go:build linux

package processes

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadStartTimeTicksCurrentProcess(t *testing.T) {
	t.Parallel()

	startTicks, err := ReadStartTimeTicks(os.Getpid())
	require.NoError(t, err)
	require.Greater(t, startTicks, uint64(0))
}

func TestInspectRunningRecord(t *testing.T) {
	t.Parallel()

	startTicks, err := ReadStartTimeTicks(os.Getpid())
	require.NoError(t, err)

	state := Inspect(Record{
		PID:            os.Getpid(),
		StartTimeTicks: startTicks,
	})
	require.Equal(t, StatusRunning, state.Status)
	require.True(t, state.Running)
}

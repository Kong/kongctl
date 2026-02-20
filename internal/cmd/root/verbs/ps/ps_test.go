package ps

import (
	"testing"
	"time"

	"github.com/kong/kongctl/internal/processes"
	"github.com/stretchr/testify/require"
)

func TestNewPSCmd(t *testing.T) {
	t.Parallel()

	cmd, err := NewPSCmd()
	require.NoError(t, err)
	require.NotNil(t, cmd)
	require.Equal(t, "ps", cmd.Use)
	require.NotNil(t, cmd.Commands())
}

func TestResolveTargets(t *testing.T) {
	t.Parallel()

	psCommand := &psCmd{stopTimeout: 5 * time.Second}
	records := []processes.StoredRecord{
		{Record: processes.Record{PID: 100}},
		{Record: processes.Record{PID: 200}},
	}

	t.Run("single pid", func(t *testing.T) {
		t.Parallel()

		targets, err := psCommand.resolveTargets([]string{"100"}, records)
		require.NoError(t, err)
		require.Len(t, targets, 1)
		require.Equal(t, 100, targets[0].PID)
	})

	t.Run("all flag", func(t *testing.T) {
		t.Parallel()

		cmd := &psCmd{stopAll: true}
		targets, err := cmd.resolveTargets(nil, records)
		require.NoError(t, err)
		require.Len(t, targets, 2)
	})
}

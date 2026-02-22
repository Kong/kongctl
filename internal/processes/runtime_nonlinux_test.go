//go:build !linux

package processes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInspectNonLinuxReturnsUnknown(t *testing.T) {
	t.Parallel()

	state := Inspect(Record{PID: 12345})
	require.Equal(t, StatusUnknown, state.Status)
	require.False(t, state.Running)
	require.NotEmpty(t, state.CheckError)
}

func TestTerminateNonLinuxReturnsNotSupported(t *testing.T) {
	t.Parallel()

	err := Terminate(12345, time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not supported")
}

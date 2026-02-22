//go:build !linux

package processes

import (
	"fmt"
	"runtime"
	"time"
)

func unsupportedRuntimeOpError(op string) error {
	return fmt.Errorf("%s is not supported on %s", op, runtime.GOOS)
}

// Inspect returns StatusUnknown on non-Linux targets.
func Inspect(record Record) RuntimeState {
	state := RuntimeState{
		Status:  StatusUnknown,
		Running: false,
	}
	if record.PID <= 0 {
		state.CheckError = "invalid process pid"
		return state
	}

	state.CheckError = unsupportedRuntimeOpError("process inspection").Error()
	return state
}

// Terminate is not currently supported on non-Linux targets.
func Terminate(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("invalid process pid")
	}
	_ = timeout
	return unsupportedRuntimeOpError("process termination")
}

// WaitForStartTimeTicks returns 0 on non-Linux targets.
func WaitForStartTimeTicks(pid int, timeout time.Duration) uint64 {
	_ = pid
	_ = timeout
	return 0
}

// ReadStartTimeTicks is not supported on non-Linux targets.
func ReadStartTimeTicks(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid process pid")
	}
	return 0, unsupportedRuntimeOpError("process start-time inspection")
}

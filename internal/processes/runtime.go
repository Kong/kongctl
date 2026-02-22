//go:build linux

package processes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Inspect evaluates whether a recorded process is still running.
func Inspect(record Record) RuntimeState {
	state := RuntimeState{
		Status:  StatusUnknown,
		Running: false,
	}
	if record.PID <= 0 {
		state.CheckError = "invalid process pid"
		return state
	}

	exists, err := processExists(record.PID)
	if err != nil {
		state.CheckError = err.Error()
		return state
	}
	if !exists {
		state.Status = StatusExited
		return state
	}

	state.Status = StatusRunning
	state.Running = true

	startTicks, err := ReadStartTimeTicks(record.PID)
	if err != nil {
		state.CheckError = err.Error()
		return state
	}
	state.ObservedStartTimeTicks = startTicks

	if record.StartTimeTicks > 0 && startTicks > 0 && startTicks != record.StartTimeTicks {
		state.Status = StatusStale
		state.Running = false
	}

	return state
}

// Terminate sends SIGTERM to pid and waits for process exit.
func Terminate(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("invalid process pid")
	}
	if timeout <= 0 {
		timeout = defaultStopTimeout
	}

	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}

	deadline := time.Now().Add(timeout)
	for {
		exists, err := processExists(pid)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("process %d did not exit within %s", pid, timeout)
		}
		time.Sleep(defaultProbeInterval)
	}
}

// WaitForStartTimeTicks waits for /proc stat start time to become available.
func WaitForStartTimeTicks(pid int, timeout time.Duration) uint64 {
	if pid <= 0 {
		return 0
	}
	if timeout <= 0 {
		timeout = defaultStartProbeWait
	}

	deadline := time.Now().Add(timeout)
	for {
		startTicks, err := ReadStartTimeTicks(pid)
		if err == nil && startTicks > 0 {
			return startTicks
		}

		if time.Now().After(deadline) {
			return 0
		}
		time.Sleep(defaultStartProbeInterval)
	}
}

// ReadStartTimeTicks reads the process start time ticks from /proc/<pid>/stat.
func ReadStartTimeTicks(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid process pid")
	}

	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	raw, err := os.ReadFile(statPath)
	if err != nil {
		return 0, err
	}

	line := strings.TrimSpace(string(raw))
	closing := strings.LastIndex(line, ")")
	if closing < 0 || closing+1 >= len(line) {
		return 0, fmt.Errorf("unexpected /proc stat format")
	}

	rest := strings.TrimSpace(line[closing+1:])
	fields := strings.Fields(rest)
	if len(fields) <= 19 {
		return 0, fmt.Errorf("unexpected /proc stat field count")
	}

	return strconv.ParseUint(fields[19], 10, 64)
}

func processExists(pid int) (bool, error) {
	if pid <= 0 {
		return false, fmt.Errorf("invalid process pid")
	}

	err := syscall.Kill(pid, syscall.Signal(0))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.EPERM) {
		return true, nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}

	return false, err
}

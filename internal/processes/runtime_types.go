package processes

import "time"

const (
	defaultStopTimeout        = 15 * time.Second
	defaultStartProbeWait     = 500 * time.Millisecond
	defaultProbeInterval      = 100 * time.Millisecond
	defaultStartProbeInterval = 20 * time.Millisecond
)

// Status represents the runtime state of a detached process record.
type Status string

const (
	StatusRunning Status = "running"
	StatusExited  Status = "exited"
	StatusStale   Status = "stale"
	StatusUnknown Status = "unknown"
)

// RuntimeState captures live process status for a stored record.
type RuntimeState struct {
	Status                 Status `json:"status" yaml:"status"`
	Running                bool   `json:"running" yaml:"running"`
	ObservedStartTimeTicks uint64 `json:"observed_start_time_ticks,omitempty" yaml:"observed_start_time_ticks,omitempty"`
	CheckError             string `json:"check_error,omitempty" yaml:"check_error,omitempty"`
}

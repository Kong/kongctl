package telemetry

import "time"

// SchemaVersion identifies the shape of Event.
const SchemaVersion = 1

// Event is the single telemetry record kongctl emits per command execution.
type Event struct {
	SchemaVersion int       `json:"schema_version"`
	Timestamp     time.Time `json:"timestamp"`

	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`

	CommandPath string `json:"command_path"`
}

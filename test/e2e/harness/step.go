//go:build e2e

package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Step groups inputs, commands, snapshots, and checks under a per-step directory.
// Example layout:
// <TestDir>/steps/001-init/{inputs,commands,snapshots,checks.log}
type Step struct {
	Name         string
	Dir          string
	InputsDir    string
	SnapshotsDir string
	ChecksPath   string
	cli          *CLI
}

// NewStep initializes a new step directory under the CLI's TestDir and
// sets the CLI to capture command artifacts under this step. Command numbering
// is reset for readability within the step.
func NewStep(t *testing.T, cli *CLI, name string) (*Step, error) {
	t.Helper()
	if cli == nil {
		return nil, fmt.Errorf("nil cli")
	}
	if cli.TestDir == "" {
		return nil, fmt.Errorf("cli.TestDir not set")
	}
	dir := filepath.Join(cli.TestDir, "steps", sanitizeName(name))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	inputs := filepath.Join(dir, "inputs")
	if err := os.MkdirAll(inputs, 0o755); err != nil {
		return nil, err
	}
	snaps := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snaps, 0o755); err != nil {
		return nil, err
	}
	s := &Step{
		Name:         name,
		Dir:          dir,
		InputsDir:    inputs,
		SnapshotsDir: snaps,
		ChecksPath:   filepath.Join(dir, "checks.log"),
		cli:          cli,
	}
	// Point CLI captures at this step and reset command sequence.
	cli.StepDir = dir
	cli.cmdSeq = 0
	Infof("Step initialized: %s", dir)
	return s, nil
}

// SaveJSON writes v as pretty JSON to a file relative to the step directory.
func (s *Step) SaveJSON(rel string, v any) error {
	if s == nil {
		return fmt.Errorf("nil step")
	}
	path := filepath.Join(s.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// AppendCheck appends a human-readable assertion message to checks.log.
func (s *Step) AppendCheck(format string, args ...any) {
	if s == nil || s.ChecksPath == "" {
		return
	}
	msg := fmt.Sprintf(format, args...)
	f, err := os.OpenFile(s.ChecksPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(msg + "\n")
}

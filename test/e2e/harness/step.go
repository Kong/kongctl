//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/yaml"
)

// Step groups inputs, per-command captures, and checks under a per-step directory.
// Example layout:
// <TestDir>/steps/001-init/{inputs,commands,checks.log}
type Step struct {
	Name       string
	Dir        string
	InputsDir  string
	ChecksPath string
	cli        *CLI
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
	s := &Step{
		Name:       name,
		Dir:        dir,
		InputsDir:  inputs,
		ChecksPath: filepath.Join(dir, "checks.log"),
		cli:        cli,
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

// WriteObservation writes a single observation.json tied to the last executed command under
// <step>/commands/<SEQ>-<slug>/observation.json.
// - For list-like reads, pass all (full parsed result), target (filtered subset), and optional selector.
// - For other cases, consider WriteApplyObservation for apply summaries.
func (s *Step) WriteObservation(all any, target any, selector any) error {
	if s == nil || s.cli == nil || s.cli.LastCommandDir == "" {
		return fmt.Errorf("no last command directory available for observations")
	}
	obsPath := filepath.Join(s.cli.LastCommandDir, "observation.json")
	payload := map[string]any{
		"type":   "list_observation",
		"all":    all,
		"target": target,
	}
	if selector != nil {
		payload["selector"] = selector
	}
	return s.writePrettyJSON(obsPath, payload)
}

func (s *Step) writePrettyJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// WriteApplyObservation records an apply summary as observation.json in the last command directory.
// The value 'summary' is typically the decoded apply response struct used in tests.
func (s *Step) WriteApplyObservation(summary any) error {
	if s == nil || s.cli == nil || s.cli.LastCommandDir == "" {
		return fmt.Errorf("no last command directory available for observations")
	}
	obsPath := filepath.Join(s.cli.LastCommandDir, "observation.json")
	payload := map[string]any{
		"type": "apply_summary",
		"data": summary,
	}
	return s.writePrettyJSON(obsPath, payload)
}

// CopyInput copies a file from src into the step inputs directory as dstName and returns the destination path.
func (s *Step) CopyInput(src, dstName string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("nil step")
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	dstPath := filepath.Join(s.InputsDir, dstName)
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return "", err
	}
	out, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	if err := out.Sync(); err != nil {
		return "", err
	}
	return dstPath, nil
}

// ExpectFromYAML unmarshals a YAML file into out. Useful for deriving expectations from inputs.
func (s *Step) ExpectFromYAML(path string, out any) error {
	if s == nil {
		return fmt.Errorf("nil step")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, out)
}

// ApplySummary is the parsed structure used by tests for apply assertions.
type ApplySummary struct {
	Execution struct {
		DryRun bool `json:"dry_run"`
	} `json:"execution"`
	Summary struct {
		Applied      int    `json:"applied"`
		Failed       int    `json:"failed"`
		Status       string `json:"status"`
		TotalChanges int    `json:"total_changes"`
	} `json:"summary"`
}

// Apply runs kongctl apply for the provided manifests, records an apply_summary observation,
// and returns the parsed summary for assertions.
func (s *Step) Apply(manifests ...string) (ApplySummary, error) {
	var out ApplySummary
	if s == nil || s.cli == nil {
		return out, fmt.Errorf("nil step/cli")
	}
	args := []string{"apply"}
	for _, m := range manifests {
		args = append(args, "-f", m)
	}
	args = append(args, "--auto-approve")
	res, err := s.cli.RunJSON(context.Background(), &out, args...)
	if err != nil {
		_ = res
		return out, err
	}
	_ = s.WriteApplyObservation(out)
	return out, nil
}

// GetAndObserve runs kongctl get <resource>, decodes into out, and writes a list_observation with the provided selector.
func (s *Step) GetAndObserve(resource string, out any, selector any) error {
	if s == nil || s.cli == nil {
		return fmt.Errorf("nil step/cli")
	}
	if _, err := s.cli.RunJSON(context.Background(), out, "get", resource); err != nil {
		return err
	}
	return s.WriteObservation(out, nil, selector)
}

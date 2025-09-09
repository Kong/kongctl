//go:build e2e

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// GetKonnectJSON performs an HTTP GET to the Konnect API and records a synthetic command
// with a single observation.json under the step's commands directory.
// - name: slug used in the command folder (e.g., "get_publication")
// - path: request path starting with '/v3/...'
// - out: decoded JSON target
// - selector: optional map describing the filter/identifiers used
func (s *Step) GetKonnectJSON(name string, path string, out any, selector any) error {
	if s == nil || s.cli == nil {
		return fmt.Errorf("nil step/cli")
	}
	baseDir := s.cli.TestDir
	if s.cli.StepDir != "" {
		baseDir = s.cli.StepDir
	}
	commandsDir := filepath.Join(baseDir, "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		return err
	}
	seq := s.cli.cmdSeq
	s.cli.cmdSeq++
	slug := sanitizeName(name)
	dir := filepath.Join(commandsDir, fmt.Sprintf("%03d-%s", seq, slug))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	baseURL := os.Getenv("KONGCTL_E2E_KONNECT_BASE_URL")
	if baseURL == "" {
		baseURL = "https://us.api.konghq.com"
	}
	fullURL := strings.TrimRight(baseURL, "/") + path
	token := os.Getenv("KONGCTL_E2E_KONNECT_PAT")
	if token == "" {
		return fmt.Errorf("KONGCTL_E2E_KONNECT_PAT not set")
	}

	// Write command.txt
	_ = os.WriteFile(filepath.Join(dir, "command.txt"), []byte("HTTP GET "+fullURL+"\n"), 0o644)

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	start := time.Now()
	resp, err := client.Do(req)
	dur := time.Since(start)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// stdout/stderr
	_ = os.WriteFile(filepath.Join(dir, "stdout.txt"), body, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "stderr.txt"), []byte{}, 0o644)

	// env.json (sanitized)
	envMap := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			k := kv[:i]
			v := kv[i+1:]
			ku := strings.ToUpper(k)
			if strings.Contains(ku, "TOKEN") || strings.Contains(ku, "PAT") || strings.Contains(ku, "PASSWORD") ||
				strings.Contains(ku, "SECRET") {
				if v != "" {
					v = "***"
				}
			}
			envMap[k] = v
		}
	}
	if b, err := json.MarshalIndent(envMap, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "env.json"), b, 0o644)
	}

	// meta.json
	meta := map[string]any{
		"method":   http.MethodGet,
		"url":      fullURL,
		"status":   resp.StatusCode,
		"duration": dur.String(),
		"started":  start,
		"finished": time.Now(),
	}
	if b, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "meta.json"), b, 0o644)
	}

	// Decode body into out (best-effort)
	var decodeErr error
	if len(body) > 0 && out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			decodeErr = err
		}
	}

	// observation.json
	obs := map[string]any{
		"type":     "http_observation",
		"data":     out,
		"selector": selector,
		"status":   resp.StatusCode,
	}
	if b, err := json.MarshalIndent(obs, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "observation.json"), b, 0o644)
	}

	// record LastCommandDir for consistency
	s.cli.LastCommandDir = dir

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decodeErr != nil {
			return fmt.Errorf("http %d; decode error: %v", resp.StatusCode, decodeErr)
		}
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	if decodeErr != nil {
		return decodeErr
	}
	return nil
}

// ResetOrg runs the destructive reset with artifacts captured under this step's commands.
// Returns an error if the reset executes and any endpoint operation fails. Records "skipped"
// observation when reset is disabled or PAT missing.
func (s *Step) ResetOrg(stage string) error {
	if s == nil || s.cli == nil {
		return fmt.Errorf("nil step/cli")
	}
	// Prepare command dir
	baseDir := s.cli.StepDir
	if baseDir == "" {
		baseDir = s.cli.TestDir
	}
	commandsDir := filepath.Join(baseDir, "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		return err
	}
	seq := s.cli.cmdSeq
	s.cli.cmdSeq++
	dir := filepath.Join(commandsDir, fmt.Sprintf("%03d-%s", seq, "reset_org"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// command.txt
	_ = os.WriteFile(filepath.Join(dir, "command.txt"), []byte(fmt.Sprintf("RESET ORG (stage=%s)\n", stage)), 0o644)

	// Check env gating
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("KONGCTL_E2E_RESET"))); v != "" && v != "1" && v != "true" &&
		v != "yes" &&
		v != "on" &&
		v != "y" {
		// Skipped due to env
		obs := map[string]any{
			"type":     "reset_summary",
			"executed": false,
			"status":   "skipped",
			"reason":   "reset disabled",
		}
		_ = s.writePrettyJSON(filepath.Join(dir, "observation.json"), obs)
		return nil
	}
	baseURL := os.Getenv("KONGCTL_E2E_KONNECT_BASE_URL")
	if baseURL == "" {
		baseURL = "https://us.api.konghq.com"
	}
	token := os.Getenv("KONGCTL_E2E_KONNECT_PAT")
	if token == "" {
		obs := map[string]any{"type": "reset_summary", "executed": false, "status": "skipped", "reason": "missing PAT"}
		_ = s.writePrettyJSON(filepath.Join(dir, "observation.json"), obs)
		return nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	details := []map[string]any{}
	var firstErr error
	// application-auth-strategies
	tot1, del1, err1 := deleteAll(client, baseURL, token, "v2", "application-auth-strategies")
	if err1 != nil {
		firstErr = err1
	}
	details = append(
		details,
		map[string]any{
			"api_version": "v2",
			"endpoint":    "application-auth-strategies",
			"total":       tot1,
			"deleted":     del1,
			"error":       errorString(err1),
		},
	)
	// apis
	tot2, del2, err2 := deleteAll(client, baseURL, token, "v3", "apis")
	if err2 != nil && firstErr == nil {
		firstErr = err2
	}
	details = append(
		details,
		map[string]any{
			"api_version": "v3",
			"endpoint":    "apis",
			"total":       tot2,
			"deleted":     del2,
			"error":       errorString(err2),
		},
	)
	// portals
	tot3, del3, err3 := deleteAll(client, baseURL, token, "v3", "portals")
	if err3 != nil && firstErr == nil {
		firstErr = err3
	}
	details = append(
		details,
		map[string]any{
			"api_version": "v3",
			"endpoint":    "portals",
			"total":       tot3,
			"deleted":     del3,
			"error":       errorString(err3),
		},
	)

	status := "ok"
	if firstErr != nil {
		status = "error"
	}
	obs := map[string]any{
		"type":     "reset_summary",
		"executed": true,
		"status":   status,
		"base_url": baseURL,
		"details":  details,
	}
	_ = s.writePrettyJSON(filepath.Join(dir, "observation.json"), obs)
	if firstErr != nil {
		return firstErr
	}
	return nil
}

func errorString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

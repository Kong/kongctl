//go:build e2e

package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type CLI struct {
	BinPath   string
	Env       []string
	WorkDir   string
	Profile   string
	ConfigDir string // XDG_CONFIG_HOME
	Timeout   time.Duration
	TestName  string
	TestDir   string
	// Optional step scope: when set, command artifacts are captured under this directory
	// instead of the test root. Intended to group inputs/commands/snapshots per step.
	StepDir string
	// LastCommandDir records the directory where the most recent command's artifacts
	// were captured. Useful for attaching observations to a specific command.
	LastCommandDir string
	// If set, inject --log-level into command args unless caller overrides.
	AutoLogLevel string
	AutoOutput   string
	cmdSeq       int
	nextOutput   struct {
		set     bool
		disable bool
		value   string
	}
}

// NewCLI constructs a CLI instance with a temp config dir and default profile "e2e".
// Kept for backward-compat; prefer NewCLIT to place artifacts under the run dir.
func NewCLI() (*CLI, error) {
	bin, err := BinPath()
	if err != nil {
		return nil, err
	}
	cfgDir, err := os.MkdirTemp("", "kongctl-e2e-xdg-")
	if err != nil {
		return nil, err
	}
	env := append(os.Environ(), fmt.Sprintf("XDG_CONFIG_HOME=%s", cfgDir))
	cli := &CLI{
		BinPath:      bin,
		Env:          env,
		WorkDir:      "",
		Profile:      "e2e",
		ConfigDir:    cfgDir,
		Timeout:      60 * time.Second,
		TestName:     "",
		TestDir:      "",
		AutoLogLevel: getHarnessLogLevel(),
		AutoOutput:   getHarnessDefaultOutput(),
	}
	// Pre-write a minimal profile config to align artifacts with e2e defaults.
	_ = writeProfileConfig(cli.ConfigDir, cli.Profile, cli.AutoOutput, cli.AutoLogLevel)
	Infof(
		"TestConfig: bin=%s configDir=%s profile=%s timeout=%s log-level=%s output=%s",
		bin,
		cfgDir,
		cli.Profile,
		cli.Timeout,
		cli.AutoLogLevel,
		cli.AutoOutput,
	)
	return cli, nil
}

// NewCLIT constructs a CLI instance under the per-run artifacts dir using the test's name.
func NewCLIT(t *testing.T) (*CLI, error) {
	t.Helper()
	bin, err := BinPath()
	if err != nil {
		return nil, err
	}
	rd, err := ensureRunDir()
	if err != nil {
		return nil, err
	}
	name := sanitizeName(t.Name())
	testDir := filepath.Join(rd, "tests", name)
	cfgDir := filepath.Join(testDir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return nil, err
	}
	env := append(os.Environ(), fmt.Sprintf("XDG_CONFIG_HOME=%s", cfgDir))
	cli := &CLI{
		BinPath:      bin,
		Env:          env,
		WorkDir:      "",
		Profile:      "e2e",
		ConfigDir:    cfgDir,
		Timeout:      60 * time.Second,
		TestName:     name,
		TestDir:      testDir,
		AutoLogLevel: getHarnessLogLevel(),
		AutoOutput:   getHarnessDefaultOutput(),
	}
	// Pre-write a minimal profile config to align artifacts with e2e defaults.
	_ = writeProfileConfig(cli.ConfigDir, cli.Profile, cli.AutoOutput, cli.AutoLogLevel)
	Infof(
		"TestConfig: test=%s dir=%s bin=%s configDir=%s log-level=%s output=%s",
		name,
		testDir,
		bin,
		cfgDir,
		cli.AutoLogLevel,
		cli.AutoOutput,
	)
	return cli, nil
}

// WithEnv sets additional environment variables.
func (c *CLI) WithEnv(kv map[string]string) *CLI {
	if len(kv) == 0 {
		return c
	}
	for k, v := range kv {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", k, v))
		Debugf("WithEnv: set %s", redactEnv(fmt.Sprintf("%s=%s", k, v)))
	}
	return c
}

// WithWorkdir sets the working directory for command execution.
func (c *CLI) WithWorkdir(dir string) *CLI {
	c.WorkDir = dir
	Debugf("WithWorkdir: %s", dir)
	return c
}

// WithProfile sets the CLI profile.
func (c *CLI) WithProfile(p string) *CLI {
	if p != "" {
		c.Profile = p
	}
	Debugf("WithProfile: %s", c.Profile)
	return c
}

// OverrideNextOutput sets the output format used for the next kongctl command
// executed by Run. The override is cleared after the command completes.
func (c *CLI) OverrideNextOutput(format string) {
	c.nextOutput.set = true
	c.nextOutput.disable = false
	c.nextOutput.value = format
}

// DisableNextOutput instructs the CLI to skip injecting any -o/--output flag for
// the next command executed by Run. The override is cleared after the command completes.
func (c *CLI) DisableNextOutput() {
	c.nextOutput.set = true
	c.nextOutput.disable = true
	c.nextOutput.value = ""
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Run executes kongctl with the provided args and returns a Result.
func (c *CLI) Run(ctx context.Context, args ...string) (Result, error) {
	// Auto-append --profile unless already set in args.
	haveProfile := false
	for i := range args {
		if args[i] == "--profile" || strings.HasPrefix(args[i], "--profile=") {
			haveProfile = true
			break
		}
	}
	if !haveProfile && c.Profile != "" {
		args = append(args, "--profile", c.Profile)
	}

	// Inject --log-level if set at harness level and not provided by caller.
	if lvl := c.AutoLogLevel; lvl != "" {
		haveLevel := false
		for i := range args {
			if args[i] == "--log-level" || strings.HasPrefix(args[i], "--log-level=") {
				haveLevel = true
				break
			}
		}
		if !haveLevel {
			args = append(args, "--log-level", lvl)
		}
	}

	// Handle output overrides/injection for this command.
	outOverride := c.nextOutput
	c.nextOutput = struct {
		set     bool
		disable bool
		value   string
	}{}
	haveOut := hasOutputArg(args)
	switch {
	case outOverride.set && outOverride.disable:
		// Explicitly disable auto output injection for this command.
	case outOverride.set && !outOverride.disable:
		if !haveOut && strings.TrimSpace(outOverride.value) != "" {
			args = append(args, "-o", outOverride.value)
		}
	default:
		if out := c.AutoOutput; out != "" && !haveOut {
			args = append(args, "-o", out)
		}
	}

	var cancel context.CancelFunc
	if c.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, c.BinPath, args...)
	if c.WorkDir != "" {
		cmd.Dir = c.WorkDir
	}
	cmd.Env = c.Env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	Debugf("Run: %s (dir=%s) env[XDG_CONFIG_HOME]=%s", strings.Join(cmd.Args, " "), cmd.Dir, c.ConfigDir)
	err := cmd.Run()
	dur := time.Since(start)

	res := Result{Stdout: stdout.String(), Stderr: stderr.String(), Duration: dur}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
		}
		Debugf("Run: exit=%d duration=%s stderr=%q", res.ExitCode, dur, res.Stderr)
		c.captureCommand(cmd, args, res, start, time.Now())
		return res, err
	}
	res.ExitCode = 0
	Debugf("Run: exit=0 duration=%s", dur)
	c.captureCommand(cmd, args, res, start, time.Now())
	return res, nil
}

// RunJSON runs the command forcing JSON output and unmarshals stdout into out.
func (c *CLI) RunJSON(ctx context.Context, out any, args ...string) (Result, error) {
	// ensure -o json is set unless caller already set it
	hasOut := false
	for i := range args {
		if args[i] == "-o" || args[i] == "--output" || strings.HasPrefix(args[i], "--output=") {
			hasOut = true
			break
		}
	}
	if !hasOut {
		args = append(args, "-o", "json")
	}
	res, err := c.Run(ctx, args...)
	if err != nil {
		return res, err
	}
	dec := json.NewDecoder(strings.NewReader(res.Stdout))
	if jsonStrictEnabled() {
		Debugf("RunJSON: strict unknown-field checking enabled")
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(out); err != nil {
		return res, err
	}
	return res, nil
}

// TempWorkdir creates and assigns a temp working directory.
func (c *CLI) TempWorkdir() (string, error) {
	if c.WorkDir != "" {
		return c.WorkDir, nil
	}
	var dir string
	var err error
	if c.TestDir != "" {
		dir = filepath.Join(c.TestDir, "inputs")
		err = os.MkdirAll(dir, 0o755)
	} else {
		dir, err = os.MkdirTemp("", "kongctl-e2e-work-")
	}
	if err == nil {
		// ensure path is absolute for clarity
		dir, _ = filepath.Abs(dir)
		c.WorkDir = dir
	}
	return c.WorkDir, err
}

func sanitizeName(s string) string {
	// Replace characters that may not be friendly in dir names
	repl := s
	bad := []string{"/", "\\", " ", ":", "*", "?", "\"", "<", ">", "|"}
	for _, b := range bad {
		repl = strings.ReplaceAll(repl, b, "_")
	}
	return repl
}

func (c *CLI) allocateCommandDir(slug string) (string, error) {
	seq := c.cmdSeq
	c.cmdSeq++
	if !captureEnabled || c.TestDir == "" {
		return "", nil
	}
	baseDir := c.TestDir
	if c.StepDir != "" {
		baseDir = c.StepDir
	}
	commandsDir := filepath.Join(baseDir, "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		return "", err
	}
	if strings.TrimSpace(slug) == "" {
		slug = "cmd"
	}
	slug = sanitizeName(slug)
	dir := filepath.Join(commandsDir, fmt.Sprintf("%03d-%s", seq, slug))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	c.LastCommandDir = dir
	return dir, nil
}

// captureCommand writes per-command artifacts (args, stdout, stderr, meta).
func (c *CLI) captureCommand(cmd *exec.Cmd, args []string, res Result, start, end time.Time) {
	dir, err := c.allocateCommandDir(slugFromArgs(args))
	if err != nil {
		Warnf("capture: mkdir dir failed: %v", err)
		return
	}
	if dir == "" {
		return
	}
	// Write files
	_ = os.WriteFile(filepath.Join(dir, "command.txt"), []byte(strings.Join(cmd.Args, " ")+"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "stdout.txt"), []byte(res.Stdout), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "stderr.txt"), []byte(res.Stderr), 0o644)
	// Sanitized env snapshot
	envMap := map[string]string{}
	for _, kv := range cmd.Env {
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
	// Meta JSON
	meta := struct {
		ExitCode   int       `json:"exit_code"`
		Duration   string    `json:"duration"`
		Started    time.Time `json:"started"`
		Finished   time.Time `json:"finished"`
		Bin        string    `json:"bin"`
		WorkDir    string    `json:"work_dir"`
		Profile    string    `json:"profile"`
		ConfigDir  string    `json:"config_dir"`
		ConfigFile string    `json:"config_file"`
		Args       []string  `json:"args"`
	}{res.ExitCode, res.Duration.String(), start, end, c.BinPath, cmd.Dir, c.Profile, c.ConfigDir, filepath.Join(c.ConfigDir, "kongctl", "config.yaml"), cmd.Args}
	if b, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "meta.json"), b, 0o644)
	}
	// Record the directory for potential observation attachments.
	c.LastCommandDir = dir
}

func slugFromArgs(args []string) string {
	// Take first 1-2 positional tokens until a flag, join with '_'.
	var parts []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			break
		}
		parts = append(parts, a)
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) == 0 {
		return "cmd"
	}
	// sanitize tokens
	s := strings.Join(parts, "_")
	return sanitizeName(s)
}

func hasOutputArg(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-o" {
			return true
		}
		if a == "--output" {
			return true
		}
		if strings.HasPrefix(a, "--output=") {
			return true
		}
		if strings.HasPrefix(a, "-o=") {
			return true
		}
	}
	return false
}

// writeProfileConfig writes a minimal config.yaml under <cfgDir>/kongctl with the given profile defaults.
func writeProfileConfig(cfgDir, profile, output, logLevel string) error {
	appDir := filepath.Join(cfgDir, "kongctl")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return err
	}
	// Minimal YAML; avoid importing extra libs for this.
	// Example:
	// e2e:
	//   output: json
	//   log-level: info
	y := []byte(fmt.Sprintf("%s:\n  output: %s\n  log-level: %s\n", profile, output, logLevel))
	path := filepath.Join(appDir, "config.yaml")
	// Best-effort write; if file exists, do not overwrite.
	if _, err := os.Stat(path); err == nil {
		Debugf("Config exists, not overwriting: %s", path)
		return nil
	}
	if err := os.WriteFile(path, y, fs.FileMode(0o644)); err != nil {
		Warnf("Failed to write profile config: %v", err)
		return err
	}
	Infof("Wrote profile config: %s", path)
	return nil
}

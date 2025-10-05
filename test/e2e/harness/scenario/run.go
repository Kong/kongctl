//go:build e2e

package scenario

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	jmespath "github.com/jmespath/go-jmespath"
	"github.com/kong/kongctl/test/e2e/harness"
	"sigs.k8s.io/yaml"
)

// Run executes the scenario using the e2e harness.
func Run(t *testing.T, scenarioPath string) error {
	t.Helper()
	s, err := Load(scenarioPath)
	if err != nil {
		return err
	}

	harness.RequireBinary(t)
	_ = harness.RequirePAT(t, "e2e")

	cli, err := harness.NewCLIT(t)
	if err != nil {
		return fmt.Errorf("harness init failed: %w", err)
	}
	if len(s.Env) > 0 {
		cli.WithEnv(s.Env)
	}
	if s.Vars == nil {
		s.Vars = map[string]any{}
	}

	// Execute steps
	startIdx := 0
	for i, st := range s.Steps {
		stepName := st.Name
		if strings.TrimSpace(stepName) == "" {
			stepName = fmt.Sprintf("step-%03d", startIdx+i)
		}
		step, err := harness.NewStep(t, cli, stepName)
		if err != nil {
			return err
		}

		// Prepare inputs: copy base into step inputs unless skipInputs
		if !st.SkipInputs {
			if s.BaseInputsPath == "" {
				return fmt.Errorf("baseInputsPath is required")
			}
			basePath := s.BaseInputsPath
			if !filepath.IsAbs(basePath) {
				basePath = filepath.Join(ScenarioRoot(scenarioPath), basePath)
			}
			if err := copyTree(basePath, step.InputsDir); err != nil {
				return fmt.Errorf("copy base inputs failed: %w", err)
			}
		}

		// Apply overlays (dirs) only when inputs are present
		tmplCtx := map[string]any{
			"vars":     s.Vars,
			"scenario": filepath.Dir(scenarioPath),
			"step":     stepName,
			"workdir":  step.InputsDir,
		}
		if !st.SkipInputs {
			for _, od := range st.InputOverlayDirs {
				if err := ApplyOverlayDir(step.InputsDir, filepath.Join(ScenarioRoot(scenarioPath), od), tmplCtx); err != nil {
					return fmt.Errorf("overlay dir %s failed: %w", od, err)
				}
			}
			for _, opf := range st.InputOverlayOpsFiles {
				if err := ApplyOverlayOps(step.InputsDir, filepath.Join(ScenarioRoot(scenarioPath), opf), tmplCtx); err != nil {
					return fmt.Errorf("overlay ops %s failed: %w", opf, err)
				}
			}
			if len(st.InputOverlayOps) > 0 {
				if err := ApplyOverlayOpsInline(step.InputsDir, st.InputOverlayOps, tmplCtx); err != nil {
					return fmt.Errorf("overlay inline ops failed: %w", err)
				}
			}
		}

		// Execute commands
		for j, cmd := range st.Commands {
			cmdName := cmd.Name
			if strings.TrimSpace(cmdName) == "" {
				cmdName = fmt.Sprintf("command-%03d", j)
			}
			// Handle resetOrg synthetic command
			if cmd.ResetOrg {
				if err := step.ResetOrg("scenario"); err != nil {
					return fmt.Errorf("command %s resetOrg failed: %w", cmdName, err)
				}
				// no assertions for reset
				continue
			}
			if cmd.Create != nil {
				if len(cmd.Run) > 0 {
					return fmt.Errorf("command %s: create commands cannot set run", cmdName)
				}
				if cmd.ExpectFail != nil {
					return fmt.Errorf("command %s: expectFailure not supported for create commands", cmdName)
				}
				attempts, interval := effectiveRetry(s.Defaults.Retry, st.Retry, cmd.Retry, Retry{})
				var (
					lastErr error
					result  harness.CreateResourceResult
				)
				for atry := 0; atry < attempts; atry++ {
					if strings.TrimSpace(cmd.Name) != "" {
						cli.OverrideNextCommandSlug(cmd.Name)
					}
					payload, perr := prepareCreatePayload(cmd.Create, scenarioPath, tmplCtx)
					if perr != nil {
						return fmt.Errorf("command %s build payload failed: %w", cmdName, perr)
					}
					result, lastErr = step.CreateResource(
						cmd.Create.Resource,
						payload,
						harness.CreateResourceOptions{
							Slug:         cmdName,
							ExpectStatus: cmd.Create.ExpectStatus,
						},
					)
					if lastErr == nil {
						if err := maybeRecordVar(&s, cmd.Create.RecordVar, result.Parsed, step); err != nil {
							return fmt.Errorf("command %s recordVar failed: %w", cmdName, err)
						}
						step.AppendCheck("PASS: created %s (status=%d)", strings.TrimSpace(cmd.Create.Resource), result.Status)
						break
					}
					if atry+1 < attempts {
						time.Sleep(interval)
					}
				}
				if lastErr != nil {
					return fmt.Errorf("command %s create failed: %w", cmdName, lastErr)
				}
				parseMode := strings.TrimSpace(cmd.ParseAs)
				stdout := string(result.Body)
				if err := writeStdoutFile(cmd.StdoutFile, stdout, tmplCtx, step); err != nil {
					return fmt.Errorf("command %s stdoutFile failed: %w", cmdName, err)
				}
				parentData, err := parseCommandOutput(parseMode, stdout)
				if err != nil {
					mode := parseMode
					if strings.TrimSpace(mode) == "" || strings.EqualFold(mode, "inherit") {
						mode = "json"
					}
					snippet := stdout
					if len(snippet) > 2048 {
						snippet = snippet[:2048] + "…"
					}
					t.Errorf("failed to parse stdout (parseAs=%s) for command %s: %v\nstdout: %q", mode, cmdName, err, snippet)
					return fmt.Errorf("command %s produced unparsable output: %w", cmdName, err)
				}
				if err := executeAssertions(cli, scenarioPath, s, st, cmd, parentData.Value(), step.InputsDir, stepName, cmdName); err != nil {
					return err
				}
				continue
			}

			parseMode := strings.TrimSpace(cmd.ParseAs)
			if err := configureCommandOutput(cli, strings.TrimSpace(cmd.OutputFormat)); err != nil {
				return fmt.Errorf("command %s outputFormat invalid: %w", cmdName, err)
			}
			if strings.TrimSpace(cmd.Name) != "" {
				cli.OverrideNextCommandSlug(cmd.Name)
			}
			// Render args
			args := make([]string, 0, len(cmd.Run))
			for _, a := range cmd.Run {
				ra := renderString(a, tmplCtx)
				args = append(args, ra)
			}
			res, err := cli.Run(context.Background(), args...)
			if cmd.ExpectFail != nil {
				if err == nil {
					return fmt.Errorf("command %s expected failure but succeeded", cmdName)
				}
				if cmd.ExpectFail.ExitCode != nil && res.ExitCode != *cmd.ExpectFail.ExitCode {
					return fmt.Errorf(
						"command %s expected exit code %d but got %d",
						cmdName,
						*cmd.ExpectFail.ExitCode,
						res.ExitCode,
					)
				}
				if substr := strings.TrimSpace(cmd.ExpectFail.Contains); substr != "" {
					combined := res.Stderr + res.Stdout
					if !strings.Contains(combined, substr) {
						return fmt.Errorf(
							"command %s expected failure output to contain %q\nstderr: %s",
							cmdName,
							substr,
							res.Stderr,
						)
					}
				}
				// expected failure satisfied; skip assertions for this command
				continue
			}
			if err != nil {
				snippet := strings.TrimSpace(res.Stderr)
				maxLen := 2048
				if len(snippet) > maxLen {
					snippet = snippet[:maxLen] + "…"
				}
				artifactHint := cli.LastCommandDir
				msg := fmt.Sprintf("command %s failed (exit=%d): %v", cmdName, res.ExitCode, err)
				if snippet != "" {
					msg += fmt.Sprintf("\nstderr:\n%s", snippet)
				}
				if artifactHint != "" {
					msg += fmt.Sprintf("\nartifacts: %s", artifactHint)
				}
				return fmt.Errorf("%s", msg)
			}

			if err := writeStdoutFile(cmd.StdoutFile, res.Stdout, tmplCtx, step); err != nil {
				return fmt.Errorf("command %s stdoutFile failed: %w", cmdName, err)
			}

			// Parent source data (JSON/YAML) used by assertions
			parentData, err := parseCommandOutput(parseMode, res.Stdout)
			if err != nil {
				mode := parseMode
				if strings.TrimSpace(mode) == "" || strings.EqualFold(mode, "inherit") {
					mode = "json"
				}
				snippet := res.Stdout
				if len(snippet) > 2048 {
					snippet = snippet[:2048] + "…"
				}
				t.Errorf("failed to parse stdout (parseAs=%s) for command %s: %v\nstdout: %q", mode, cmdName, err, snippet)
				return fmt.Errorf("command %s produced unparsable output: %w", cmdName, err)
			}

			if err := executeAssertions(cli, scenarioPath, s, st, cmd, parentData.Value(), step.InputsDir, stepName, cmdName); err != nil {
				return err
			}
		}
	}

	return nil
}

func prepareCreatePayload(spec *CreateSpec, scenarioPath string, tmplCtx map[string]any) ([]byte, error) {
	if spec == nil {
		return nil, fmt.Errorf("create spec missing")
	}
	var obj any
	switch {
	case strings.TrimSpace(spec.Payload.File) != "":
		path := spec.Payload.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(ScenarioRoot(scenarioPath), path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		b, err = renderTemplate(b, tmplCtx)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(strings.ToLower(path), ".json") {
			if err := json.Unmarshal(b, &obj); err != nil {
				return nil, err
			}
		} else {
			if err := yaml.Unmarshal(b, &obj); err != nil {
				return nil, err
			}
		}
	case len(spec.Payload.Inline) > 0:
		b, err := yaml.Marshal(spec.Payload.Inline)
		if err != nil {
			return nil, err
		}
		b, err = renderTemplate(b, tmplCtx)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(b, &obj); err != nil {
			return nil, err
		}
	default:
		obj = map[string]any{}
	}
	body, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func maybeRecordVar(s *Scenario, spec *RecordVar, parsed any, step *harness.Step) error {
	if spec == nil {
		return nil
	}
	if strings.TrimSpace(spec.Name) == "" {
		return fmt.Errorf("recordVar name is required")
	}
	if parsed == nil {
		return fmt.Errorf("response body missing for recordVar %q", spec.Name)
	}
	path := strings.TrimSpace(spec.ResponsePath)
	if path == "" {
		path = "id"
	}
	val, err := jmespath.Search(path, parsed)
	if err != nil {
		return err
	}
	if val == nil {
		return fmt.Errorf("recordVar %q path %q not found", spec.Name, path)
	}
	strVal := ""
	switch v := val.(type) {
	case string:
		strVal = v
	default:
		strVal = fmt.Sprint(v)
	}
	if strings.TrimSpace(strVal) == "" {
		return fmt.Errorf("recordVar %q resolved to empty value", spec.Name)
	}
	if s.Vars == nil {
		s.Vars = map[string]any{}
	}
	s.Vars[spec.Name] = strVal
	if step != nil {
		step.AppendCheck("SET VAR: %s=%s", spec.Name, strVal)
	}
	return nil
}

func executeAssertions(cli *harness.CLI, scenarioPath string, sc Scenario, st Step, cmd Command, parent any, workdir string, stepName, cmdName string) error {
	if len(cmd.Assertions) == 0 {
		return nil
	}
	for k, as := range cmd.Assertions {
		asName := as.Name
		if strings.TrimSpace(asName) == "" {
			asName = fmt.Sprintf("assert-%03d", k)
		}
		attempts, interval := effectiveRetry(sc.Defaults.Retry, st.Retry, cmd.Retry, as.Retry)
		var lastErr error
		parentDir := cli.LastCommandDir
		tmplCtx := map[string]any{
			"vars":     sc.Vars,
			"scenario": filepath.Dir(scenarioPath),
			"step":     stepName,
			"workdir":  workdir,
		}
		for atry := 0; atry < attempts; atry++ {
			lastErr = runAssertion(
				cli,
				scenarioPath,
				workdir,
				sc,
				st,
				cmd,
				as,
				parent,
				asName,
				atry,
				parentDir,
				tmplCtx,
			)
			if lastErr == nil {
				break
			}
			if atry+1 < attempts {
				time.Sleep(interval)
			}
		}
		if lastErr != nil {
			return fmt.Errorf("%s/%s/%s: %w", stepName, cmdName, asName, lastErr)
		}
	}
	return nil
}

func configureCommandOutput(cli *harness.CLI, format string) error {
	clean := strings.TrimSpace(format)
	switch strings.ToLower(clean) {
	case "", "inherit":
		return nil
	case "none", "disable":
		cli.DisableNextOutput()
		return nil
	case "json", "yaml", "text":
		cli.OverrideNextOutput(clean)
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

type assertionData struct {
	Map   map[string]any
	Array []any
}

func (a assertionData) Value() any {
	if a.Map != nil {
		return a.Map
	}
	return a.Array
}

func parseCommandOutput(mode string, stdout string) (assertionData, error) {
	if strings.TrimSpace(stdout) == "" {
		return assertionData{}, nil
	}
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "", "json", "inherit":
		return decodeJSONOutput([]byte(stdout))
	case "yaml":
		jb, err := yaml.YAMLToJSON([]byte(stdout))
		if err != nil {
			return assertionData{}, err
		}
		return decodeJSONOutput(jb)
	case "raw":
		return assertionData{Map: map[string]any{"stdout": stdout}}, nil
	default:
		return assertionData{}, fmt.Errorf("unsupported parseAs %q", mode)
	}
}

func decodeJSONOutput(data []byte) (assertionData, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		return assertionData{Map: obj}, nil
	}
	var arr []any
	if err := json.Unmarshal(data, &arr); err == nil {
		return assertionData{Array: arr}, nil
	}
	return assertionData{}, fmt.Errorf("unrecognized structured output")
}

func writeStdoutFile(pathTemplate, stdout string, tmplCtx map[string]any, step *harness.Step) error {
	if strings.TrimSpace(pathTemplate) == "" {
		return nil
	}
	resolved := renderString(pathTemplate, tmplCtx)
	if strings.TrimSpace(resolved) == "" {
		return fmt.Errorf("stdoutFile resolved to empty path")
	}
	outPath := resolved
	if !filepath.IsAbs(outPath) {
		base := ""
		if step != nil {
			base = step.Dir
		}
		if strings.TrimSpace(base) == "" {
			var err error
			base, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		outPath = filepath.Join(base, outPath)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(stdout), 0o644)
}

func renderString(s string, data any) string {
	// lightweight: only replace known tokens for .workdir to reduce template deps in args
	// we still support full templating in overlays.
	if strings.Contains(s, "{{ .workdir }}") {
		if m, ok := data.(map[string]any); ok {
			if wd, ok2 := m["workdir"].(string); ok2 {
				return strings.ReplaceAll(s, "{{ .workdir }}", wd)
			}
		}
	}
	// simple vars usage: {{ .vars.KEY }}
	if strings.Contains(s, "{{ .vars.") {
		if m, ok := data.(map[string]any); ok {
			if vs, ok2 := m["vars"].(map[string]any); ok2 {
				for k, v := range vs {
					ph := fmt.Sprintf("{{ .vars.%s }}", k)
					if sv, ok3 := v.(string); ok3 {
						s = strings.ReplaceAll(s, ph, sv)
					}
				}
			}
		}
	}
	return s
}

func effectiveRetry(d, s, c, a Retry) (int, time.Duration) {
	attempts := d.Attempts
	interval := parseDur(d.Interval)
	if s.Attempts != 0 {
		attempts = s.Attempts
	}
	if s.Interval != "" {
		interval = parseDur(s.Interval)
	}
	if c.Attempts != 0 {
		attempts = c.Attempts
	}
	if c.Interval != "" {
		interval = parseDur(c.Interval)
	}
	if a.Attempts != 0 {
		attempts = a.Attempts
	}
	if a.Interval != "" {
		interval = parseDur(a.Interval)
	}
	if attempts < 1 {
		attempts = 1
	}
	return attempts, interval
}

func parseDur(s string) time.Duration {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		out := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
}

func runAssertion(
	cli *harness.CLI,
	scenarioPath, workdir string,
	sc Scenario,
	st Step,
	cmd Command,
	as Assertion,
	parent any,
	asName string,
	attempt int,
	parentDir string,
	tmplCtx map[string]any,
) error {
	// Resolve source
	var src any
	var err error
	if as.Source.Get != "" {
		// run fresh get, carefully tracking the command capture dir to relocate under retries
		prevCmdDir := cli.LastCommandDir
		var raw any
		if _, err = cli.RunJSON(context.Background(), &raw, "get", as.Source.Get); err != nil {
			return fmt.Errorf("source.get %s failed: %w", as.Source.Get, err)
		}
		src = raw
		getCmdDir := cli.LastCommandDir
		// restore parent command dir to avoid confusing subsequent artifact writes
		cli.LastCommandDir = prevCmdDir
		// Move captured get command under parent command retries to avoid inflating command counts
		if getCmdDir != "" && parentDir != "" {
			dstBase := filepath.Join(parentDir, "assertions", asName, "retries", fmt.Sprintf("%03d", attempt))
			_ = os.MkdirAll(dstBase, 0o755)
			dst := filepath.Join(dstBase, filepath.Base(getCmdDir))
			_ = os.Rename(getCmdDir, dst)
		}
	} else {
		src = parent
	}
	// Apply selector
	var observed any = src
	selUsed := ""
	if strings.TrimSpace(as.Select) != "" {
		// Render template vars inside select before evaluation
		selTpl := as.Select
		if b, rerr := renderTemplate([]byte(selTpl), tmplCtx); rerr == nil {
			selTpl = string(b)
		}
		selUsed = selTpl
		observed, err = jmespath.Search(selTpl, src)
		if err != nil {
			return fmt.Errorf("select eval failed: %w", err)
		}
	}
	// Mask dropKeys (union across scopes)
	keys := unionKeys(sc.Defaults.Mask.DropKeys, st.Mask.DropKeys, cmd.Mask.DropKeys, as.Mask.DropKeys)
	observed = dropKeysDeep(observed, keys)

	// Build expected and comparison target
	var (
		exp     any
		expPath string
		diff    string
	)
	fieldsMode := len(as.Expect.Fields) > 0
	if fieldsMode {
		// Inline fields comparison: extract actual subset and compare against provided fields
		actualSubset := map[string]any{}
		expectedSubset := map[string]any{}
		for k, v := range as.Expect.Fields {
			// Template string values in expected
			var ev any = v
			if sv, ok := v.(string); ok {
				if rb, rerr := renderTemplate([]byte(sv), tmplCtx); rerr == nil {
					ev = string(rb)
				}
			}
			expectedSubset[k] = ev
			// Resolve from observed using JMESPath over the selected object
			av, aerr := jmespath.Search(k, observed)
			if aerr != nil {
				// record nil if not found
				actualSubset[k] = nil
			} else {
				actualSubset[k] = av
			}
		}
		// Normalize numeric types in both subsets to avoid int vs float64 diffs
		actualSubset = normalizeNumbersDeep(actualSubset).(map[string]any)
		expectedSubset = normalizeNumbersDeep(expectedSubset).(map[string]any)
		// Write expected as the subset map for artifacts
		exp = expectedSubset
		// Compute diff on subsets
		diff = cmp.Diff(expectedSubset, actualSubset)
	} else {
		// File-based expected
		if strings.TrimSpace(as.Expect.File) == "" {
			return fmt.Errorf("expect.file not set and no expect.fields provided")
		}
		expPath = filepath.Join(ScenarioRoot(scenarioPath), as.Expect.File)
		var load any
		load, err = readYAMLOrJSON(expPath)
		if err != nil {
			return fmt.Errorf("read expect.file: %w", err)
		}
		for _, ov := range as.Expect.Overlays {
			ovPath := filepath.Join(ScenarioRoot(scenarioPath), ov)
			ovVal, err := readYAMLOrJSON(ovPath)
			if err != nil {
				return fmt.Errorf("read expect overlay: %w", err)
			}
			load = mergeGeneric(load, ovVal)
		}
		// Apply masking to expected load for symmetry
		exp = dropKeysDeep(load, keys)
		diff = cmp.Diff(exp, observed)
	}

	// Prepare assertion artifacts dir (always write observed/expected for clarity)
	baseDir := parentDir
	if baseDir == "" {
		baseDir = cli.LastCommandDir
	}
	asDir := filepath.Join(baseDir, "assertions", asName)
	_ = os.MkdirAll(asDir, 0o755)
	// Persist the selector used for this assertion (post-templating)
	if selUsed != "" {
		_ = os.WriteFile(filepath.Join(asDir, "select.txt"), []byte(selUsed+"\n"), 0o644)
	}
	_ = writeJSON(filepath.Join(asDir, "observed.json"), observed)
	_ = writeJSON(filepath.Join(asDir, "expected.json"), exp)

	// Compare and write single result.txt
	updateMode := os.Getenv("KONGCTL_E2E_UPDATE_EXPECT") == "1"
	pass := diff == "" || updateMode
	// If update mode and there is a diff, update the source expect.file
	if updateMode && diff != "" && !fieldsMode {
		if err := writeJSON(expPath, observed); err != nil {
			return fmt.Errorf("update expect failed: %w", err)
		}
	}
	// Build result.txt contents
	var result strings.Builder
	if pass {
		result.WriteString("pass\n")
	} else {
		result.WriteString("fail\n")
	}
	result.WriteString("------\n")
	if diff == "" {
		result.WriteString("(no diff)\n")
	} else {
		result.WriteString(diff)
		if !strings.HasSuffix(diff, "\n") {
			result.WriteString("\n")
		}
	}
	_ = os.WriteFile(filepath.Join(asDir, "result.txt"), []byte(result.String()), 0o644)
	if pass {
		return nil
	}
	return fmt.Errorf("assertion mismatch; see %s", asDir)
}

func unionKeys(sets ...[]string) []string {
	m := map[string]struct{}{}
	for _, s := range sets {
		for _, k := range s {
			if k == "" {
				continue
			}
			m[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func dropKeysDeep(v any, keys []string) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case map[string]any:
		// remove keys and recurse
		out := make(map[string]any, len(t))
		for k, val := range t {
			if contains(keys, k) {
				continue
			}
			out[k] = dropKeysDeep(val, keys)
		}
		return out
	case []any:
		arr := make([]any, len(t))
		for i := range t {
			arr[i] = dropKeysDeep(t[i], keys)
		}
		return arr
	default:
		return v
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// normalizeNumbersDeep recursively converts all integer and float32-like values to float64.
//
// This normalization is necessary for test assertions that compare data structures
// deserialized from JSON and YAML, as JSON numbers are always float64, while YAML
// numbers may be int, uint, or float types. By converting all numeric types to float64,
// we ensure that numbers compare equal regardless of their original representation.
//
// Note: Converting large uint64 values to float64 may result in precision loss,
// as float64 cannot exactly represent all uint64 values above 2^53. This is
// acceptable for test assertions, but should be considered if exact numeric
// fidelity is required.
func normalizeNumbersDeep(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeNumbersDeep(val)
		}
		return out
	case []any:
		arr := make([]any, len(t))
		for i := range t {
			arr[i] = normalizeNumbersDeep(t[i])
		}
		return arr
	case int:
		return float64(t)
	case int8:
		return float64(t)
	case int16:
		return float64(t)
	case int32:
		return float64(t)
	case int64:
		return float64(t)
	case uint:
		return float64(t)
	case uint8:
		return float64(t)
	case uint16:
		return float64(t)
	case uint32:
		return float64(t)
	case uint64:
		// may lose precision for very large values, acceptable for test assertions
		return float64(t)
	case float32:
		return float64(t)
	default:
		return v
	}
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func readYAMLOrJSON(path string) (any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// If YAML, convert to JSON bytes then unmarshal as generic
	var out any
	if strings.HasSuffix(strings.ToLower(path), ".yaml") || strings.HasSuffix(strings.ToLower(path), ".yml") {
		jb, err := yaml.YAMLToJSON(b)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(jb, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// mergeGeneric merges o into b (returning a new value).
// - map[string]any: deep merge
// - []any: replace
// - other: replace
func mergeGeneric(base, overlay any) any {
	switch b := base.(type) {
	case map[string]any:
		o, ok := overlay.(map[string]any)
		if !ok {
			return overlay
		}
		out := make(map[string]any, len(b))
		for k, v := range b {
			out[k] = v
		}
		for k, v := range o {
			if bv, ok2 := out[k]; ok2 {
				out[k] = mergeGeneric(bv, v)
			} else {
				out[k] = v
			}
		}
		return out
	case []any:
		// replace
		return overlay
	default:
		return overlay
	}
}

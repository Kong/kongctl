//go:build e2e

package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type CreateResourceOptions struct {
	Slug         string
	ExpectStatus int
	PathParams   map[string]string
}

type CreateResourceResult struct {
	Status int
	Body   []byte
	Parsed any
	Method string
	URL    string
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

type resourceEndpoint struct {
	Method    string
	Path      string
	ParamKeys []string
	UseGlobal bool // Use global URL instead of regional URL; necessary for Identity API resources
}

func (re resourceEndpoint) expandPath(params map[string]string) (string, error) {
	path := re.Path
	if len(re.ParamKeys) == 0 {
		if strings.Contains(path, "{") {
			return "", fmt.Errorf("endpoint path %q contains placeholders but no parameters provided", path)
		}
		return path, nil
	}

	for _, key := range re.ParamKeys {
		value, ok := params[key]
		if !ok {
			return "", fmt.Errorf("missing endpoint parameter %q", key)
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return "", fmt.Errorf("endpoint parameter %q resolved to empty string", key)
		}
		path = strings.ReplaceAll(path, "{"+key+"}", url.PathEscape(trimmed))
	}

	if strings.Contains(path, "{") {
		return "", fmt.Errorf("endpoint path still contains placeholders after substitution: %q", path)
	}

	return path, nil
}

var createResourceEndpoints = map[string]resourceEndpoint{
	"portal":        {Method: http.MethodPost, Path: "/v3/portals"},
	"portals":       {Method: http.MethodPost, Path: "/v3/portals"},
	"api":           {Method: http.MethodPost, Path: "/v3/apis"},
	"apis":          {Method: http.MethodPost, Path: "/v3/apis"},
	"auth-strategy": {Method: http.MethodPost, Path: "/v2/application-auth-strategies"},
	"auth_strategy": {Method: http.MethodPost, Path: "/v2/application-auth-strategies"},
	"authstrategy":  {Method: http.MethodPost, Path: "/v2/application-auth-strategies"},
	"control-plane": {Method: http.MethodPost, Path: "/v2/control-planes"},
	"control_plane": {Method: http.MethodPost, Path: "/v2/control-planes"},
	"controlplane":  {Method: http.MethodPost, Path: "/v2/control-planes"},
	"gateway-service": {
		Method:    http.MethodPost,
		Path:      "/v2/control-planes/{controlPlaneId}/core-entities/services",
		ParamKeys: []string{"controlPlaneId"},
	},
	"gateway_service": {
		Method:    http.MethodPost,
		Path:      "/v2/control-planes/{controlPlaneId}/core-entities/services",
		ParamKeys: []string{"controlPlaneId"},
	},
	"gatewayservice": {
		Method:    http.MethodPost,
		Path:      "/v2/control-planes/{controlPlaneId}/core-entities/services",
		ParamKeys: []string{"controlPlaneId"},
	},
	"core-entity-service": {
		Method:    http.MethodPost,
		Path:      "/v2/control-planes/{controlPlaneId}/core-entities/services",
		ParamKeys: []string{"controlPlaneId"},
	},
	"portal-team-developer": {
		Method:    http.MethodPost,
		Path:      "/v3/portals/{portalId}/teams/{teamId}/developers",
		ParamKeys: []string{"portalId", "teamId"},
	},
	"portal_team_developer": {
		Method:    http.MethodPost,
		Path:      "/v3/portals/{portalId}/teams/{teamId}/developers",
		ParamKeys: []string{"portalId", "teamId"},
	},
	"system-account": {Method: http.MethodPost, Path: "/v3/system-accounts", UseGlobal: true},
	"system_account": {Method: http.MethodPost, Path: "/v3/system-accounts", UseGlobal: true},
	"systemaccount":  {Method: http.MethodPost, Path: "/v3/system-accounts", UseGlobal: true},
	"teams":          {Method: http.MethodPost, Path: "/v3/teams", UseGlobal: true},
	"team":           {Method: http.MethodPost, Path: "/v3/teams", UseGlobal: true},
}

func defaultStatusForMethod(method string) int {
	switch method {
	case http.MethodPost:
		return http.StatusCreated
	default:
		return http.StatusOK
	}
}

// CreateResource issues an authenticated Konnect API call to create an unmanaged resource and
// records artifacts under the current step similar to CLI commands.
func (s *Step) CreateResource(resource string, body []byte, opts CreateResourceOptions) (CreateResourceResult, error) {
	var result CreateResourceResult
	if s == nil || s.cli == nil {
		return result, fmt.Errorf("nil step/cli")
	}
	endpoint, ok := createResourceEndpoints[strings.ToLower(strings.TrimSpace(resource))]
	if !ok {
		return result, fmt.Errorf("unsupported resource %q", resource)
	}
	pathParams := opts.PathParams
	if pathParams == nil {
		pathParams = map[string]string{}
	}
	path, err := endpoint.expandPath(pathParams)
	if err != nil {
		return result, err
	}
	slug := opts.Slug
	if strings.TrimSpace(slug) == "" {
		slug = fmt.Sprintf("create-%s", sanitizeName(resource))
	}
	dir, err := s.cli.allocateCommandDir(slug)
	if err != nil {
		return result, err
	}
	baseURL := os.Getenv("KONGCTL_E2E_KONNECT_BASE_URL")
	if baseURL == "" {
		baseURL = "https://us.api.konghq.com"
	}
	if endpoint.UseGlobal {
		baseURL = "https://global.api.konghq.com"
	}
	fullURL := strings.TrimRight(baseURL, "/") + path
	token := os.Getenv("KONGCTL_E2E_KONNECT_PAT")
	if token == "" {
		return result, fmt.Errorf("KONGCTL_E2E_KONNECT_PAT not set")
	}
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, endpoint.Method, fullURL, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("failed to read response body: %w", err)
	}
	duration := time.Since(start)
	end := time.Now()
	result.Status = resp.StatusCode
	result.Body = append([]byte(nil), bodyBytes...)
	result.Method = endpoint.Method
	result.URL = fullURL
	if len(bodyBytes) > 0 {
		var parsed any
		if err := json.Unmarshal(bodyBytes, &parsed); err == nil {
			result.Parsed = parsed
		}
	}
	expect := opts.ExpectStatus
	if expect == 0 {
		expect = defaultStatusForMethod(endpoint.Method)
	}
	if resp.StatusCode != expect {
		snippet := strings.TrimSpace(string(bodyBytes))
		if len(snippet) > 2048 {
			snippet = snippet[:2048] + "…"
		}
		return result, fmt.Errorf("unexpected status %d (expected %d): %s", resp.StatusCode, expect, snippet)
	}
	if dir != "" {
		_ = os.WriteFile(
			filepath.Join(dir, "command.txt"),
			[]byte(fmt.Sprintf("HTTP %s %s\n", endpoint.Method, fullURL)),
			0o644,
		)
		if len(body) > 0 {
			var reqObj any
			if err := json.Unmarshal(body, &reqObj); err == nil {
				_ = s.writePrettyJSON(filepath.Join(dir, "request.json"), reqObj)
			} else {
				_ = os.WriteFile(filepath.Join(dir, "request.json"), body, 0o644)
			}
		}
		_ = os.WriteFile(filepath.Join(dir, "stdout.txt"), bodyBytes, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "stderr.txt"), []byte{}, 0o644)
		envMap := snapshotEnv(os.Environ())
		if b, err := json.MarshalIndent(envMap, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(dir, "env.json"), b, 0o644)
		}
		meta := map[string]any{
			"method":   endpoint.Method,
			"url":      fullURL,
			"status":   resp.StatusCode,
			"duration": duration.String(),
			"started":  start,
			"finished": end,
		}
		if b, err := json.MarshalIndent(meta, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(dir, "meta.json"), b, 0o644)
		}
		if result.Parsed != nil {
			_ = s.writePrettyJSON(filepath.Join(dir, "response.json"), result.Parsed)
		} else if len(bodyBytes) > 0 {
			_ = os.WriteFile(filepath.Join(dir, "response.json"), bodyBytes, 0o644)
		}
		obs := map[string]any{
			"type":   "http_observation",
			"status": resp.StatusCode,
			"data":   result.Parsed,
		}
		_ = s.writePrettyJSON(filepath.Join(dir, "observation.json"), obs)
		// ensure LastCommandDir reflects this synthetic command
		s.cli.LastCommandDir = dir
	}
	return result, nil
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
	if b, err := json.MarshalIndent(snapshotEnv(os.Environ()), "", "  "); err == nil {
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

func snapshotEnv(environ []string) map[string]string {
	envMap := map[string]string{}
	for _, kv := range environ {
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
	return envMap
}

// ResetOrg runs the destructive reset using the default base URL or env overrides.
func (s *Step) ResetOrg(stage string) error {
	return s.ResetOrgForRegions(stage, nil)
}

// ResetOrgForRegions runs the destructive reset across the provided regions/base URLs.
// If regions is nil or empty, the legacy single-base behavior is used.
func (s *Step) ResetOrgForRegions(stage string, regions []string) error {
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
	token := os.Getenv("KONGCTL_E2E_KONNECT_PAT")
	if token == "" {
		obs := map[string]any{"type": "reset_summary", "executed": false, "status": "skipped", "reason": "missing PAT"}
		_ = s.writePrettyJSON(filepath.Join(dir, "observation.json"), obs)
		return nil
	}

	baseURLs, err := resolveResetBaseURLs(regions)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var (
		summaries []map[string]any
		firstErr  error
	)

	for _, baseURL := range baseURLs {
		result, runErr := executeReset(client, baseURL, token)
		details := make([]map[string]any, 0, len(result.Details))
		for _, d := range result.Details {
			details = append(details, map[string]any{
				"api_version": d.APIVersion,
				"endpoint":    d.Endpoint,
				"total":       d.Total,
				"deleted":     d.Deleted,
				"error":       d.Error,
			})
		}
		status := "ok"
		reason := ""
		if runErr != nil {
			status = "error"
			reason = runErr.Error()
			if firstErr == nil {
				firstErr = runErr
			}
		}
		summaries = append(summaries, map[string]any{
			"base_url": baseURL,
			"status":   status,
			"reason":   reason,
			"details":  details,
		})
	}

	overallStatus := "ok"
	overallReason := ""
	if firstErr != nil {
		overallStatus = "error"
		overallReason = firstErr.Error()
	}

	obs := map[string]any{
		"type":     "reset_summary",
		"executed": true,
		"status":   overallStatus,
		"reason":   overallReason,
		"regions":  summaries,
	}
	_ = s.writePrettyJSON(filepath.Join(dir, "observation.json"), obs)
	return firstErr
}

func resolveResetBaseURLs(regions []string) ([]string, error) {
	if len(regions) == 0 {
		baseURL := os.Getenv("KONGCTL_E2E_KONNECT_BASE_URL")
		if baseURL == "" {
			baseURL = "https://us.api.konghq.com"
		}
		return []string{baseURL}, nil
	}

	seen := make(map[string]struct{})
	baseURLs := make([]string, 0, len(regions))
	for _, region := range regions {
		r := strings.TrimSpace(region)
		if r == "" {
			continue
		}
		baseURL := r
		if !strings.HasPrefix(r, "http://") && !strings.HasPrefix(r, "https://") {
			baseURL = fmt.Sprintf("https://%s.api.konghq.com", r)
		}
		if _, ok := seen[baseURL]; ok {
			continue
		}
		seen[baseURL] = struct{}{}
		baseURLs = append(baseURLs, baseURL)
	}
	if len(baseURLs) == 0 {
		return nil, fmt.Errorf("no valid regions provided for reset")
	}
	return baseURLs, nil
}

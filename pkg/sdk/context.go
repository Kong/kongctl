// Package sdk provides helper APIs for Go-based kongctl extensions.
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	konnectsdk "github.com/Kong/sdk-konnect-go"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/auth"
	konglog "github.com/kong/kongctl/internal/log"
)

const (
	// ContextEnvName is the environment variable that points to the extension
	// runtime context file written by the parent kongctl process.
	ContextEnvName = "KONGCTL_EXTENSION_CONTEXT"

	// KonnectPATEnvName carries a transient parent PAT for child extension
	// processes. The runtime context file itself never stores secrets.
	KonnectPATEnvName = "KONGCTL_EXTENSION_KONNECT_PAT" // #nosec G101
)

const defaultProfile = "default"

// RuntimeContext is the extension runtime context provided by the parent
// kongctl process.
type RuntimeContext struct {
	SchemaVersion      int                    `json:"schema_version"`
	MatchedCommandPath MatchedCommandPath     `json:"matched_command_path"`
	Invocation         InvocationContext      `json:"invocation"`
	Resolved           ResolvedContext        `json:"resolved"`
	OutputSettings     OutputContext          `json:"output"`
	Host               HostContext            `json:"host"`
	Session            DispatchSessionContext `json:"session"`
}

type MatchedCommandPath struct {
	ID          string   `json:"id"`
	ExtensionID string   `json:"extension_id"`
	Path        []string `json:"path"`
}

type InvocationContext struct {
	OriginalArgs  []string `json:"original_args"`
	RemainingArgs []string `json:"remaining_args"`
}

type ResolvedContext struct {
	Profile          string `json:"profile"`
	BaseURL          string `json:"base_url"`
	Output           string `json:"output"`
	LogLevel         string `json:"log_level"`
	ColorTheme       string `json:"color_theme,omitempty"`
	ConfigFile       string `json:"config_file"`
	ExtensionDataDir string `json:"extension_data_dir"`
	AuthMode         string `json:"auth_mode"`
	AuthSource       string `json:"auth_source"`
}

type OutputContext struct {
	Format     string    `json:"format"`
	ColorTheme string    `json:"color_theme,omitempty"`
	JQ         JQContext `json:"jq,omitempty"`
}

type JQContext struct {
	Expression string `json:"expression,omitempty"`
	RawOutput  bool   `json:"raw_output,omitempty"`
	Color      string `json:"color,omitempty"`
	ColorTheme string `json:"color_theme,omitempty"`
}

type HostContext struct {
	KongctlPath    string `json:"kongctl_path"`
	KongctlVersion string `json:"kongctl_version"`
}

type DispatchSessionContext struct {
	ID                string   `json:"id"`
	ContributionStack []string `json:"contribution_stack"`
	Depth             int      `json:"depth"`
	MaxDepth          int      `json:"max_depth"`
}

// LoadRuntimeContextFromEnv reads the runtime context file referenced by
// KONGCTL_EXTENSION_CONTEXT.
func LoadRuntimeContextFromEnv() (*RuntimeContext, error) {
	path := strings.TrimSpace(os.Getenv(ContextEnvName))
	if path == "" {
		return nil, fmt.Errorf("%s is not set", ContextEnvName)
	}
	return LoadRuntimeContextFile(path)
}

// LoadRuntimeContextFile reads a kongctl extension runtime context file.
func LoadRuntimeContextFile(path string) (*RuntimeContext, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open runtime context: %w", err)
	}
	defer file.Close()

	var runtimeCtx RuntimeContext
	if err := json.NewDecoder(file).Decode(&runtimeCtx); err != nil {
		return nil, fmt.Errorf("decode runtime context: %w", err)
	}
	runtimeCtx.applyDefaults()
	return &runtimeCtx, nil
}

func (r *RuntimeContext) applyDefaults() {
	if strings.TrimSpace(r.Resolved.Output) == "" {
		r.Resolved.Output = cmdcommon.DefaultOutputFormat
	}
	if strings.TrimSpace(r.Resolved.LogLevel) == "" {
		r.Resolved.LogLevel = cmdcommon.DefaultLogLevel
	}
	if strings.TrimSpace(r.OutputSettings.Format) == "" {
		r.OutputSettings.Format = r.Resolved.Output
	}
	if strings.TrimSpace(r.OutputSettings.ColorTheme) == "" {
		r.OutputSettings.ColorTheme = r.Resolved.ColorTheme
	}
	if strings.TrimSpace(r.OutputSettings.JQ.Color) == "" {
		r.OutputSettings.JQ.Color = cmdcommon.DefaultColorMode
	}
	if strings.TrimSpace(r.OutputSettings.JQ.ColorTheme) == "" {
		r.OutputSettings.JQ.ColorTheme = jqoutput.DefaultTheme
	}
}

// Args returns the arguments passed through to the extension executable after
// host-owned flags have been consumed.
func (r *RuntimeContext) Args() []string {
	return append([]string(nil), r.Invocation.RemainingArgs...)
}

// OriginalArgs returns the original matched kongctl command path and arguments.
func (r *RuntimeContext) OriginalArgs() []string {
	return append([]string(nil), r.Invocation.OriginalArgs...)
}

// DataDir returns the stable extension-owned data directory.
func (r *RuntimeContext) DataDir() string {
	return r.Resolved.ExtensionDataDir
}

// KongctlPath returns the parent kongctl executable path, falling back to
// "kongctl" when the runtime context did not provide one.
func (r *RuntimeContext) KongctlPath() string {
	path := strings.TrimSpace(r.Host.KongctlPath)
	if path == "" {
		return "kongctl"
	}
	return path
}

// KongctlCommand creates a session-aware child kongctl command.
func (r *RuntimeContext) KongctlCommand(ctx context.Context, args ...string) *exec.Cmd {
	// #nosec G204 -- extensions intentionally reenter the kongctl executable
	// path recorded by the parent runtime context.
	command := exec.CommandContext(ctx, r.KongctlPath(), args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = os.Environ()
	return command
}

// RunKongctl runs a session-aware child kongctl command.
func (r *RuntimeContext) RunKongctl(ctx context.Context, args ...string) error {
	return r.KongctlCommand(ctx, args...).Run()
}

// KonnectSDK returns an authenticated sdk-konnect-go client configured from
// the parent kongctl invocation context.
func (r *RuntimeContext) KonnectSDK(_ context.Context) (*konnectsdk.SDK, error) {
	cfg, err := r.loadConfig()
	if err != nil {
		return nil, err
	}

	logger := r.newLogger()

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return nil, err
	}

	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return nil, err
	}

	timeout, err := konnectcommon.ResolveHTTPTimeout(cfg)
	if err != nil {
		return nil, err
	}

	transportOptions, err := konnectcommon.ResolveHTTPTransportOptions(cfg)
	if err != nil {
		return nil, err
	}

	sdk, _, err := auth.GetAuthenticatedClient(baseURL, token, timeout, transportOptions, logger)
	if err != nil {
		return nil, err
	}
	return sdk, nil
}

func (r *RuntimeContext) loadConfig() (config.Hook, error) {
	defaultPath, err := config.GetDefaultConfigFilePath()
	if err != nil {
		return nil, err
	}

	configPath := strings.TrimSpace(r.Resolved.ConfigFile)
	if configPath == "" {
		configPath = defaultPath
	}
	profile := strings.TrimSpace(r.Resolved.Profile)
	if profile == "" {
		profile = defaultProfile
	}

	cfg, err := config.GetConfig(configPath, profile, defaultPath)
	if err != nil {
		return nil, err
	}
	r.applyConfigOverlay(cfg)
	return cfg, nil
}

func (r *RuntimeContext) applyConfigOverlay(cfg config.Hook) {
	if value := strings.TrimSpace(r.Resolved.BaseURL); value != "" {
		cfg.SetString(konnectcommon.BaseURLConfigPath, value)
	}
	if value := strings.TrimSpace(r.Resolved.Output); value != "" {
		cfg.SetString(cmdcommon.OutputConfigPath, value)
	}
	if value := strings.TrimSpace(r.Resolved.LogLevel); value != "" {
		cfg.SetString(cmdcommon.LogLevelConfigPath, value)
	}
	if value := strings.TrimSpace(r.Resolved.ColorTheme); value != "" {
		cfg.SetString(cmdcommon.ColorThemeConfigPath, value)
	}
	if value := strings.TrimSpace(r.OutputSettings.JQ.Expression); value != "" {
		cfg.SetString(jqoutput.DefaultExpressionConfigPath, value)
	}
	if value := strings.TrimSpace(r.OutputSettings.JQ.Color); value != "" {
		cfg.SetString(jqoutput.ColorEnabledConfigPath, value)
	}
	if value := strings.TrimSpace(r.OutputSettings.JQ.ColorTheme); value != "" {
		cfg.SetString(jqoutput.ColorThemeConfigPath, value)
	}
	cfg.Set(jqoutput.RawOutputConfigPath, r.OutputSettings.JQ.RawOutput)

	if pat := strings.TrimSpace(os.Getenv(KonnectPATEnvName)); pat != "" {
		cfg.SetString(konnectcommon.PATConfigPath, pat)
	}
}

func (r *RuntimeContext) newLogger() *slog.Logger {
	level := konglog.ConfigLevelStringToSlogLevel(r.Resolved.LogLevel)
	handler := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

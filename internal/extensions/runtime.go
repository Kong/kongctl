package extensions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/build"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/meta"
)

const (
	ContextEnvName = "KONGCTL_EXTENSION_CONTEXT"
	MaxDepth       = 5
)

type RuntimeContext struct {
	SchemaVersion      int                `json:"schema_version"`
	MatchedCommandPath MatchedCommandPath `json:"matched_command_path"`
	Invocation         InvocationContext  `json:"invocation"`
	Resolved           ResolvedContext    `json:"resolved"`
	Host               HostContext        `json:"host"`
	Session            SessionContext     `json:"session"`
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
	ConfigFile       string `json:"config_file"`
	ExtensionDataDir string `json:"extension_data_dir"`
	AuthMode         string `json:"auth_mode"`
	AuthSource       string `json:"auth_source"`
}

type HostContext struct {
	KongctlPath    string `json:"kongctl_path"`
	KongctlVersion string `json:"kongctl_version"`
}

type SessionContext struct {
	ID                string   `json:"id"`
	ContributionStack []string `json:"contribution_stack"`
	Depth             int      `json:"depth"`
	MaxDepth          int      `json:"max_depth"`
}

func LoadRuntimeContextFromEnv() (*RuntimeContext, error) {
	path := strings.TrimSpace(os.Getenv(ContextEnvName))
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var ctx RuntimeContext
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&ctx); err != nil {
		return nil, err
	}
	if ctx.SchemaVersion != ManifestSchemaV1 {
		return nil, fmt.Errorf("unsupported extension context schema_version %d", ctx.SchemaVersion)
	}
	return &ctx, nil
}

func (s Store) Dispatch(
	ctx context.Context,
	streams *iostreams.IOStreams,
	cfg config.Hook,
	buildInfo *build.Info,
	ext Extension,
	contribution CommandPath,
	originalArgs []string,
	remainingArgs []string,
	profileOverride string,
) error {
	runtimePath, err := s.ResolveRuntime(ext)
	if err != nil {
		return err
	}

	contributionID := contribution.ID
	parent, err := LoadRuntimeContextFromEnv()
	if err != nil {
		return err
	}
	stack := []string{}
	depth := 1
	sessionID := ""
	if parent != nil {
		stack = append(stack, parent.Session.ContributionStack...)
		depth = parent.Session.Depth + 1
		sessionID = parent.Session.ID
	}
	if slices.Contains(stack, contributionID) {
		return fmt.Errorf("extension recursion detected for contribution %q", contributionID)
	}
	if depth > MaxDepth {
		return fmt.Errorf("extension dispatch depth %d exceeds max depth %d", depth, MaxDepth)
	}
	stack = append(stack, contributionID)
	if sessionID == "" {
		sessionID, err = randomSessionID()
		if err != nil {
			return err
		}
	}

	dataDir, err := s.DataDir(ext.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}

	runtimeContext, err := s.buildRuntimeContext(
		cfg,
		buildInfo,
		ext,
		contribution,
		originalArgs,
		remainingArgs,
		profileOverride,
		dataDir,
		sessionID,
		stack,
		depth,
	)
	if err != nil {
		return err
	}

	contextPath, cleanup, err := s.writeRuntimeContext(runtimeContext)
	if err != nil {
		return err
	}
	defer cleanup()

	command := exec.CommandContext(ctx, runtimePath, remainingArgs...)
	command.Stdin = streams.In
	command.Stdout = streams.Out
	command.Stderr = streams.ErrOut
	command.Env = append(os.Environ(), ContextEnvName+"="+contextPath)
	return command.Run()
}

func (s Store) buildRuntimeContext(
	cfg config.Hook,
	buildInfo *build.Info,
	ext Extension,
	contribution CommandPath,
	originalArgs []string,
	remainingArgs []string,
	profileOverride string,
	dataDir string,
	sessionID string,
	stack []string,
	depth int,
) (RuntimeContext, error) {
	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return RuntimeContext{}, err
	}
	output := strings.TrimSpace(cfg.GetString(cmdcommon.OutputConfigPath))
	if output == "" {
		output = cmdcommon.DefaultOutputFormat
	}
	logLevel := strings.TrimSpace(cfg.GetString(cmdcommon.LogLevelConfigPath))
	if logLevel == "" {
		logLevel = cmdcommon.DefaultLogLevel
	}
	profile := strings.TrimSpace(profileOverride)
	if profile == "" {
		profile = cfg.GetProfile()
	}
	version := meta.DefaultCLIVersion
	if buildInfo != nil && strings.TrimSpace(buildInfo.Version) != "" {
		version = buildInfo.Version
	}
	kongctlPath, _ := os.Executable()
	authMode, authSource := authMetadata(cfg)

	return RuntimeContext{
		SchemaVersion: ManifestSchemaV1,
		MatchedCommandPath: MatchedCommandPath{
			ID:          contribution.ID,
			ExtensionID: ext.ID,
			Path:        CommandPathNames(contribution),
		},
		Invocation: InvocationContext{
			OriginalArgs:  append([]string(nil), originalArgs...),
			RemainingArgs: append([]string(nil), remainingArgs...),
		},
		Resolved: ResolvedContext{
			Profile:          profile,
			BaseURL:          baseURL,
			Output:           output,
			LogLevel:         logLevel,
			ConfigFile:       cfg.GetPath(),
			ExtensionDataDir: dataDir,
			AuthMode:         authMode,
			AuthSource:       authSource,
		},
		Host: HostContext{
			KongctlPath:    kongctlPath,
			KongctlVersion: version,
		},
		Session: SessionContext{
			ID:                sessionID,
			ContributionStack: append([]string(nil), stack...),
			Depth:             depth,
			MaxDepth:          MaxDepth,
		},
	}, nil
}

func (s Store) writeRuntimeContext(runtimeContext RuntimeContext) (string, func(), error) {
	sessionDir := filepath.Join(s.RuntimeDir(), runtimeContext.Session.ID)
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return "", nil, err
	}
	if err := os.Chmod(sessionDir, 0o700); err != nil {
		return "", nil, err
	}
	contextPath := filepath.Join(sessionDir, "context.json")
	if err := writeJSON(contextPath, runtimeContext); err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = os.RemoveAll(sessionDir)
	}
	return contextPath, cleanup, nil
}

func authMetadata(cfg config.Hook) (string, string) {
	if strings.TrimSpace(cfg.GetString(konnectcommon.PATConfigPath)) != "" {
		return "pat", "flag_or_config"
	}
	if strings.TrimSpace(cfg.GetString(konnectcommon.AuthTokenConfigPath)) != "" {
		return "device", "token_store"
	}
	return "unknown", "none"
}

func randomSessionID() (string, error) {
	var data [6]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}

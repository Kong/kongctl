package supportdata

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"github.com/kong/kong-deployment-toolkit/pkg/collector"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
)

// Config path constants for support data settings
const (
	configOutputDir      = "support_data.output_dir"
	configSanitize       = "support_data.sanitize"
	configLineLimit      = "support_data.line_limit"
	configLogsSince      = "support_data.logs_since"
	configRedactTerms    = "support_data.redact_terms"
	configDisableKDD     = "support_data.disable_kdd"
	configDumpWorkspaces = "support_data.dump_workspace_configs"
	configTLSSkipVerify  = "support_data.tls_skip_verify"
	configCACertPath     = "support_data.ca_cert_path"
)

// Runtime identifiers accepted by the --runtime flag.
const (
	runtimeKubernetes = "kubernetes"
	runtimeDocker     = "docker"
	runtimeVM         = "vm"
)

// CommonFlags holds flags shared between konnect and on-prem targets.
type CommonFlags struct {
	OutputDir      string
	Sanitize       bool
	LineLimit      int64
	LogsSince      string
	RedactTerms    []string
	DisableKDD     bool
	DumpWorkspaces bool
	TLSSkipVerify  bool
	CACertPath     string
}

// RegisterCommonFlags adds common flags to a command.
func RegisterCommonFlags(cmd *cobra.Command, flags *CommonFlags) {
	f := cmd.Flags()

	f.StringVar(&flags.OutputDir, "output-dir", "",
		fmt.Sprintf(`Directory to write the support data archive.
Defaults to current directory if not specified.
- Config path: [ %s ]`, configOutputDir))

	f.BoolVar(&flags.Sanitize, "sanitize", false,
		fmt.Sprintf(`Sanitize collected configurations.
Removes sensitive data like credentials and tokens.
Recommended when sharing data externally.
- Config path: [ %s ]`, configSanitize))

	f.Int64Var(&flags.LineLimit, "line-limit", 0,
		fmt.Sprintf(`Maximum number of log lines to collect per source.
0 means use default (1000 lines).
- Config path: [ %s ]`, configLineLimit))

	f.StringVar(&flags.LogsSince, "logs-since", "",
		fmt.Sprintf(`Collect logs since this time.
Use a duration string (e.g., "1h", "30m", "600s").
For Kubernetes, the duration is converted internally to seconds.
- Config path: [ %s ]`, configLogsSince))

	f.StringSliceVar(&flags.RedactTerms, "redact", nil,
		fmt.Sprintf(`Terms to redact from collected logs.
Can be specified multiple times or comma-separated.
Example: --redact password,secret,api_key
- Config path: [ %s ]`, configRedactTerms))

	f.BoolVar(&flags.DisableKDD, "disable-kdd", false,
		fmt.Sprintf(`Disable Kong Declarative Dump collection.
Use when Admin API is unavailable or KDD is not needed.
- Config path: [ %s ]`, configDisableKDD))

	f.BoolVar(&flags.DumpWorkspaces, "dump-workspaces", false,
		fmt.Sprintf(`Include per-workspace configuration dumps.
Creates separate config files for each workspace.
- Config path: [ %s ]`, configDumpWorkspaces))

	f.BoolVar(&flags.TLSSkipVerify, "tls-skip-verify", false,
		fmt.Sprintf(`Skip TLS certificate verification for the Kong Admin API.
WARNING: this allows an on-path attacker to intercept credentials
sent to the Admin API. Prefer --ca-cert for private CAs.
- Config path: [ %s ]`, configTLSSkipVerify))

	f.StringVar(&flags.CACertPath, "ca-cert", "",
		fmt.Sprintf(`Path to a PEM-encoded CA certificate bundle used to verify
the Kong Admin API's TLS certificate.
Use for self-signed or private-CA deployments.
- Config path: [ %s ]`, configCACertPath))
}

// validateRuntimeNamespace returns an error when the Kubernetes runtime is
// selected without a namespace, which is required to locate pods.
func validateRuntimeNamespace(collectorCfg *collector.Config) error {
	if collectorCfg.Runtime == runtimeKubernetes && collectorCfg.Namespace == "" {
		return fmt.Errorf("--namespace is required when runtime is kubernetes")
	}
	return nil
}

// applyLogsSince sets the per-runtime log window from a single user-supplied
// value. Docker accepts RFC3339 and Unix timestamps in addition to durations,
// so a value that is not a Go duration is still passed through to it.
// Kubernetes only understands a second count, so it is reset rather than left
// holding a stale value from an earlier configuration layer.
func applyLogsSince(since string, collectorCfg *collector.Config) {
	collectorCfg.DockerLogsSince = since
	if d, err := time.ParseDuration(since); err == nil {
		collectorCfg.K8sLogsSinceSeconds = int64(d.Seconds())
		return
	}
	collectorCfg.K8sLogsSinceSeconds = 0
}

// ApplyCommonConfig applies common configuration from config file to collector config.
func ApplyCommonConfig(cfg config.Hook, collectorCfg *collector.Config) {
	if dir := cfg.GetString(configOutputDir); dir != "" {
		collectorCfg.OutputDir = dir
	}
	if cfg.GetBool(configSanitize) {
		collectorCfg.SanitizeConfigs = true
	}
	if limit := cfg.GetInt(configLineLimit); limit > 0 {
		collectorCfg.LineLimit = int64(limit)
	}
	if since := cfg.GetString(configLogsSince); since != "" {
		applyLogsSince(since, collectorCfg)
	}
	if terms := cfg.GetStringSlice(configRedactTerms); len(terms) > 0 {
		collectorCfg.RedactTerms = terms
	}
	if cfg.GetBool(configDisableKDD) {
		collectorCfg.DisableKDD = true
	}
	if cfg.GetBool(configDumpWorkspaces) {
		collectorCfg.DumpWorkspaceConfigs = true
	}
	if cfg.GetBool(configTLSSkipVerify) {
		collectorCfg.TLSSkipVerify = true
	}
	if path := cfg.GetString(configCACertPath); path != "" {
		collectorCfg.CACertPath = path
	}

	// Forward log level to collector. Route collector's logrus output to stderr
	// so it doesn't mix with kongctl's structured command output on stdout.
	logLevel := cfg.GetString(common.LogLevelConfigPath)
	if logLevel == "debug" || logLevel == "trace" {
		collectorCfg.Debug = true
		collectorCfg.Logger = os.Stderr
	} else {
		collectorCfg.Logger = io.Discard
	}
}

// ApplyCommonFlags applies common flag values to collector config.
func ApplyCommonFlags(flags *CommonFlags, collectorCfg *collector.Config) {
	if flags.OutputDir != "" {
		collectorCfg.OutputDir = flags.OutputDir
	}
	if flags.Sanitize {
		collectorCfg.SanitizeConfigs = true
	}
	if flags.LineLimit > 0 {
		collectorCfg.LineLimit = flags.LineLimit
	}
	if flags.LogsSince != "" {
		applyLogsSince(flags.LogsSince, collectorCfg)
	}
	if len(flags.RedactTerms) > 0 {
		collectorCfg.RedactTerms = flags.RedactTerms
	}
	if flags.DisableKDD {
		collectorCfg.DisableKDD = true
	}
	if flags.DumpWorkspaces {
		collectorCfg.DumpWorkspaceConfigs = true
	}
	if flags.TLSSkipVerify {
		collectorCfg.TLSSkipVerify = true
	}
	if flags.CACertPath != "" {
		collectorCfg.CACertPath = flags.CACertPath
	}

	// Default sanitize to true when dumping workspaces
	if collectorCfg.DumpWorkspaceConfigs && !collectorCfg.SanitizeConfigs {
		collectorCfg.SanitizeConfigs = true
	}
}

// FormatOutput formats the collection result based on output format.
func FormatOutput(streams *iostreams.IOStreams, format common.OutputFormat, result *collector.Result) error {
	switch format {
	case common.JSON:
		enc := json.NewEncoder(streams.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(summarizeResult(result))

	case common.YAML:
		yamlBytes, err := yaml.Marshal(summarizeResult(result))
		if err != nil {
			return fmt.Errorf("failed to marshal collection result: %w", err)
		}
		_, err = streams.Out.Write(yamlBytes)
		return err

	case common.TEXT, common.HELM, common.TOKEN, common.ENV:
		return formatTextOutput(streams, result)

	default:
		return formatTextOutput(streams, result)
	}
}

// summarizeResult builds the structured view of a collection result shared by
// the JSON and YAML output formats.
func summarizeResult(result *collector.Result) map[string]any {
	return map[string]any{
		"archive_path":    result.ArchivePath,
		"runtime":         result.Runtime,
		"files_collected": len(result.CollectedFiles),
		"warnings":        warningsToStrings(result.Warnings),
	}
}

// formatTextOutput renders the collection result as human-readable text.
func formatTextOutput(streams *iostreams.IOStreams, result *collector.Result) error {
	fmt.Fprintf(streams.Out, "Support data archive: %s\n", result.ArchivePath)
	fmt.Fprintf(streams.Out, "Runtime: %s\n", result.Runtime)
	fmt.Fprintf(streams.Out, "Files collected: %d\n", len(result.CollectedFiles))
	if len(result.Warnings) > 0 {
		fmt.Fprintln(streams.ErrOut, "\nWarnings:")
		for _, warn := range result.Warnings {
			fmt.Fprintf(streams.ErrOut, "  - %v\n", warn)
		}
	}
	return nil
}

// warningsToStrings converts a slice of errors to a slice of strings.
func warningsToStrings(warnings []error) []string {
	strs := make([]string, len(warnings))
	for i, w := range warnings {
		strs[i] = w.Error()
	}
	return strs
}

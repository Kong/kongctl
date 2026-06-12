package supportdata

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/kong/kong-deployment-toolkit/pkg/collector"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
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
		collectorCfg.DockerLogsSince = since
		if d, err := time.ParseDuration(since); err == nil {
			collectorCfg.K8sLogsSinceSeconds = int64(d.Seconds())
		}
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
		collectorCfg.DockerLogsSince = flags.LogsSince
		if d, err := time.ParseDuration(flags.LogsSince); err == nil {
			collectorCfg.K8sLogsSinceSeconds = int64(d.Seconds())
		}
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

	// Default sanitize to true when dumping workspaces
	if collectorCfg.DumpWorkspaceConfigs && !collectorCfg.SanitizeConfigs {
		collectorCfg.SanitizeConfigs = true
	}
}

// FormatOutput formats the collection result based on output format.
func FormatOutput(format common.OutputFormat, result *collector.Result) error {
	switch format {
	case common.JSON:
		output := map[string]interface{}{
			"archive_path":    result.ArchivePath,
			"runtime":         result.Runtime,
			"files_collected": len(result.CollectedFiles),
			"warnings":        warningsToStrings(result.Warnings),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)

	case common.YAML:
		output := map[string]interface{}{
			"archive_path":    result.ArchivePath,
			"runtime":         result.Runtime,
			"files_collected": len(result.CollectedFiles),
			"warnings":        warningsToStrings(result.Warnings),
		}
		return yaml.NewEncoder(os.Stdout).Encode(output)

	default: // TEXT
		fmt.Printf("Support data archive: %s\n", result.ArchivePath)
		fmt.Printf("Runtime: %s\n", result.Runtime)
		fmt.Printf("Files collected: %d\n", len(result.CollectedFiles))
		if len(result.Warnings) > 0 {
			fmt.Println("\nWarnings:")
			for _, warn := range result.Warnings {
				fmt.Fprintf(os.Stderr, "  - %v\n", warn)
			}
		}
		return nil
	}
}

// warningsToStrings converts a slice of errors to a slice of strings.
func warningsToStrings(warnings []error) []string {
	strs := make([]string, len(warnings))
	for i, w := range warnings {
		strs[i] = w.Error()
	}
	return strs
}

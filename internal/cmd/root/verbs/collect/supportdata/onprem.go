package supportdata

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kong/kong-deployment-toolkit/pkg/collector"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
)

// Config path constants for on-prem settings
const (
	configOnPremRuntime      = "support_data.on_prem.runtime"
	configOnPremKongAddr     = "support_data.on_prem.kong_addr"
	configOnPremRBACHeaders  = "support_data.on_prem.rbac_headers"
	configOnPremPrefixDir    = "support_data.on_prem.prefix_dir"
	configOnPremTargetImages = "support_data.on_prem.target_images"
	configOnPremTargetPods   = "support_data.on_prem.target_pods"
	configOnPremNamespace    = "support_data.on_prem.namespace"
)

var (
	onPremUse = "on-prem"

	onPremShort = i18n.T("root.verbs.collect.supportdata.onprem.onPremShort",
		"Collect support data from self-managed Kong Gateway")

	onPremLong = normalizers.LongDesc(i18n.T("root.verbs.collect.supportdata.onprem.onPremLong",
		`Collect logs, configuration, and system information from self-managed
Kong Gateway deployments running on Docker, Kubernetes, or VM.

Runtime is auto-detected if not specified. The command produces a ZIP archive
containing Kong configuration, container/pod logs, and system information.`))

	onPremExamples = normalizers.Examples(i18n.T("root.verbs.collect.supportdata.onprem.onPremExamples",
		fmt.Sprintf(`
        # Auto-detect runtime and collect support data
        %[1]s collect support-data on-prem

        # Collect from Kubernetes with specific namespace
        %[1]s collect support-data on-prem --runtime kubernetes --namespace kong

        # Collect from Kubernetes with specific output directory
        %[1]s collect support-data on-prem --runtime kubernetes --namespace kong --output-dir ./support

        # Collect with sanitization (removes sensitive data)
        %[1]s collect support-data on-prem --sanitize

        # Collect from specific pods only
        %[1]s collect support-data on-prem --runtime kubernetes --namespace kong --target-pods kong-gateway-0,kong-gateway-1

        # Collect with custom log line limit
        %[1]s collect support-data on-prem --line-limit 5000

        # Collect with RBAC authentication
        %[1]s collect support-data on-prem --rbac-header "Kong-Admin-Token:my-secret-token"

        # Collect from VM with custom prefix directory
        %[1]s collect support-data on-prem --runtime vm --prefix-dir /opt/kong
        `, meta.CLIName)))
)

// onPremFlags holds on-prem specific flags.
type onPremFlags struct {
	Runtime      string
	KongAddr     string
	RBACHeaders  []string
	PrefixDir    string
	TargetImages []string
	TargetPods   []string
	Namespace    string
}

// NewOnPremCmd creates the on-prem target command.
func NewOnPremCmd() *cobra.Command {
	var commonFlags CommonFlags
	var flags onPremFlags

	cmd := &cobra.Command{
		Use:     onPremUse,
		Aliases: []string{"onprem", "self-managed", "gateway"},
		Short:   onPremShort,
		Long:    onPremLong,
		Example: onPremExamples,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOnPrem(cmd.Context(), cmd, args, &commonFlags, &flags)
		},
	}

	// Register common flags
	RegisterCommonFlags(cmd, &commonFlags)

	// Register on-prem specific flags
	registerOnPremFlags(cmd, &flags)

	return cmd
}

func registerOnPremFlags(cmd *cobra.Command, flags *onPremFlags) {
	f := cmd.Flags()

	f.StringVar(&flags.Runtime, "runtime", "",
		fmt.Sprintf(`Runtime environment to collect from.
Valid values: docker, kubernetes, vm
If not specified, runtime is auto-detected.
- Config path: [ %s ]`, configOnPremRuntime))

	f.StringVar(&flags.KongAddr, "kong-addr", "",
		fmt.Sprintf(`Kong Admin API address.
Example: http://localhost:8001
- Config path: [ %s ]`, configOnPremKongAddr))

	f.StringSliceVarP(&flags.RBACHeaders, "rbac-header", "H", nil,
		fmt.Sprintf(`RBAC headers for Kong Admin API authentication.
Format: Header-Name:value (can be specified multiple times).
Example: --rbac-header "Kong-Admin-Token:my-token"
- Config path: [ %s ]`, configOnPremRBACHeaders))

	f.StringVarP(&flags.PrefixDir, "prefix-dir", "k", "",
		fmt.Sprintf(`Kong prefix directory for VM log collection.
Used to locate log files on VM deployments.
Default: /usr/local/kong
- Config path: [ %s ]`, configOnPremPrefixDir))

	f.StringSliceVar(&flags.TargetImages, "target-images", nil,
		fmt.Sprintf(`Container images to collect from.
Defaults to kong-gateway and kubernetes-ingress-controller.
Example: --target-images kong-gateway,custom-kong
- Config path: [ %s ]`, configOnPremTargetImages))

	f.StringSliceVar(&flags.TargetPods, "target-pods", nil,
		fmt.Sprintf(`Specific pod names to collect from (Kubernetes only).
If not specified, collects from all matching pods.
Example: --target-pods kong-gateway-0,kong-gateway-1
- Config path: [ %s ]`, configOnPremTargetPods))

	f.StringVarP(&flags.Namespace, "namespace", "n", "",
		fmt.Sprintf(`Kubernetes namespace to collect from.
Required when runtime is kubernetes.
Example: --namespace kong
- Config path: [ %s ]`, configOnPremNamespace))
}

func runOnPrem(
	ctx context.Context,
	cmd *cobra.Command,
	args []string,
	commonFlags *CommonFlags,
	flags *onPremFlags,
) error {
	helper := cmdpkg.BuildHelper(cmd, args)

	cfg, err := helper.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get configuration: %w", err)
	}

	// Build collector config
	collectorCfg := buildOnPremConfig(cfg, commonFlags, flags)

	// Validate namespace requirement for Kubernetes runtime
	if collectorCfg.Runtime == "kubernetes" && collectorCfg.Namespace == "" {
		return fmt.Errorf("--namespace is required when runtime is kubernetes")
	}

	// Execute the collection
	result, err := collector.Collect(ctx, collectorCfg)
	if err != nil {
		return fmt.Errorf("support data collection failed: %w", err)
	}

	// Get output format
	outputFormat, err := helper.GetOutputFormat()
	if err != nil {
		outputFormat = common.TEXT
	}

	return FormatOutput(outputFormat, result)
}

func buildOnPremConfig(
	cfg config.Hook,
	commonFlags *CommonFlags,
	flags *onPremFlags,
) *collector.Config {
	// Start with library defaults
	collectorCfg := collector.DefaultConfig()

	// Layer 1: Apply common config from file
	ApplyCommonConfig(cfg, collectorCfg)

	// Layer 2: Apply on-prem config from file
	if r := cfg.GetString(configOnPremRuntime); r != "" {
		collectorCfg.Runtime = r
	}
	if addr := cfg.GetString(configOnPremKongAddr); addr != "" {
		collectorCfg.KongAddr = addr
	}
	if headers := cfg.GetStringSlice(configOnPremRBACHeaders); len(headers) > 0 {
		collectorCfg.RBACHeaders = headers
	}
	if dir := cfg.GetString(configOnPremPrefixDir); dir != "" {
		collectorCfg.PrefixDir = dir
	}
	if images := cfg.GetStringSlice(configOnPremTargetImages); len(images) > 0 {
		collectorCfg.TargetImages = images
	}
	if pods := cfg.GetStringSlice(configOnPremTargetPods); len(pods) > 0 {
		collectorCfg.TargetPods = pods
	}
	if ns := cfg.GetString(configOnPremNamespace); ns != "" {
		collectorCfg.Namespace = ns
	}

	// Layer 3: Apply common flags (override config)
	ApplyCommonFlags(commonFlags, collectorCfg)

	// Layer 4: Apply on-prem flags (override config)
	if flags.Runtime != "" {
		collectorCfg.Runtime = flags.Runtime
	}
	if flags.KongAddr != "" {
		collectorCfg.KongAddr = flags.KongAddr
	}
	if len(flags.RBACHeaders) > 0 {
		collectorCfg.RBACHeaders = flags.RBACHeaders
	}
	if flags.PrefixDir != "" {
		collectorCfg.PrefixDir = flags.PrefixDir
	}
	if len(flags.TargetImages) > 0 {
		collectorCfg.TargetImages = flags.TargetImages
	}
	if len(flags.TargetPods) > 0 {
		collectorCfg.TargetPods = flags.TargetPods
	}
	if flags.Namespace != "" {
		collectorCfg.Namespace = flags.Namespace
	}

	return collectorCfg
}

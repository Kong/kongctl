package supportdata

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kong/kong-deployment-toolkit/pkg/collector"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
)

// Config path constants for Konnect settings
const (
	configKonnectControlPlane = "support_data.konnect.control_plane"
	configKonnectRuntime      = "support_data.konnect.runtime"
	configKonnectTargetImages = "support_data.konnect.target_images"
	configKonnectTargetPods   = "support_data.konnect.target_pods"
	configKonnectPrefixDir    = "support_data.konnect.prefix_dir"
	configKonnectNamespace    = "support_data.konnect.namespace"
)

var (
	konnectUse = "konnect"

	konnectShort = i18n.T("root.verbs.collect.supportdata.konnect.konnectShort",
		"Collect support data from Konnect-managed data planes")

	konnectLong = normalizers.LongDesc(i18n.T("root.verbs.collect.supportdata.konnect.konnectLong",
		`Collect logs, configuration, and system information from Konnect-managed
Kong Gateway data planes.

Authentication uses your Konnect Personal Access Token (PAT) from the
configuration file or the --pat flag.

The command produces a ZIP archive containing Kong configuration and
diagnostic information from the specified control plane. When a runtime
is specified (docker, kubernetes, or vm), data plane logs and system
information are also collected.`))

	konnectExamples = normalizers.Examples(i18n.T("root.verbs.collect.supportdata.konnect.konnectExamples",
		fmt.Sprintf(`
        # Collect from a Konnect control plane (uses PAT from config)
        %[1]s collect support-data konnect --control-plane my-control-plane

        # Collect with explicit PAT
        %[1]s collect support-data konnect --control-plane my-cp --pat $KONNECT_PAT

        # Collect from a specific region
        %[1]s collect support-data konnect --control-plane my-cp --region eu

        # Collect with sanitization
        %[1]s collect support-data konnect --control-plane my-cp --sanitize

        # Collect with custom output directory
        %[1]s collect support-data konnect --control-plane my-cp --output-dir ./support

        # Collect with data plane runtime logs from Docker
        %[1]s collect support-data konnect --control-plane my-cp --runtime docker

        # Collect with data plane runtime logs from Kubernetes
        %[1]s collect support-data konnect --control-plane my-cp --runtime kubernetes --namespace kong

        # Collect from VM with custom prefix directory
        %[1]s collect support-data konnect --control-plane my-cp --runtime vm --prefix-dir /opt/kong
        `, meta.CLIName)))
)

// konnectFlags holds Konnect-specific flags.
type konnectFlags struct {
	ControlPlane string
	PAT          string
	BaseURL      string
	Region       string
	Runtime      string
	TargetImages []string
	TargetPods   []string
	PrefixDir    string
	Namespace    string
}

// NewKonnectCmd creates the Konnect target command.
func NewKonnectCmd() *cobra.Command {
	var commonFlags CommonFlags
	var flags konnectFlags

	cmd := &cobra.Command{
		Use:     konnectUse,
		Short:   konnectShort,
		Long:    konnectLong,
		Example: konnectExamples,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runKonnect(cmd.Context(), cmd, args, &commonFlags, &flags)
		},
	}

	// Register common flags
	RegisterCommonFlags(cmd, &commonFlags)

	// Register Konnect-specific flags
	registerKonnectFlags(cmd, &flags)

	return cmd
}

func registerKonnectFlags(cmd *cobra.Command, flags *konnectFlags) {
	f := cmd.Flags()

	f.StringVar(&flags.ControlPlane, "control-plane", "",
		fmt.Sprintf(`Konnect control plane name or ID.
Required for Konnect data collection.
- Config path: [ %s ]`, configKonnectControlPlane))

	f.StringVar(&flags.PAT, "pat", "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT).
Overrides token from configuration file.
- Config path: [ %s ]`, konnectcommon.PATConfigPath))

	f.StringVar(&flags.BaseURL, "base-url", "",
		fmt.Sprintf(`Konnect API base URL.
- Config path: [ %s ]
- Default: [ %s ]`, konnectcommon.BaseURLConfigPath, konnectcommon.BaseURLDefault))

	f.StringVar(&flags.Region, "region", "",
		fmt.Sprintf(`Konnect region identifier (e.g., "us", "eu").
Used to construct the base URL when --base-url is not provided.
- Config path: [ %s ]`, konnectcommon.RegionConfigPath))

	f.StringVar(&flags.Runtime, "runtime", "",
		fmt.Sprintf(`Runtime environment to collect data plane logs from.
Valid values: docker, kubernetes, vm
If not specified, runtime is auto-detected.
- Config path: [ %s ]`, configKonnectRuntime))

	f.StringSliceVar(&flags.TargetImages, "target-images", nil,
		fmt.Sprintf(`Container images to collect from.
Defaults to kong-gateway and kubernetes-ingress-controller.
Example: --target-images kong-gateway,custom-kong
- Config path: [ %s ]`, configKonnectTargetImages))

	f.StringSliceVar(&flags.TargetPods, "target-pods", nil,
		fmt.Sprintf(`Specific pod names to collect from (Kubernetes only).
If not specified, collects from all matching pods.
Example: --target-pods kong-gateway-0,kong-gateway-1
- Config path: [ %s ]`, configKonnectTargetPods))

	f.StringVarP(&flags.PrefixDir, "prefix-dir", "k", "",
		fmt.Sprintf(`Kong prefix directory for VM log collection.
Used to locate log files on VM deployments.
Default: /usr/local/kong
- Config path: [ %s ]`, configKonnectPrefixDir))

	f.StringVarP(&flags.Namespace, "namespace", "n", "",
		fmt.Sprintf(`Kubernetes namespace to collect from.
Required when runtime is kubernetes.
Example: --namespace kong
- Config path: [ %s ]`, configKonnectNamespace))
}

func runKonnect(
	ctx context.Context,
	cmd *cobra.Command,
	args []string,
	commonFlags *CommonFlags,
	flags *konnectFlags,
) error {
	helper := cmdpkg.BuildHelper(cmd, args)

	cfg, err := helper.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get configuration: %w", err)
	}

	// Build collector config
	collectorCfg, err := buildKonnectConfig(cfg, commonFlags, flags)
	if err != nil {
		return err
	}

	// Validate namespace requirement for Kubernetes runtime
	if collectorCfg.Runtime == "kubernetes" && collectorCfg.Namespace == "" {
		return fmt.Errorf("--namespace is required when runtime is kubernetes")
	}

	// Validate required fields
	if collectorCfg.KonnectControlPlaneName == "" {
		return fmt.Errorf("--control-plane is required for Konnect data collection")
	}
	if len(collectorCfg.RBACHeaders) == 0 || collectorCfg.RBACHeaders[0] == "" {
		return fmt.Errorf("a Konnect Personal Access Token is required; use --pat or set %s in config", konnectcommon.PATConfigPath)
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

func buildKonnectConfig(
	cfg config.Hook,
	commonFlags *CommonFlags,
	flags *konnectFlags,
) (*collector.Config, error) {
	// Start with library defaults
	collectorCfg := collector.DefaultConfig()

	// Enable Konnect mode
	collectorCfg.KonnectMode = true

	// Layer 1: Apply common config from file
	ApplyCommonConfig(cfg, collectorCfg)

	// Layer 2: Apply Konnect config from file
	if cp := cfg.GetString(configKonnectControlPlane); cp != "" {
		collectorCfg.KonnectControlPlaneName = cp
	}
	if r := cfg.GetString(configKonnectRuntime); r != "" {
		collectorCfg.Runtime = r
	}
	if images := cfg.GetStringSlice(configKonnectTargetImages); len(images) > 0 {
		collectorCfg.TargetImages = images
	}
	if pods := cfg.GetStringSlice(configKonnectTargetPods); len(pods) > 0 {
		collectorCfg.TargetPods = pods
	}
	if dir := cfg.GetString(configKonnectPrefixDir); dir != "" {
		collectorCfg.PrefixDir = dir
	}
	if ns := cfg.GetString(configKonnectNamespace); ns != "" {
		collectorCfg.Namespace = ns
	}

	// Resolve base URL (handles region -> URL conversion)
	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Konnect base URL: %w", err)
	}
	collectorCfg.KongAddr = baseURL

	// Get token from config
	if token := cfg.GetString(konnectcommon.PATConfigPath); token != "" {
		collectorCfg.RBACHeaders = []string{token}
	}

	// Layer 3: Apply common flags (override config)
	ApplyCommonFlags(commonFlags, collectorCfg)

	// Layer 4: Apply Konnect flags (override config)
	if flags.ControlPlane != "" {
		collectorCfg.KonnectControlPlaneName = flags.ControlPlane
	}

	// Handle base URL / region flags
	if flags.BaseURL != "" {
		collectorCfg.KongAddr = flags.BaseURL
	} else if flags.Region != "" {
		regionURL, err := konnectcommon.BuildBaseURLFromRegion(flags.Region)
		if err != nil {
			return nil, fmt.Errorf("invalid region: %w", err)
		}
		collectorCfg.KongAddr = regionURL
	}

	// PAT flag overrides config
	if flags.PAT != "" {
		collectorCfg.RBACHeaders = []string{flags.PAT}
	}

	// Runtime flags override config
	if flags.Runtime != "" {
		collectorCfg.Runtime = flags.Runtime
	}
	if len(flags.TargetImages) > 0 {
		collectorCfg.TargetImages = flags.TargetImages
	}
	if len(flags.TargetPods) > 0 {
		collectorCfg.TargetPods = flags.TargetPods
	}
	if flags.PrefixDir != "" {
		collectorCfg.PrefixDir = flags.PrefixDir
	}
	if flags.Namespace != "" {
		collectorCfg.Namespace = flags.Namespace
	}

	return collectorCfg, nil
}

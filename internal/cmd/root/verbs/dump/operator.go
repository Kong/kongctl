package dump

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	gatewayCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/operatorexport"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
)

const (
	formatOperator = "operator"

	operatorNamespaceFlagName      = "namespace"
	operatorAuthRefFlagName        = "auth-ref"
	operatorAdoptModeFlagName      = "adopt-mode"
	operatorIncludeSecretsFlagName = "include-secrets"

	operatorDefaultNamespace = "default"
	operatorDefaultAuthRef   = "konnect-api-auth"
)

type operatorOptions struct {
	outputFile string
	export     operatorexport.Options
}

var (
	operatorShort = i18n.T("root.verbs.dump.operator.short",
		"Export gateway entities as Kong Operator resources that adopt them")

	operatorLong = normalizers.LongDesc(i18n.T("root.verbs.dump.operator.long",
		`Export the entities of a Konnect gateway control plane as Kong Operator
custom resources that adopt the existing entities through spec.adopt.

The control plane is exported as a KonnectGatewayControlPlane that mirrors it,
and every service, route, upstream, target, consumer, consumer group,
credential, certificate, CA certificate, SNI, key, key set, vault, data plane
client certificate, and plugin is exported as the matching Kong Operator
resource. Applying the output with kubectl hands those entities to the
operator.

Once the operator adopts an entity it owns it: it writes the resource's full
spec back to Konnect, and deleting the resource deletes the entity in Konnect.
The control plane mirror is never modified or deleted.

Entities the operator cannot represent are skipped, and fields it cannot hold
are reported, as warnings on stderr. Entities that need secret material
(certificates, key-auth, JWT and HMAC credentials, private keys, and vaults
whose config holds credentials) are skipped unless --include-secrets is set,
which writes the secrets inline in the output. Basic-auth credentials are
always skipped because Konnect only returns the password hash.`))

	operatorExamples = normalizers.Examples(i18n.T("root.verbs.dump.operator.examples",
		fmt.Sprintf(`
	# Export a control plane's entities for the kong namespace
	%[1]s dump operator --control-plane-name gateway-control-plane --namespace kong > adopt.yaml

	# Adopt every entity in match mode, including plugin bindings and upstreams
	%[1]s dump operator --control-plane-id <id> --adopt-mode match --output-file adopt.yaml

	# Include certificates and credentials, writing their secrets inline
	%[1]s dump operator --control-plane-name gateway-control-plane --include-secrets --output-file adopt.yaml
	`, meta.CLIName)))
)

func newOperatorCmd() *cobra.Command {
	opts := &operatorOptions{}

	cmd := &cobra.Command{
		Use:     formatOperator,
		Short:   operatorShort,
		Long:    operatorLong,
		Example: operatorExamples,
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			helper := cmdpkg.BuildHelper(c, args)
			if err := opts.export.Validate(); err != nil {
				return &cmdpkg.ConfigurationError{Err: err}
			}
			if err := ensureNonNegativePageSize(helper); err != nil {
				return err
			}
			return runOperatorDump(helper, *opts)
		},
	}

	gatewayCommon.AddControlPlaneFlags(cmd)

	cmd.Flags().StringVar(&opts.export.Namespace, operatorNamespaceFlagName, operatorDefaultNamespace,
		"Kubernetes namespace for the generated resources.")
	cmd.Flags().StringVar(&opts.export.AuthRef, operatorAuthRefFlagName, operatorDefaultAuthRef,
		"Name of the KonnectAPIAuthConfiguration the control plane mirror uses.")
	cmd.Flags().StringVar(&opts.export.AdoptMode, operatorAdoptModeFlagName, operatorexport.AdoptModeAuto,
		fmt.Sprintf(`Adoption mode written to spec.adopt.mode.
- %s    : %s for plugin bindings and upstreams, %s for everything else
- %s   : adopt only when the resource matches the existing entity
- %s: adopt and update the entity to match the resource`,
			operatorexport.AdoptModeAuto, operatorexport.AdoptModeOverride, operatorexport.AdoptModeMatch,
			operatorexport.AdoptModeMatch, operatorexport.AdoptModeOverride))
	cmd.Flags().BoolVar(&opts.export.IncludeSecrets, operatorIncludeSecretsFlagName, false,
		"Export entities that need secret material, writing the secrets inline in the output.")
	cmd.Flags().StringVar(&opts.outputFile, "output-file", "",
		"File to write the output to. If not specified, output is written to stdout.")

	cmd.Flags().String(konnectCommon.BaseURLFlagName, "",
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
			konnectCommon.BaseURLConfigPath, konnectCommon.BaseURLDefault))
	cmd.Flags().String(konnectCommon.RegionFlagName, "",
		fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
			konnectCommon.BaseURLFlagName, konnectCommon.RegionConfigPath),
	)
	cmd.Flags().String(konnectCommon.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			konnectCommon.PATConfigPath))
	cmd.Flags().Int(konnectCommon.RequestPageSizeFlagName, konnectCommon.DefaultRequestPageSize,
		fmt.Sprintf(`Max number of results to include per response page.
- Config path: [ %s ]`, konnectCommon.RequestPageSizeConfigPath))

	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(c, args)
		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}

		bindings := []struct{ flag, configPath string }{
			{konnectCommon.BaseURLFlagName, konnectCommon.BaseURLConfigPath},
			{konnectCommon.RegionFlagName, konnectCommon.RegionConfigPath},
			{konnectCommon.PATFlagName, konnectCommon.PATConfigPath},
			{konnectCommon.RequestPageSizeFlagName, konnectCommon.RequestPageSizeConfigPath},
			{gatewayCommon.ControlPlaneIDFlagName, gatewayCommon.ControlPlaneIDConfigPath},
			{gatewayCommon.ControlPlaneNameFlagName, gatewayCommon.ControlPlaneNameConfigPath},
		}
		for _, b := range bindings {
			if err := bindFlag(cfg, c.Flags(), b.flag, b.configPath); err != nil {
				return err
			}
		}
		return nil
	}

	return cmd
}

func runOperatorDump(helper cmdpkg.Helper, opts operatorOptions) error {
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	ctx := helper.GetContext()

	sdkAPI, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}
	konnectSDK, ok := sdkAPI.(*helpers.KonnectSDK)
	if !ok || konnectSDK.SDK == nil {
		return fmt.Errorf("dump operator requires the Konnect SDK client")
	}

	cpID := cfg.GetString(gatewayCommon.ControlPlaneIDConfigPath)
	if cpID == "" {
		cpName := cfg.GetString(gatewayCommon.ControlPlaneNameConfigPath)
		if cpName == "" {
			return &cmdpkg.ConfigurationError{Err: fmt.Errorf(
				"a control plane is required: set --%s or --%s",
				gatewayCommon.ControlPlaneIDFlagName, gatewayCommon.ControlPlaneNameFlagName)}
		}
		cpID, err = helpers.GetControlPlaneID(ctx, sdkAPI.GetControlPlaneAPI(), cpName)
		if err != nil {
			return fmt.Errorf("failed to find control plane %q: %w", cpName, err)
		}
	}

	cpRes, err := sdkAPI.GetControlPlaneAPI().GetControlPlane(ctx, cpID)
	if err != nil {
		return fmt.Errorf("failed to get control plane %s: %w", cpID, err)
	}
	if cpRes.ControlPlane == nil {
		return fmt.Errorf("control plane %s not found", cpID)
	}
	controlPlane := operatorexport.ControlPlane{ID: cpRes.ControlPlane.ID, Name: cpRes.ControlPlane.Name}
	if clusterType := cpRes.ControlPlane.Config.ClusterType; clusterType != nil {
		controlPlane.ClusterType = string(*clusterType)
	}

	pageSize := int64(cfg.GetIntOrElse(
		konnectCommon.RequestPageSizeConfigPath, konnectCommon.DefaultRequestPageSize))
	snap, fetchWarnings, err := fetchOperatorSnapshot(ctx, konnectSDK.SDK, controlPlane, pageSize)
	if err != nil {
		return err
	}

	result, err := operatorexport.Build(snap, opts.export)
	if err != nil {
		return err
	}

	writer, cleanup, err := getDumpWriter(helper, opts.outputFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = cleanup()
	}()
	if err := operatorexport.Write(writer, result.Manifests); err != nil {
		return err
	}

	return writeOperatorWarnings(helper.GetStreams().ErrOut, fetchWarnings, result)
}

// writeOperatorWarnings reports skipped entities and caveats on stderr so they
// never mix with the YAML on stdout.
func writeOperatorWarnings(w io.Writer, fetchWarnings []string, result operatorexport.Result) error {
	for _, warning := range fetchWarnings {
		if _, err := fmt.Fprintf(w, "Warning: %s\n", warning); err != nil {
			return err
		}
	}
	for _, warning := range result.Warnings {
		if _, err := fmt.Fprintf(w, "Warning: %s\n", warning); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "Exported %d resources with %d warnings.\n",
		len(result.Manifests), len(fetchWarnings)+len(result.Warnings))
	return err
}

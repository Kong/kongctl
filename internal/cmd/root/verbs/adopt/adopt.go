package adopt

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Adopt
)

var (
	adoptUse = Verb.String()

	adoptShort = i18n.T("root.verbs.adopt.adoptShort", "Adopt existing resources into declarative management")

	adoptLong = normalizers.LongDesc(i18n.T("root.verbs.adopt.adoptLong",
		"Apply the KONGCTL-namespace label to existing Konnect resources so they become managed by kongctl.\n"+
			"By default, adopt refuses to replace an existing namespace label. Use --overwrite-namespace "+
			"to explicitly move an already adopted resource to the provided --namespace value."))

	adoptExamples = normalizers.Examples(i18n.T("root.verbs.adopt.adoptExamples",
		fmt.Sprintf(`  # Adopt a portal by name into the "team-alpha" namespace
  %[1]s adopt portal my-portal --namespace team-alpha
  # Move an already adopted portal to a different namespace
  %[1]s adopt --namespace platform --overwrite-namespace portal my-portal
  # Adopt a control plane by ID
  %[1]s adopt control-plane 22cd8a0b-72e7-4212-9099-0764f8e9c5ac --namespace platform
  # Adopt a dashboard by ID
  %[1]s adopt analytics dashboard 22cd8a0b-72e7-4212-9099-0764f8e9c5ac --namespace analytics
  # Adopt a DCR provider by name
  %[1]s adopt dcr-provider my-dcr-provider --namespace team-alpha
  # Adopt an API explicitly via the konnect product
  %[1]s adopt konnect api my-api --namespace team-alpha
`, meta.CLIName)))
)

func NewAdoptCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     adoptUse,
		Short:   adoptShort,
		Long:    adoptLong,
		Example: adoptExamples,
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			return bindKonnectFlags(c, args)
		},
	}

	// Add Konnect-specific flags as persistent flags so they appear in help
	cmd.PersistentFlags().String(common.BaseURLFlagName, "",
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
			common.BaseURLConfigPath, common.BaseURLDefault))

	cmd.PersistentFlags().String(
		common.RegionFlagName, "",
		fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
			common.BaseURLFlagName, common.RegionConfigPath),
	)

	cmd.PersistentFlags().String(common.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			common.PATConfigPath))

	if err := adoptCommon.AddAdoptFlags(cmd); err != nil {
		return nil, err
	}

	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(konnectCmd)

	portalCmd, err := NewDirectPortalCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(portalCmd)

	controlPlaneCmd, err := NewDirectControlPlaneCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(controlPlaneCmd)

	apiCmd, err := NewDirectAPICmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(apiCmd)

	authStrategyCmd, err := NewDirectAuthStrategyCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(authStrategyCmd)

	dcrProviderCmd, err := NewDirectDCRProviderCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(dcrProviderCmd)

	identityCmd, err := NewDirectIdentityCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(identityCmd)

	analyticsCmd, err := NewDirectAnalyticsCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(analyticsCmd)

	eventGatewayCmd, err := NewDirectEventGatewayCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(eventGatewayCmd)

	orgCmd, err := NewDirectOrganizationCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(orgCmd)

	return cmd, nil
}

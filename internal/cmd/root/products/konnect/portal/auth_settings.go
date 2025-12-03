package portal

import (
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const authSettingsCommandName = "auth-settings"

var (
	authSettingsUse = authSettingsCommandName

	authSettingsShort = i18n.T("root.products.konnect.portal.authSettingsShort",
		"Retrieve portal authentication settings")
	authSettingsLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.authSettingsLong",
		`Use the auth-settings command to fetch authentication settings for a Konnect portal.`))
	authSettingsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.authSettingsExamples",
			fmt.Sprintf(`
# Get auth settings for a portal by ID
%[1]s get portal auth-settings --portal-id <portal-id>
# Get auth settings for a portal by name
%[1]s get portal auth-settings --portal-name my-portal
`, meta.CLIName)))
)

func newGetPortalAuthSettingsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     authSettingsUse,
		Short:   authSettingsShort,
		Long:    authSettingsLong,
		Example: authSettingsExample,
		Aliases: []string{"auth-setting", "auth"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			return bindPortalChildFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			return runGetPortalAuthSettings(c, args)
		},
	}

	addPortalChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

func runGetPortalAuthSettings(c *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(args, ", "))
	}

	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	interactive, err := helper.IsInteractive()
	if err != nil {
		return err
	}

	var printer cli.PrintFlusher
	if !interactive {
		printer, err = cli.Format(outType.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	portalID, portalName := getPortalIdentifiers(cfg)
	if portalID == "" && portalName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("either --%s or --%s is required", portalIDFlagName, portalNameFlagName),
		}
	}

	if portalID == "" {
		portalID, err = resolvePortalIDByName(portalName, sdk.GetPortalAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	authAPI := sdk.GetPortalAuthSettingsAPI()
	if authAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal auth settings client is not available",
			Err: fmt.Errorf("portal auth settings client not configured"),
		}
	}

	res, err := authAPI.GetPortalAuthenticationSettings(helper.GetContext(), portalID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get portal auth settings", err, helper.GetCmd(), attrs...)
	}

	if res.PortalAuthenticationSettingsResponse == nil {
		return &cmd.ExecutionError{
			Msg: "Failed to get portal auth settings",
			Err: fmt.Errorf("empty response from Konnect"),
		}
	}

	settings := res.PortalAuthenticationSettingsResponse
	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		portalAuthSettingsToRecord(settings),
		settings,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func portalAuthSettingsToRecord(settings *kkComps.PortalAuthenticationSettingsResponse) any {
	oidcConfig := settings.GetOidcConfig()

	return struct {
		BasicAuthEnabled       string `json:"basic_auth_enabled"`
		OidcAuthEnabled        string `json:"oidc_auth_enabled"`
		SamlAuthEnabled        string `json:"saml_auth_enabled"`
		OidcTeamMappingEnabled string `json:"oidc_team_mapping_enabled"`
		IdpMappingEnabled      string `json:"idp_mapping_enabled"`
		KonnectMappingEnabled  string `json:"konnect_mapping_enabled"`
		OidcIssuer             string `json:"oidc_config.issuer"`
		OidcClientID           string `json:"oidc_config.client_id"`
		OidcScopes             string `json:"oidc_config.scopes"`
		OidcClaimMappings      string `json:"oidc_config.claim_mappings"`
	}{
		BasicAuthEnabled:       fmt.Sprintf("%v", settings.BasicAuthEnabled),
		OidcAuthEnabled:        fmt.Sprintf("%v", settings.OidcAuthEnabled),
		SamlAuthEnabled:        fmt.Sprintf("%v", valueOrNA(settings.SamlAuthEnabled)),
		OidcTeamMappingEnabled: fmt.Sprintf("%v", settings.OidcTeamMappingEnabled),
		IdpMappingEnabled:      fmt.Sprintf("%v", valueOrNA(settings.IdpMappingEnabled)),
		KonnectMappingEnabled:  fmt.Sprintf("%v", settings.KonnectMappingEnabled),
		OidcIssuer:             fmt.Sprintf("%v", valueOrNAString(oidcConfig.GetIssuer())),
		OidcClientID:           fmt.Sprintf("%v", valueOrNAString(oidcConfig.GetClientID())),
		OidcScopes:             fmt.Sprintf("%v", sliceOrNA(oidcConfig.GetScopes())),
		OidcClaimMappings:      fmt.Sprintf("%v", valueOrNA(oidcConfig.GetClaimMappings())),
	}
}

func valueOrNA(value any) any {
	switch v := value.(type) {
	case *bool:
		if v == nil {
			return valueNA
		}
		return *v
	case nil:
		return valueNA
	default:
		return v
	}
}

func valueOrNAString(value string) any {
	if strings.TrimSpace(value) == "" {
		return valueNA
	}
	return value
}

func sliceOrNA(values []string) any {
	if len(values) == 0 {
		return valueNA
	}
	return strings.Join(values, ",")
}

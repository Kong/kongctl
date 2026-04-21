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

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

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

	return tableview.RenderForFormat(helper,
		false,
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
	return struct {
		BasicAuthEnabled      string `json:"basic_auth_enabled"`
		IdpMappingEnabled     string `json:"idp_mapping_enabled"`
		KonnectMappingEnabled string `json:"konnect_mapping_enabled"`
	}{
		BasicAuthEnabled:      fmt.Sprintf("%v", settings.BasicAuthEnabled),
		IdpMappingEnabled:     fmt.Sprintf("%v", valueOrNA(settings.IdpMappingEnabled)),
		KonnectMappingEnabled: fmt.Sprintf("%v", settings.KonnectMappingEnabled),
	}
}

func portalAuthSettingsDetailView(settings *kkComps.PortalAuthenticationSettingsResponse) string {
	if settings == nil {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Basic Auth Enabled: %t\n", settings.BasicAuthEnabled)
	fmt.Fprintf(&b, "IdP Mapping Enabled: %v\n", valueOrNA(settings.IdpMappingEnabled))
	fmt.Fprintf(&b, "Konnect Mapping Enabled: %t\n", settings.KonnectMappingEnabled)

	return strings.TrimRight(b.String(), "\n")
}

func buildPortalAuthSettingsChildView(settings *kkComps.PortalAuthenticationSettingsResponse) tableview.ChildView {
	return tableview.ChildView{
		Title:          "Authentication Settings",
		Mode:           tableview.ChildViewModeDetail,
		DetailRenderer: func(int) string { return portalAuthSettingsDetailView(settings) },
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

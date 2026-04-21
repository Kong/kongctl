package portal

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	identityProvidersCommandName = "identity-providers"
	identityProviderTypeFlagName = "type"
)

type portalIdentityProviderRecord struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Enabled   string `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

var (
	identityProvidersShort = i18n.T("root.products.konnect.portal.identityProvidersShort",
		"List portal identity providers")
	identityProvidersLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.identityProvidersLong",
		`Use the identity-providers command to list identity providers for a Konnect portal.`))
	identityProvidersExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.identityProvidersExamples",
			fmt.Sprintf(`
# List identity providers for a portal by ID
%[1]s get portal identity-providers --portal-id <portal-id>
# Filter identity providers by type
%[1]s get portal identity-providers --portal-name my-portal --type oidc
`, meta.CLIName)))
)

func newGetPortalIdentityProvidersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     identityProvidersCommandName,
		Short:   identityProvidersShort,
		Long:    identityProvidersLong,
		Example: identityProvidersExample,
		Aliases: []string{"identity-provider", "idps", "idp"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			return bindPortalChildFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			return runGetPortalIdentityProviders(c, args)
		},
	}

	addPortalChildFlags(cmd)
	cmd.Flags().String(identityProviderTypeFlagName, "", "Filter identity providers by type")

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

func runGetPortalIdentityProviders(c *cobra.Command, args []string) error {
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

	identityProviderAPI := sdk.GetPortalIdentityProviderAPI()
	if identityProviderAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal identity providers client is not available",
			Err: fmt.Errorf("portal identity providers client not configured"),
		}
	}

	var filter *kkOps.GetPortalIdentityProvidersQueryParamFilter
	providerType, _ := c.Flags().GetString(identityProviderTypeFlagName)
	providerType = strings.TrimSpace(providerType)
	if providerType != "" {
		filter = &kkOps.GetPortalIdentityProvidersQueryParamFilter{
			Type: &kkComps.StringFieldEqualsFilter{Eq: &providerType},
		}
	}

	resp, err := identityProviderAPI.ListPortalIdentityProviders(
		helper.GetContext(),
		kkOps.GetPortalIdentityProvidersRequest{PortalID: portalID, Filter: filter},
	)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get portal identity providers", err, helper.GetCmd(), attrs...)
	}

	providers := resp.IdentityProviders
	records := make([]portalIdentityProviderRecord, 0, len(providers))
	for _, provider := range providers {
		records = append(records, portalIdentityProviderToRecord(provider))
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		providers,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func portalIdentityProviderToRecord(provider kkComps.IdentityProvider) portalIdentityProviderRecord {
	return portalIdentityProviderRecord{
		ID:        optionalPtr(provider.GetID()),
		Type:      portalIdentityProviderType(provider),
		Enabled:   portalIdentityProviderEnabled(provider),
		CreatedAt: formatTimePtr(provider.GetCreatedAt()),
		UpdatedAt: formatTimePtr(provider.GetUpdatedAt()),
	}
}

func portalIdentityProviderType(provider kkComps.IdentityProvider) string {
	if provider.Type == nil {
		return valueNA
	}
	return string(*provider.Type)
}

func portalIdentityProviderEnabled(provider kkComps.IdentityProvider) string {
	if provider.Enabled == nil {
		return valueNA
	}
	return fmt.Sprintf("%v", *provider.Enabled)
}

func portalIdentityProviderDetailView(provider kkComps.IdentityProvider) string {
	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", optionalPtr(provider.GetID()))
	fmt.Fprintf(&b, "type: %s\n", portalIdentityProviderType(provider))
	fmt.Fprintf(&b, "enabled: %s\n", portalIdentityProviderEnabled(provider))
	fmt.Fprintf(&b, "created_at: %s\n", formatTimePtr(provider.GetCreatedAt()))
	fmt.Fprintf(&b, "updated_at: %s\n", formatTimePtr(provider.GetUpdatedAt()))
	fmt.Fprintf(&b, "config:\n")

	return strings.TrimRight(b.String(), "\n")
}

func buildPortalIdentityProvidersChildView(providers []kkComps.IdentityProvider) tableview.ChildView {
	rows := make([]table.Row, 0, len(providers))
	for _, provider := range providers {
		record := portalIdentityProviderToRecord(provider)
		rows = append(rows, table.Row{record.ID, record.Type, record.Enabled})
	}

	return tableview.ChildView{
		Headers: []string{"ID", "TYPE", "ENABLED"},
		Rows:    rows,
		Title:   "Identity Providers",
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(providers) {
				return ""
			}
			return portalIdentityProviderDetailView(providers[index])
		},
		ParentType: "portal-identity-provider",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(providers) {
				return nil
			}
			return providers[index]
		},
	}
}

package portal

import (
	"fmt"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	applicationsCommandName = "applications"
)

type portalApplicationSummaryRecord struct {
	ID                string
	Name              string
	Type              string
	AuthStrategy      string
	CredentialDetail  string
	RegistrationCount int
	LocalCreatedTime  string
	LocalUpdatedTime  string
}

type portalApplicationDetailRecord struct {
	ID                string
	Name              string
	Type              string
	AuthStrategy      string
	CredentialDetail  string
	ClientID          string
	GrantedScopes     string
	RegistrationCount int
	LocalCreatedTime  string
	LocalUpdatedTime  string
}

var (
	applicationsUse = applicationsCommandName

	applicationsShort = i18n.T("root.products.konnect.portal.applicationsShort",
		"Manage portal applications for a Konnect portal")
	applicationsLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.applicationsLong",
		`Use the applications command to list or retrieve applications for a specific Konnect portal.`))
	applicationsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.applicationsExamples",
			fmt.Sprintf(`
# List applications for a portal by ID
%[1]s get portal applications --portal-id <portal-id>
# List applications for a portal by name
%[1]s get portal applications --portal-name my-portal
# Get a specific application by ID
%[1]s get portal applications --portal-id <portal-id> <application-id>
# Get a specific application by name
%[1]s get portal applications --portal-id <portal-id> checkout-app
`, meta.CLIName)))
)

func newGetPortalApplicationsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     applicationsUse,
		Short:   applicationsShort,
		Long:    applicationsLong,
		Example: applicationsExample,
		Aliases: []string{"application", "apps"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindPortalChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := portalApplicationsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addPortalChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type portalApplicationsHandler struct {
	cmd *cobra.Command
}

func (h portalApplicationsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing portal applications requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
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

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	portalID, portalName := getPortalIdentifiers(cfg)
	if portalID != "" && portalName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", portalIDFlagName, portalNameFlagName),
		}
	}

	if portalID == "" && portalName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"a portal identifier is required. Provide --%s or --%s",
				portalIDFlagName,
				portalNameFlagName,
			),
		}
	}

	if portalID == "" {
		portalID, err = resolvePortalIDByName(portalName, sdk.GetPortalAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	appAPI := sdk.GetPortalApplicationAPI()
	if appAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal applications client is not available",
			Err: fmt.Errorf("portal applications client not configured"),
		}
	}

	if len(args) == 1 {
		appIdentifier := strings.TrimSpace(args[0])
		return h.getSingleApplication(
			helper,
			appAPI,
			portalID,
			appIdentifier,
			interactive,
			outType,
			printer,
			cfg,
		)
	}

	return h.listApplications(helper, appAPI, portalID, interactive, outType, printer, cfg)
}

func (h portalApplicationsHandler) listApplications(
	helper cmd.Helper,
	appAPI helpers.PortalApplicationAPI,
	portalID string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	apps, err := fetchPortalApplications(helper, appAPI, portalID, cfg)
	if err != nil {
		return err
	}

	records := make([]portalApplicationSummaryRecord, 0, len(apps))
	for _, app := range apps {
		records = append(records, portalApplicationSummaryToRecord(app))
	}

	tableRows := make([]table.Row, 0, len(apps))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(apps) {
			return ""
		}
		return portalApplicationDetailViewFromUnion(apps[index])
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		records,
		apps,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME"}, tableRows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h portalApplicationsHandler) getSingleApplication(
	helper cmd.Helper,
	appAPI helpers.PortalApplicationAPI,
	portalID string,
	identifier string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	appID := identifier
	if !util.IsValidUUID(identifier) {
		apps, err := fetchPortalApplications(helper, appAPI, portalID, cfg)
		if err != nil {
			return err
		}
		match := findApplicationByName(apps, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("application %q not found", identifier),
			}
		}
		appID = matchID(*match)
	}

	res, err := appAPI.GetApplication(helper.GetContext(), portalID, appID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get portal application", err, helper.GetCmd(), attrs...)
	}

	app := res.GetGetApplicationResponse()
	if app == nil {
		return &cmd.ExecutionError{
			Msg: "Portal application response was empty",
			Err: fmt.Errorf("no application returned for id %s", appID),
		}
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		portalApplicationDetailToRecord(app),
		app,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchPortalApplications(
	helper cmd.Helper,
	appAPI helpers.PortalApplicationAPI,
	portalID string,
	cfg config.Hook,
) ([]kkComps.Application, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.Application

	for {
		req := kkOps.ListApplicationsRequest{
			PortalID:   portalID,
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := appAPI.ListApplications(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list portal applications", err, helper.GetCmd(), attrs...)
		}

		if res.GetListApplicationsResponse() == nil {
			break
		}

		data := res.GetListApplicationsResponse().GetData()
		all = append(all, data...)

		total := int(res.GetListApplicationsResponse().GetMeta().Page.Total)
		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return all, nil
}

func findApplicationByName(apps []kkComps.Application, identifier string) *kkComps.Application {
	lowered := strings.ToLower(identifier)
	for _, app := range apps {
		id, name := helpers.ApplicationSummary(app)
		if name != "" && strings.ToLower(name) == lowered {
			appCopy := app
			return &appCopy
		}
		if id != "" && strings.ToLower(id) == lowered {
			appCopy := app
			return &appCopy
		}
	}
	return nil
}

func portalApplicationSummaryToRecord(app kkComps.Application) portalApplicationSummaryRecord {
	switch app.Type {
	case kkComps.ApplicationTypeKeyAuthApplication:
		key := app.KeyAuthApplication
		if key == nil {
			return portalApplicationSummaryRecord{
				Type:             "key-auth",
				ID:               valueNA,
				Name:             valueNA,
				AuthStrategy:     valueNA,
				CredentialDetail: valueNA,
				LocalCreatedTime: valueNA,
				LocalUpdatedTime: valueNA,
			}
		}
		strategy := key.GetAuthStrategy()
		return portalApplicationSummaryRecord{
			ID:                util.AbbreviateUUID(key.GetID()),
			Name:              key.GetName(),
			Type:              "key-auth",
			AuthStrategy:      stringPtrOrNA(strategy.GetName()),
			CredentialDetail:  joinOrNA(strategy.KeyNames),
			RegistrationCount: int(key.GetRegistrationCount()),
			LocalCreatedTime:  formatTime(key.GetCreatedAt()),
			LocalUpdatedTime:  formatTime(key.GetUpdatedAt()),
		}
	case kkComps.ApplicationTypeClientCredentialsApplication:
		client := app.ClientCredentialsApplication
		if client == nil {
			return portalApplicationSummaryRecord{
				Type:             "client-credentials",
				ID:               valueNA,
				Name:             valueNA,
				AuthStrategy:     valueNA,
				CredentialDetail: valueNA,
				LocalCreatedTime: valueNA,
				LocalUpdatedTime: valueNA,
			}
		}
		strategy := client.GetAuthStrategy()
		return portalApplicationSummaryRecord{
			ID:                util.AbbreviateUUID(client.GetID()),
			Name:              client.GetName(),
			Type:              "client-credentials",
			AuthStrategy:      stringPtrOrNA(strategy.GetName()),
			CredentialDetail:  joinOrNA(strategy.AuthMethods),
			RegistrationCount: int(client.GetRegistrationCount()),
			LocalCreatedTime:  formatTime(client.GetCreatedAt()),
			LocalUpdatedTime:  formatTime(client.GetUpdatedAt()),
		}
	default:
		return portalApplicationSummaryRecord{
			ID:               valueNA,
			Name:             valueNA,
			Type:             string(app.Type),
			AuthStrategy:     valueNA,
			CredentialDetail: valueNA,
			LocalCreatedTime: valueNA,
			LocalUpdatedTime: valueNA,
		}
	}
}

func portalApplicationDetailToRecord(app *kkComps.GetApplicationResponse) portalApplicationDetailRecord {
	if app.KeyAuthApplication != nil {
		key := app.KeyAuthApplication
		strategy := key.GetAuthStrategy()
		return portalApplicationDetailRecord{
			ID:                util.AbbreviateUUID(key.GetID()),
			Name:              key.GetName(),
			Type:              "key-auth",
			AuthStrategy:      stringPtrOrNA(strategy.GetName()),
			CredentialDetail:  joinOrNA(strategy.KeyNames),
			ClientID:          valueNA,
			GrantedScopes:     valueNA,
			RegistrationCount: int(key.GetRegistrationCount()),
			LocalCreatedTime:  formatTime(key.GetCreatedAt()),
			LocalUpdatedTime:  formatTime(key.GetUpdatedAt()),
		}
	}

	if app.ClientCredentialsApplication != nil {
		client := app.ClientCredentialsApplication
		strategy := client.GetAuthStrategy()
		return portalApplicationDetailRecord{
			ID:                util.AbbreviateUUID(client.GetID()),
			Name:              client.GetName(),
			Type:              "client-credentials",
			AuthStrategy:      stringPtrOrNA(strategy.GetName()),
			CredentialDetail:  joinOrNA(strategy.AuthMethods),
			ClientID:          nonEmptyOrNA(client.GetClientID()),
			GrantedScopes:     joinOrNA(client.GetGrantedScopes()),
			RegistrationCount: int(client.GetRegistrationCount()),
			LocalCreatedTime:  formatTime(client.GetCreatedAt()),
			LocalUpdatedTime:  formatTime(client.GetUpdatedAt()),
		}
	}

	return portalApplicationDetailRecord{
		ID:                valueNA,
		Name:              valueNA,
		Type:              valueNA,
		AuthStrategy:      valueNA,
		CredentialDetail:  valueNA,
		ClientID:          valueNA,
		GrantedScopes:     valueNA,
		RegistrationCount: 0,
		LocalCreatedTime:  valueNA,
		LocalUpdatedTime:  valueNA,
	}
}

func joinOrNA(values []string) string {
	if len(values) == 0 {
		return valueNA
	}
	joined := strings.Join(values, ", ")
	if joined == "" {
		return valueNA
	}
	return joined
}

func nonEmptyOrNA(val string) string {
	if strings.TrimSpace(val) == "" {
		return valueNA
	}
	return val
}

func stringPtrOrNA(val *string) string {
	if val == nil {
		return valueNA
	}
	return nonEmptyOrNA(*val)
}

func matchID(app kkComps.Application) string {
	if app.ClientCredentialsApplication != nil {
		return app.ClientCredentialsApplication.GetID()
	}
	if app.KeyAuthApplication != nil {
		return app.KeyAuthApplication.GetID()
	}
	return ""
}

func portalApplicationDetailViewFromUnion(app kkComps.Application) string {
	var b strings.Builder
	missing := valueNA

	switch app.Type {
	case kkComps.ApplicationTypeKeyAuthApplication:
		key := app.KeyAuthApplication
		if key == nil {
			break
		}
		strategy := key.GetAuthStrategy()
		fmt.Fprintf(&b, "Name: %s\n", key.GetName())
		fmt.Fprintf(&b, "ID: %s\n", key.GetID())
		fmt.Fprintf(&b, "Type: key-auth\n")
		fmt.Fprintf(&b, "Auth Strategy: %s\n", stringPtrOrNA(strategy.GetName()))
		fmt.Fprintf(&b, "Credential Detail: %s\n", joinOrNA(strategy.KeyNames))
		fmt.Fprintf(&b, "Registration Count: %.0f\n", key.GetRegistrationCount())
		fmt.Fprintf(&b, "Created: %s\n", formatTime(key.GetCreatedAt()))
		fmt.Fprintf(&b, "Updated: %s\n", formatTime(key.GetUpdatedAt()))
	case kkComps.ApplicationTypeClientCredentialsApplication:
		client := app.ClientCredentialsApplication
		if client == nil {
			break
		}
		strategy := client.GetAuthStrategy()
		fmt.Fprintf(&b, "Name: %s\n", client.GetName())
		fmt.Fprintf(&b, "ID: %s\n", client.GetID())
		fmt.Fprintf(&b, "Type: client-credentials\n")
		fmt.Fprintf(&b, "Auth Strategy: %s\n", stringPtrOrNA(strategy.GetName()))
		fmt.Fprintf(&b, "Credential Detail: %s\n", joinOrNA(strategy.AuthMethods))
		fmt.Fprintf(&b, "Client ID: %s\n", nonEmptyOrNA(client.GetClientID()))
		fmt.Fprintf(&b, "Granted Scopes: %s\n", joinOrNA(client.GetGrantedScopes()))
		fmt.Fprintf(&b, "Registration Count: %.0f\n", client.GetRegistrationCount())
		fmt.Fprintf(&b, "Created: %s\n", formatTime(client.GetCreatedAt()))
		fmt.Fprintf(&b, "Updated: %s\n", formatTime(client.GetUpdatedAt()))
	default:
		fmt.Fprintf(&b, "Type: %s\n", string(app.Type))
		fmt.Fprintf(&b, "Name: %s\n", missing)
		fmt.Fprintf(&b, "ID: %s\n", missing)
	}

	return b.String()
}

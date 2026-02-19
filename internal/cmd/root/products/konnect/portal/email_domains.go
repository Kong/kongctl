package portal

import (
	"fmt"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	emailDomainsCommandName = "email-domains"
)

type portalEmailDomainRecord struct {
	Domain       string `json:"domain"`
	Status       string `json:"status"`
	LastChecked  string `json:"last_checked"`
	LastSuccess  string `json:"last_success"`
	DNSRecords   int    `json:"dns_records"`
	LocalCreated string `json:"created_at"`
	LocalUpdated string `json:"updated_at"`
}

var (
	emailDomainsUse = emailDomainsCommandName

	emailDomainsShort = i18n.T("root.products.konnect.portal.emailDomainsShort",
		"List portal email domains configured for the organization")
	emailDomainsLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.emailDomainsLong",
		`Use the email-domains command to list custom email domains that can be used for portal emails.`))
	emailDomainsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.emailDomainsExamples",
			fmt.Sprintf(`
# List all portal email domains
%[1]s get portal email-domains
# Show details for a specific domain
%[1]s get portal email-domains example.com
`, meta.CLIName)))
)

func newGetPortalEmailDomainsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     emailDomainsUse,
		Short:   emailDomainsShort,
		Long:    emailDomainsLong,
		Example: emailDomainsExample,
		Aliases: []string{"email-domain", "emails"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := portalEmailDomainsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type portalEmailDomainsHandler struct {
	cmd *cobra.Command
}

func (h portalEmailDomainsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Provide zero arguments to list or a single domain to view details"),
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

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	emailAPI := sdk.GetPortalEmailsAPI()
	if emailAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal emails client is not available",
			Err: fmt.Errorf("portal emails client not configured"),
		}
	}

	if len(args) == 1 {
		domain := strings.TrimSpace(args[0])
		if domain == "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("domain cannot be empty"),
			}
		}

		res, err := emailAPI.GetEmailDomain(helper.GetContext(), domain)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return cmd.PrepareExecutionError("Failed to get portal email domain", err, helper.GetCmd(), attrs...)
		}

		if res.EmailDomain == nil {
			return &cmd.ExecutionError{
				Msg: "Portal email domain response was empty",
				Err: fmt.Errorf("no portal email domain returned for %s", domain),
			}
		}

		record := portalEmailDomainToRecord(*res.EmailDomain)

		return tableview.RenderForFormat(helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			record,
			res.EmailDomain,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
		)
	}

	domains, err := fetchPortalEmailDomains(helper, emailAPI, cfg)
	if err != nil {
		return err
	}

	records := make([]portalEmailDomainRecord, 0, len(domains))
	for _, domain := range domains {
		records = append(records, portalEmailDomainToRecord(domain))
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		domains,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchPortalEmailDomains(
	helper cmd.Helper,
	emailAPI helpers.PortalEmailsAPI,
	cfg config.Hook,
) ([]kkComps.EmailDomain, error) {
	var pageNumber int64 = 1
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = int64(common.DefaultRequestPageSize)
	}

	var all []kkComps.EmailDomain

	for {
		req := kkOps.ListEmailDomainsRequest{
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := emailAPI.ListEmailDomains(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list portal email domains", err, helper.GetCmd(), attrs...)
		}

		if res.ListDomains == nil {
			break
		}

		data := res.ListDomains.Data
		all = append(all, data...)

		total := int(res.ListDomains.Meta.Page.Total)
		if total == 0 || len(all) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return all, nil
}

func portalEmailDomainToRecord(domain kkComps.EmailDomain) portalEmailDomainRecord {
	status := "unknown"
	lastChecked := ""
	lastSuccess := ""

	if domain.Verification.Status != "" {
		status = string(domain.Verification.Status)
	}

	if !domain.Verification.LastTimeChecked.IsZero() {
		lastChecked = formatTime(domain.Verification.LastTimeChecked)
	}

	if !domain.Verification.LastTimeSuccess.IsZero() {
		lastSuccess = formatTime(domain.Verification.LastTimeSuccess)
	}

	return portalEmailDomainRecord{
		Domain:       domain.Domain,
		Status:       status,
		LastChecked:  lastChecked,
		LastSuccess:  lastSuccess,
		DNSRecords:   len(domain.DNSValidationRecords),
		LocalCreated: formatTime(domain.CreatedAt),
		LocalUpdated: formatTime(domain.UpdatedAt),
	}
}

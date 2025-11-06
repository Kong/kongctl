package organization

import (
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	getOrganizationShort = i18n.T("root.products.konnect.organization.getOrganizationShort",
		"Get current organization information")

	getOrganizationLong = i18n.T("root.products.konnect.organization.getOrganizationLong",
		`Use the get verb with the organization command to retrieve the organization associated with your
authentication token.`)

	getOrganizationExample = normalizers.Examples(
		i18n.T("root.products.konnect.organization.getOrganizationExamples",
			fmt.Sprintf(`
	# Get current organization information
	%[1]s get organization
	`, meta.CLIName)))
)

type textDisplayRecord struct {
	ID                  string
	Name                string
	State               string
	OwnerID             string
	LoginPath           string
	RetentionPeriodDays string
	LocalCreatedTime    string
	LocalUpdatedTime    string
}

func organizationToDisplayRecord(org *kkComps.MeOrganization) textDisplayRecord {
	const missing = "n/a"

	record := textDisplayRecord{
		ID:                  missing,
		Name:                missing,
		State:               missing,
		OwnerID:             missing,
		LoginPath:           missing,
		RetentionPeriodDays: missing,
		LocalCreatedTime:    missing,
		LocalUpdatedTime:    missing,
	}

	if org == nil {
		return record
	}

	if id := org.GetID(); id != nil && *id != "" {
		record.ID = util.AbbreviateUUID(*id)
	}

	if name := org.GetName(); name != nil && *name != "" {
		record.Name = *name
	}

	if state := org.GetState(); state != nil && *state != "" {
		record.State = string(*state)
	}

	if ownerID := org.GetOwnerID(); ownerID != nil && *ownerID != "" {
		record.OwnerID = util.AbbreviateUUID(*ownerID)
	}

	if loginPath := org.GetLoginPath(); loginPath != nil && *loginPath != "" {
		record.LoginPath = *loginPath
	}

	if retention := org.GetRetentionPeriodDays(); retention != nil {
		record.RetentionPeriodDays = fmt.Sprintf("%d", *retention)
	}

	if created := org.GetCreatedAt(); created != nil {
		record.LocalCreatedTime = created.In(time.Local).Format("2006-01-02 15:04:05")
	}

	if updated := org.GetUpdatedAt(); updated != nil {
		record.LocalUpdatedTime = updated.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

type getOrganizationCmd struct {
	*cobra.Command
}

func runGetOrganization(meAPI helpers.MeAPI, helper cmd.Helper) (*kkComps.MeOrganization, error) {
	res, err := meAPI.GetOrganizationsMe(helper.GetContext())
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get current organization", err, helper.GetCmd(), attrs...)
	}

	return res.GetMeOrganization(), nil
}

func (c *getOrganizationCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("the organization command does not accept arguments"),
		}
	}
	return nil
}

func (c *getOrganizationCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if err := c.validate(helper); err != nil {
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

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	org, err := runGetOrganization(sdk.GetMeAPI(), helper)
	if err != nil {
		return err
	}

	return tableview.RenderForFormat(
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		organizationToDisplayRecord(org),
		org,
		"Current Organization",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func newGetOrganizationCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getOrganizationCmd {
	cmd := getOrganizationCmd{
		Command: baseCmd,
	}

	cmd.Short = getOrganizationShort
	cmd.Long = getOrganizationLong
	cmd.Example = getOrganizationExample

	if parentPreRun != nil {
		cmd.PreRunE = parentPreRun
	}
	cmd.RunE = cmd.runE

	if addParentFlags != nil {
		addParentFlags(verb, cmd.Command)
	}

	return &cmd
}

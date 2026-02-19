package systemaccount

import (
	"fmt"
	"strings"
	"time"

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
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type getSystemAccountCmd struct {
	*cobra.Command
}

func newGetSystemAccountCmd(
	verb verbs.VerbValue,
	base *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getSystemAccountCmd {
	cmd := &getSystemAccountCmd{
		Command: &cobra.Command{
			Use:     base.Use,
			Short:   "List or get Konnect system accounts",
			Long:    `Use the get verb with the system account command to query Konnect system accounts.`,
			Aliases: base.Aliases,
			PreRunE: parentPreRun,
		},
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd.Command)
	}

	cmd.RunE = cmd.runE

	return cmd
}

type textDisplayRecord struct {
	ID               string
	Name             string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
	KonnectManaged   string
}

func systemAccountToDisplayRecord(s *kkComps.SystemAccount) textDisplayRecord {
	const missing = "n/a"

	record := textDisplayRecord{
		ID:               missing,
		Name:             missing,
		Description:      missing,
		KonnectManaged:   missing,
		LocalCreatedTime: missing,
		LocalUpdatedTime: missing,
	}

	if s == nil {
		return record
	}

	if id := s.GetID(); id != nil && *id != "" {
		record.ID = util.AbbreviateUUID(*id)
	}

	if name := s.GetName(); name != nil && *name != "" {
		record.Name = *name
	}

	if description := s.GetDescription(); description != nil && *description != "" {
		record.Description = *description
	}

	if konnectManaged := s.GetKonnectManaged(); konnectManaged != nil {
		record.KonnectManaged = fmt.Sprintf("%t", *konnectManaged)
	}

	if createdAt := s.GetCreatedAt(); createdAt != nil {
		record.LocalCreatedTime = createdAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	if updatedAt := s.GetUpdatedAt(); updatedAt != nil {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

func renderSystemAccountsList(
	helper cmd.Helper,
	rootLabel string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	systemAccounts []kkComps.SystemAccount,
) error {
	displayRecords := make([]textDisplayRecord, 0, len(systemAccounts))
	for i := range systemAccounts {
		displayRecords = append(displayRecords, systemAccountToDisplayRecord(&systemAccounts[i]))
	}

	options := []tableview.Option{
		tableview.WithRootLabel(rootLabel),
		tableview.WithDetailHelper(helper),
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		systemAccounts,
		"",
		options...,
	)
}

func (s *getSystemAccountCmd) runE(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)

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

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	// No args: list all
	if len(args) == 0 {
		systemAccounts, err := runList(sdk.GetSystemAccountAPI(), helper, cfg)
		if err != nil {
			return err
		}

		return renderSystemAccountsList(helper, helper.GetCmd().Name(), outType, printer, systemAccounts)
	}

	// One arg: get specific systemAccount by ID or name
	if len(args) == 1 {
		id := helper.GetArgs()[0]
		isUUID := util.IsValidUUID(id)

		var systemAccount *kkComps.SystemAccount

		if !isUUID {
			systemAccount, err = runListByName(id, sdk.GetSystemAccountAPI(), helper, cfg)
			if err != nil {
				return err
			}
		} else {
			systemAccount, err = runGet(id, sdk.GetSystemAccountAPI(), helper)
			if err != nil {
				return err
			}
		}

		return tableview.RenderForFormat(helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			systemAccountToDisplayRecord(systemAccount),
			systemAccount,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
			tableview.WithDetailContext("system_account", func(int) any {
				return systemAccount
			}),
			tableview.WithDetailHelper(helper),
		)
	}

	return fmt.Errorf("too many arguments")
}

func runGet(id string, kkClient helpers.SystemAccountAPI, helper cmd.Helper) (*kkComps.SystemAccount, error) {
	res, err := kkClient.GetSystemAccount(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get System Account", err, helper.GetCmd(), attrs...)
	}

	return res.GetSystemAccount(), nil
}

func runList(kkClient helpers.SystemAccountAPI, helper cmd.Helper, cfg config.Hook) ([]kkComps.SystemAccount, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.SystemAccount

	for {
		req := kkOps.GetSystemAccountsRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := kkClient.ListSystemAccounts(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list System Accounts", err, helper.GetCmd(), attrs...)
		}

		allData = append(allData, res.GetSystemAccountCollection().Data...)
		totalItems := res.GetSystemAccountCollection().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runListByName(name string, kkClient helpers.SystemAccountAPI, helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.SystemAccount, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.SystemAccount
	for {
		req := kkOps.GetSystemAccountsRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
			Filter: &kkOps.GetSystemAccountsQueryParamFilter{
				Name: &kkComps.LegacyStringFieldFilter{
					Eq: kk.String(name),
				},
			},
		}

		res, err := kkClient.ListSystemAccounts(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list System Accounts", err, helper.GetCmd(), attrs...)
		}

		allData = append(allData, res.GetSystemAccountCollection().Data...)
		totalItems := res.GetSystemAccountCollection().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	if len(allData) > 0 {
		return &allData[0], nil
	}
	return nil, fmt.Errorf("system account with name %s not found", name)
}

func buildSystemAccountChildView(accounts []kkComps.SystemAccount) tableview.ChildView {
	rows := make([]table.Row, 0, len(accounts))
	for i := range accounts {
		record := systemAccountToDisplayRecord(&accounts[i])
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(accounts) {
			return ""
		}
		return systemAccountDetailView(&accounts[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           rows,
		DetailRenderer: detailFn,
		Title:          "System Accounts",
		ParentType:     "system-account",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(accounts) {
				return nil
			}
			return &accounts[index]
		},
	}
}

func systemAccountDetailView(account *kkComps.SystemAccount) string {
	if account == nil {
		return ""
	}

	const missing = "n/a"

	id := missing
	if value := account.GetID(); value != nil && *value != "" {
		id = util.AbbreviateUUID(*value)
	}

	name := missing
	if value := account.GetName(); value != nil && *value != "" {
		name = *value
	}

	description := missing
	if value := account.GetDescription(); value != nil && *value != "" {
		description = *value
	}

	konnectManaged := missing
	if value := account.GetKonnectManaged(); value != nil {
		konnectManaged = fmt.Sprintf("%t", *value)
	}

	createdAt := missing
	if value := account.GetCreatedAt(); value != nil {
		createdAt = value.In(time.Local).Format("2006-01-02 15:04:05")
	}

	updatedAt := missing
	if value := account.GetUpdatedAt(); value != nil {
		updatedAt = value.In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "konnect_managed: %s\n", konnectManaged)
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

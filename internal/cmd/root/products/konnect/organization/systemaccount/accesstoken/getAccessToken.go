package accesstoken

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
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

type getAccessTokenCmd struct {
	*cobra.Command
}

func newGetAccessTokenCmd(
	verb verbs.VerbValue,
	base *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getAccessTokenCmd {
	c := &getAccessTokenCmd{
		Command: &cobra.Command{
			Use:     base.Use,
			Short:   "List or get Konnect system account access tokens",
			Long:    `Use the get verb with the access-token command to query system account access tokens.`,
			Aliases: base.Aliases,
			Args:    cobra.RangeArgs(1, 2),
			PreRunE: parentPreRun,
		},
	}

	if addParentFlags != nil {
		addParentFlags(verb, c.Command)
	}

	c.RunE = c.runE

	return c
}

type accessTokenDisplayRecord struct {
	ID               string
	Name             string
	LocalCreatedTime string
	LocalUpdatedTime string
	ExpiresAt        string
	LastUsedAt       string
}

func accessTokenToDisplayRecord(t *kkComps.SystemAccountAccessToken) accessTokenDisplayRecord {
	const missing = "n/a"

	record := accessTokenDisplayRecord{
		ID:               missing,
		Name:             missing,
		LocalCreatedTime: missing,
		LocalUpdatedTime: missing,
		ExpiresAt:        missing,
		LastUsedAt:       missing,
	}

	if t == nil {
		return record
	}

	if id := t.GetID(); id != nil && *id != "" {
		record.ID = util.AbbreviateUUID(*id)
	}

	if name := t.GetName(); name != nil && *name != "" {
		record.Name = *name
	}

	if createdAt := t.GetCreatedAt(); createdAt != nil {
		record.LocalCreatedTime = createdAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	if updatedAt := t.GetUpdatedAt(); updatedAt != nil {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	if expiresAt := t.GetExpiresAt(); expiresAt != nil {
		record.ExpiresAt = expiresAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	if lastUsedAt := t.GetLastUsedAt(); lastUsedAt != nil {
		record.LastUsedAt = lastUsedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

func renderAccessTokenList(
	helper cmd.Helper,
	rootLabel string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	tokens []kkComps.SystemAccountAccessToken,
) error {
	displayRecords := make([]accessTokenDisplayRecord, 0, len(tokens))
	for i := range tokens {
		displayRecords = append(displayRecords, accessTokenToDisplayRecord(&tokens[i]))
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
		tokens,
		"",
		options...,
	)
}

func (g *getAccessTokenCmd) runE(c *cobra.Command, args []string) error {
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

	accountID, err := resolveSystemAccountID(args[0], sdk, helper, cfg)
	if err != nil {
		return err
	}

	if len(args) == 2 {
		token, err := runGetAccessToken(accountID, args[1], sdk.GetSystemAccountAccessTokenAPI(), helper)
		if err != nil {
			return err
		}

		return tableview.RenderForFormat(helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			accessTokenToDisplayRecord(token),
			token,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
			tableview.WithDetailContext("access_token", func(int) any {
				return token
			}),
			tableview.WithDetailHelper(helper),
		)
	}

	tokens, err := runListAccessTokens(accountID, sdk.GetSystemAccountAccessTokenAPI(), helper, cfg)
	if err != nil {
		return err
	}

	return renderAccessTokenList(helper, helper.GetCmd().Name(), outType, printer, tokens)
}

func resolveSystemAccountID(
	idOrName string,
	sdk helpers.SDKAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (string, error) {
	if util.IsValidUUID(idOrName) {
		return idOrName, nil
	}

	account, err := resolveSystemAccountByName(idOrName, sdk.GetSystemAccountAPI(), helper, cfg)
	if err != nil {
		return "", err
	}

	id := account.GetID()
	if id == nil || *id == "" {
		return "", fmt.Errorf("resolved system account has no ID")
	}

	return *id, nil
}

func resolveSystemAccountByName(
	name string,
	kkClient helpers.SystemAccountAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.SystemAccount, error) {
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var pageNumber int64 = 1
	var allData []kkComps.SystemAccount

	for {
		req := kkOps.GetSystemAccountsRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
			Filter: &kkOps.GetSystemAccountsQueryParamFilter{
				Name: &kkComps.LegacyStringFieldFilter{
					Eq: &name,
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

	return nil, fmt.Errorf("system account with name %q not found", name)
}

func runGetAccessToken(
	accountID string,
	tokenID string,
	kkClient helpers.SystemAccountAccessTokenAPI,
	helper cmd.Helper,
) (*kkComps.SystemAccountAccessToken, error) {
	res, err := kkClient.GetSystemAccountAccessToken(helper.GetContext(), accountID, tokenID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get access token", err, helper.GetCmd(), attrs...)
	}

	return res.GetSystemAccountAccessToken(), nil
}

func runListAccessTokens(
	accountID string,
	kkClient helpers.SystemAccountAccessTokenAPI,
	helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.SystemAccountAccessToken, error) {
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var pageNumber int64 = 1
	var allData []kkComps.SystemAccountAccessToken

	for {
		req := kkOps.GetSystemAccountIDAccessTokensRequest{
			AccountID:  accountID,
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		res, err := kkClient.ListSystemAccountAccessTokens(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list access tokens", err, helper.GetCmd(), attrs...)
		}

		collection := res.GetSystemAccountAccessTokenCollection()
		allData = append(allData, collection.Data...)

		if collection.Meta == nil || len(allData) >= int(collection.Meta.Page.Total) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func buildAccessTokenChildView(tokens []kkComps.SystemAccountAccessToken) tableview.ChildView {
	rows := make([]table.Row, 0, len(tokens))
	for i := range tokens {
		record := accessTokenToDisplayRecord(&tokens[i])
		rows = append(rows, table.Row{record.ID, record.Name, record.ExpiresAt})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(tokens) {
			return ""
		}
		return accessTokenDetailView(&tokens[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME", "EXPIRES_AT"},
		Rows:           rows,
		DetailRenderer: detailFn,
		Title:          "Access Tokens",
		ParentType:     "access-token",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(tokens) {
				return nil
			}
			return &tokens[index]
		},
	}
}

func accessTokenDetailView(token *kkComps.SystemAccountAccessToken) string {
	if token == nil {
		return ""
	}

	const missing = "n/a"

	id := missing
	if value := token.GetID(); value != nil && *value != "" {
		id = util.AbbreviateUUID(*value)
	}

	name := missing
	if value := token.GetName(); value != nil && *value != "" {
		name = *value
	}

	createdAt := missing
	if value := token.GetCreatedAt(); value != nil {
		createdAt = value.In(time.Local).Format("2006-01-02 15:04:05")
	}

	updatedAt := missing
	if value := token.GetUpdatedAt(); value != nil {
		updatedAt = value.In(time.Local).Format("2006-01-02 15:04:05")
	}

	expiresAt := missing
	if value := token.GetExpiresAt(); value != nil {
		expiresAt = value.In(time.Local).Format("2006-01-02 15:04:05")
	}

	lastUsedAt := missing
	if value := token.GetLastUsedAt(); value != nil {
		lastUsedAt = value.In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)
	fmt.Fprintf(&b, "expires_at: %s\n", expiresAt)
	fmt.Fprintf(&b, "last_used_at: %s\n", lastUsedAt)

	return strings.TrimRight(b.String(), "\n")
}

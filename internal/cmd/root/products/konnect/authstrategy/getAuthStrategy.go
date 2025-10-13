package authstrategy

import (
	"fmt"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
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

var (
	getAuthStrategiesShort = i18n.T("root.products.konnect.authstrategy.getAuthStrategiesShort",
		"List or get Konnect authentication strategies")
	getAuthStrategiesLong = i18n.T("root.products.konnect.authstrategy.getAuthStrategiesLong",
		`Use the get verb with the auth-strategy command to query Konnect authentication strategies.`)
	getAuthStrategiesExample = normalizers.Examples(
		i18n.T("root.products.konnect.authstrategy.getAuthStrategyExamples",
			fmt.Sprintf(`
	# List all the auth strategies for the organization
	%[1]s get auth-strategies
	# Get details for an auth strategy with a specific ID 
	%[1]s get auth-strategy 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
	# Get details for an auth strategy with a specific name
	%[1]s get auth-strategy my-oauth-strategy 
	# List auth strategies of a specific type
	%[1]s get auth-strategies --type key_auth
	# Get all the auth strategies using command aliases
	%[1]s get as
	`, meta.CLIName)))
)

const (
	typeFlagName = "type"
)

// Represents a text display record for an Auth Strategy
type textDisplayRecord struct {
	ID               string
	Name             string
	DisplayName      string
	StrategyType     string
	Active           string
	DCRProvider      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func authStrategyToDisplayRecord(strategy kkComps.AppAuthStrategy) textDisplayRecord {
	missing := "n/a"

	var record textDisplayRecord

	// Handle the union type - check which variant we have
	if strategy.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse != nil {
		keyAuth := strategy.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse
		record.ID = util.AbbreviateUUID(keyAuth.ID)
		record.Name = keyAuth.Name
		record.DisplayName = keyAuth.DisplayName
		record.StrategyType = string(keyAuth.StrategyType)
		record.Active = fmt.Sprintf("%t", keyAuth.Active)

		if keyAuth.DcrProvider != nil {
			record.DCRProvider = keyAuth.DcrProvider.Name
		} else {
			record.DCRProvider = missing
		}

		record.LocalCreatedTime = keyAuth.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
		record.LocalUpdatedTime = keyAuth.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	} else if strategy.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse != nil {
		openID := strategy.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse
		record.ID = util.AbbreviateUUID(openID.ID)
		record.Name = openID.Name
		record.DisplayName = openID.DisplayName
		record.StrategyType = string(openID.StrategyType)
		record.Active = fmt.Sprintf("%t", openID.Active)

		if openID.DcrProvider != nil {
			record.DCRProvider = openID.DcrProvider.Name
		} else {
			record.DCRProvider = missing
		}

		record.LocalCreatedTime = openID.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
		record.LocalUpdatedTime = openID.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	} else {
		// This shouldn't happen, but handle gracefully
		record.ID = missing
		record.Name = missing
		record.DisplayName = missing
		record.StrategyType = missing
		record.Active = missing
		record.DCRProvider = missing
		record.LocalCreatedTime = missing
		record.LocalUpdatedTime = missing
	}

	return record
}

func authStrategyDetailView(strategy kkComps.AppAuthStrategy) string {
	var b strings.Builder
	missing := "n/a"

	if strategy.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse != nil {
		keyAuth := strategy.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse
		fmt.Fprintf(&b, "Name: %s\n", keyAuth.Name)
		fmt.Fprintf(&b, "ID: %s\n", keyAuth.ID)
		fmt.Fprintf(&b, "Display Name: %s\n", keyAuth.DisplayName)
		fmt.Fprintf(&b, "Strategy Type: %s\n", keyAuth.StrategyType)
		fmt.Fprintf(&b, "Active: %t\n", keyAuth.Active)
		if keyAuth.DcrProvider != nil {
			fmt.Fprintf(&b, "DCR Provider: %s\n", keyAuth.DcrProvider.Name)
		} else {
			fmt.Fprintf(&b, "DCR Provider: %s\n", missing)
		}
		fmt.Fprintf(&b, "Created: %s\n", keyAuth.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&b, "Updated: %s\n", keyAuth.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05"))
		return b.String()
	}

	if strategy.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse != nil {
		openID := strategy.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse
		fmt.Fprintf(&b, "Name: %s\n", openID.Name)
		fmt.Fprintf(&b, "ID: %s\n", openID.ID)
		fmt.Fprintf(&b, "Display Name: %s\n", openID.DisplayName)
		fmt.Fprintf(&b, "Strategy Type: %s\n", openID.StrategyType)
		fmt.Fprintf(&b, "Active: %t\n", openID.Active)
		if openID.DcrProvider != nil {
			fmt.Fprintf(&b, "DCR Provider: %s\n", openID.DcrProvider.Name)
		} else {
			fmt.Fprintf(&b, "DCR Provider: %s\n", missing)
		}
		fmt.Fprintf(&b, "Created: %s\n", openID.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&b, "Updated: %s\n", openID.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05"))
		return b.String()
	}

	fmt.Fprintf(&b, "Name: %s\n", missing)
	fmt.Fprintf(&b, "ID: %s\n", missing)
	fmt.Fprintf(&b, "Display Name: %s\n", missing)
	fmt.Fprintf(&b, "Strategy Type: %s\n", missing)
	fmt.Fprintf(&b, "Active: %s\n", missing)
	fmt.Fprintf(&b, "DCR Provider: %s\n", missing)
	return b.String()
}

type getAuthStrategyCmd struct {
	*cobra.Command
	strategyType string
}

func runListByName(name string, strategyType string, kkClient helpers.AppAuthStrategiesAPI, helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.AppAuthStrategy, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.AppAuthStrategy

	for {
		req := kkOps.ListAppAuthStrategiesRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		// Apply type filter if specified
		if strategyType != "" {
			req.Filter = &kkOps.Filter{
				StrategyType: &kkComps.StringFieldFilter{
					Eq: kk.String(strategyType),
				},
			}
		}

		res, err := kkClient.ListAppAuthStrategies(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list auth strategies", err, helper.GetCmd(), attrs...)
		}

		// Filter by name since SDK doesn't support name filtering directly
		for _, strategy := range res.GetListAppAuthStrategiesResponse().Data {
			strategyName := getStrategyName(strategy)
			if strategyName == name {
				return &strategy, nil
			}
		}

		allData = append(allData, res.GetListAppAuthStrategiesResponse().Data...)
		totalItems := res.GetListAppAuthStrategiesResponse().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return nil, fmt.Errorf("auth strategy with name %s not found", name)
}

func runList(strategyType string, kkClient helpers.AppAuthStrategiesAPI, helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.AppAuthStrategy, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []kkComps.AppAuthStrategy

	for {
		req := kkOps.ListAppAuthStrategiesRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		// Apply type filter if specified
		if strategyType != "" {
			req.Filter = &kkOps.Filter{
				StrategyType: &kkComps.StringFieldFilter{
					Eq: kk.String(strategyType),
				},
			}
		}

		res, err := kkClient.ListAppAuthStrategies(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list auth strategies", err, helper.GetCmd(), attrs...)
		}

		allData = append(allData, res.GetListAppAuthStrategiesResponse().Data...)
		totalItems := res.GetListAppAuthStrategiesResponse().Meta.Page.Total

		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runGet(id string, kkClient helpers.AppAuthStrategiesAPI, helper cmd.Helper,
) (*kkComps.AppAuthStrategy, error) {
	res, err := kkClient.GetAppAuthStrategy(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get auth strategy", err, helper.GetCmd(), attrs...)
	}

	// Convert CreateAppAuthStrategyResponse to AppAuthStrategy
	createResponse := res.GetCreateAppAuthStrategyResponse()
	if createResponse == nil {
		return nil, fmt.Errorf("unexpected nil response from GetAppAuthStrategy")
	}

	// Convert the response to AppAuthStrategy union type
	var strategy kkComps.AppAuthStrategy
	if createResponse.AppAuthStrategyKeyAuthResponse != nil {
		keyAuthResp := createResponse.AppAuthStrategyKeyAuthResponse
		strategy = kkComps.CreateAppAuthStrategyKeyAuth(
			kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse{
				ID:          keyAuthResp.ID,
				Name:        keyAuthResp.Name,
				DisplayName: keyAuthResp.DisplayName,
				StrategyType: kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyStrategyType(
					keyAuthResp.StrategyType),
				Configs: kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyConfigs{
					KeyAuth: keyAuthResp.Configs.KeyAuth,
				},
				Active: keyAuthResp.Active,
				DcrProvider: (*kkComps.AppAuthStrategyKeyAuthResponseDcrProvider)(
					keyAuthResp.DcrProvider),
				Labels:    keyAuthResp.Labels,
				CreatedAt: keyAuthResp.CreatedAt,
				UpdatedAt: keyAuthResp.UpdatedAt,
			},
		)
	} else if createResponse.AppAuthStrategyOpenIDConnectResponse != nil {
		openIDResp := createResponse.AppAuthStrategyOpenIDConnectResponse
		strategy = kkComps.CreateAppAuthStrategyOpenidConnect(
			kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse{
				ID:          openIDResp.ID,
				Name:        openIDResp.Name,
				DisplayName: openIDResp.DisplayName,
				StrategyType: kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyStrategyType(
					openIDResp.StrategyType),
				Configs: kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyConfigs{
					OpenidConnect: openIDResp.Configs.OpenidConnect,
				},
				Active: openIDResp.Active,
				DcrProvider: (*kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyDcrProvider)(
					openIDResp.DcrProvider),
				Labels:    openIDResp.Labels,
				CreatedAt: openIDResp.CreatedAt,
				UpdatedAt: openIDResp.UpdatedAt,
			},
		)
	} else {
		return nil, fmt.Errorf("unexpected response type from GetAppAuthStrategy")
	}

	return &strategy, nil
}

// Helper function to get strategy name from union type
func getStrategyName(strategy kkComps.AppAuthStrategy) string {
	if strategy.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse != nil {
		return strategy.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse.Name
	} else if strategy.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse != nil {
		return strategy.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse.Name
	}
	return ""
}

func (c *getAuthStrategyCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing auth strategies requires 0 or 1 arguments (name or ID)"),
		}
	}

	// Validate strategy type if provided
	if c.strategyType != "" && c.strategyType != "key_auth" && c.strategyType != "openid_connect" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("invalid strategy type '%s'. Must be 'key_auth' or 'openid_connect'", c.strategyType),
		}
	}

	config, err := helper.GetConfig()
	if err != nil {
		return err
	}

	pageSize := config.GetInt(common.RequestPageSizeConfigPath)
	if pageSize < 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("%s must be greater than 0", common.RequestPageSizeFlagName),
		}
	}
	return nil
}

func (c *getAuthStrategyCmd) runE(cobraCmd *cobra.Command, args []string) error {
	var e error
	helper := cmd.BuildHelper(cobraCmd, args)
	if e = c.validate(helper); e != nil {
		return e
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	var printer cli.PrintFlusher
	if outType != cmdCommon.INTERACTIVE {
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

	// 'get auth-strategies' can be run in various ways:
	//	> get auth-strategies <id>    # Get by UUID
	//  > get auth-strategies <name>	# Get by name
	//  > get auth-strategies         # List all
	if len(helper.GetArgs()) == 1 { // validate above checks that args is 0 or 1
		id := strings.TrimSpace(helper.GetArgs()[0])

		isUUID := util.IsValidUUID(id)

		if !isUUID {
			// If the ID is not a UUID, then it is a name
			// search for the auth strategy by name
			strategy, err := runListByName(id, c.strategyType, sdk.GetAppAuthStrategiesAPI(), helper, cfg)
			if err != nil {
				return err
			}
			return tableview.RenderForFormat(
				outType,
				printer,
				helper.GetStreams(),
				authStrategyToDisplayRecord(*strategy),
				strategy,
				"",
				tableview.WithRootLabel(helper.GetCmd().Name()),
			)
		}
		strategy, err := runGet(id, sdk.GetAppAuthStrategiesAPI(), helper)
		if err != nil {
			return err
		}
		return tableview.RenderForFormat(
			outType,
			printer,
			helper.GetStreams(),
			authStrategyToDisplayRecord(*strategy),
			strategy,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
		)
	}

	strategies, err := runList(c.strategyType, sdk.GetAppAuthStrategiesAPI(), helper, cfg)
	if err != nil {
		return err
	}
	displayRecords := make([]textDisplayRecord, 0, len(strategies))
	for i := range strategies {
		displayRecords = append(displayRecords, authStrategyToDisplayRecord(strategies[i]))
	}
	tableRows := make([]table.Row, 0, len(displayRecords))
	for _, record := range displayRecords {
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(strategies) {
			return ""
		}
		return authStrategyDetailView(strategies[index])
	}

	return tableview.RenderForFormat(
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		strategies,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME"}, tableRows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func newGetAuthStrategyCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getAuthStrategyCmd {
	rv := getAuthStrategyCmd{
		Command: baseCmd,
	}

	rv.Short = getAuthStrategiesShort
	rv.Long = getAuthStrategiesLong
	rv.Example = getAuthStrategiesExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	// Add type filter flag
	rv.Flags().StringVar(&rv.strategyType, typeFlagName, "",
		i18n.T("root.products.konnect.authstrategy.typeDesc",
			"Filter auth strategies by type (key_auth, openid_connect)"))

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return &rv
}

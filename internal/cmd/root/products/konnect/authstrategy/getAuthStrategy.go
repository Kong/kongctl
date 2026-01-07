package authstrategy

import (
	"fmt"
	"sort"
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

type authStrategyVariantInfo struct {
	id           string
	name         string
	displayName  string
	strategyType string
	active       bool
	dcrProvider  string
	createdAt    time.Time
	updatedAt    time.Time
	labels       map[string]string
	rawParent    any
}

func extractAuthStrategyVariant(strategy kkComps.AppAuthStrategy) authStrategyVariantInfo {
	if keyAuth := strategy.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse; keyAuth != nil {
		provider := ""
		if keyAuth.DcrProvider != nil {
			provider = strings.TrimSpace(keyAuth.DcrProvider.Name)
		}
		return authStrategyVariantInfo{
			id:           keyAuth.ID,
			name:         keyAuth.Name,
			displayName:  keyAuth.DisplayName,
			strategyType: string(keyAuth.StrategyType),
			active:       keyAuth.Active,
			dcrProvider:  provider,
			createdAt:    keyAuth.CreatedAt,
			updatedAt:    keyAuth.UpdatedAt,
			labels:       keyAuth.Labels,
			rawParent:    keyAuth,
		}
	}

	if openID := strategy.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse; openID != nil {
		provider := ""
		if openID.DcrProvider != nil {
			provider = strings.TrimSpace(openID.DcrProvider.Name)
		}
		return authStrategyVariantInfo{
			id:           openID.ID,
			name:         openID.Name,
			displayName:  openID.DisplayName,
			strategyType: string(openID.StrategyType),
			active:       openID.Active,
			dcrProvider:  provider,
			createdAt:    openID.CreatedAt,
			updatedAt:    openID.UpdatedAt,
			labels:       openID.Labels,
			rawParent:    openID,
		}
	}

	return authStrategyVariantInfo{}
}

func summarizeLabels(labels map[string]string, missing string) string {
	switch {
	case labels == nil:
		return missing
	case len(labels) == 0:
		return "[]"
	default:
		pairs := make([]string, 0, len(labels))
		for k, v := range labels {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(pairs)
		return strings.Join(pairs, ", ")
	}
}

func authStrategyToDisplayRecord(strategy kkComps.AppAuthStrategy) textDisplayRecord {
	missing := "n/a"

	info := extractAuthStrategyVariant(strategy)
	record := textDisplayRecord{
		ID:               missing,
		Name:             missing,
		DisplayName:      missing,
		StrategyType:     missing,
		Active:           missing,
		DCRProvider:      missing,
		LocalCreatedTime: missing,
		LocalUpdatedTime: missing,
	}

	if info.rawParent != nil {
		if info.id != "" {
			record.ID = util.AbbreviateUUID(info.id)
		}
		if strings.TrimSpace(info.name) != "" {
			record.Name = info.name
		}
		if strings.TrimSpace(info.displayName) != "" {
			record.DisplayName = info.displayName
		}
		if strings.TrimSpace(info.strategyType) != "" {
			record.StrategyType = info.strategyType
		}
		record.Active = fmt.Sprintf("%t", info.active)
		if strings.TrimSpace(info.dcrProvider) != "" {
			record.DCRProvider = info.dcrProvider
		}
		if !info.createdAt.IsZero() {
			record.LocalCreatedTime = info.createdAt.In(time.Local).Format("2006-01-02 15:04:05")
		}
		if !info.updatedAt.IsZero() {
			record.LocalUpdatedTime = info.updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
		}
	}

	return record
}

func authStrategyDetailView(strategy kkComps.AppAuthStrategy) string {
	const missing = "n/a"

	info := extractAuthStrategyVariant(strategy)
	if info.rawParent == nil {
		return ""
	}

	valueOrMissing := func(val string) string {
		val = strings.TrimSpace(val)
		if val == "" {
			return missing
		}
		return val
	}

	id := valueOrMissing(info.id)
	name := valueOrMissing(info.name)
	displayName := valueOrMissing(info.displayName)
	strategyType := valueOrMissing(info.strategyType)
	activeValue := fmt.Sprintf("%t", info.active)
	dcrProvider := valueOrMissing(info.dcrProvider)
	created := missing
	if !info.createdAt.IsZero() {
		created = info.createdAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	updated := missing
	if !info.updatedAt.IsZero() {
		updated = info.updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	labels := summarizeLabels(info.labels, missing)

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "display_name: %s\n", displayName)
	fmt.Fprintf(&b, "strategy_type: %s\n", strategyType)
	fmt.Fprintf(&b, "active: %s\n", activeValue)
	fmt.Fprintf(&b, "dcr_provider: %s\n", dcrProvider)
	fmt.Fprintf(&b, "configs: %s\n", missing)
	fmt.Fprintf(&b, "labels: %s\n", labels)
	fmt.Fprintf(&b, "created_at: %s\n", created)
	fmt.Fprintf(&b, "updated_at: %s\n", updated)

	return strings.TrimRight(b.String(), "\n")
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
			req.Filter = &kkOps.QueryParamFilter{
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
			req.Filter = &kkOps.QueryParamFilter{
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

		// Convert DcrProvider type if present
		var dcrProvider *kkComps.AppAuthStrategyKeyAuthResponseDcrProvider
		if keyAuthResp.DcrProvider != nil {
			dcrProvider = &kkComps.AppAuthStrategyKeyAuthResponseDcrProvider{
				ID:   keyAuthResp.DcrProvider.ID,
				Name: keyAuthResp.DcrProvider.Name,
			}
		}

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
				Active:      keyAuthResp.Active,
				DcrProvider: dcrProvider,
				Labels:      keyAuthResp.Labels,
				CreatedAt:   keyAuthResp.CreatedAt,
				UpdatedAt:   keyAuthResp.UpdatedAt,
			},
		)
	} else if createResponse.AppAuthStrategyOpenIDConnectResponse != nil {
		openIDResp := createResponse.AppAuthStrategyOpenIDConnectResponse

		// Convert DcrProvider type if present
		var dcrProvider *kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyDcrProvider
		if openIDResp.DcrProvider != nil {
			dcrProvider = &kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyDcrProvider{
				ID:   openIDResp.DcrProvider.ID,
				Name: openIDResp.DcrProvider.Name,
			}
		}

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
				Active:      openIDResp.Active,
				DcrProvider: dcrProvider,
				Labels:      openIDResp.Labels,
				CreatedAt:   openIDResp.CreatedAt,
				UpdatedAt:   openIDResp.UpdatedAt,
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
				false,
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
			false,
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
	displayRecords := make([]textDisplayRecord, len(strategies))
	for i := range strategies {
		displayRecords[i] = authStrategyToDisplayRecord(strategies[i])
	}
	return renderAuthStrategyList(helper, helper.GetCmd().Name(), outType, printer, strategies)
}

func renderAuthStrategyList(
	helper cmd.Helper,
	rootLabel string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	strategies []kkComps.AppAuthStrategy,
) error {
	displayRecords := make([]textDisplayRecord, 0, len(strategies))
	for i := range strategies {
		displayRecords = append(displayRecords, authStrategyToDisplayRecord(strategies[i]))
	}

	childView := buildAuthStrategyChildView(strategies)

	options := []tableview.Option{
		tableview.WithCustomTable(childView.Headers, childView.Rows),
		tableview.WithRootLabel(rootLabel),
		tableview.WithDetailHelper(helper),
	}
	if childView.DetailRenderer != nil {
		options = append(options, tableview.WithDetailRenderer(childView.DetailRenderer))
	}
	if childView.DetailContext != nil {
		options = append(options, tableview.WithDetailContext(childView.ParentType, childView.DetailContext))
	} else if childView.ParentType != "" {
		options = append(options, tableview.WithDetailContext(childView.ParentType, func(int) any { return nil }))
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		strategies,
		"",
		options...,
	)
}

func buildAuthStrategyChildView(strategies []kkComps.AppAuthStrategy) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(strategies))
	for i := range strategies {
		record := authStrategyToDisplayRecord(strategies[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(strategies) {
			return ""
		}
		return authStrategyDetailView(strategies[index])
	}

	return tableview.ChildView{
		Headers:        []string{"id", "name"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Application Auth Strategies",
		ParentType:     "auth-strategy",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(strategies) {
				return nil
			}
			info := extractAuthStrategyVariant(strategies[index])
			return info.rawParent
		},
	}
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

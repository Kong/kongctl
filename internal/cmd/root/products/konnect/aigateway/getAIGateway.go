package aigateway

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
	getAIGatewaysShort = i18n.T(
		"root.products.konnect.ai-gateway.getAIGatewaysShort",
		"List or get Konnect AI Gateways",
	)
	getAIGatewaysLong = i18n.T(
		"root.products.konnect.ai-gateway.getAIGatewaysLong",
		`Use the get verb with the ai-gateway command to query Konnect AI Gateways.`,
	)
	getAIGatewaysExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.getAIGatewaysExamples",
			fmt.Sprintf(`# List all AI Gateways for the organization
%[1]s get ai-gateway
# Get details for an AI Gateway with a specific ID
%[1]s get ai-gateway 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
# Get details for an AI Gateway with a specific display name
%[1]s get ai-gateway "Customer Support Gateway"
# Get all AI Gateways using command aliases
%[1]s get aigw
`, meta.CLIName)),
	)
)

type aiGatewayDisplayRecord struct {
	ID               string
	DisplayName      string
	Description      string
	ProxyURLCount    string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type getAIGatewayCmd struct {
	*cobra.Command
}

func aiGatewayToDisplayRecord(gateway kkComps.AIGateway) aiGatewayDisplayRecord {
	const missing = "n/a"

	record := aiGatewayDisplayRecord{
		ID:               missing,
		DisplayName:      missing,
		Description:      missing,
		ProxyURLCount:    fmt.Sprintf("%d", len(gateway.ProxyUrls)),
		LocalCreatedTime: missing,
		LocalUpdatedTime: missing,
	}

	if strings.TrimSpace(gateway.ID) != "" {
		record.ID = util.AbbreviateUUID(gateway.ID)
	}
	if strings.TrimSpace(gateway.DisplayName) != "" {
		record.DisplayName = gateway.DisplayName
	}
	if gateway.Description != nil && strings.TrimSpace(*gateway.Description) != "" {
		record.Description = strings.TrimSpace(*gateway.Description)
	}
	if !gateway.CreatedAt.IsZero() {
		record.LocalCreatedTime = gateway.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	if !gateway.UpdatedAt.IsZero() {
		record.LocalUpdatedTime = gateway.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

func runList(
	kkClient helpers.AIGatewayAPI,
	helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.AIGateway, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.AIGateway
	for {
		res, err := kkClient.ListAiGateways(helper.GetContext(), &requestPageSize, &pageNumber)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateways", err, helper.GetCmd(), attrs...)
		}
		if res == nil || res.ListAIGatewaysResponse == nil {
			return allData, nil
		}

		pageData := res.ListAIGatewaysResponse.Data
		allData = append(allData, pageData...)

		totalItems := int(res.ListAIGatewaysResponse.Meta.Page.Total)
		if totalItems > 0 {
			if len(allData) >= totalItems {
				break
			}
		} else if len(pageData) < int(requestPageSize) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runListByDisplayName(
	displayName string,
	kkClient helpers.AIGatewayAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.AIGateway, error) {
	displayName = strings.TrimSpace(displayName)
	gateways, err := runList(kkClient, helper, cfg)
	if err != nil {
		return nil, err
	}

	var matches []kkComps.AIGateway
	for _, gateway := range gateways {
		if gateway.DisplayName == displayName {
			matches = append(matches, gateway)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("AI Gateway with display_name %q not found", displayName)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("AI Gateway display_name %q matches %d gateways", displayName, len(matches))
	}
}

func runGet(id string, kkClient helpers.AIGatewayAPI, helper cmd.Helper) (*kkComps.AIGateway, error) {
	res, err := kkClient.GetAiGateway(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get AI Gateway", err, helper.GetCmd(), attrs...)
	}

	return res.GetAIGateway(), nil
}

func (c *getAIGatewayCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateways requires 0 or 1 arguments (display name or ID)"),
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

func (c *getAIGatewayCmd) runE(cobraCmd *cobra.Command, args []string) error {
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

	api := sdk.GetAIGatewayAPI()
	if len(helper.GetArgs()) == 1 {
		identifier := strings.TrimSpace(helper.GetArgs()[0])
		var gateway *kkComps.AIGateway
		if util.IsValidUUID(identifier) {
			gateway, err = runGet(identifier, api, helper)
		} else {
			gateway, err = runListByDisplayName(identifier, api, helper, cfg)
		}
		if err != nil {
			return err
		}

		return tableview.RenderForFormat(
			helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			aiGatewayToDisplayRecord(*gateway),
			gateway,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
			tableview.WithDetailHelper(helper),
			tableview.WithDetailContext(common.ViewParentAIGateway, func(index int) any {
				if index != 0 {
					return nil
				}
				return gateway
			}),
		)
	}

	gateways, err := runList(api, helper, cfg)
	if err != nil {
		return err
	}

	return renderAIGatewayList(helper, helper.GetCmd().Name(), false, outType, printer, gateways)
}

func renderAIGatewayList(
	helper cmd.Helper,
	rootLabel string,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	gateways []kkComps.AIGateway,
) error {
	displayRecords := make([]aiGatewayDisplayRecord, 0, len(gateways))
	for i := range gateways {
		displayRecords = append(displayRecords, aiGatewayToDisplayRecord(gateways[i]))
	}

	return tableview.RenderForFormat(
		helper,
		interactive,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		gateways,
		"",
		tableview.WithRootLabel(rootLabel),
		tableview.WithDetailHelper(helper),
	)
}

func buildAIGatewayChildView(gateways []kkComps.AIGateway) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(gateways))
	for i := range gateways {
		record := aiGatewayToDisplayRecord(gateways[i])
		tableRows = append(tableRows, table.Row{record.ID, record.DisplayName})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(gateways) {
			return ""
		}
		return aiGatewayDetailView(&gateways[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "DISPLAY NAME"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "AI Gateways",
		ParentType:     common.ViewParentAIGateway,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(gateways) {
				return nil
			}
			return &gateways[index]
		},
	}
}

func aiGatewayDetailView(gateway *kkComps.AIGateway) string {
	if gateway == nil {
		return ""
	}

	const missing = "n/a"
	valueOrMissing := func(value string) string {
		value = strings.TrimSpace(value)
		if value == "" {
			return missing
		}
		return value
	}

	description := missing
	if gateway.Description != nil && strings.TrimSpace(*gateway.Description) != "" {
		description = strings.TrimSpace(*gateway.Description)
	}

	configHash := missing
	if gateway.ConfigHash != nil {
		configHash = valueOrMissing(*gateway.ConfigHash)
	}

	createdAt := missing
	if !gateway.CreatedAt.IsZero() {
		createdAt = gateway.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	updatedAt := missing
	if !gateway.UpdatedAt.IsZero() {
		updatedAt = gateway.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", valueOrMissing(gateway.ID))
	fmt.Fprintf(&b, "display_name: %s\n", valueOrMissing(gateway.DisplayName))
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "proxy_urls: %s\n", formatProxyURLs(gateway.ProxyUrls))
	fmt.Fprintf(
		&b, "endpoints: configuration=%s, telemetry=%s\n",
		valueOrMissing(gateway.Endpoints.Configuration),
		valueOrMissing(gateway.Endpoints.Telemetry),
	)
	fmt.Fprintf(&b, "config_hash: %s\n", configHash)
	fmt.Fprintf(&b, "labels: %s\n", formatLabelPairs(gateway.Labels, missing))
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

func formatLabelPairs(labels map[string]string, missing string) string {
	switch {
	case labels == nil:
		return missing
	case len(labels) == 0:
		return "[]"
	default:
		keys := slices.Sorted(maps.Keys(labels))
		pairs := make([]string, 0, len(labels))
		for _, key := range keys {
			pairs = append(pairs, fmt.Sprintf("%s=%s", key, labels[key]))
		}
		return strings.Join(pairs, ", ")
	}
}

func formatProxyURLs(proxyURLs []kkComps.AIGatewayProxyURL) string {
	if len(proxyURLs) == 0 {
		return "[]"
	}

	values := make([]string, 0, len(proxyURLs))
	for _, proxyURL := range proxyURLs {
		values = append(values, fmt.Sprintf("%s://%s:%d", proxyURL.Protocol, proxyURL.Host, proxyURL.Port))
	}
	return strings.Join(values, ", ")
}

func newGetAIGatewayCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getAIGatewayCmd {
	rv := getAIGatewayCmd{
		Command: baseCmd,
	}

	rv.Short = getAIGatewaysShort
	rv.Long = getAIGatewaysLong
	rv.Example = getAIGatewaysExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return &rv
}

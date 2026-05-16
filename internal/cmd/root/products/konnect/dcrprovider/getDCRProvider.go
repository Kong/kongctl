package dcrprovider

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
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
	getDCRProvidersShort = i18n.T("root.products.konnect.dcrprovider.getDCRProvidersShort",
		"List or get Konnect DCR providers")
	getDCRProvidersLong = i18n.T("root.products.konnect.dcrprovider.getDCRProvidersLong",
		`Use the get verb with the dcr-provider command to query Konnect Dynamic Client Registration providers.`)
	getDCRProvidersExample = normalizers.Examples(
		i18n.T("root.products.konnect.dcrprovider.getDCRProviderExamples",
			fmt.Sprintf(`
	# List all the DCR providers for the organization
	%[1]s get dcr-providers
	# Get details for a DCR provider with a specific ID
	%[1]s get dcr-provider 22cd8a0b-72e7-4212-9099-0764f8e9c5ac
	# Get details for a DCR provider with a specific name
	%[1]s get dcr-provider my-okta-dcr-provider
	# Get all the DCR providers using command aliases
	%[1]s get dcrps
	`, meta.CLIName)))
)

type textDisplayRecord struct {
	ID               string
	Name             string
	DisplayName      string
	ProviderType     string
	Issuer           string
	Active           string
	LocalCreatedTime string
	LocalUpdatedTime string
}

type dcrProvider struct {
	ID           string            `json:"id" yaml:"id"`
	Name         string            `json:"name" yaml:"name"`
	DisplayName  string            `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	ProviderType string            `json:"provider_type" yaml:"provider_type"`
	Issuer       string            `json:"issuer" yaml:"issuer"`
	DCRConfig    map[string]any    `json:"dcr_config,omitempty" yaml:"dcr_config,omitempty"`
	Labels       map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Active       *bool             `json:"active,omitempty" yaml:"active,omitempty"`
	CreatedAt    *time.Time        `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt    *time.Time        `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

type getDCRProviderCmd struct {
	*cobra.Command
}

func normalizeDCRProvider(data any) (dcrProvider, error) {
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		return dcrProvider{}, fmt.Errorf("failed to marshal DCR provider payload: %w", err)
	}

	var provider dcrProvider
	if err := json.Unmarshal(payloadBytes, &provider); err != nil {
		return dcrProvider{}, fmt.Errorf("failed to unmarshal DCR provider payload: %w", err)
	}

	return provider, nil
}

func summarizeLabels(labels map[string]string, missing string) string {
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

func summarizeDCRConfig(config map[string]any, missing string) string {
	switch {
	case config == nil:
		return missing
	case len(config) == 0:
		return "{}"
	default:
		return strings.Join(slices.Sorted(maps.Keys(config)), ", ")
	}
}

func dcrProviderToDisplayRecord(provider dcrProvider) textDisplayRecord {
	missing := "n/a"
	record := textDisplayRecord{
		ID:               missing,
		Name:             missing,
		DisplayName:      missing,
		ProviderType:     missing,
		Issuer:           missing,
		Active:           missing,
		LocalCreatedTime: missing,
		LocalUpdatedTime: missing,
	}

	if strings.TrimSpace(provider.ID) != "" {
		record.ID = util.AbbreviateUUID(provider.ID)
	}
	if strings.TrimSpace(provider.Name) != "" {
		record.Name = provider.Name
	}
	if strings.TrimSpace(provider.DisplayName) != "" {
		record.DisplayName = provider.DisplayName
	}
	if strings.TrimSpace(provider.ProviderType) != "" {
		record.ProviderType = provider.ProviderType
	}
	if strings.TrimSpace(provider.Issuer) != "" {
		record.Issuer = provider.Issuer
	}
	if provider.Active != nil {
		record.Active = fmt.Sprintf("%t", *provider.Active)
	}
	if provider.CreatedAt != nil && !provider.CreatedAt.IsZero() {
		record.LocalCreatedTime = provider.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	if provider.UpdatedAt != nil && !provider.UpdatedAt.IsZero() {
		record.LocalUpdatedTime = provider.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

func dcrProviderDetailView(provider dcrProvider) string {
	const missing = "n/a"

	valueOrMissing := func(val string) string {
		val = strings.TrimSpace(val)
		if val == "" {
			return missing
		}
		return val
	}

	active := missing
	if provider.Active != nil {
		active = fmt.Sprintf("%t", *provider.Active)
	}

	created := missing
	if provider.CreatedAt != nil && !provider.CreatedAt.IsZero() {
		created = provider.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	updated := missing
	if provider.UpdatedAt != nil && !provider.UpdatedAt.IsZero() {
		updated = provider.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", valueOrMissing(provider.ID))
	fmt.Fprintf(&b, "name: %s\n", valueOrMissing(provider.Name))
	fmt.Fprintf(&b, "display_name: %s\n", valueOrMissing(provider.DisplayName))
	fmt.Fprintf(&b, "provider_type: %s\n", valueOrMissing(provider.ProviderType))
	fmt.Fprintf(&b, "issuer: %s\n", valueOrMissing(provider.Issuer))
	fmt.Fprintf(&b, "active: %s\n", active)
	fmt.Fprintf(&b, "dcr_config: %s\n", summarizeDCRConfig(provider.DCRConfig, missing))
	fmt.Fprintf(&b, "labels: %s\n", summarizeLabels(provider.Labels, missing))
	fmt.Fprintf(&b, "created_at: %s\n", created)
	fmt.Fprintf(&b, "updated_at: %s\n", updated)

	return strings.TrimRight(b.String(), "\n")
}

func runList(
	kkClient helpers.DCRProvidersAPI,
	helper cmd.Helper,
	cfg config.Hook,
) ([]dcrProvider, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))

	var allData []dcrProvider
	for {
		req := kkOps.ListDcrProvidersRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		res, err := kkClient.ListDcrProviderPayloads(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list DCR providers", err, helper.GetCmd(), attrs...)
		}
		if res == nil {
			return allData, nil
		}

		for _, providerPayload := range res.Data {
			provider, err := normalizeDCRProvider(providerPayload)
			if err != nil {
				return nil, err
			}
			allData = append(allData, provider)
		}

		totalItems := res.Total
		if len(allData) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runListByIdentifier(
	identifier string,
	kkClient helpers.DCRProvidersAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (*dcrProvider, error) {
	identifier = strings.TrimSpace(identifier)
	providers, err := runList(kkClient, helper, cfg)
	if err != nil {
		return nil, err
	}

	for i := range providers {
		if providers[i].ID == identifier || providers[i].Name == identifier {
			return &providers[i], nil
		}
	}

	return nil, fmt.Errorf("DCR provider with name or ID %q not found", identifier)
}

func (c *getDCRProviderCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing DCR providers requires 0 or 1 arguments (name or ID)"),
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

func (c *getDCRProviderCmd) runE(cobraCmd *cobra.Command, args []string) error {
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

	if len(helper.GetArgs()) == 1 {
		provider, err := runListByIdentifier(helper.GetArgs()[0], sdk.GetDCRProvidersAPI(), helper, cfg)
		if err != nil {
			return err
		}

		return tableview.RenderForFormat(helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			dcrProviderToDisplayRecord(*provider),
			provider,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
		)
	}

	providers, err := runList(sdk.GetDCRProvidersAPI(), helper, cfg)
	if err != nil {
		return err
	}

	return renderDCRProviderList(helper, helper.GetCmd().Name(), outType, printer, providers)
}

func renderDCRProviderList(
	helper cmd.Helper,
	rootLabel string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	providers []dcrProvider,
) error {
	displayRecords := make([]textDisplayRecord, 0, len(providers))
	for i := range providers {
		displayRecords = append(displayRecords, dcrProviderToDisplayRecord(providers[i]))
	}

	childView := buildDCRProviderChildView(providers)
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
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		providers,
		"",
		options...,
	)
}

func buildDCRProviderChildView(providers []dcrProvider) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(providers))
	for i := range providers {
		record := dcrProviderToDisplayRecord(providers[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(providers) {
			return ""
		}
		return dcrProviderDetailView(providers[index])
	}

	return tableview.ChildView{
		Headers:        []string{"id", "name"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "DCR Providers",
		ParentType:     common.ViewParentDCRProvider,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(providers) {
				return nil
			}
			return providers[index]
		},
	}
}

func newGetDCRProviderCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getDCRProviderCmd {
	rv := getDCRProviderCmd{
		Command: baseCmd,
	}

	rv.Short = getDCRProvidersShort
	rv.Long = getDCRProvidersLong
	rv.Example = getDCRProvidersExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return &rv
}

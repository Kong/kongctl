package aigateway

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
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
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"

	"github.com/kong/kongctl/internal/cmd"
)

const aiGatewayProvidersUse = "model-providers [model-provider-id|model-provider-name]"

var (
	aiGatewayProvidersShort = i18n.T(
		"root.products.konnect.ai-gateway.providersShort",
		"List or get model providers for a Konnect AI Gateway",
	)
	aiGatewayProvidersLong = i18n.T(
		"root.products.konnect.ai-gateway.providersLong",
		`Use the model-providers command to list or retrieve AI Gateway Model Providers for a specific AI Gateway.`,
	)
	aiGatewayProvidersExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.providersExamples",
			fmt.Sprintf(`# List model providers for an AI Gateway by ID
%[1]s get ai-gateways model-providers --gateway-id <gateway-id>
# List model providers for an AI Gateway by display name
%[1]s get ai-gateways model-providers --gateway-name "Customer Support Gateway"
# Get a model provider by ID or name
%[1]s get ai-gateways model-providers --gateway-name "Customer Support Gateway" openai-provider
# Get a model provider by flag
%[1]s get ai-gateways model-providers --gateway-id <gateway-id> --model-provider-name openai-provider
`, meta.CLIName)),
	)
)

type aiGatewayProviderDisplayRecord struct {
	ID               string
	Name             string
	Type             string
	DisplayName      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func newGetAIGatewayProvidersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayProvidersUse,
		Short:   aiGatewayProvidersShort,
		Long:    aiGatewayProvidersLong,
		Example: aiGatewayProvidersExample,
		Aliases: []string{"model-provider"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayProviderFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayProvidersHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayProviderFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayProvidersHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayProvidersHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway Model Providers requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		providerID, providerName := getAIGatewayProviderIdentifiers(cfg)
		if providerID != "" || providerName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayProviderIDFlagName,
					aiGatewayProviderNameFlagName,
				),
			}
		}
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

	gatewayID, gatewayName := getAIGatewayIdentifiers(cfg)
	if gatewayID != "" && gatewayName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", aiGatewayIDFlagName, aiGatewayNameFlagName),
		}
	}
	if gatewayID == "" && gatewayName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"an AI Gateway identifier is required. Provide --%s or --%s",
				aiGatewayIDFlagName,
				aiGatewayNameFlagName,
			),
		}
	}
	if gatewayID == "" {
		gatewayID, err = resolveAIGatewayIDByName(gatewayName, sdk.GetAIGatewayAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	providerAPI := sdk.GetAIGatewayProvidersAPI()
	if providerAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Model Providers client is not available",
			Err: fmt.Errorf("AI Gateway Model Providers client not configured"),
		}
	}

	providerID, providerName := getAIGatewayProviderIdentifiers(cfg)
	if providerID != "" && providerName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayProviderIDFlagName,
				aiGatewayProviderNameFlagName,
			),
		}
	}

	identifier := ""
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if providerID != "" {
		identifier = providerID
	} else if providerName != "" {
		identifier = providerName
	}

	if identifier != "" {
		return h.getSingleProvider(helper, providerAPI, gatewayID, identifier, outType, printer, cfg)
	}
	return h.listProviders(helper, providerAPI, gatewayID, outType, printer, cfg)
}

func (h aiGatewayProvidersHandler) listProviders(
	helper cmd.Helper,
	providerAPI helpers.AIGatewayProvidersAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	providers, err := fetchAIGatewayProviders(helper, providerAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayProviderDisplayRecord, 0, len(providers))
	for _, provider := range providers {
		records = append(records, aiGatewayProviderToDisplayRecord(provider))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Type, record.DisplayName})
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		providers,
		"",
		tableview.WithCustomTable(
			[]string{aiGatewayHeaderID, aiGatewayHeaderName, aiGatewayHeaderType, aiGatewayHeaderDisplayName},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext(common.ViewParentAIGatewayProvider, func(index int) any {
			if index < 0 || index >= len(providers) {
				return nil
			}
			return &providers[index]
		}),
	)
}

func (h aiGatewayProvidersHandler) getSingleProvider(
	helper cmd.Helper,
	providerAPI helpers.AIGatewayProvidersAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	providerIdentifier := identifier
	if !util.IsValidUUID(identifier) {
		providers, err := fetchAIGatewayProviders(helper, providerAPI, gatewayID, cfg)
		if err != nil {
			return err
		}
		match := findAIGatewayProviderByNameOrID(providers, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Model Provider %q not found", identifier),
			}
		}
		providerIdentifier = aiGatewayProviderStringField(*match, aiGatewayFieldID)
		if providerIdentifier == "" {
			providerIdentifier = aiGatewayProviderStringField(*match, aiGatewayFieldName)
		}
		if providerIdentifier == "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Model Provider %q does not have an ID or name", identifier),
			}
		}
	}

	res, err := providerAPI.GetAiGatewayProvider(helper.GetContext(), gatewayID, providerIdentifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Model Provider", err, helper.GetCmd(), attrs...)
	}
	provider := res.GetAIGatewayModelProvider()
	if provider == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Model Provider response was empty",
			Err: fmt.Errorf("no model provider returned for id or name %s", providerIdentifier),
		}
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		aiGatewayProviderToDisplayRecord(*provider),
		provider,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext(common.ViewParentAIGatewayProvider, func(index int) any {
			if index != 0 {
				return nil
			}
			return provider
		}),
	)
}

func fetchAIGatewayProviders(
	helper cmd.Helper,
	providerAPI helpers.AIGatewayProvidersAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayModelProvider, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayModelProvider

	for {
		req := kkOps.ListAiGatewayModelProvidersRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := providerAPI.ListAiGatewayProviders(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateway Model Providers", err, helper.GetCmd(), attrs...)
		}
		if res.GetListAIGatewayModelProvidersResponse() == nil {
			break
		}

		data := res.GetListAIGatewayModelProvidersResponse().Data
		allData = append(allData, data...)

		nextCursor := pagination.ExtractPageAfterCursor(res.GetListAIGatewayModelProvidersResponse().Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func resolveAIGatewayIDByName(
	identifier string,
	gatewayAPI helpers.AIGatewayAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (string, error) {
	identifier = strings.TrimSpace(identifier)
	gateway, err := runListByNameOrDisplayName(identifier, gatewayAPI, helper, cfg)
	if err != nil {
		return "", err
	}
	if gateway == nil || strings.TrimSpace(gateway.ID) == "" {
		return "", fmt.Errorf("AI Gateway with name or display_name %q does not have an ID", identifier)
	}
	return gateway.ID, nil
}

func buildAIGatewayProviderChildView(providers []kkComps.AIGatewayModelProvider) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(providers))
	for i := range providers {
		record := aiGatewayProviderToDisplayRecord(providers[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Type, record.DisplayName})
	}

	return tableview.ChildView{
		Headers: []string{aiGatewayHeaderID, aiGatewayHeaderName, aiGatewayHeaderType, aiGatewayHeaderDisplayName},
		Rows:    tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(providers) {
				return ""
			}
			return aiGatewayProviderDetailView(&providers[index])
		},
		Title:      "AI Gateway Model Providers",
		ParentType: common.ViewParentAIGatewayProvider,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(providers) {
				return nil
			}
			return &providers[index]
		},
	}
}

func aiGatewayProviderToDisplayRecord(provider kkComps.AIGatewayModelProvider) aiGatewayProviderDisplayRecord {
	raw := aiGatewayProviderRawMap(provider)

	id := aiGatewayProviderStringFieldFromRaw(raw, aiGatewayFieldID)
	if id != "" {
		id = util.AbbreviateUUID(id)
	} else {
		id = aiGatewayMissingValue
	}

	name := aiGatewayProviderStringFieldFromRaw(raw, aiGatewayFieldName)
	if name == "" {
		name = aiGatewayMissingValue
	}
	providerType := aiGatewayProviderStringFieldFromRaw(raw, aiGatewayFieldType)
	if providerType == "" {
		providerType = aiGatewayMissingValue
	}
	displayName := aiGatewayProviderStringFieldFromRaw(raw, aiGatewayFieldDisplayName)
	if displayName == "" {
		displayName = aiGatewayMissingValue
	}

	return aiGatewayProviderDisplayRecord{
		ID:               id,
		Name:             name,
		Type:             providerType,
		DisplayName:      displayName,
		LocalCreatedTime: aiGatewayProviderTimeField(raw, aiGatewayFieldCreatedAt),
		LocalUpdatedTime: aiGatewayProviderTimeField(raw, aiGatewayFieldUpdatedAt),
	}
}

func aiGatewayProviderDetailView(provider *kkComps.AIGatewayModelProvider) string {
	if provider == nil {
		return ""
	}
	raw := aiGatewayProviderRawMap(*provider)

	var b strings.Builder
	writeProviderField := func(key string) {
		value, ok := raw[key]
		if !ok || value == nil {
			fmt.Fprintf(&b, "%s: %s\n", key, aiGatewayMissingValue)
			return
		}
		switch key {
		case aiGatewayFieldLabels, aiGatewayFieldManagedBy, aiGatewayFieldConfig:
			fmt.Fprintf(&b, "%s: %s\n", key, formatAIGatewayProviderJSONValue(value))
		case aiGatewayFieldCreatedAt, aiGatewayFieldUpdatedAt:
			fmt.Fprintf(&b, "%s: %s\n", key, aiGatewayProviderTimeField(raw, key))
		default:
			fmt.Fprintf(&b, "%s: %s\n", key, aiGatewayProviderStringFieldFromRaw(raw, key))
		}
	}

	for _, key := range []string{
		aiGatewayFieldID,
		aiGatewayFieldName,
		aiGatewayFieldType,
		aiGatewayFieldDisplayName,
		aiGatewayFieldLabels,
		aiGatewayFieldManagedBy,
		aiGatewayFieldConfig,
		aiGatewayFieldCreatedAt, aiGatewayFieldUpdatedAt,
	} {
		writeProviderField(key)
	}
	return strings.TrimRight(b.String(), "\n")
}

func findAIGatewayProviderByNameOrID(
	providers []kkComps.AIGatewayModelProvider,
	identifier string,
) *kkComps.AIGatewayModelProvider {
	lowered := strings.ToLower(strings.TrimSpace(identifier))
	for i := range providers {
		raw := aiGatewayProviderRawMap(providers[i])
		id := strings.ToLower(aiGatewayProviderStringFieldFromRaw(raw, aiGatewayFieldID))
		name := strings.ToLower(aiGatewayProviderStringFieldFromRaw(raw, aiGatewayFieldName))
		if id == lowered || name == lowered {
			return &providers[i]
		}
	}
	return nil
}

func aiGatewayProviderStringField(provider kkComps.AIGatewayModelProvider, key string) string {
	return aiGatewayProviderStringFieldFromRaw(aiGatewayProviderRawMap(provider), key)
}

func aiGatewayProviderRawMap(provider kkComps.AIGatewayModelProvider) map[string]any {
	data, err := json.Marshal(provider)
	if err != nil {
		return map[string]any{}
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string]any{}
	}
	return raw
}

func aiGatewayProviderStringFieldFromRaw(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func aiGatewayProviderTimeField(raw map[string]any, key string) string {
	value := aiGatewayProviderStringFieldFromRaw(raw, key)
	if value == "" {
		return aiGatewayMissingValue
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return parsed.In(time.Local).Format("2006-01-02 15:04:05")
}

func formatAIGatewayProviderJSONValue(value any) string {
	if value == nil {
		return aiGatewayMissingValue
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprint(value)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" || trimmed == "[]" {
		return aiGatewayMissingValue
	}
	return trimmed
}

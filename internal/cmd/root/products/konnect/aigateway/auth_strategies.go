package aigateway

import (
	"encoding/json"
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
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const aiGatewayAuthStrategiesUse = "auth-strategies [auth-strategy-id|auth-strategy-name]"

var (
	aiGatewayAuthStrategiesShort = i18n.T(
		"root.products.konnect.ai-gateway.authStrategiesShort",
		"List or get auth strategies for a Konnect AI Gateway",
	)
	aiGatewayAuthStrategiesLong = i18n.T(
		"root.products.konnect.ai-gateway.authStrategiesLong",
		`Use the auth-strategies command to list or retrieve AI Gateway Auth Strategies for a specific AI Gateway.`,
	)
	aiGatewayAuthStrategiesExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.authStrategiesExamples",
			fmt.Sprintf(`# List auth strategies for an AI Gateway by ID
%[1]s get ai-gateways auth-strategies --gateway-id <gateway-id>
# List auth strategies for an AI Gateway by display name
%[1]s get ai-gateways auth-strategies --gateway-name "Customer Support Gateway"
# Get an auth strategy by ID or name
%[1]s get ai-gateways auth-strategies --gateway-name "Customer Support Gateway" support-key-auth
# Get an auth strategy by flag
%[1]s get ai-gateways auth-strategies --gateway-id <gateway-id> --auth-strategy-name support-key-auth
`, meta.CLIName)),
	)
)

type aiGatewayAuthStrategyRecord struct {
	ID               string
	Name             string
	Type             string
	DisplayName      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func newGetAIGatewayAuthStrategiesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayAuthStrategiesUse,
		Short:   aiGatewayAuthStrategiesShort,
		Long:    aiGatewayAuthStrategiesLong,
		Example: aiGatewayAuthStrategiesExample,
		Aliases: []string{"auth-strategy"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayAuthStrategyFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayAuthStrategiesHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayAuthStrategyFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayAuthStrategiesHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayAuthStrategiesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway Auth Strategies requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		providerID, providerName := getAIGatewayAuthStrategyIdentifiers(cfg)
		if providerID != "" || providerName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayAuthStrategyIDFlagName,
					aiGatewayAuthStrategyNameFlagName,
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

	providerAPI := sdk.GetAIGatewayAuthStrategiesAPI()
	if providerAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Auth Strategies client is not available",
			Err: fmt.Errorf("AI Gateway Auth Strategies client not configured"),
		}
	}

	providerID, providerName := getAIGatewayAuthStrategyIdentifiers(cfg)
	if providerID != "" && providerName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayAuthStrategyIDFlagName,
				aiGatewayAuthStrategyNameFlagName,
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

func (h aiGatewayAuthStrategiesHandler) listProviders(
	helper cmd.Helper,
	providerAPI helpers.AIGatewayAuthStrategiesAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	providers, err := fetchAIGatewayAuthStrategies(helper, providerAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayAuthStrategyRecord, 0, len(providers))
	rawProviders := make([]map[string]any, 0, len(providers))
	for _, provider := range providers {
		records = append(records, aiGatewayAuthStrategyToDisplayRecord(provider))
		rawProviders = append(rawProviders, aiGatewayAuthStrategyRedactedRawMap(provider))
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
		rawProviders,
		"",
		tableview.WithCustomTable(
			[]string{aiGatewayHeaderID, aiGatewayHeaderName, aiGatewayHeaderType, aiGatewayHeaderDisplayName},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext(common.ViewParentAIGatewayAuthStrategy, func(index int) any {
			if index < 0 || index >= len(providers) {
				return nil
			}
			return &providers[index]
		}),
	)
}

func (h aiGatewayAuthStrategiesHandler) getSingleProvider(
	helper cmd.Helper,
	providerAPI helpers.AIGatewayAuthStrategiesAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	providerIdentifier := identifier
	if !util.IsValidUUID(identifier) {
		providers, err := fetchAIGatewayAuthStrategies(helper, providerAPI, gatewayID, cfg)
		if err != nil {
			return err
		}
		match := findAIGatewayAuthStrategyByNameOrID(providers, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Auth Strategy %q not found", identifier),
			}
		}
		providerIdentifier = aiGatewayAuthStrategyStringField(*match, aiGatewayFieldID)
		if providerIdentifier == "" {
			providerIdentifier = aiGatewayAuthStrategyStringField(*match, aiGatewayFieldName)
		}
		if providerIdentifier == "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Auth Strategy %q does not have an ID or name", identifier),
			}
		}
	}

	res, err := providerAPI.GetAiGatewayAuthStrategy(helper.GetContext(), gatewayID, providerIdentifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Auth Strategy", err, helper.GetCmd(), attrs...)
	}
	provider := res.GetAIGatewayAuthStrategy()
	if provider == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Auth Strategy response was empty",
			Err: fmt.Errorf("no auth strategy returned for id or name %s", providerIdentifier),
		}
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		aiGatewayAuthStrategyToDisplayRecord(*provider),
		aiGatewayAuthStrategyRedactedRawMap(*provider),
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext(common.ViewParentAIGatewayAuthStrategy, func(index int) any {
			if index != 0 {
				return nil
			}
			return provider
		}),
	)
}

func fetchAIGatewayAuthStrategies(
	helper cmd.Helper,
	providerAPI helpers.AIGatewayAuthStrategiesAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayAuthStrategy, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayAuthStrategy

	for {
		req := kkOps.ListAiGatewayAuthStrategiesRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := providerAPI.ListAiGatewayAuthStrategies(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateway Auth Strategies", err, helper.GetCmd(), attrs...)
		}
		if res.GetListAIGatewayAuthStrategiesResponse() == nil {
			break
		}

		data := res.GetListAIGatewayAuthStrategiesResponse().Data
		allData = append(allData, data...)

		nextCursor := pagination.ExtractPageAfterCursor(res.GetListAIGatewayAuthStrategiesResponse().Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func buildAIGatewayAuthStrategyChildView(providers []kkComps.AIGatewayAuthStrategy) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(providers))
	for i := range providers {
		record := aiGatewayAuthStrategyToDisplayRecord(providers[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Type, record.DisplayName})
	}

	return tableview.ChildView{
		Headers: []string{aiGatewayHeaderID, aiGatewayHeaderName, aiGatewayHeaderType, aiGatewayHeaderDisplayName},
		Rows:    tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(providers) {
				return ""
			}
			return aiGatewayAuthStrategyDetailView(&providers[index])
		},
		Title:      "AI Gateway Auth Strategies",
		ParentType: common.ViewParentAIGatewayAuthStrategy,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(providers) {
				return nil
			}
			return &providers[index]
		},
	}
}

func aiGatewayAuthStrategyToDisplayRecord(
	provider kkComps.AIGatewayAuthStrategy,
) aiGatewayAuthStrategyRecord {
	raw := aiGatewayAuthStrategyRawMap(provider)

	id := aiGatewayAuthStrategyStringFieldFromRaw(raw, aiGatewayFieldID)
	if id != "" {
		id = strings.TrimSpace(id)
	} else {
		id = aiGatewayMissingValue
	}

	name := aiGatewayAuthStrategyStringFieldFromRaw(raw, aiGatewayFieldName)
	if name == "" {
		name = aiGatewayMissingValue
	}
	providerType := aiGatewayAuthStrategyStringFieldFromRaw(raw, aiGatewayFieldType)
	if providerType == "" {
		providerType = aiGatewayMissingValue
	}
	displayName := aiGatewayAuthStrategyStringFieldFromRaw(raw, aiGatewayFieldDisplayName)
	if displayName == "" {
		displayName = aiGatewayMissingValue
	}

	return aiGatewayAuthStrategyRecord{
		ID:               id,
		Name:             name,
		Type:             providerType,
		DisplayName:      displayName,
		LocalCreatedTime: aiGatewayAuthStrategyTimeField(raw, aiGatewayFieldCreatedAt),
		LocalUpdatedTime: aiGatewayAuthStrategyTimeField(raw, aiGatewayFieldUpdatedAt),
	}
}

func aiGatewayAuthStrategyDetailView(provider *kkComps.AIGatewayAuthStrategy) string {
	if provider == nil {
		return ""
	}
	raw := aiGatewayAuthStrategyRawMap(*provider)
	raw = redactAIGatewayAuthStrategySecrets(raw)

	var b strings.Builder
	writeProviderField := func(key string) {
		value, ok := raw[key]
		if !ok || value == nil {
			fmt.Fprintf(&b, "%s: %s\n", key, aiGatewayMissingValue)
			return
		}
		switch key {
		case aiGatewayFieldLabels, aiGatewayFieldManagedBy, aiGatewayFieldConfig:
			fmt.Fprintf(&b, "%s: %s\n", key, formatAIGatewayAuthStrategyJSONValue(value))
		case aiGatewayFieldCreatedAt, aiGatewayFieldUpdatedAt:
			fmt.Fprintf(&b, "%s: %s\n", key, aiGatewayAuthStrategyTimeField(raw, key))
		default:
			fmt.Fprintf(&b, "%s: %s\n", key, aiGatewayAuthStrategyStringFieldFromRaw(raw, key))
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

func findAIGatewayAuthStrategyByNameOrID(
	providers []kkComps.AIGatewayAuthStrategy,
	identifier string,
) *kkComps.AIGatewayAuthStrategy {
	lowered := strings.ToLower(strings.TrimSpace(identifier))
	for i := range providers {
		raw := aiGatewayAuthStrategyRawMap(providers[i])
		id := strings.ToLower(aiGatewayAuthStrategyStringFieldFromRaw(raw, aiGatewayFieldID))
		name := strings.ToLower(aiGatewayAuthStrategyStringFieldFromRaw(raw, aiGatewayFieldName))
		if id == lowered || name == lowered {
			return &providers[i]
		}
	}
	return nil
}

func aiGatewayAuthStrategyStringField(provider kkComps.AIGatewayAuthStrategy, key string) string {
	return aiGatewayAuthStrategyStringFieldFromRaw(aiGatewayAuthStrategyRawMap(provider), key)
}

func aiGatewayAuthStrategyRawMap(provider kkComps.AIGatewayAuthStrategy) map[string]any {
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

func aiGatewayAuthStrategyRedactedRawMap(provider kkComps.AIGatewayAuthStrategy) map[string]any {
	return redactAIGatewayAuthStrategySecrets(aiGatewayAuthStrategyRawMap(provider))
}

func redactAIGatewayAuthStrategySecrets(value any) map[string]any {
	raw, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}

	result := make(map[string]any, len(raw))
	for key, val := range raw {
		if strings.EqualFold(key, "client_secret") {
			result[key] = "[redacted]"
			continue
		}
		result[key] = redactAIGatewayAuthStrategyValue(val)
	}
	return result
}

func redactAIGatewayAuthStrategyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			if strings.EqualFold(key, "client_secret") {
				result[key] = "[redacted]"
				continue
			}
			result[key] = redactAIGatewayAuthStrategyValue(val)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i := range typed {
			result[i] = redactAIGatewayAuthStrategyValue(typed[i])
		}
		return result
	default:
		return value
	}
}

func aiGatewayAuthStrategyStringFieldFromRaw(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func aiGatewayAuthStrategyTimeField(raw map[string]any, key string) string {
	value := aiGatewayAuthStrategyStringFieldFromRaw(raw, key)
	if value == "" {
		return aiGatewayMissingValue
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return parsed.In(time.Local).Format("2006-01-02 15:04:05")
}

func formatAIGatewayAuthStrategyJSONValue(value any) string {
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

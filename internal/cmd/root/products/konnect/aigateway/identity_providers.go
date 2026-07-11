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

const aiGatewayIdentityProvidersUse = "identity-providers [identity-provider-id|identity-provider-name]"

var (
	aiGatewayIdentityProvidersShort = i18n.T(
		"root.products.konnect.ai-gateway.identityProvidersShort",
		"List or get identity providers for a Konnect AI Gateway",
	)
	aiGatewayIdentityProvidersLong = i18n.T(
		"root.products.konnect.ai-gateway.identityProvidersLong",
		`Use the identity-providers command to list or retrieve AI Gateway Identity Providers for a specific AI Gateway.`,
	)
	aiGatewayIdentityProvidersExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.identityProvidersExamples",
			fmt.Sprintf(`# List identity providers for an AI Gateway by ID
%[1]s get ai-gateways identity-providers --gateway-id <gateway-id>
# List identity providers for an AI Gateway by display name
%[1]s get ai-gateways identity-providers --gateway-name "Customer Support Gateway"
# Get an identity provider by ID or name
%[1]s get ai-gateways identity-providers --gateway-name "Customer Support Gateway" support-key-auth
# Get an identity provider by flag
%[1]s get ai-gateways identity-providers --gateway-id <gateway-id> --identity-provider-name support-key-auth
`, meta.CLIName)),
	)
)

type aiGatewayIdentityProviderRecord struct {
	ID               string
	Name             string
	Type             string
	DisplayName      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func newGetAIGatewayIdentityProvidersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayIdentityProvidersUse,
		Short:   aiGatewayIdentityProvidersShort,
		Long:    aiGatewayIdentityProvidersLong,
		Example: aiGatewayIdentityProvidersExample,
		Aliases: []string{"identity-provider", "identity"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayIdentityProviderFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayIdentityProvidersHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayIdentityProviderFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayIdentityProvidersHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayIdentityProvidersHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway Identity Providers requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		providerID, providerName := getAIGatewayIdentityProviderIdentifiers(cfg)
		if providerID != "" || providerName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayIdentityProviderIDFlagName,
					aiGatewayIdentityProviderNameFlagName,
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

	providerAPI := sdk.GetAIGatewayIdentityProvidersAPI()
	if providerAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Identity Providers client is not available",
			Err: fmt.Errorf("AI Gateway Identity Providers client not configured"),
		}
	}

	providerID, providerName := getAIGatewayIdentityProviderIdentifiers(cfg)
	if providerID != "" && providerName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayIdentityProviderIDFlagName,
				aiGatewayIdentityProviderNameFlagName,
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

func (h aiGatewayIdentityProvidersHandler) listProviders(
	helper cmd.Helper,
	providerAPI helpers.AIGatewayIdentityProvidersAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	providers, err := fetchAIGatewayIdentityProviders(helper, providerAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayIdentityProviderRecord, 0, len(providers))
	rawProviders := make([]map[string]any, 0, len(providers))
	for _, provider := range providers {
		records = append(records, aiGatewayIdentityProviderToDisplayRecord(provider))
		rawProviders = append(rawProviders, aiGatewayIdentityProviderRedactedRawMap(provider))
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
		tableview.WithDetailContext(common.ViewParentAIGatewayIdentityProvider, func(index int) any {
			if index < 0 || index >= len(providers) {
				return nil
			}
			return &providers[index]
		}),
	)
}

func (h aiGatewayIdentityProvidersHandler) getSingleProvider(
	helper cmd.Helper,
	providerAPI helpers.AIGatewayIdentityProvidersAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	providerIdentifier := identifier
	if !util.IsValidUUID(identifier) {
		providers, err := fetchAIGatewayIdentityProviders(helper, providerAPI, gatewayID, cfg)
		if err != nil {
			return err
		}
		match := findAIGatewayIdentityProviderByNameOrID(providers, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Identity Provider %q not found", identifier),
			}
		}
		providerIdentifier = aiGatewayIdentityProviderStringField(*match, aiGatewayFieldID)
		if providerIdentifier == "" {
			providerIdentifier = aiGatewayIdentityProviderStringField(*match, aiGatewayFieldName)
		}
		if providerIdentifier == "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Identity Provider %q does not have an ID or name", identifier),
			}
		}
	}

	res, err := providerAPI.GetAiGatewayIdentityProvider(helper.GetContext(), gatewayID, providerIdentifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Identity Provider", err, helper.GetCmd(), attrs...)
	}
	provider := res.GetAIGatewayIdentityProvider()
	if provider == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Identity Provider response was empty",
			Err: fmt.Errorf("no identity provider returned for id or name %s", providerIdentifier),
		}
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		aiGatewayIdentityProviderToDisplayRecord(*provider),
		aiGatewayIdentityProviderRedactedRawMap(*provider),
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext(common.ViewParentAIGatewayIdentityProvider, func(index int) any {
			if index != 0 {
				return nil
			}
			return provider
		}),
	)
}

func fetchAIGatewayIdentityProviders(
	helper cmd.Helper,
	providerAPI helpers.AIGatewayIdentityProvidersAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayIdentityProvider, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayIdentityProvider

	for {
		req := kkOps.ListAiGatewayIdentityProvidersRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := providerAPI.ListAiGatewayIdentityProviders(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateway Identity Providers", err, helper.GetCmd(), attrs...)
		}
		if res.GetListAIGatewayIdentityProvidersResponse() == nil {
			break
		}

		data := res.GetListAIGatewayIdentityProvidersResponse().Data
		allData = append(allData, data...)

		nextCursor := pagination.ExtractPageAfterCursor(res.GetListAIGatewayIdentityProvidersResponse().Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func buildAIGatewayIdentityProviderChildView(providers []kkComps.AIGatewayIdentityProvider) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(providers))
	for i := range providers {
		record := aiGatewayIdentityProviderToDisplayRecord(providers[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Type, record.DisplayName})
	}

	return tableview.ChildView{
		Headers: []string{aiGatewayHeaderID, aiGatewayHeaderName, aiGatewayHeaderType, aiGatewayHeaderDisplayName},
		Rows:    tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(providers) {
				return ""
			}
			return aiGatewayIdentityProviderDetailView(&providers[index])
		},
		Title:      "AI Gateway Identity Providers",
		ParentType: common.ViewParentAIGatewayIdentityProvider,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(providers) {
				return nil
			}
			return &providers[index]
		},
	}
}

func aiGatewayIdentityProviderToDisplayRecord(
	provider kkComps.AIGatewayIdentityProvider,
) aiGatewayIdentityProviderRecord {
	raw := aiGatewayIdentityProviderRawMap(provider)

	id := aiGatewayIdentityProviderStringFieldFromRaw(raw, aiGatewayFieldID)
	if id != "" {
		id = util.AbbreviateUUID(id)
	} else {
		id = aiGatewayMissingValue
	}

	name := aiGatewayIdentityProviderStringFieldFromRaw(raw, aiGatewayFieldName)
	if name == "" {
		name = aiGatewayMissingValue
	}
	providerType := aiGatewayIdentityProviderStringFieldFromRaw(raw, aiGatewayFieldType)
	if providerType == "" {
		providerType = aiGatewayMissingValue
	}
	displayName := aiGatewayIdentityProviderStringFieldFromRaw(raw, aiGatewayFieldDisplayName)
	if displayName == "" {
		displayName = aiGatewayMissingValue
	}

	return aiGatewayIdentityProviderRecord{
		ID:               id,
		Name:             name,
		Type:             providerType,
		DisplayName:      displayName,
		LocalCreatedTime: aiGatewayIdentityProviderTimeField(raw, aiGatewayFieldCreatedAt),
		LocalUpdatedTime: aiGatewayIdentityProviderTimeField(raw, aiGatewayFieldUpdatedAt),
	}
}

func aiGatewayIdentityProviderDetailView(provider *kkComps.AIGatewayIdentityProvider) string {
	if provider == nil {
		return ""
	}
	raw := aiGatewayIdentityProviderRawMap(*provider)
	raw = redactAIGatewayIdentityProviderSecrets(raw)

	var b strings.Builder
	writeProviderField := func(key string) {
		value, ok := raw[key]
		if !ok || value == nil {
			fmt.Fprintf(&b, "%s: %s\n", key, aiGatewayMissingValue)
			return
		}
		switch key {
		case aiGatewayFieldLabels, aiGatewayFieldManagedBy, aiGatewayFieldConfig:
			fmt.Fprintf(&b, "%s: %s\n", key, formatAIGatewayIdentityProviderJSONValue(value))
		case aiGatewayFieldCreatedAt, aiGatewayFieldUpdatedAt:
			fmt.Fprintf(&b, "%s: %s\n", key, aiGatewayIdentityProviderTimeField(raw, key))
		default:
			fmt.Fprintf(&b, "%s: %s\n", key, aiGatewayIdentityProviderStringFieldFromRaw(raw, key))
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

func findAIGatewayIdentityProviderByNameOrID(
	providers []kkComps.AIGatewayIdentityProvider,
	identifier string,
) *kkComps.AIGatewayIdentityProvider {
	lowered := strings.ToLower(strings.TrimSpace(identifier))
	for i := range providers {
		raw := aiGatewayIdentityProviderRawMap(providers[i])
		id := strings.ToLower(aiGatewayIdentityProviderStringFieldFromRaw(raw, aiGatewayFieldID))
		name := strings.ToLower(aiGatewayIdentityProviderStringFieldFromRaw(raw, aiGatewayFieldName))
		if id == lowered || name == lowered {
			return &providers[i]
		}
	}
	return nil
}

func aiGatewayIdentityProviderStringField(provider kkComps.AIGatewayIdentityProvider, key string) string {
	return aiGatewayIdentityProviderStringFieldFromRaw(aiGatewayIdentityProviderRawMap(provider), key)
}

func aiGatewayIdentityProviderRawMap(provider kkComps.AIGatewayIdentityProvider) map[string]any {
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

func aiGatewayIdentityProviderRedactedRawMap(provider kkComps.AIGatewayIdentityProvider) map[string]any {
	return redactAIGatewayIdentityProviderSecrets(aiGatewayIdentityProviderRawMap(provider))
}

func redactAIGatewayIdentityProviderSecrets(value any) map[string]any {
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
		result[key] = redactAIGatewayIdentityProviderValue(val)
	}
	return result
}

func redactAIGatewayIdentityProviderValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			if strings.EqualFold(key, "client_secret") {
				result[key] = "[redacted]"
				continue
			}
			result[key] = redactAIGatewayIdentityProviderValue(val)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i := range typed {
			result[i] = redactAIGatewayIdentityProviderValue(typed[i])
		}
		return result
	default:
		return value
	}
}

func aiGatewayIdentityProviderStringFieldFromRaw(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func aiGatewayIdentityProviderTimeField(raw map[string]any, key string) string {
	value := aiGatewayIdentityProviderStringFieldFromRaw(raw, key)
	if value == "" {
		return aiGatewayMissingValue
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return parsed.In(time.Local).Format("2006-01-02 15:04:05")
}

func formatAIGatewayIdentityProviderJSONValue(value any) string {
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

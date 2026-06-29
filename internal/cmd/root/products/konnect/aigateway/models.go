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
	declresources "github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	aiGatewayModelGatewayIDFlag   = "gateway-id"
	aiGatewayModelGatewayNameFlag = "gateway-name"
	aiGatewayModelIDFlag          = "model-id"
	aiGatewayModelNameFlag        = "model-name"
)

type aiGatewayModelRecord struct {
	ID               string
	Name             string
	DisplayName      string
	Type             string
	Enabled          string
	LocalUpdatedTime string
}

type aiGatewayModelsHandler struct {
	cmd *cobra.Command
}

var (
	aiGatewayModelsShort = i18n.T(
		"root.products.konnect.ai-gateway.modelsShort",
		"List or get models for a Konnect AI Gateway",
	)
	aiGatewayModelsLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.modelsLong",
		`Use the models command to list or retrieve models for a specific Konnect AI Gateway.`,
	))
	aiGatewayModelsExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.modelsExamples",
			fmt.Sprintf(`# List models for an AI Gateway by display name
%[1]s get ai-gateway models --gateway-name "Customer Support Gateway"
# List models for an AI Gateway by ID
%[1]s get ai-gateway models --gateway-id <gateway-id>
# Get a model by name
%[1]s get ai-gateway models --gateway-name "Customer Support Gateway" support-gpt
# Get a model by ID
%[1]s get ai-gateway models --gateway-id <gateway-id> --model-id <model-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayModelsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	modelsCmd := &cobra.Command{
		Use:     "models [model-id|model-name]",
		Short:   aiGatewayModelsShort,
		Long:    aiGatewayModelsLong,
		Example: aiGatewayModelsExample,
		Aliases: []string{"model"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := aiGatewayModelsHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	modelsCmd.Flags().String(aiGatewayModelGatewayIDFlag, "", "AI Gateway ID.")
	modelsCmd.Flags().String(aiGatewayModelGatewayNameFlag, "", "AI Gateway display name.")
	modelsCmd.Flags().String(aiGatewayModelIDFlag, "", "AI Gateway model ID.")
	modelsCmd.Flags().String(aiGatewayModelNameFlag, "", "AI Gateway model name.")

	if addParentFlags != nil {
		addParentFlags(verb, modelsCmd)
	}

	return modelsCmd
}

func (h aiGatewayModelsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if err := h.validate(args); err != nil {
		return err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
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

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	gatewayID, err := h.resolveGatewayID(helper, sdk.GetAIGatewayAPI())
	if err != nil {
		return err
	}

	modelAPI := sdk.GetAIGatewayModelAPI()
	if modelAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway model client is not available",
			Err: fmt.Errorf("AI Gateway model client not configured"),
		}
	}

	modelID, modelName := h.modelSelector(args)
	if modelID != "" || modelName != "" {
		return h.getModel(helper, modelAPI, gatewayID, modelID, modelName, outType, printer)
	}
	return h.listModels(helper, modelAPI, gatewayID, outType, printer)
}

func (h aiGatewayModelsHandler) validate(args []string) error {
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway models requires 0 or 1 arguments (model ID or name)"),
		}
	}

	gatewayID, _ := h.cmd.Flags().GetString(aiGatewayModelGatewayIDFlag)
	gatewayName, _ := h.cmd.Flags().GetString(aiGatewayModelGatewayNameFlag)
	if strings.TrimSpace(gatewayID) != "" && strings.TrimSpace(gatewayName) != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided",
				aiGatewayModelGatewayIDFlag, aiGatewayModelGatewayNameFlag),
		}
	}
	if strings.TrimSpace(gatewayID) == "" && strings.TrimSpace(gatewayName) == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("one of --%s or --%s is required",
				aiGatewayModelGatewayIDFlag, aiGatewayModelGatewayNameFlag),
		}
	}

	modelID, _ := h.cmd.Flags().GetString(aiGatewayModelIDFlag)
	modelName, _ := h.cmd.Flags().GetString(aiGatewayModelNameFlag)
	if strings.TrimSpace(modelID) != "" && strings.TrimSpace(modelName) != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided",
				aiGatewayModelIDFlag, aiGatewayModelNameFlag),
		}
	}
	if len(args) == 1 && (strings.TrimSpace(modelID) != "" || strings.TrimSpace(modelName) != "") {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("model selector can be provided either positionally or with --%s/--%s, not both",
				aiGatewayModelIDFlag, aiGatewayModelNameFlag),
		}
	}

	return nil
}

func (h aiGatewayModelsHandler) resolveGatewayID(helper cmd.Helper, gatewayAPI helpers.AIGatewayAPI) (string, error) {
	gatewayID, _ := h.cmd.Flags().GetString(aiGatewayModelGatewayIDFlag)
	gatewayName, _ := h.cmd.Flags().GetString(aiGatewayModelGatewayNameFlag)
	gatewayID = strings.TrimSpace(gatewayID)
	if gatewayID != "" {
		return gatewayID, nil
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return "", err
	}
	gateway, err := runListByDisplayName(strings.TrimSpace(gatewayName), gatewayAPI, helper, cfg)
	if err != nil {
		return "", err
	}
	return gateway.ID, nil
}

func (h aiGatewayModelsHandler) modelSelector(args []string) (string, string) {
	modelID, _ := h.cmd.Flags().GetString(aiGatewayModelIDFlag)
	modelName, _ := h.cmd.Flags().GetString(aiGatewayModelNameFlag)
	modelID = strings.TrimSpace(modelID)
	modelName = strings.TrimSpace(modelName)
	if len(args) == 1 {
		identifier := strings.TrimSpace(args[0])
		if util.IsValidUUID(identifier) {
			modelID = identifier
		} else {
			modelName = identifier
		}
	}
	return modelID, modelName
}

func (h aiGatewayModelsHandler) listModels(
	helper cmd.Helper,
	modelAPI helpers.AIGatewayModelAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	models, err := listAIGatewayModels(helper, modelAPI, gatewayID)
	if err != nil {
		return err
	}

	records := make([]aiGatewayModelRecord, 0, len(models))
	for i := range models {
		records = append(records, aiGatewayModelToRecord(models[i]))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.Enabled,
			record.LocalUpdatedTime,
		})
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		models,
		"",
		tableview.WithCustomTable(
			[]string{
				aiGatewayHeaderID,
				aiGatewayHeaderName,
				aiGatewayHeaderDisplayName,
				aiGatewayHeaderType,
				"ENABLED",
				aiGatewayHeaderUpdated,
			},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(models) {
				return ""
			}
			return aiGatewayModelDetailView(models[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayModel, func(index int) any {
			if index < 0 || index >= len(models) {
				return nil
			}
			return &models[index]
		}),
	)
}

func (h aiGatewayModelsHandler) getModel(
	helper cmd.Helper,
	modelAPI helpers.AIGatewayModelAPI,
	gatewayID string,
	modelID string,
	modelName string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	identifier := modelID
	if identifier == "" {
		identifier = modelName
	}

	res, err := modelAPI.GetAiGatewayModel(helper.GetContext(), gatewayID, identifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway model", err, helper.GetCmd(), attrs...)
	}
	model := res.GetAIGatewayModel()
	if model == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway model response was empty",
			Err: fmt.Errorf("no model returned for %s", identifier),
		}
	}

	record := aiGatewayModelToRecord(*model)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		model,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayModelDetailView(*model)
		}),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext(common.ViewParentAIGatewayModel, func(index int) any {
			if index != 0 {
				return nil
			}
			return model
		}),
	)
}

func listAIGatewayModels(
	helper cmd.Helper,
	modelAPI helpers.AIGatewayModelAPI,
	gatewayID string,
) ([]kkComps.AIGatewayModel, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return nil, err
	}
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.AIGatewayModel
	var pageAfter *string
	for {
		res, err := modelAPI.ListAiGatewayModels(helper.GetContext(), kkOps.ListAiGatewayModelsRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
			PageAfter: pageAfter,
		})
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateway models", err, helper.GetCmd(), attrs...)
		}
		if res == nil || res.ListAIGatewayModelsResponse == nil {
			return allData, nil
		}

		allData = append(allData, res.ListAIGatewayModelsResponse.Data...)
		nextCursor := pagination.ExtractPageAfterCursor(res.ListAIGatewayModelsResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayModelToRecord(model kkComps.AIGatewayModel) aiGatewayModelRecord {
	const missing = "n/a"
	record := aiGatewayModelRecord{
		ID:               missing,
		Name:             valueOrMissing(declresources.AIGatewayModelName(model)),
		DisplayName:      valueOrMissing(declresources.AIGatewayModelDisplayName(model)),
		Type:             valueOrMissing(declresources.AIGatewayModelType(model)),
		Enabled:          missing,
		LocalUpdatedTime: missing,
	}
	if id := declresources.AIGatewayModelID(model); id != "" {
		record.ID = util.AbbreviateUUID(id)
	}
	if enabled := declresources.AIGatewayModelEnabled(model); enabled != nil {
		record.Enabled = fmt.Sprintf("%t", *enabled)
	}
	if updatedAt := declresources.AIGatewayModelUpdatedAt(model); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayModelDetailView(model kkComps.AIGatewayModel) string {
	payload := make(map[string]any)
	data, err := json.Marshal(model)
	if err == nil {
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		"id",
		"name",
		"display_name",
		"type",
		"enabled",
		"capabilities",
		"labels",
		aiGatewayFieldCreatedAt,
		aiGatewayFieldUpdatedAt,
	}

	var b strings.Builder
	for _, field := range order {
		fmt.Fprintf(&b, "%s: %s\n", field, formatAIGatewayModelDetailValue(payload[field]))
	}
	return strings.TrimRight(b.String(), "\n")
}

func valueOrMissing(value string) string {
	if strings.TrimSpace(value) == "" {
		return "n/a"
	}
	return value
}

func formatAIGatewayModelDetailValue(value any) string {
	if value == nil {
		return "n/a"
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return "n/a"
		}
		return typed
	case bool:
		return fmt.Sprintf("%t", typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(data)
	}
}

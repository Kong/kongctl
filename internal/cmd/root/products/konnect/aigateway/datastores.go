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

type aiGatewayDatastoreRecord struct {
	ID               string
	Name             string
	Type             string
	Description      string
	LocalUpdatedTime string
}

var (
	aiGatewayDatastoresUse   = "datastores [datastore-id|datastore-name]"
	aiGatewayDatastoresShort = i18n.T(
		"root.products.konnect.ai-gateway.datastoresShort",
		"List or get Datastores for a Konnect AI Gateway",
	)
	aiGatewayDatastoresLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.datastoresLong",
		`Use the datastores command to list or retrieve Datastores for a specific Konnect AI Gateway.`,
	))
	aiGatewayDatastoresExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.datastoresExamples",
			fmt.Sprintf(`# List Datastores for an AI Gateway by display name
%[1]s get ai-gateway datastores --gateway-name "Customer Support Gateway"
# List Datastores for an AI Gateway by ID
%[1]s get ai-gateway datastores --gateway-id <gateway-id>
# Get a Datastore by name
%[1]s get ai-gateway datastores --gateway-name "Customer Support Gateway" support-env
# Get a Datastore by ID
%[1]s get ai-gateway datastores --gateway-id <gateway-id> --datastore-id <datastore-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayDatastoresCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayDatastoresUse,
		Short:   aiGatewayDatastoresShort,
		Long:    aiGatewayDatastoresLong,
		Example: aiGatewayDatastoresExample,
		Aliases: []string{"datastore"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayDatastoreFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayDatastoresHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayDatastoreFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayDatastoresHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayDatastoresHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing AI Gateway Datastores requires 0 or 1 arguments (ID or name)",
			),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		datastoreID, datastoreName := getAIGatewayDatastoreIdentifiers(cfg)
		if datastoreID != "" || datastoreName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayDatastoreIDFlagName,
					aiGatewayDatastoreNameFlagName,
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
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayIDFlagName,
				aiGatewayNameFlagName,
			),
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

	datastoreAPI := sdk.GetAIGatewayDatastoresAPI()
	if datastoreAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Datastores client is not available",
			Err: fmt.Errorf("AI Gateway Datastores client not configured"),
		}
	}

	datastoreID, datastoreName := getAIGatewayDatastoreIdentifiers(cfg)
	if datastoreID != "" && datastoreName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayDatastoreIDFlagName,
				aiGatewayDatastoreNameFlagName,
			),
		}
	}

	identifier := ""
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if datastoreID != "" {
		identifier = datastoreID
	} else if datastoreName != "" {
		identifier = datastoreName
	}

	if identifier != "" {
		return h.getSingleDatastore(
			helper,
			datastoreAPI,
			gatewayID,
			identifier,
			outType,
			printer,
			cfg,
		)
	}
	return h.listDatastores(helper, datastoreAPI, gatewayID, outType, printer, cfg)
}

func (h aiGatewayDatastoresHandler) listDatastores(
	helper cmd.Helper,
	datastoreAPI helpers.AIGatewayDatastoresAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	datastores, err := fetchAIGatewayDatastores(helper, datastoreAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayDatastoreRecord, 0, len(datastores))
	tableRows := make([]table.Row, 0, len(datastores))
	for _, datastore := range datastores {
		record := aiGatewayDatastoreToRecord(datastore)
		records = append(records, record)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.Type,
			record.Description,
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
		datastores,
		"",
		tableview.WithCustomTable(
			[]string{
				aiGatewayHeaderID,
				aiGatewayHeaderName,
				aiGatewayHeaderType,
				"DESCRIPTION",
				aiGatewayHeaderUpdated,
			},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(datastores) {
				return ""
			}
			return aiGatewayDatastoreDetailView(datastores[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayDatastore, func(index int) any {
			if index < 0 || index >= len(datastores) {
				return nil
			}
			return &datastores[index]
		}),
	)
}

func (h aiGatewayDatastoresHandler) getSingleDatastore(
	helper cmd.Helper,
	datastoreAPI helpers.AIGatewayDatastoresAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	datastoreIdentifier := identifier
	if !util.IsValidUUID(identifier) {
		datastores, err := fetchAIGatewayDatastores(helper, datastoreAPI, gatewayID, cfg)
		if err != nil {
			return err
		}
		match := findAIGatewayDatastoreByNameOrID(datastores, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Datastore %q not found", identifier),
			}
		}
		datastoreIdentifier = declresources.AIGatewayDatastoreID(*match)
		if datastoreIdentifier == "" {
			datastoreIdentifier = declresources.AIGatewayDatastoreName(*match)
		}
		if datastoreIdentifier == "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Datastore %q does not have an ID or name", identifier),
			}
		}
	}

	res, err := datastoreAPI.GetAiGatewayDatastore(
		helper.GetContext(),
		gatewayID,
		datastoreIdentifier,
	)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError(
			"Failed to get AI Gateway Datastore",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}
	datastore := res.GetAIGatewayDatastore()
	if datastore == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Datastore response was empty",
			Err: fmt.Errorf("no Datastore returned for id or name %s", datastoreIdentifier),
		}
	}

	record := aiGatewayDatastoreToRecord(*datastore)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		datastore,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayDatastoreDetailView(*datastore)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayDatastore, func(index int) any {
			if index != 0 {
				return nil
			}
			return datastore
		}),
	)
}

func fetchAIGatewayDatastores(
	helper cmd.Helper,
	datastoreAPI helpers.AIGatewayDatastoresAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayDatastoreSummary, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayDatastoreSummary

	for {
		req := kkOps.ListAiGatewayDatastoresRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := datastoreAPI.ListAiGatewayDatastores(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError(
				"Failed to list AI Gateway Datastores",
				err,
				helper.GetCmd(),
				attrs...,
			)
		}
		if res.GetListAIGatewayDatastoresResponse() == nil {
			break
		}

		allData = append(allData, res.GetListAIGatewayDatastoresResponse().Data...)
		nextCursor := pagination.ExtractPageAfterCursor(
			res.GetListAIGatewayDatastoresResponse().Meta.Page.Next,
		)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayDatastoreToRecord(datastore any) aiGatewayDatastoreRecord {
	record := aiGatewayDatastoreRecord{
		ID:               aiGatewayMissingValue,
		Name:             valueOrMissing(declresources.AIGatewayDatastoreName(datastore)),
		Type:             valueOrMissing(declresources.AIGatewayDatastoreType(datastore)),
		Description:      valueOrMissing(declresources.AIGatewayDatastoreDescription(datastore)),
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if id := declresources.AIGatewayDatastoreID(datastore); id != "" {
		record.ID = id
	}
	if updatedAt := declresources.AIGatewayDatastoreUpdatedAt(datastore); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func findAIGatewayDatastoreByNameOrID(
	datastores []kkComps.AIGatewayDatastoreSummary,
	identifier string,
) *kkComps.AIGatewayDatastoreSummary {
	for i := range datastores {
		if declresources.AIGatewayDatastoreID(datastores[i]) == identifier ||
			declresources.AIGatewayDatastoreName(datastores[i]) == identifier {
			return &datastores[i]
		}
	}
	return nil
}

func aiGatewayDatastoreDetailView(datastore any) string {
	payload := make(map[string]any)
	data, err := json.Marshal(datastore)
	if err == nil {
		// Detail views are best-effort; leave missing fields as n/a if SDK union data cannot round-trip.
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		aiGatewayFieldID,
		aiGatewayFieldName,
		"description",
		aiGatewayFieldType,
		"config",
		aiGatewayFieldLabels,
		aiGatewayFieldManagedBy,
		aiGatewayFieldCreatedAt,
		aiGatewayFieldUpdatedAt,
	}

	var b strings.Builder
	for _, field := range order {
		fmt.Fprintf(&b, "%s: %s\n", field, formatAIGatewayModelDetailValue(payload[field]))
	}
	return strings.TrimRight(b.String(), "\n")
}

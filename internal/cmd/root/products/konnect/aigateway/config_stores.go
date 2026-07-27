package aigateway

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
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type aiGatewayConfigStoreRecord struct {
	ID               string
	Name             string
	DisplayName      string
	LocalUpdatedTime string
}

var (
	aiGatewayConfigStoresUse   = "config-stores [config-store-id|config-store-name]"
	aiGatewayConfigStoresShort = i18n.T(
		"root.products.konnect.ai-gateway.configStoresShort",
		"List or get Config Stores for a Konnect AI Gateway",
	)
	aiGatewayConfigStoresLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.configStoresLong",
		`Use the config-stores command to list or retrieve Config Stores for a Konnect AI Gateway.`,
	))
	aiGatewayConfigStoresExample = normalizers.Examples(fmt.Sprintf(`# List Config Stores by gateway name
%[1]s get ai-gateway config-stores --gateway-name "Customer Support Gateway"
# Get a Config Store by name
%[1]s get ai-gateway config-stores --gateway-id <gateway-id> support-store
`, meta.CLIName))
)

func newGetAIGatewayConfigStoresCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayConfigStoresUse,
		Short:   aiGatewayConfigStoresShort,
		Long:    aiGatewayConfigStoresLong,
		Example: aiGatewayConfigStoresExample,
		Aliases: []string{"config-store"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayConfigStoreFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			return (aiGatewayConfigStoresHandler{cmd: c}).run(args)
		},
	}
	addAIGatewayChildFlags(c)
	addAIGatewayConfigStoreFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayConfigStoresHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayConfigStoresHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{Err: fmt.Errorf(
			"too many arguments. Listing AI Gateway Config Stores requires 0 or 1 arguments (ID or name)",
		)}
	}
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	if len(args) == 1 {
		id, name := getAIGatewayConfigStoreIdentifiers(cfg)
		if id != "" || name != "" {
			return &cmd.ConfigurationError{Err: fmt.Errorf(
				"cannot specify both positional argument and --%s or --%s flags",
				aiGatewayConfigStoreIDFlagName,
				aiGatewayConfigStoreNameFlagName,
			)}
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
		return &cmd.ConfigurationError{Err: fmt.Errorf(
			"only one of --%s or --%s can be provided",
			aiGatewayIDFlagName,
			aiGatewayNameFlagName,
		)}
	}
	if gatewayID == "" && gatewayName == "" {
		return &cmd.ConfigurationError{Err: fmt.Errorf(
			"an AI Gateway identifier is required. Provide --%s or --%s",
			aiGatewayIDFlagName,
			aiGatewayNameFlagName,
		)}
	}
	if gatewayID == "" {
		gatewayID, err = resolveAIGatewayIDByName(gatewayName, sdk.GetAIGatewayAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}
	api := sdk.GetAIGatewayConfigStoresAPI()
	if api == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Config Stores client is not available",
			Err: fmt.Errorf("AI Gateway Config Stores client not configured"),
		}
	}
	storeID, storeName := getAIGatewayConfigStoreIdentifiers(cfg)
	if storeID != "" && storeName != "" {
		return &cmd.ConfigurationError{Err: fmt.Errorf(
			"only one of --%s or --%s can be provided",
			aiGatewayConfigStoreIDFlagName,
			aiGatewayConfigStoreNameFlagName,
		)}
	}
	identifier := ""
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if storeID != "" {
		identifier = storeID
	} else if storeName != "" {
		identifier = storeName
	}
	if identifier != "" {
		return h.getSingle(helper, api, gatewayID, identifier, outType, printer)
	}
	return h.list(helper, api, gatewayID, outType, printer, cfg)
}

func (h aiGatewayConfigStoresHandler) list(
	helper cmd.Helper,
	api helpers.AIGatewayConfigStoresAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	stores, err := fetchAIGatewayConfigStores(helper, api, gatewayID, cfg)
	if err != nil {
		return err
	}
	records, rows := aiGatewayConfigStoreRows(stores)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		stores,
		"",
		tableview.WithCustomTable(
			[]string{aiGatewayHeaderID, aiGatewayHeaderName, aiGatewayHeaderDisplayName, aiGatewayHeaderUpdated},
			rows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(stores) {
				return ""
			}
			return aiGatewayConfigStoreDetailView(stores[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayConfigStore, func(index int) any {
			if index < 0 || index >= len(stores) {
				return nil
			}
			return &stores[index]
		}),
	)
}

func (h aiGatewayConfigStoresHandler) getSingle(
	helper cmd.Helper,
	api helpers.AIGatewayConfigStoresAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	res, err := api.GetAiGatewayConfigStore(helper.GetContext(), gatewayID, identifier)
	if err != nil {
		return cmd.PrepareExecutionError(
			"Failed to get AI Gateway Config Store",
			err,
			helper.GetCmd(),
			cmd.TryConvertErrorToAttrs(err)...,
		)
	}
	store := res.GetAIGatewayConfigStore()
	if store == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Config Store response was empty",
			Err: fmt.Errorf("no Config Store returned for id or name %s", identifier),
		}
	}
	record := aiGatewayConfigStoreToRecord(*store)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		store,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index == 0 {
				return aiGatewayConfigStoreDetailView(*store)
			}
			return ""
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayConfigStore, func(index int) any {
			if index == 0 {
				return store
			}
			return nil
		}),
	)
}

func fetchAIGatewayConfigStores(
	helper cmd.Helper,
	api helpers.AIGatewayConfigStoresAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayConfigStore, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var stores []kkComps.AIGatewayConfigStore
	for {
		res, err := api.ListAiGatewayConfigStores(helper.GetContext(), kkOps.ListAiGatewayConfigStoresRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
			PageAfter: pageAfter,
		})
		if err != nil {
			return nil, cmd.PrepareExecutionError(
				"Failed to list AI Gateway Config Stores",
				err,
				helper.GetCmd(),
				cmd.TryConvertErrorToAttrs(err)...,
			)
		}
		body := res.GetListAIGatewayConfigStoresResponse()
		if body == nil {
			break
		}
		stores = append(stores, body.Data...)
		next := pagination.ExtractPageAfterCursor(body.Meta.Page.Next)
		if next == "" {
			break
		}
		pageAfter = &next
	}
	return stores, nil
}

func aiGatewayConfigStoreToRecord(store kkComps.AIGatewayConfigStore) aiGatewayConfigStoreRecord {
	record := aiGatewayConfigStoreRecord{
		ID:               util.AbbreviateUUID(store.ID),
		Name:             valueOrMissing(store.Name),
		DisplayName:      aiGatewayMissingValue,
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if store.DisplayName != nil {
		record.DisplayName = valueOrMissing(*store.DisplayName)
	}
	if !store.UpdatedAt.IsZero() {
		record.LocalUpdatedTime = store.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayConfigStoreRows(
	stores []kkComps.AIGatewayConfigStore,
) ([]aiGatewayConfigStoreRecord, []table.Row) {
	records := make([]aiGatewayConfigStoreRecord, 0, len(stores))
	rows := make([]table.Row, 0, len(stores))
	for _, store := range stores {
		record := aiGatewayConfigStoreToRecord(store)
		records = append(records, record)
		rows = append(rows, table.Row{record.ID, record.Name, record.DisplayName, record.LocalUpdatedTime})
	}
	return records, rows
}

func aiGatewayConfigStoreDetailView(store kkComps.AIGatewayConfigStore) string {
	displayName := aiGatewayMissingValue
	if store.DisplayName != nil {
		displayName = valueOrMissing(*store.DisplayName)
	}
	createdAt := aiGatewayMissingValue
	if !store.CreatedAt.IsZero() {
		createdAt = store.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	updatedAt := aiGatewayMissingValue
	if !store.UpdatedAt.IsZero() {
		updatedAt = store.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return fmt.Sprintf(
		"id: %s\nname: %s\ndisplay_name: %s\ncreated_at: %s\nupdated_at: %s",
		valueOrMissing(store.ID),
		valueOrMissing(store.Name),
		displayName,
		createdAt,
		updatedAt,
	)
}

func buildAIGatewayConfigStoreChildView(stores []kkComps.AIGatewayConfigStore) tableview.ChildView {
	_, rows := aiGatewayConfigStoreRows(stores)
	return tableview.ChildView{
		Headers: []string{aiGatewayHeaderID, aiGatewayHeaderName, aiGatewayHeaderDisplayName, aiGatewayHeaderUpdated},
		Rows:    rows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(stores) {
				return ""
			}
			return aiGatewayConfigStoreDetailView(stores[index])
		},
		Title:      "AI Gateway Config Stores",
		ParentType: common.ViewParentAIGatewayConfigStore,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(stores) {
				return nil
			}
			return &stores[index]
		},
	}
}

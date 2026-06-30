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

type aiGatewayVaultRecord struct {
	ID               string
	Name             string
	Type             string
	Description      string
	LocalUpdatedTime string
}

var (
	aiGatewayVaultsUse   = "vaults [vault-id|vault-name]"
	aiGatewayVaultsShort = i18n.T(
		"root.products.konnect.ai-gateway.vaultsShort",
		"List or get Vaults for a Konnect AI Gateway",
	)
	aiGatewayVaultsLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.vaultsLong",
		`Use the vaults command to list or retrieve Vaults for a specific Konnect AI Gateway.`,
	))
	aiGatewayVaultsExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.vaultsExamples",
			fmt.Sprintf(`# List Vaults for an AI Gateway by display name
%[1]s get ai-gateway vaults --gateway-name "Customer Support Gateway"
# List Vaults for an AI Gateway by ID
%[1]s get ai-gateway vaults --gateway-id <gateway-id>
# Get a Vault by name
%[1]s get ai-gateway vaults --gateway-name "Customer Support Gateway" support-env
# Get a Vault by ID
%[1]s get ai-gateway vaults --gateway-id <gateway-id> --vault-id <vault-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayVaultsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayVaultsUse,
		Short:   aiGatewayVaultsShort,
		Long:    aiGatewayVaultsLong,
		Example: aiGatewayVaultsExample,
		Aliases: []string{"vault"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayVaultFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayVaultsHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayVaultFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayVaultsHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayVaultsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway Vaults requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		vaultID, vaultName := getAIGatewayVaultIdentifiers(cfg)
		if vaultID != "" || vaultName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayVaultIDFlagName,
					aiGatewayVaultNameFlagName,
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
		gatewayID, err = resolveAIGatewayIDByDisplayName(gatewayName, sdk.GetAIGatewayAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	vaultAPI := sdk.GetAIGatewayVaultsAPI()
	if vaultAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Vaults client is not available",
			Err: fmt.Errorf("AI Gateway Vaults client not configured"),
		}
	}

	vaultID, vaultName := getAIGatewayVaultIdentifiers(cfg)
	if vaultID != "" && vaultName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayVaultIDFlagName,
				aiGatewayVaultNameFlagName,
			),
		}
	}

	identifier := ""
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if vaultID != "" {
		identifier = vaultID
	} else if vaultName != "" {
		identifier = vaultName
	}

	if identifier != "" {
		return h.getSingleVault(helper, vaultAPI, gatewayID, identifier, outType, printer, cfg)
	}
	return h.listVaults(helper, vaultAPI, gatewayID, outType, printer, cfg)
}

func (h aiGatewayVaultsHandler) listVaults(
	helper cmd.Helper,
	vaultAPI helpers.AIGatewayVaultsAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	vaults, err := fetchAIGatewayVaults(helper, vaultAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayVaultRecord, 0, len(vaults))
	tableRows := make([]table.Row, 0, len(vaults))
	for _, vault := range vaults {
		record := aiGatewayVaultToRecord(vault)
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
		vaults,
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
			if index < 0 || index >= len(vaults) {
				return ""
			}
			return aiGatewayVaultDetailView(vaults[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayVault, func(index int) any {
			if index < 0 || index >= len(vaults) {
				return nil
			}
			return &vaults[index]
		}),
	)
}

func (h aiGatewayVaultsHandler) getSingleVault(
	helper cmd.Helper,
	vaultAPI helpers.AIGatewayVaultsAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	vaultIdentifier := identifier
	if !util.IsValidUUID(identifier) {
		vaults, err := fetchAIGatewayVaults(helper, vaultAPI, gatewayID, cfg)
		if err != nil {
			return err
		}
		match := findAIGatewayVaultByNameOrID(vaults, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Vault %q not found", identifier),
			}
		}
		vaultIdentifier = declresources.AIGatewayVaultID(*match)
		if vaultIdentifier == "" {
			vaultIdentifier = declresources.AIGatewayVaultName(*match)
		}
		if vaultIdentifier == "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Vault %q does not have an ID or name", identifier),
			}
		}
	}

	res, err := vaultAPI.GetAiGatewayVault(helper.GetContext(), gatewayID, vaultIdentifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Vault", err, helper.GetCmd(), attrs...)
	}
	vault := res.GetAIGatewayVault()
	if vault == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Vault response was empty",
			Err: fmt.Errorf("no Vault returned for id or name %s", vaultIdentifier),
		}
	}

	record := aiGatewayVaultToRecord(*vault)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		vault,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayVaultDetailView(*vault)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayVault, func(index int) any {
			if index != 0 {
				return nil
			}
			return vault
		}),
	)
}

func fetchAIGatewayVaults(
	helper cmd.Helper,
	vaultAPI helpers.AIGatewayVaultsAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayVault, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayVault

	for {
		req := kkOps.ListAiGatewayVaultsRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := vaultAPI.ListAiGatewayVaults(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateway Vaults", err, helper.GetCmd(), attrs...)
		}
		if res.GetListAIGatewayVaultsResponse() == nil {
			break
		}

		allData = append(allData, res.GetListAIGatewayVaultsResponse().Data...)
		nextCursor := pagination.ExtractPageAfterCursor(res.GetListAIGatewayVaultsResponse().Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayVaultToRecord(vault kkComps.AIGatewayVault) aiGatewayVaultRecord {
	record := aiGatewayVaultRecord{
		ID:               aiGatewayMissingValue,
		Name:             valueOrMissing(declresources.AIGatewayVaultName(vault)),
		Type:             valueOrMissing(declresources.AIGatewayVaultType(vault)),
		Description:      valueOrMissing(declresources.AIGatewayVaultDescription(vault)),
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if id := declresources.AIGatewayVaultID(vault); id != "" {
		record.ID = util.AbbreviateUUID(id)
	}
	if updatedAt := declresources.AIGatewayVaultUpdatedAt(vault); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func findAIGatewayVaultByNameOrID(vaults []kkComps.AIGatewayVault, identifier string) *kkComps.AIGatewayVault {
	for i := range vaults {
		if declresources.AIGatewayVaultID(vaults[i]) == identifier ||
			declresources.AIGatewayVaultName(vaults[i]) == identifier {
			return &vaults[i]
		}
	}
	return nil
}

func aiGatewayVaultDetailView(vault kkComps.AIGatewayVault) string {
	payload := make(map[string]any)
	data, err := json.Marshal(vault)
	if err == nil {
		// Detail views are best-effort; leave missing fields as n/a if SDK union data cannot round-trip.
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		"id",
		"name",
		"description",
		"type",
		"config",
		"labels",
		"managed_by",
		aiGatewayFieldCreatedAt,
		aiGatewayFieldUpdatedAt,
	}

	var b strings.Builder
	for _, field := range order {
		fmt.Fprintf(&b, "%s: %s\n", field, formatAIGatewayModelDetailValue(payload[field]))
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildAIGatewayVaultChildView(vaults []kkComps.AIGatewayVault) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(vaults))
	for i := range vaults {
		record := aiGatewayVaultToRecord(vaults[i])
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.Type,
			record.Description,
			record.LocalUpdatedTime,
		})
	}

	return tableview.ChildView{
		Headers: []string{
			aiGatewayHeaderID,
			aiGatewayHeaderName,
			aiGatewayHeaderType,
			"DESCRIPTION",
			aiGatewayHeaderUpdated,
		},
		Rows: tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(vaults) {
				return ""
			}
			return aiGatewayVaultDetailView(vaults[index])
		},
		Title:      "AI Gateway Vaults",
		ParentType: common.ViewParentAIGatewayVault,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(vaults) {
				return nil
			}
			return &vaults[index]
		},
	}
}

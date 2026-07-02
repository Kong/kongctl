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

type aiGatewayPolicyRecord struct {
	ID               string
	Name             string
	DisplayName      string
	Type             string
	Enabled          string
	Global           string
	LocalUpdatedTime string
}

var (
	aiGatewayPoliciesUse   = "policies [policy-id|policy-name]"
	aiGatewayPoliciesShort = i18n.T(
		"root.products.konnect.ai-gateway.policiesShort",
		"List or get Policies for a Konnect AI Gateway",
	)
	aiGatewayPoliciesLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.policiesLong",
		`Use the policies command to list or retrieve Policies for a specific Konnect AI Gateway.`,
	))
	aiGatewayPoliciesExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.policiesExamples",
			fmt.Sprintf(`# List Policies for an AI Gateway by display name
%[1]s get ai-gateway policies --gateway-name "Customer Support Gateway"
# List Policies for an AI Gateway by ID
%[1]s get ai-gateway policies --gateway-id <gateway-id>
# Get a Policy by name
%[1]s get ai-gateway policies --gateway-name "Customer Support Gateway" mask-sensitive-data
# Get a Policy by ID
%[1]s get ai-gateway policies --gateway-id <gateway-id> --policy-id <policy-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayPoliciesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayPoliciesUse,
		Short:   aiGatewayPoliciesShort,
		Long:    aiGatewayPoliciesLong,
		Example: aiGatewayPoliciesExample,
		Aliases: []string{"policy"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayPolicyFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayPoliciesHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayPolicyFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayPoliciesHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayPoliciesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway Policies requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		policyID, policyName := getAIGatewayPolicyIdentifiers(cfg)
		if policyID != "" || policyName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayPolicyIDFlagName,
					aiGatewayPolicyNameFlagName,
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

	policyAPI := sdk.GetAIGatewayPoliciesAPI()
	if policyAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Policies client is not available",
			Err: fmt.Errorf("AI Gateway Policies client not configured"),
		}
	}

	policyID, policyName := getAIGatewayPolicyIdentifiers(cfg)
	if policyID != "" && policyName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayPolicyIDFlagName,
				aiGatewayPolicyNameFlagName,
			),
		}
	}

	identifier := ""
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if policyID != "" {
		identifier = policyID
	} else if policyName != "" {
		identifier = policyName
	}

	if identifier != "" {
		return h.getSinglePolicy(helper, policyAPI, gatewayID, identifier, outType, printer)
	}
	return h.listPolicies(helper, policyAPI, gatewayID, outType, printer, cfg)
}

func (h aiGatewayPoliciesHandler) listPolicies(
	helper cmd.Helper,
	policyAPI helpers.AIGatewayPoliciesAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	policies, err := fetchAIGatewayPolicies(helper, policyAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayPolicyRecord, 0, len(policies))
	tableRows := make([]table.Row, 0, len(policies))
	for _, policy := range policies {
		record := aiGatewayPolicyToRecord(policy)
		records = append(records, record)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.Enabled,
			record.Global,
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
		policies,
		"",
		tableview.WithCustomTable(
			[]string{
				aiGatewayHeaderID,
				aiGatewayHeaderName,
				aiGatewayHeaderDisplayName,
				aiGatewayHeaderType,
				"ENABLED",
				"GLOBAL",
				aiGatewayHeaderUpdated,
			},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(policies) {
				return ""
			}
			return aiGatewayPolicyDetailView(policies[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayPolicy, func(index int) any {
			if index < 0 || index >= len(policies) {
				return nil
			}
			return &policies[index]
		}),
	)
}

func (h aiGatewayPoliciesHandler) getSinglePolicy(
	helper cmd.Helper,
	policyAPI helpers.AIGatewayPoliciesAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	res, err := policyAPI.GetAiGatewayPolicy(helper.GetContext(), gatewayID, identifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Policy", err, helper.GetCmd(), attrs...)
	}
	policy := res.GetAIGatewayPolicy()
	if policy == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Policy response was empty",
			Err: fmt.Errorf("no Policy returned for id or name %s", identifier),
		}
	}

	record := aiGatewayPolicyToRecord(*policy)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		policy,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayPolicyDetailView(*policy)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayPolicy, func(index int) any {
			if index != 0 {
				return nil
			}
			return policy
		}),
	)
}

func fetchAIGatewayPolicies(
	helper cmd.Helper,
	policyAPI helpers.AIGatewayPoliciesAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayPolicy, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayPolicy

	for {
		req := kkOps.ListAiGatewayPoliciesRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := policyAPI.ListAiGatewayPolicies(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list AI Gateway Policies", err, helper.GetCmd(), attrs...)
		}
		if res.GetListAIGatewayPoliciesResponse() == nil {
			break
		}

		allData = append(allData, res.GetListAIGatewayPoliciesResponse().Data...)
		nextCursor := pagination.ExtractPageAfterCursor(res.GetListAIGatewayPoliciesResponse().Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayPolicyToRecord(policy kkComps.AIGatewayPolicy) aiGatewayPolicyRecord {
	record := aiGatewayPolicyRecord{
		ID:               aiGatewayMissingValue,
		Name:             valueOrMissing(declresources.AIGatewayPolicyName(policy)),
		DisplayName:      valueOrMissing(declresources.AIGatewayPolicyDisplayName(policy)),
		Type:             valueOrMissing(declresources.AIGatewayPolicyType(policy)),
		Enabled:          aiGatewayMissingValue,
		Global:           aiGatewayMissingValue,
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if id := declresources.AIGatewayPolicyID(policy); id != "" {
		record.ID = util.AbbreviateUUID(id)
	}
	if enabled := declresources.AIGatewayPolicyEnabled(policy); enabled != nil {
		record.Enabled = fmt.Sprintf("%t", *enabled)
	}
	if global := declresources.AIGatewayPolicyGlobal(policy); global != nil {
		record.Global = fmt.Sprintf("%t", *global)
	}
	if updatedAt := declresources.AIGatewayPolicyUpdatedAt(policy); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayPolicyDetailView(policy kkComps.AIGatewayPolicy) string {
	payload := make(map[string]any)
	data, err := json.Marshal(policy)
	if err == nil {
		// Detail views are best-effort; leave missing fields as n/a if SDK union data cannot round-trip.
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		"id",
		"name",
		"display_name",
		"type",
		"enabled",
		"global",
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

func buildAIGatewayPolicyChildView(policies []kkComps.AIGatewayPolicy) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(policies))
	for i := range policies {
		record := aiGatewayPolicyToRecord(policies[i])
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.Type,
			record.DisplayName,
			record.Enabled,
			record.Global,
		})
	}

	return tableview.ChildView{
		Headers: []string{
			aiGatewayHeaderID,
			aiGatewayHeaderName,
			aiGatewayHeaderType,
			aiGatewayHeaderDisplayName,
			"ENABLED",
			"GLOBAL",
		},
		Rows: tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(policies) {
				return ""
			}
			return aiGatewayPolicyDetailView(policies[index])
		},
		Title:      "AI Gateway Policies",
		ParentType: common.ViewParentAIGatewayPolicy,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(policies) {
				return nil
			}
			return &policies[index]
		},
	}
}

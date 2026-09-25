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
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type aiGatewayCustomPolicyRecord struct {
	ID               string
	Name             string
	DisplayName      string
	Type             string
	LocalUpdatedTime string
}

var (
	aiGatewayCustomPoliciesUse   = "custom-policies [custom-policy-id|custom-policy-name]"
	aiGatewayCustomPoliciesShort = i18n.T(
		"root.products.konnect.ai-gateway.customPoliciesShort",
		"List or get Custom Policies for a Konnect AI Gateway",
	)
	aiGatewayCustomPoliciesLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.customPoliciesLong",
		`Use the custom-policies command to list or retrieve Custom Policies for a specific Konnect AI Gateway.`,
	))
	aiGatewayCustomPoliciesExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.customPoliciesExamples",
			fmt.Sprintf(`# List Custom Policies for an AI Gateway by display name
%[1]s get ai-gateway custom-policies --gateway-name "Customer Support Gateway"
# List Custom Policies for an AI Gateway by ID
%[1]s get ai-gateway custom-policies --gateway-id <gateway-id>
# Get a Custom Policy by name
%[1]s get ai-gateway custom-policies --gateway-name "Customer Support Gateway" mask-sensitive-data
# Get a Custom Policy by ID
%[1]s get ai-gateway custom-policies --gateway-id <gateway-id> --custom-policy-id <policy-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayCustomPoliciesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayCustomPoliciesUse,
		Short:   aiGatewayCustomPoliciesShort,
		Long:    aiGatewayCustomPoliciesLong,
		Example: aiGatewayCustomPoliciesExample,
		Aliases: []string{"custom-policy"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayCustomPolicyFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayCustomPoliciesHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayCustomPolicyFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayCustomPoliciesHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayCustomPoliciesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing AI Gateway Custom Policies requires 0 or 1 arguments (ID or name)",
			),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		policyID, policyName := getAIGatewayCustomPolicyIdentifiers(cfg)
		if policyID != "" || policyName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayCustomPolicyIDFlagName,
					aiGatewayCustomPolicyNameFlagName,
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

	policyAPI := sdk.GetAIGatewayCustomPoliciesAPI()
	if policyAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Custom Policies client is not available",
			Err: fmt.Errorf("AI Gateway Custom Policies client not configured"),
		}
	}

	policyID, policyName := getAIGatewayCustomPolicyIdentifiers(cfg)
	if policyID != "" && policyName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayCustomPolicyIDFlagName,
				aiGatewayCustomPolicyNameFlagName,
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

func (h aiGatewayCustomPoliciesHandler) listPolicies(
	helper cmd.Helper,
	policyAPI helpers.AIGatewayCustomPoliciesAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	policies, err := fetchAIGatewayCustomPolicies(helper, policyAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayCustomPolicyRecord, 0, len(policies))
	tableRows := make([]table.Row, 0, len(policies))
	for _, policy := range policies {
		record := aiGatewayCustomPolicyToRecord(policy)
		records = append(records, record)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
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
			return aiGatewayCustomPolicyDetailView(policies[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayCustomPolicy, func(index int) any {
			if index < 0 || index >= len(policies) {
				return nil
			}
			return &policies[index]
		}),
	)
}

func (h aiGatewayCustomPoliciesHandler) getSinglePolicy(
	helper cmd.Helper,
	policyAPI helpers.AIGatewayCustomPoliciesAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	res, err := policyAPI.GetAiGatewayCustomPolicy(helper.GetContext(), gatewayID, identifier)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Custom Policy", err, helper.GetCmd(), attrs...)
	}
	policy := res.GetAIGatewayCustomPolicy()
	if policy == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Custom Policy response was empty",
			Err: fmt.Errorf("no Policy returned for id or name %s", identifier),
		}
	}

	record := aiGatewayCustomPolicyToRecord(*policy)
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
			return aiGatewayCustomPolicyDetailView(*policy)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayCustomPolicy, func(index int) any {
			if index != 0 {
				return nil
			}
			return policy
		}),
	)
}

func fetchAIGatewayCustomPolicies(
	helper cmd.Helper,
	policyAPI helpers.AIGatewayCustomPoliciesAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayCustomPolicy, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayCustomPolicy

	for {
		req := kkOps.ListAiGatewayCustomPoliciesRequest{
			GatewayID: gatewayID,
			PageSize:  &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := policyAPI.ListAiGatewayCustomPolicies(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError(
				"Failed to list AI Gateway Custom Policies",
				err,
				helper.GetCmd(),
				attrs...,
			)
		}
		if res.GetListAIGatewayCustomPoliciesResponse() == nil {
			break
		}

		allData = append(allData, res.GetListAIGatewayCustomPoliciesResponse().Data...)
		nextCursor := pagination.ExtractPageAfterCursor(res.GetListAIGatewayCustomPoliciesResponse().Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func aiGatewayCustomPolicyToRecord(policy kkComps.AIGatewayCustomPolicy) aiGatewayCustomPolicyRecord {
	record := aiGatewayCustomPolicyRecord{
		ID:               aiGatewayMissingValue,
		Name:             valueOrMissing(declresources.AIGatewayCustomPolicyName(policy)),
		DisplayName:      valueOrMissing(declresources.AIGatewayPolicyDisplayName(policy)),
		Type:             valueOrMissing(declresources.AIGatewayPolicyType(policy)),
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if id := declresources.AIGatewayCustomPolicyID(policy); id != "" {
		record.ID = id
	}
	if updatedAt := declresources.AIGatewayPolicyUpdatedAt(policy); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayCustomPolicyDetailView(policy kkComps.AIGatewayCustomPolicy) string {
	payload := make(map[string]any)
	data, err := json.Marshal(policy)
	if err == nil {
		// Detail views are best-effort; leave missing fields as n/a if SDK union data cannot round-trip.
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		aiGatewayFieldID,
		aiGatewayFieldName,
		aiGatewayFieldDisplayName,
		aiGatewayFieldType,
		"schema",
		"handler",
		aiGatewayFieldCreatedAt,
		aiGatewayFieldUpdatedAt,
	}

	var b strings.Builder
	for _, field := range order {
		fmt.Fprintf(&b, "%s: %s\n", field, formatAIGatewayModelDetailValue(payload[field]))
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildAIGatewayCustomPolicyChildView(policies []kkComps.AIGatewayCustomPolicy) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(policies))
	for i := range policies {
		record := aiGatewayCustomPolicyToRecord(policies[i])
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.LocalUpdatedTime,
		})
	}

	return tableview.ChildView{
		Headers: []string{
			aiGatewayHeaderID,
			aiGatewayHeaderName,
			aiGatewayHeaderDisplayName,
			aiGatewayHeaderType,
			aiGatewayHeaderUpdated,
		},
		Rows: tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(policies) {
				return ""
			}
			return aiGatewayCustomPolicyDetailView(policies[index])
		},
		Title:      "AI Gateway Custom Policies",
		ParentType: common.ViewParentAIGatewayCustomPolicy,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(policies) {
				return nil
			}
			return &policies[index]
		},
	}
}

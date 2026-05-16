package eventgateway

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
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	consumePoliciesCommandName = "consume-policies"

	consumePolicyIDFlagName   = "consume-policy-id"
	consumePolicyNameFlagName = "consume-policy-name"

	consumePolicyIDConfigPath   = "konnect.event-gateway.consume-policy.id"
	consumePolicyNameConfigPath = "konnect.event-gateway.consume-policy.name"
)

type consumePolicySummaryRecord struct {
	ID               string
	Name             string
	Type             string
	Description      string
	Enabled          string
	LocalCreatedTime string
	LocalUpdatedTime string
}

// consumePolicyWithConfig is a wrapper that includes the full config from raw API response.
// The SDK's EventGatewayPolicyConfig struct is empty, so we use map[string]any to capture actual config.
type consumePolicyWithConfig struct {
	Type        string            `json:"type" yaml:"type"`
	Name        *string           `json:"name,omitempty" yaml:"name,omitempty"`
	Description *string           `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	ID          string            `json:"id" yaml:"id"`
	Config      map[string]any    `json:"config" yaml:"config"`
	CreatedAt   time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" yaml:"updated_at"`
}

var (
	consumePoliciesUse = consumePoliciesCommandName

	consumePoliciesShort = i18n.T("root.products.konnect.eventgateway.consumePoliciesShort",
		"Manage consume policies for an Event Gateway Virtual Cluster")
	consumePoliciesLong = normalizers.LongDesc(i18n.T("root.products.konnect.eventgateway.consumePoliciesLong",
		`Use the consume-policies command to list or retrieve consume policies for a specific Event Gateway Virtual Cluster.`)) //nolint:lll
	consumePoliciesExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.consumePoliciesExamples",
			fmt.Sprintf(`
# List consume policies for a virtual cluster by ID
%[1]s get event-gateway virtual-clusters consume-policies --gateway-id <gw-id> --virtual-cluster-id <vc-id>
# List consume policies for a virtual cluster by name
%[1]s get event-gateway vc consume-policies --gateway-name my-gw --virtual-cluster-name my-vc
# Get a specific consume policy by ID (positional argument)
%[1]s get event-gateway vc consume-policies --gateway-id <gw-id> --virtual-cluster-id <vc-id> <policy-id>
# Get a specific consume policy by name
%[1]s get event-gateway vc consume-policies --gateway-id <gw-id> --virtual-cluster-id <vc-id> my-policy
# Get a specific consume policy by ID (flag)
%[1]s get event-gateway vc consume-policies --gateway-id <id> --virtual-cluster-id <id> --consume-policy-id <id>
# Get a specific consume policy by name (flag)
%[1]s get event-gateway vc consume-policies --gateway-name gw --virtual-cluster-name vc --consume-policy-name p
`, meta.CLIName)))
)

func newGetEventGatewayConsumePoliciesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     consumePoliciesUse,
		Short:   consumePoliciesShort,
		Long:    consumePoliciesLong,
		Example: consumePoliciesExample,
		Aliases: []string{"consume-policy", "consumes"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindEventGatewayChildFlags(c, args); err != nil {
				return err
			}
			if err := bindVirtualClusterChildFlags(c, args); err != nil {
				return err
			}
			return bindConsumePolicyChildFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := consumePoliciesHandler{cmd: c}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(c)
	addVirtualClusterChildFlags(c)
	addConsumePolicyChildFlags(c)

	if addParentFlags != nil {
		addParentFlags(verb, c)
	}

	return c
}

func addConsumePolicyChildFlags(c *cobra.Command) {
	c.Flags().String(consumePolicyIDFlagName, "",
		fmt.Sprintf(`The ID of the consume policy to retrieve.
- Config path: [ %s ]`, consumePolicyIDConfigPath))
	c.Flags().String(consumePolicyNameFlagName, "",
		fmt.Sprintf(`The name of the consume policy to retrieve.
- Config path: [ %s ]`, consumePolicyNameConfigPath))
	c.MarkFlagsMutuallyExclusive(consumePolicyIDFlagName, consumePolicyNameFlagName)
}

func bindConsumePolicyChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(consumePolicyIDFlagName); flag != nil {
		if err := cfg.BindFlag(consumePolicyIDConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(consumePolicyNameFlagName); flag != nil {
		if err := cfg.BindFlag(consumePolicyNameConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func getConsumePolicyIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(consumePolicyIDConfigPath), cfg.GetString(consumePolicyNameConfigPath)
}

type consumePoliciesHandler struct {
	cmd *cobra.Command
}

func (h consumePoliciesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing consume policies requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		policyID, policyName := getConsumePolicyIdentifiers(cfg)
		if policyID != "" || policyName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					consumePolicyIDFlagName,
					consumePolicyNameFlagName,
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

	// Resolve gateway ID
	gatewayID, gatewayName := getEventGatewayIdentifiers(cfg)
	if gatewayID != "" && gatewayName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", gatewayIDFlagName, gatewayNameFlagName),
		}
	}

	if gatewayID == "" && gatewayName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"an event gateway identifier is required. Provide --%s or --%s",
				gatewayIDFlagName,
				gatewayNameFlagName,
			),
		}
	}

	if gatewayID == "" {
		gatewayID, err = resolveEventGatewayIDByName(gatewayName, sdk.GetEventGatewayControlPlaneAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	// Resolve virtual cluster ID
	virtualClusterID, virtualClusterName := getVirtualClusterIdentifiers(cfg)
	if virtualClusterID != "" && virtualClusterName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				virtualClusterIDFlagName,
				virtualClusterNameFlagName,
			),
		}
	}

	if virtualClusterID == "" && virtualClusterName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"a virtual cluster identifier is required. Provide --%s or --%s",
				virtualClusterIDFlagName,
				virtualClusterNameFlagName,
			),
		}
	}

	virtualClusterAPI := sdk.GetEventGatewayVirtualClusterAPI()
	if virtualClusterAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Virtual Clusters client is not available",
			Err: fmt.Errorf("virtual clusters client not configured"),
		}
	}

	if virtualClusterID == "" {
		virtualClusterID, err = resolveVirtualClusterIDByName(
			virtualClusterName,
			virtualClusterAPI,
			gatewayID,
			helper,
			cfg,
		)
		if err != nil {
			return err
		}
	}

	policyAPI := sdk.GetEventGatewayConsumePolicyAPI()
	if policyAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Consume Policies client is not available",
			Err: fmt.Errorf("consume policies client not configured"),
		}
	}

	// Determine if we're getting a single policy or listing all
	policyID, policyName := getConsumePolicyIdentifiers(cfg)
	var policyIdentifier string

	if len(args) == 1 {
		policyIdentifier = strings.TrimSpace(args[0])
	} else if policyID != "" {
		policyIdentifier = policyID
	} else if policyName != "" {
		policyIdentifier = policyName
	}

	if policyIdentifier != "" {
		return h.getSinglePolicy(
			helper,
			policyAPI,
			gatewayID,
			virtualClusterID,
			policyIdentifier,
			outType,
			printer,
			cfg,
		)
	}

	return h.listPolicies(helper, policyAPI, gatewayID, virtualClusterID, outType, printer, cfg)
}

func (h consumePoliciesHandler) listPolicies(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayConsumePolicyAPI,
	gatewayID string,
	virtualClusterID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	_ config.Hook,
) error {
	policies, rawPolicies, err := fetchConsumePolicies(helper, policyAPI, gatewayID, virtualClusterID)
	if err != nil {
		return err
	}

	records := make([]consumePolicySummaryRecord, 0, len(policies))
	for _, policy := range policies {
		records = append(records, consumePolicyToRecord(policy))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Type})
	}

	// Use raw policies with full config for JSON/YAML output if available
	var outputData any = policies
	if len(rawPolicies) > 0 {
		outputData = rawPolicies
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		outputData,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME", "TYPE"}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h consumePoliciesHandler) getSinglePolicy(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayConsumePolicyAPI,
	gatewayID string,
	virtualClusterID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	_ config.Hook,
) error {
	policyID := identifier
	if !util.IsValidUUID(identifier) {
		policies, _, err := fetchConsumePolicies(helper, policyAPI, gatewayID, virtualClusterID)
		if err != nil {
			return err
		}
		match := findConsumePolicyByName(policies, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("consume policy %q not found", identifier),
			}
		}
		if match.ID != "" {
			policyID = match.ID
		} else {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("consume policy %q does not have an ID", identifier),
			}
		}
	}

	req := kkOps.GetEventGatewayVirtualClusterConsumePolicyRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
		PolicyID:         policyID,
	}

	res, err := policyAPI.GetEventGatewayVirtualClusterConsumePolicy(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get consume policy", err, helper.GetCmd(), attrs...)
	}

	policy := res.EventGatewayPolicy
	if policy == nil {
		return &cmd.ExecutionError{
			Msg: "Consume policy response was empty",
			Err: fmt.Errorf("no consume policy returned for id %s", policyID),
		}
	}

	// Parse raw response to get full config (SDK's EventGatewayPolicyConfig is empty)
	var policyWithConfig *consumePolicyWithConfig
	var parsed consumePolicyWithConfig
	if parseRawConsumePolicyResponse(helper, res, &parsed) {
		policyWithConfig = &parsed
	}

	// If we successfully parsed the raw response with config, use that
	if policyWithConfig != nil {
		return tableview.RenderForFormat(
			helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			consumePolicyToRecord(*policy),
			policyWithConfig,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
		)
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		consumePolicyToRecord(*policy),
		policy,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

// parseRawConsumePolicyResponse parses the raw HTTP response body to extract consume policies with full config.
// The SDK's EventGatewayPolicyConfig struct is empty, so we use this to capture actual config.
func parseRawConsumePolicyResponse[T any](helper cmd.Helper, sdkResponse sdkResponseWithRawBody, target *T) bool {
	return parseRawPolicyResponse(helper, sdkResponse, target)
}

func fetchConsumePolicies(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayConsumePolicyAPI,
	gatewayID string,
	virtualClusterID string,
) ([]kkComps.EventGatewayPolicy, []consumePolicyWithConfig, error) {
	req := kkOps.ListEventGatewayVirtualClusterConsumePoliciesRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
	}

	res, err := policyAPI.ListEventGatewayVirtualClusterConsumePolicies(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, nil, cmd.PrepareExecutionError("Failed to list consume policies", err, helper.GetCmd(), attrs...)
	}

	if res.ListConsumePoliciesResponse == nil {
		return []kkComps.EventGatewayPolicy{}, nil, nil
	}

	// Parse raw response to get full config (SDK's EventGatewayPolicyConfig is empty)
	var rawPolicies []consumePolicyWithConfig
	parseRawPolicyResponse(helper, res, &rawPolicies)

	return res.ListConsumePoliciesResponse, rawPolicies, nil
}

func findConsumePolicyByName(
	policies []kkComps.EventGatewayPolicy,
	identifier string,
) *kkComps.EventGatewayPolicy {
	lowered := strings.ToLower(identifier)
	for _, policy := range policies {
		if policy.Name != nil && strings.ToLower(*policy.Name) == lowered {
			policyCopy := policy
			return &policyCopy
		}
	}
	return nil
}

func consumePolicyToRecord(policy kkComps.EventGatewayPolicy) consumePolicySummaryRecord {
	recordID := policy.ID
	if recordID != "" {
		recordID = util.AbbreviateUUID(recordID)
	} else {
		recordID = valueNA
	}

	recordName := valueNA
	if policy.Name != nil && *policy.Name != "" {
		recordName = *policy.Name
	}

	recordType := policy.Type
	if recordType == "" {
		recordType = valueNA
	}

	recordDesc := valueNA
	if policy.Description != nil && *policy.Description != "" {
		recordDesc = *policy.Description
	}

	return consumePolicySummaryRecord{
		ID:               recordID,
		Name:             recordName,
		Type:             recordType,
		Description:      recordDesc,
		Enabled:          formatEnabledBool(policy.Enabled),
		LocalCreatedTime: policy.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: policy.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func consumePolicyWithConfigToRecord(policy consumePolicyWithConfig) consumePolicySummaryRecord {
	id := policy.ID
	if id != "" {
		id = util.AbbreviateUUID(id)
	} else {
		id = valueNA
	}

	name := valueNA
	if policy.Name != nil && *policy.Name != "" {
		name = *policy.Name
	}

	policyType := policy.Type
	if policyType == "" {
		policyType = valueNA
	}

	desc := valueNA
	if policy.Description != nil && *policy.Description != "" {
		desc = *policy.Description
	}

	return consumePolicySummaryRecord{
		ID:               id,
		Name:             name,
		Type:             policyType,
		Description:      desc,
		Enabled:          formatEnabledBool(policy.Enabled),
		LocalCreatedTime: policy.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: policy.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func consumePolicyWithConfigDetailView(policy *consumePolicyWithConfig) string {
	if policy == nil {
		return ""
	}

	id := strings.TrimSpace(policy.ID)
	if id == "" {
		id = valueNA
	}

	policyType := strings.TrimSpace(policy.Type)
	if policyType == "" {
		policyType = valueNA
	}

	name := valueNA
	if policy.Name != nil && strings.TrimSpace(*policy.Name) != "" {
		name = strings.TrimSpace(*policy.Name)
	}

	description := valueNA
	if policy.Description != nil && strings.TrimSpace(*policy.Description) != "" {
		description = strings.TrimSpace(*policy.Description)
	}

	enabled := formatEnabledBool(policy.Enabled)
	labels := formatLabelPairs(policy.Labels)
	config := formatJSONValue(policy.Config)

	createdAt := policy.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := policy.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "type: %s\n", policyType)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "enabled: %s\n", enabled)
	fmt.Fprintf(&b, "labels: %s\n", labels)
	fmt.Fprintf(&b, "config: %s\n", config)
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

func buildConsumePolicyChildView(policies []consumePolicyWithConfig) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(policies))
	for i := range policies {
		record := consumePolicyWithConfigToRecord(policies[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Type})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(policies) {
			return ""
		}
		return consumePolicyWithConfigDetailView(&policies[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME", "TYPE"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Consume Policies",
		ParentType:     common.ViewParentConsumePolicy,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(policies) {
				return nil
			}
			return &policies[index]
		},
	}
}

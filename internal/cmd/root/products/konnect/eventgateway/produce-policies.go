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
	producePoliciesCommandName = "produce-policies"

	producePolicyIDFlagName   = "policy-id"
	producePolicyNameFlagName = "policy-name"

	producePolicyIDConfigPath   = "konnect.event-gateway.produce-policy.id"
	producePolicyNameConfigPath = "konnect.event-gateway.produce-policy.name"
)

type producePolicySummaryRecord struct {
	ID               string
	Name             string
	Type             string
	Description      string
	Enabled          string
	LocalCreatedTime string
	LocalUpdatedTime string
}

// producePolicyWithConfig is a wrapper that includes the full config from raw API response.
// The SDK's EventGatewayPolicyConfig struct is empty, so we use map[string]any to capture actual config.
type producePolicyWithConfig struct {
	Type           string            `json:"type" yaml:"type"`
	Name           *string           `json:"name,omitempty" yaml:"name,omitempty"`
	Description    *string           `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled        *bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Labels         map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	ID             string            `json:"id" yaml:"id"`
	Config         map[string]any    `json:"config" yaml:"config"`
	CreatedAt      time.Time         `json:"created_at" yaml:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" yaml:"updated_at"`
	ParentPolicyID *string           `json:"parent_policy_id,omitempty" yaml:"parent_policy_id,omitempty"`
	Condition      *string           `json:"condition,omitempty" yaml:"condition,omitempty"`
}

var (
	producePoliciesUse = producePoliciesCommandName

	producePoliciesShort = i18n.T("root.products.konnect.eventgateway.producePoliciesShort",
		"Manage produce policies for an Event Gateway Virtual Cluster")
	producePoliciesLong = normalizers.LongDesc(i18n.T("root.products.konnect.eventgateway.producePoliciesLong",
		`Use the produce-policies command to list or retrieve produce policies for a specific Event Gateway Virtual Cluster.

Produce policies operate on Kafka messages before they are written to the Kafka cluster.
Where possible, apply transformations to the data using produce policies rather than consume policies for maximum efficiency.`))  //nolint:lll
	producePoliciesExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.producePoliciesExamples",
			fmt.Sprintf(`
# List produce policies for a virtual cluster by ID
%[1]s get event-gateway virtual-clusters produce-policies --gateway-id <gw-id> --virtual-cluster-id <vc-id>
# List produce policies for a virtual cluster by name
%[1]s get event-gateway vc produce-policies --gateway-name my-gw --virtual-cluster-name my-vc
# Get a specific produce policy by ID (positional argument)
%[1]s get event-gateway vc produce-policies --gateway-id <gw-id> --virtual-cluster-id <vc-id> <policy-id>
# Get a specific produce policy by name (positional argument)
%[1]s get event-gateway vc pp --gateway-id <gw-id> --virtual-cluster-id <vc-id> my-policy
# Get a specific produce policy by ID (flag)
%[1]s get event-gateway vc pp --gateway-id <id> --virtual-cluster-id <id> --policy-id <id>
# Get a specific produce policy by name (flag)
%[1]s get event-gateway vc pp --gateway-name gw --virtual-cluster-name vc --policy-name p
`, meta.CLIName)))
)

func newGetEventGatewayProducePoliciesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     producePoliciesUse,
		Short:   producePoliciesShort,
		Long:    producePoliciesLong,
		Example: producePoliciesExample,
		Aliases: []string{"produce-policy", "pp", "pps"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			if err := bindEventGatewayChildFlags(cmd, args); err != nil {
				return err
			}
			if err := bindVirtualClusterChildFlags(cmd, args); err != nil {
				return err
			}
			return bindProducePolicyChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := producePoliciesHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(cmd)
	addVirtualClusterChildFlags(cmd)
	addProducePolicyChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

func addProducePolicyChildFlags(cmd *cobra.Command) {
	cmd.Flags().String(producePolicyIDFlagName, "",
		fmt.Sprintf(`The ID of the produce policy to retrieve.
- Config path: [ %s ]`, producePolicyIDConfigPath))
	cmd.Flags().String(producePolicyNameFlagName, "",
		fmt.Sprintf(`The name of the produce policy to retrieve.
- Config path: [ %s ]`, producePolicyNameConfigPath))
	cmd.MarkFlagsMutuallyExclusive(producePolicyIDFlagName, producePolicyNameFlagName)
}

func bindProducePolicyChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(producePolicyIDFlagName); flag != nil {
		if err := cfg.BindFlag(producePolicyIDConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(producePolicyNameFlagName); flag != nil {
		if err := cfg.BindFlag(producePolicyNameConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func getProducePolicyIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(producePolicyIDConfigPath), cfg.GetString(producePolicyNameConfigPath)
}

type producePoliciesHandler struct {
	cmd *cobra.Command
}

func (h producePoliciesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing produce policies requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		policyID, policyName := getProducePolicyIdentifiers(cfg)
		if policyID != "" || policyName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					producePolicyIDFlagName,
					producePolicyNameFlagName,
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

	policyAPI := sdk.GetEventGatewayProducePolicyAPI()
	if policyAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Produce Policies client is not available",
			Err: fmt.Errorf("produce policies client not configured"),
		}
	}

	// Determine if we're getting a single policy or listing all
	policyID, policyName := getProducePolicyIdentifiers(cfg)
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

func (h producePoliciesHandler) listPolicies(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayProducePolicyAPI,
	gatewayID string,
	virtualClusterID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	policies, rawPolicies, err := fetchProducePolicies(helper, policyAPI, gatewayID, virtualClusterID, cfg, "")
	if err != nil {
		return err
	}

	records := make([]producePolicySummaryRecord, 0, len(policies))
	for _, policy := range policies {
		records = append(records, producePolicyToRecord(policy))
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

func (h producePoliciesHandler) getSinglePolicy(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayProducePolicyAPI,
	gatewayID string,
	virtualClusterID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	policyID := identifier
	if !util.IsValidUUID(identifier) {
		policies, _, err := fetchProducePolicies(helper, policyAPI, gatewayID, virtualClusterID, cfg, identifier)
		if err != nil {
			return err
		}
		match := findProducePolicyByName(policies, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("produce policy %q not found", identifier),
			}
		}
		if match.ID != "" {
			policyID = match.ID
		} else {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("produce policy %q does not have an ID", identifier),
			}
		}
	}

	req := kkOps.GetEventGatewayVirtualClusterProducePolicyRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
		PolicyID:         policyID,
	}

	res, err := policyAPI.GetEventGatewayVirtualClusterProducePolicy(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get produce policy", err, helper.GetCmd(), attrs...)
	}

	policy := res.EventGatewayPolicy
	if policy == nil {
		return &cmd.ExecutionError{
			Msg: "Produce policy response was empty",
			Err: fmt.Errorf("no produce policy returned for id %s", policyID),
		}
	}

	// Parse raw response to get full config (SDK's EventGatewayPolicyConfig is empty)
	var policyWithConfig *producePolicyWithConfig
	var parsed producePolicyWithConfig
	if parseRawProducePolicyResponse(helper, res, &parsed) {
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
			producePolicyToRecord(*policy),
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
		producePolicyToRecord(*policy),
		policy,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

// parseRawProducePolicyResponse parses the raw HTTP response body to extract produce policies with full config.
// The SDK's EventGatewayPolicyConfig struct is empty, so we use this to capture actual config.
func parseRawProducePolicyResponse[T any](helper cmd.Helper, sdkResponse sdkResponseWithRawBody, target *T) bool {
	return parseRawPolicyResponse(helper, sdkResponse, target)
}

func fetchProducePolicies(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayProducePolicyAPI,
	gatewayID string,
	virtualClusterID string,
	_ config.Hook,
	_ string, // nameFilter - not supported by API, filtering done locally
) ([]kkComps.EventGatewayPolicy, []producePolicyWithConfig, error) {
	req := kkOps.ListEventGatewayVirtualClusterProducePoliciesRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
	}

	// Note: The produce policy list API doesn't support name filtering,
	// so we fetch all and filter locally in findProducePolicyByName

	res, err := policyAPI.ListEventGatewayVirtualClusterProducePolicies(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, nil, cmd.PrepareExecutionError("Failed to list produce policies", err, helper.GetCmd(), attrs...)
	}

	if res.ListProducePoliciesResponse == nil {
		return []kkComps.EventGatewayPolicy{}, nil, nil
	}

	// Parse raw response to get full config (SDK's EventGatewayPolicyConfig is empty)
	var rawPolicies []producePolicyWithConfig
	parseRawPolicyResponse(helper, res, &rawPolicies)

	return res.ListProducePoliciesResponse, rawPolicies, nil
}

func findProducePolicyByName(
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

func producePolicyToRecord(policy kkComps.EventGatewayPolicy) producePolicySummaryRecord {
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

	description := valueNA
	if policy.Description != nil && *policy.Description != "" {
		description = *policy.Description
	}

	enabled := valueNA
	if policy.Enabled != nil {
		if *policy.Enabled {
			enabled = "true"
		} else {
			enabled = "false"
		}
	}

	createdAt := policy.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := policy.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return producePolicySummaryRecord{
		ID:               id,
		Name:             name,
		Type:             policyType,
		Description:      description,
		Enabled:          enabled,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func producePolicyWithConfigDetailView(policy *producePolicyWithConfig) string {
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

	enabled := valueNA
	if policy.Enabled != nil {
		if *policy.Enabled {
			enabled = "true"
		} else {
			enabled = "false"
		}
	}

	labels := formatLabelPairs(policy.Labels)

	parentPolicyID := valueNA
	if policy.ParentPolicyID != nil && strings.TrimSpace(*policy.ParentPolicyID) != "" {
		parentPolicyID = strings.TrimSpace(*policy.ParentPolicyID)
	}

	condition := valueNA
	if policy.Condition != nil && strings.TrimSpace(*policy.Condition) != "" {
		condition = strings.TrimSpace(*policy.Condition)
	}

	// Use the raw config map which contains actual config data
	config := formatJSONValue(policy.Config)

	createdAt := policy.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := policy.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "type: %s\n", policyType)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "enabled: %s\n", enabled)
	fmt.Fprintf(&b, "condition: %s\n", condition)
	fmt.Fprintf(&b, "labels: %s\n", labels)
	fmt.Fprintf(&b, "parent_policy_id: %s\n", parentPolicyID)
	fmt.Fprintf(&b, "config: %s\n", config)
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

func producePolicyWithConfigToRecord(policy producePolicyWithConfig) producePolicySummaryRecord {
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

	description := valueNA
	if policy.Description != nil && *policy.Description != "" {
		description = *policy.Description
	}

	enabled := valueNA
	if policy.Enabled != nil {
		if *policy.Enabled {
			enabled = "true"
		} else {
			enabled = "false"
		}
	}

	createdAt := policy.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := policy.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return producePolicySummaryRecord{
		ID:               id,
		Name:             name,
		Type:             policyType,
		Description:      description,
		Enabled:          enabled,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func buildProducePolicyChildView(policies []producePolicyWithConfig) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(policies))
	for i := range policies {
		record := producePolicyWithConfigToRecord(policies[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Type})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(policies) {
			return ""
		}
		return producePolicyWithConfigDetailView(&policies[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME", "TYPE"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Produce Policies",
		ParentType:     common.ViewParentProducePolicy,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(policies) {
				return nil
			}
			return &policies[index]
		},
	}
}

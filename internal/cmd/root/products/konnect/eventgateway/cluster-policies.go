package eventgateway

import (
	"fmt"
	"strings"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"charm.land/bubbles/v2/table"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
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
	clusterPoliciesCommandName = "cluster-policies"

	clusterPolicyIDFlagName   = "policy-id"
	clusterPolicyNameFlagName = "policy-name"

	clusterPolicyIDConfigPath   = "konnect.event-gateway.cluster-policy.id"
	clusterPolicyNameConfigPath = "konnect.event-gateway.cluster-policy.name"
)

type clusterPolicySummaryRecord struct {
	ID               string
	Name             string
	Type             string
	Description      string
	Enabled          string
	LocalCreatedTime string
	LocalUpdatedTime string
}

// clusterPolicyWithConfig is a wrapper that includes the full config from raw API response.
// The SDK's EventGatewayPolicyConfig struct is empty, so we use map[string]any to capture actual config.
type clusterPolicyWithConfig struct {
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
}

var (
	clusterPoliciesUse = clusterPoliciesCommandName

	clusterPoliciesShort = i18n.T("root.products.konnect.eventgateway.clusterPoliciesShort",
		"Manage cluster policies for an Event Gateway Virtual Cluster")
	clusterPoliciesLong = normalizers.LongDesc(i18n.T("root.products.konnect.eventgateway.clusterPoliciesLong",
		`Use the cluster-policies command to list or retrieve cluster policies for a specific Event Gateway Virtual Cluster.`)) //nolint:lll
	clusterPoliciesExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.clusterPoliciesExamples",
			fmt.Sprintf(`
# List cluster policies for a virtual cluster by ID
%[1]s get event-gateway virtual-clusters cluster-policies --gateway-id <gw-id> --virtual-cluster-id <vc-id>
# List cluster policies for a virtual cluster by name
%[1]s get event-gateway vc cluster-policies --gateway-name my-gw --virtual-cluster-name my-vc
# Get a specific cluster policy by ID (positional argument)
%[1]s get event-gateway vc cluster-policies --gateway-id <gw-id> --virtual-cluster-id <vc-id> <policy-id>
# Get a specific cluster policy by name (positional argument)
%[1]s get event-gateway vc cp --gateway-id <gw-id> --virtual-cluster-id <vc-id> my-policy
# Get a specific cluster policy by ID (flag)
%[1]s get event-gateway vc cp --gateway-id <id> --virtual-cluster-id <id> --policy-id <id>
# Get a specific cluster policy by name (flag)
%[1]s get event-gateway vc cp --gateway-name gw --virtual-cluster-name vc --policy-name p
`, meta.CLIName)))
)

func newGetEventGatewayClusterPoliciesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     clusterPoliciesUse,
		Short:   clusterPoliciesShort,
		Long:    clusterPoliciesLong,
		Example: clusterPoliciesExample,
		Aliases: []string{"cluster-policy", "cp", "cps"},
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
			return bindClusterPolicyChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := clusterPoliciesHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(cmd)
	addVirtualClusterChildFlags(cmd)
	addClusterPolicyChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

func addClusterPolicyChildFlags(cmd *cobra.Command) {
	cmd.Flags().String(clusterPolicyIDFlagName, "",
		fmt.Sprintf(`The ID of the cluster policy to retrieve.
- Config path: [ %s ]`, clusterPolicyIDConfigPath))
	cmd.Flags().String(clusterPolicyNameFlagName, "",
		fmt.Sprintf(`The name of the cluster policy to retrieve.
- Config path: [ %s ]`, clusterPolicyNameConfigPath))
	cmd.MarkFlagsMutuallyExclusive(clusterPolicyIDFlagName, clusterPolicyNameFlagName)
}

func bindClusterPolicyChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(clusterPolicyIDFlagName); flag != nil {
		if err := cfg.BindFlag(clusterPolicyIDConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(clusterPolicyNameFlagName); flag != nil {
		if err := cfg.BindFlag(clusterPolicyNameConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func getClusterPolicyIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(clusterPolicyIDConfigPath), cfg.GetString(clusterPolicyNameConfigPath)
}

type clusterPoliciesHandler struct {
	cmd *cobra.Command
}

func (h clusterPoliciesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing cluster policies requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		policyID, policyName := getClusterPolicyIdentifiers(cfg)
		if policyID != "" || policyName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					clusterPolicyIDFlagName,
					clusterPolicyNameFlagName,
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

	policyAPI := sdk.GetEventGatewayClusterPolicyAPI()
	if policyAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Cluster Policies client is not available",
			Err: fmt.Errorf("cluster policies client not configured"),
		}
	}

	// Determine if we're getting a single policy or listing all
	policyID, policyName := getClusterPolicyIdentifiers(cfg)
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

func (h clusterPoliciesHandler) listPolicies(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayClusterPolicyAPI,
	gatewayID string,
	virtualClusterID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	policies, rawPolicies, err := fetchClusterPolicies(helper, policyAPI, gatewayID, virtualClusterID, cfg, "")
	if err != nil {
		return err
	}

	records := make([]clusterPolicySummaryRecord, 0, len(policies))
	for _, policy := range policies {
		records = append(records, clusterPolicyToRecord(policy))
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

func (h clusterPoliciesHandler) getSinglePolicy(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayClusterPolicyAPI,
	gatewayID string,
	virtualClusterID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	policyID := identifier
	if !util.IsValidUUID(identifier) {
		policies, _, err := fetchClusterPolicies(helper, policyAPI, gatewayID, virtualClusterID, cfg, identifier)
		if err != nil {
			return err
		}
		match := findClusterPolicyByName(policies, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("cluster policy %q not found", identifier),
			}
		}
		if match.ID != "" {
			policyID = match.ID
		} else {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("cluster policy %q does not have an ID", identifier),
			}
		}
	}

	req := kkOps.GetEventGatewayVirtualClusterClusterLevelPolicyRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
		PolicyID:         policyID,
	}

	res, err := policyAPI.GetEventGatewayVirtualClusterClusterLevelPolicy(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get cluster policy", err, helper.GetCmd(), attrs...)
	}

	policy := res.EventGatewayPolicy
	if policy == nil {
		return &cmd.ExecutionError{
			Msg: "Cluster policy response was empty",
			Err: fmt.Errorf("no cluster policy returned for id %s", policyID),
		}
	}

	// Parse raw response to get full config (SDK's EventGatewayPolicyConfig is empty)
	var policyWithConfig *clusterPolicyWithConfig
	var parsed clusterPolicyWithConfig
	if parseRawClusterPolicyResponse(helper, res, &parsed) {
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
			clusterPolicyToRecord(*policy),
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
		clusterPolicyToRecord(*policy),
		policy,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

// parseRawClusterPolicyResponse parses the raw HTTP response body to extract cluster policies with full config.
// The SDK's EventGatewayPolicyConfig struct is empty, so we use this to capture actual config.
func parseRawClusterPolicyResponse[T any](helper cmd.Helper, sdkResponse sdkResponseWithRawBody, target *T) bool {
	return parseRawPolicyResponse(helper, sdkResponse, target)
}

func fetchClusterPolicies(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayClusterPolicyAPI,
	gatewayID string,
	virtualClusterID string,
	_ config.Hook,
	_ string, // nameFilter - not supported by API, filtering done locally
) ([]kkComps.EventGatewayPolicy, []clusterPolicyWithConfig, error) {
	req := kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesRequest{
		GatewayID:        gatewayID,
		VirtualClusterID: virtualClusterID,
	}

	// Note: The cluster policy list API doesn't support name filtering,
	// so we fetch all and filter locally in findClusterPolicyByName
	res, err := policyAPI.ListEventGatewayVirtualClusterClusterLevelPolicies(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, nil, cmd.PrepareExecutionError("Failed to list cluster policies", err, helper.GetCmd(), attrs...)
	}

	if res.ListClusterPoliciesResponse == nil {
		return []kkComps.EventGatewayPolicy{}, nil, nil
	}

	// Parse raw response to get full config (SDK's EventGatewayPolicyConfig is empty)
	var rawPolicies []clusterPolicyWithConfig
	parseRawPolicyResponse(helper, res, &rawPolicies)

	return res.ListClusterPoliciesResponse, rawPolicies, nil
}

func findClusterPolicyByName(
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

func makeClusterPolicySummaryRecord(
	id string,
	name *string,
	policyType string,
	description *string,
	enabled *bool,
	createdAt, updatedAt time.Time,
) clusterPolicySummaryRecord {
	recordID := id
	if recordID != "" {
		recordID = util.AbbreviateUUID(recordID)
	} else {
		recordID = valueNA
	}

	recordName := valueNA
	if name != nil && *name != "" {
		recordName = *name
	}

	recordType := policyType
	if recordType == "" {
		recordType = valueNA
	}

	recordDesc := valueNA
	if description != nil && *description != "" {
		recordDesc = *description
	}

	return clusterPolicySummaryRecord{
		ID:               recordID,
		Name:             recordName,
		Type:             recordType,
		Description:      recordDesc,
		Enabled:          formatEnabledBool(enabled),
		LocalCreatedTime: createdAt.In(time.Local).Format("2006-01-02 15:04:05"),
		LocalUpdatedTime: updatedAt.In(time.Local).Format("2006-01-02 15:04:05"),
	}
}

func clusterPolicyToRecord(policy kkComps.EventGatewayPolicy) clusterPolicySummaryRecord {
	return makeClusterPolicySummaryRecord(
		policy.ID, policy.Name, policy.Type, policy.Description, policy.Enabled,
		policy.CreatedAt, policy.UpdatedAt,
	)
}

func resolveVirtualClusterIDByName(
	name string,
	virtualClusterAPI helpers.EventGatewayVirtualClusterAPI,
	gatewayID string,
	helper cmd.Helper,
	cfg config.Hook,
) (string, error) {
	clusters, err := fetchVirtualClusters(helper, virtualClusterAPI, gatewayID, cfg)
	if err != nil {
		return "", err
	}

	match := findVirtualClusterByName(clusters, name)
	if match == nil {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("virtual cluster %q not found", name),
		}
	}

	if match.ID == "" {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("virtual cluster %q does not have an ID", name),
		}
	}

	return match.ID, nil
}

func clusterPolicyWithConfigDetailView(policy *clusterPolicyWithConfig) string {
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

	parentPolicyID := valueNA
	if policy.ParentPolicyID != nil && strings.TrimSpace(*policy.ParentPolicyID) != "" {
		parentPolicyID = strings.TrimSpace(*policy.ParentPolicyID)
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
	fmt.Fprintf(&b, "labels: %s\n", labels)
	fmt.Fprintf(&b, "parent_policy_id: %s\n", parentPolicyID)
	fmt.Fprintf(&b, "config: %s\n", config)
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

func clusterPolicyWithConfigToRecord(policy clusterPolicyWithConfig) clusterPolicySummaryRecord {
	return makeClusterPolicySummaryRecord(
		policy.ID, policy.Name, policy.Type, policy.Description, policy.Enabled,
		policy.CreatedAt, policy.UpdatedAt,
	)
}

func buildClusterPolicyChildView(policies []clusterPolicyWithConfig) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(policies))
	for i := range policies {
		record := clusterPolicyWithConfigToRecord(policies[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Type})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(policies) {
			return ""
		}
		return clusterPolicyWithConfigDetailView(&policies[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME", "TYPE"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Cluster Policies",
		ParentType:     "cluster-policy",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(policies) {
				return nil
			}
			return &policies[index]
		},
	}
}

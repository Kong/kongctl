package eventgateway

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
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
	listenerPoliciesCommandName = "listener-policies"

	policyIDFlagName   = "policy-id"
	policyNameFlagName = "policy-name"

	policyIDConfigPath   = "konnect.event-gateway.listener-policy.id"
	policyNameConfigPath = "konnect.event-gateway.listener-policy.name"
)

type listenerPolicySummaryRecord struct {
	ID               string
	Name             string
	Type             string
	Description      string
	Enabled          string
	LocalCreatedTime string
	LocalUpdatedTime string
}

// listenerPolicyWithConfig is a wrapper that includes the full config from raw API response.
// The SDK's EventGatewayListenerPolicyConfig struct is empty, so we use map[string]any to capture actual config.
type listenerPolicyWithConfig struct {
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
	listenerPoliciesUse = listenerPoliciesCommandName

	listenerPoliciesShort = i18n.T("root.products.konnect.eventgateway.listenerPoliciesShort",
		"Manage listener policies for an Event Gateway Listener")
	listenerPoliciesLong = normalizers.LongDesc(i18n.T("root.products.konnect.eventgateway.listenerPoliciesLong",
		`Use the listener-policies command to list or retrieve listener policies for a specific Event Gateway Listener.`))
	listenerPoliciesExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.listenerPoliciesExamples",
			fmt.Sprintf(`
# List listener policies for a listener by ID
%[1]s get event-gateway listener-policies --gateway-id <gateway-id> --listener-id <listener-id>
# List listener policies for a listener by name
%[1]s get event-gateway listener-policies --gateway-name my-gateway --listener-name my-listener
# Get a specific listener policy by ID (positional argument)
%[1]s get event-gateway listener-policies --gateway-id <gateway-id> --listener-id <listener-id> <policy-id>
# Get a specific listener policy by name (positional argument)
%[1]s get event-gateway listener-policies --gateway-id <gateway-id> --listener-id <listener-id> my-policy
# Get a specific listener policy by ID (flag)
%[1]s get event-gateway listener-policies --gateway-id <gateway-id> --listener-id <listener-id> --policy-id <policy-id>
# Get a specific listener policy by name (flag)
%[1]s get event-gateway listener-policies --gateway-name my-gateway --listener-name my-listener --policy-name my-policy
`, meta.CLIName)))
)

func newGetEventGatewayListenerPoliciesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     listenerPoliciesUse,
		Short:   listenerPoliciesShort,
		Long:    listenerPoliciesLong,
		Example: listenerPoliciesExample,
		Aliases: []string{"listener-policy", "lp", "lps"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			if err := bindEventGatewayChildFlags(cmd, args); err != nil {
				return err
			}
			if err := bindListenerChildFlags(cmd, args); err != nil {
				return err
			}
			return bindListenerPolicyChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := listenerPoliciesHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(cmd)
	addListenerChildFlags(cmd)
	addListenerPolicyChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

func addListenerPolicyChildFlags(cmd *cobra.Command) {
	cmd.Flags().String(policyIDFlagName, "",
		fmt.Sprintf(`The ID of the listener policy to retrieve.
- Config path: [ %s ]`, policyIDConfigPath))
	cmd.Flags().String(policyNameFlagName, "",
		fmt.Sprintf(`The name of the listener policy to retrieve.
- Config path: [ %s ]`, policyNameConfigPath))
	cmd.MarkFlagsMutuallyExclusive(policyIDFlagName, policyNameFlagName)
}

func bindListenerPolicyChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(policyIDFlagName); flag != nil {
		if err := cfg.BindFlag(policyIDConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(policyNameFlagName); flag != nil {
		if err := cfg.BindFlag(policyNameConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func getListenerPolicyIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(policyIDConfigPath), cfg.GetString(policyNameConfigPath)
}

type listenerPoliciesHandler struct {
	cmd *cobra.Command
}

func (h listenerPoliciesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing listener policies requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		policyID, policyName := getListenerPolicyIdentifiers(cfg)
		if policyID != "" || policyName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					policyIDFlagName,
					policyNameFlagName,
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

	// Resolve listener ID
	listenerID, listenerName := getListenerIdentifiers(cfg)
	if listenerID != "" && listenerName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", listenerIDFlagName, listenerNameFlagName),
		}
	}

	if listenerID == "" && listenerName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"a listener identifier is required. Provide --%s or --%s",
				listenerIDFlagName,
				listenerNameFlagName,
			),
		}
	}

	listenerAPI := sdk.GetEventGatewayListenerAPI()
	if listenerAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Listeners client is not available",
			Err: fmt.Errorf("listeners client not configured"),
		}
	}

	if listenerID == "" {
		listenerID, err = resolveListenerIDByName(listenerName, listenerAPI, gatewayID, helper, cfg)
		if err != nil {
			return err
		}
	}

	policyAPI := sdk.GetEventGatewayListenerPolicyAPI()
	if policyAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Listener Policies client is not available",
			Err: fmt.Errorf("listener policies client not configured"),
		}
	}

	// Determine if we're getting a single policy or listing all
	policyID, policyName := getListenerPolicyIdentifiers(cfg)
	var policyIdentifier string

	if len(args) == 1 {
		policyIdentifier = strings.TrimSpace(args[0])
	} else if policyID != "" {
		policyIdentifier = policyID
	} else if policyName != "" {
		policyIdentifier = policyName
	}

	// Validate mutual exclusivity of policy ID and name flags
	if policyID != "" && policyName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				policyIDFlagName,
				policyNameFlagName,
			),
		}
	}

	if policyIdentifier != "" {
		return h.getSinglePolicy(
			helper,
			policyAPI,
			gatewayID,
			listenerID,
			policyIdentifier,
			outType,
			printer,
			cfg,
		)
	}

	return h.listPolicies(helper, policyAPI, gatewayID, listenerID, outType, printer, cfg)
}

func (h listenerPoliciesHandler) listPolicies(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayListenerPolicyAPI,
	gatewayID string,
	listenerID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	policies, rawPolicies, err := fetchListenerPolicies(helper, policyAPI, gatewayID, listenerID, cfg)
	if err != nil {
		return err
	}

	records := make([]listenerPolicySummaryRecord, 0, len(policies))
	for _, policy := range policies {
		records = append(records, listenerPolicyToRecord(policy))
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

func (h listenerPoliciesHandler) getSinglePolicy(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayListenerPolicyAPI,
	gatewayID string,
	listenerID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	policyID := identifier
	if !util.IsValidUUID(identifier) {
		// Search by name - fetch all policies and find by name
		policies, _, err := fetchListenerPolicies(helper, policyAPI, gatewayID, listenerID, cfg)
		if err != nil {
			return err
		}
		match := findListenerPolicyByName(policies, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("listener policy %q not found", identifier),
			}
		}
		if match.ID != "" {
			policyID = match.ID
		} else {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("listener policy %q does not have an ID", identifier),
			}
		}
	}

	req := kkOps.GetEventGatewayListenerPolicyRequest{
		GatewayID:              gatewayID,
		EventGatewayListenerID: listenerID,
		PolicyID:               policyID,
	}

	res, err := policyAPI.GetEventGatewayListenerPolicy(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get listener policy", err, helper.GetCmd(), attrs...)
	}

	policy := res.EventGatewayListenerPolicy
	if policy == nil {
		return &cmd.ExecutionError{
			Msg: "Listener policy response was empty",
			Err: fmt.Errorf("no listener policy returned for id %s", policyID),
		}
	}

	// Parse raw response to get full config (SDK's EventGatewayListenerPolicyConfig is empty)
	var policyWithConfig *listenerPolicyWithConfig
	if res.RawResponse != nil && res.RawResponse.Body != nil {
		bodyBytes, readErr := io.ReadAll(res.RawResponse.Body)
		if readErr == nil && len(bodyBytes) > 0 {
			var parsed listenerPolicyWithConfig
			if jsonErr := json.Unmarshal(bodyBytes, &parsed); jsonErr == nil {
				policyWithConfig = &parsed
			}
		}
	}

	// If we successfully parsed the raw response with config, use that
	if policyWithConfig != nil {
		return tableview.RenderForFormat(
			false,
			outType,
			printer,
			helper.GetStreams(),
			listenerPolicyToRecord(*policy),
			policyWithConfig,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
		)
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		listenerPolicyToRecord(*policy),
		policy,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchListenerPolicies(
	helper cmd.Helper,
	policyAPI helpers.EventGatewayListenerPolicyAPI,
	gatewayID string,
	listenerID string,
	_ config.Hook,
) ([]kkComps.EventGatewayListenerPolicy, []listenerPolicyWithConfig, error) {
	req := kkOps.ListEventGatewayListenerPoliciesRequest{
		GatewayID:              gatewayID,
		EventGatewayListenerID: listenerID,
	}

	res, err := policyAPI.ListEventGatewayListenerPolicies(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, nil, cmd.PrepareExecutionError("Failed to list listener policies", err, helper.GetCmd(), attrs...)
	}

	if res.ListEventGatewayListenerPoliciesResponse == nil {
		return []kkComps.EventGatewayListenerPolicy{}, nil, nil
	}

	// Try to parse raw response to get full config
	var rawPolicies []listenerPolicyWithConfig
	if res.RawResponse != nil && res.RawResponse.Body != nil {
		bodyBytes, readErr := io.ReadAll(res.RawResponse.Body)
		if readErr == nil && len(bodyBytes) > 0 {
			if jsonErr := json.Unmarshal(bodyBytes, &rawPolicies); jsonErr != nil {
				// If direct unmarshal fails, the response might be empty or malformed
				rawPolicies = nil
			}
		}
	}

	return res.ListEventGatewayListenerPoliciesResponse, rawPolicies, nil
}

func findListenerPolicyByName(
	policies []kkComps.EventGatewayListenerPolicy,
	identifier string,
) *kkComps.EventGatewayListenerPolicy {
	lowered := strings.ToLower(identifier)
	for _, policy := range policies {
		if policy.ID != "" && strings.ToLower(policy.ID) == lowered {
			policyCopy := policy
			return &policyCopy
		}
		if policy.Name != nil && strings.ToLower(*policy.Name) == lowered {
			policyCopy := policy
			return &policyCopy
		}
	}
	return nil
}

func listenerPolicyToRecord(policy kkComps.EventGatewayListenerPolicy) listenerPolicySummaryRecord {
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

	return listenerPolicySummaryRecord{
		ID:               id,
		Name:             name,
		Type:             policyType,
		Description:      description,
		Enabled:          enabled,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func resolveListenerIDByName(
	name string,
	listenerAPI helpers.EventGatewayListenerAPI,
	gatewayID string,
	helper cmd.Helper,
	cfg config.Hook,
) (string, error) {
	listeners, err := fetchListeners(helper, listenerAPI, gatewayID, cfg, name)
	if err != nil {
		return "", err
	}

	match := findListenerByName(listeners, name)
	if match == nil {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("listener %q not found", name),
		}
	}

	if match.ID == "" {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("listener %q does not have an ID", name),
		}
	}

	return match.ID, nil
}

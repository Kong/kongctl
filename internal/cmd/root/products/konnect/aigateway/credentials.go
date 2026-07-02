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

type aiGatewayConsumerCredentialRecord struct {
	ID               string
	Name             string
	DisplayName      string
	Type             string
	TTL              string
	LocalUpdatedTime string
}

var (
	aiGatewayConsumerCredentialsUse   = "credentials [credential-id|credential-name]"
	aiGatewayConsumerCredentialsShort = i18n.T(
		"root.products.konnect.ai-gateway.consumerCredentialsShort",
		"List or get Consumer Credentials for a Konnect AI Gateway Consumer",
	)
	aiGatewayConsumerCredentialsLong = normalizers.LongDesc(i18n.T(
		"root.products.konnect.ai-gateway.consumerCredentialsLong",
		`Use the credentials command to list or retrieve Credentials for a specific Konnect AI Gateway Consumer.`,
	))
	aiGatewayConsumerCredentialsExample = normalizers.Examples(
		i18n.T("root.products.konnect.ai-gateway.consumerCredentialsExamples",
			fmt.Sprintf(`# List Credentials for an AI Gateway Consumer by names
%[1]s get ai-gateway credentials --gateway-name "Customer Support Gateway" --consumer-name support-user
# List Credentials for an AI Gateway Consumer by IDs
%[1]s get ai-gateway credentials --gateway-id <gateway-id> --consumer-id <consumer-id>
# Get a Credential by name
%[1]s get ai-gateway credentials --gateway-name "Customer Support Gateway" --consumer-name support-user support-user-key
# Get a Credential by ID
%[1]s get ai-gateway credentials --gateway-id <gateway-id> --consumer-id <consumer-id> --credential-id <credential-id>
`, meta.CLIName)),
	)
)

func newGetAIGatewayConsumerCredentialsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &cobra.Command{
		Use:     aiGatewayConsumerCredentialsUse,
		Short:   aiGatewayConsumerCredentialsShort,
		Long:    aiGatewayConsumerCredentialsLong,
		Example: aiGatewayConsumerCredentialsExample,
		Aliases: []string{"credential", "consumer-credential", "consumer-credentials"},
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(c, args); err != nil {
				return err
			}
			if err := bindAIGatewayConsumerFlags(c, args); err != nil {
				return err
			}
			return bindAIGatewayConsumerCredentialFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			handler := aiGatewayConsumerCredentialsHandler{cmd: c}
			return handler.run(args)
		},
	}

	addAIGatewayChildFlags(c)
	addAIGatewayConsumerFlags(c)
	addAIGatewayConsumerCredentialFlags(c)
	if addParentFlags != nil {
		addParentFlags(verb, c)
	}
	return c
}

type aiGatewayConsumerCredentialsHandler struct {
	cmd *cobra.Command
}

func (h aiGatewayConsumerCredentialsHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing AI Gateway Consumer Credentials requires 0 or 1 arguments"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if len(args) == 1 {
		credentialID, credentialName := getAIGatewayConsumerCredentialIdentifiers(cfg)
		if credentialID != "" || credentialName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					aiGatewayConsumerCredentialIDFlagName,
					aiGatewayConsumerCredentialNameFlagName,
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

	consumerAPI := sdk.GetAIGatewayConsumersAPI()
	if consumerAPI == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Consumers client is not available",
			Err: fmt.Errorf("AI Gateway Consumers client not configured"),
		}
	}

	consumerID, consumerName := getAIGatewayConsumerIdentifiers(cfg)
	if consumerID != "" && consumerName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayConsumerIDFlagName,
				aiGatewayConsumerNameFlagName,
			),
		}
	}
	if consumerID == "" && consumerName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"an AI Gateway Consumer identifier is required. Provide --%s or --%s",
				aiGatewayConsumerIDFlagName,
				aiGatewayConsumerNameFlagName,
			),
		}
	}
	if consumerID == "" {
		consumerID, err = resolveAIGatewayConsumerIDByName(helper, consumerAPI, gatewayID, consumerName, cfg)
		if err != nil {
			return err
		}
	}

	credentialID, credentialName := getAIGatewayConsumerCredentialIdentifiers(cfg)
	if credentialID != "" && credentialName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				aiGatewayConsumerCredentialIDFlagName,
				aiGatewayConsumerCredentialNameFlagName,
			),
		}
	}

	identifier := ""
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if credentialID != "" {
		identifier = credentialID
	} else if credentialName != "" {
		identifier = credentialName
	}

	if identifier != "" {
		return h.getSingleCredential(helper, consumerAPI, gatewayID, consumerID, identifier, outType, printer, cfg)
	}
	return h.listCredentials(helper, consumerAPI, gatewayID, consumerID, outType, printer, cfg)
}

func (h aiGatewayConsumerCredentialsHandler) listCredentials(
	helper cmd.Helper,
	consumerAPI helpers.AIGatewayConsumersAPI,
	gatewayID string,
	consumerID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	credentials, err := fetchAIGatewayConsumerCredentials(helper, consumerAPI, gatewayID, consumerID, cfg)
	if err != nil {
		return err
	}

	records := make([]aiGatewayConsumerCredentialRecord, 0, len(credentials))
	tableRows := make([]table.Row, 0, len(credentials))
	for _, credential := range credentials {
		record := aiGatewayConsumerCredentialToRecord(credential)
		records = append(records, record)
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.TTL,
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
		credentials,
		"",
		tableview.WithCustomTable(
			[]string{
				aiGatewayHeaderID,
				aiGatewayHeaderName,
				aiGatewayHeaderDisplayName,
				aiGatewayHeaderType,
				aiGatewayHeaderTTL,
				aiGatewayHeaderUpdated,
			},
			tableRows,
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index < 0 || index >= len(credentials) {
				return ""
			}
			return aiGatewayConsumerCredentialDetailView(credentials[index])
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayConsumerCredential, func(index int) any {
			if index < 0 || index >= len(credentials) {
				return nil
			}
			return &credentials[index]
		}),
	)
}

func (h aiGatewayConsumerCredentialsHandler) getSingleCredential(
	helper cmd.Helper,
	consumerAPI helpers.AIGatewayConsumersAPI,
	gatewayID string,
	consumerID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	credentialID := identifier
	if !util.IsValidUUID(identifier) {
		credentials, err := fetchAIGatewayConsumerCredentials(helper, consumerAPI, gatewayID, consumerID, cfg)
		if err != nil {
			return err
		}
		match := findAIGatewayConsumerCredentialByName(credentials, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("AI Gateway Consumer Credential %q not found", identifier),
			}
		}
		credentialID = match.ID
	}

	req := kkOps.GetAiGatewayConsumerCredentialRequest{
		GatewayID:    gatewayID,
		ConsumerID:   consumerID,
		CredentialID: credentialID,
	}
	res, err := consumerAPI.GetAiGatewayConsumerCredential(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get AI Gateway Consumer Credential", err, helper.GetCmd(), attrs...)
	}
	if res == nil || res.AIGatewayConsumerCredential == nil {
		return &cmd.ExecutionError{
			Msg: "AI Gateway Consumer Credential response was empty",
			Err: fmt.Errorf("no Credential returned for id %s", credentialID),
		}
	}
	credential := res.AIGatewayConsumerCredential

	record := aiGatewayConsumerCredentialToRecord(*credential)
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		record,
		credential,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailRenderer(func(index int) string {
			if index != 0 {
				return ""
			}
			return aiGatewayConsumerCredentialDetailView(*credential)
		}),
		tableview.WithDetailContext(common.ViewParentAIGatewayConsumerCredential, func(index int) any {
			if index != 0 {
				return nil
			}
			return credential
		}),
	)
}

func fetchAIGatewayConsumerCredentials(
	helper cmd.Helper,
	consumerAPI helpers.AIGatewayConsumersAPI,
	gatewayID string,
	consumerID string,
	cfg config.Hook,
) ([]kkComps.AIGatewayConsumerCredential, error) {
	requestPageSize := common.ResolveRequestPageSize(cfg)
	var pageAfter *string
	var allData []kkComps.AIGatewayConsumerCredential

	for {
		req := kkOps.ListAiGatewayConsumerCredentialsRequest{
			GatewayID:  gatewayID,
			ConsumerID: consumerID,
			PageSize:   &requestPageSize,
		}
		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := consumerAPI.ListAiGatewayConsumerCredentials(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError(
				"Failed to list AI Gateway Consumer Credentials",
				err,
				helper.GetCmd(),
				attrs...,
			)
		}
		if res == nil || res.ListAIGatewayConsumerCredentialsResponse == nil {
			break
		}

		allData = append(allData, res.ListAIGatewayConsumerCredentialsResponse.Data...)
		nextCursor := pagination.ExtractPageAfterCursor(res.ListAIGatewayConsumerCredentialsResponse.Meta.Page.Next)
		if nextCursor == "" {
			break
		}
		pageAfter = &nextCursor
	}

	return allData, nil
}

func resolveAIGatewayConsumerIDByName(
	helper cmd.Helper,
	consumerAPI helpers.AIGatewayConsumersAPI,
	gatewayID string,
	name string,
	cfg config.Hook,
) (string, error) {
	consumers, err := fetchAIGatewayConsumers(helper, consumerAPI, gatewayID, cfg)
	if err != nil {
		return "", err
	}
	match := findAIGatewayConsumerByName(consumers, name)
	if match == nil {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("AI Gateway Consumer %q not found", name),
		}
	}
	return match.ID, nil
}

func findAIGatewayConsumerByName(consumers []kkComps.AIGatewayConsumer, name string) *kkComps.AIGatewayConsumer {
	lowered := strings.ToLower(name)
	for i := range consumers {
		if strings.ToLower(declresources.AIGatewayConsumerName(consumers[i])) == lowered {
			return &consumers[i]
		}
	}
	return nil
}

func findAIGatewayConsumerCredentialByName(
	credentials []kkComps.AIGatewayConsumerCredential,
	name string,
) *kkComps.AIGatewayConsumerCredential {
	lowered := strings.ToLower(name)
	for i := range credentials {
		if strings.ToLower(declresources.AIGatewayConsumerCredentialName(credentials[i])) == lowered {
			return &credentials[i]
		}
	}
	return nil
}

func aiGatewayConsumerCredentialToRecord(
	credential kkComps.AIGatewayConsumerCredential,
) aiGatewayConsumerCredentialRecord {
	record := aiGatewayConsumerCredentialRecord{
		ID:               aiGatewayMissingValue,
		Name:             valueOrMissing(declresources.AIGatewayConsumerCredentialName(credential)),
		DisplayName:      valueOrMissing(declresources.AIGatewayConsumerCredentialDisplayName(credential)),
		Type:             valueOrMissing(string(credential.Type)),
		TTL:              aiGatewayMissingValue,
		LocalUpdatedTime: aiGatewayMissingValue,
	}
	if id := declresources.AIGatewayConsumerCredentialID(credential); id != "" {
		record.ID = util.AbbreviateUUID(id)
	}
	if credential.TTL != nil {
		record.TTL = fmt.Sprintf("%d", *credential.TTL)
	}
	if updatedAt := declresources.AIGatewayConsumerCredentialUpdatedAt(credential); !updatedAt.IsZero() {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

func aiGatewayConsumerCredentialDetailView(credential kkComps.AIGatewayConsumerCredential) string {
	payload := make(map[string]any)
	data, err := json.Marshal(credential)
	if err == nil {
		_ = json.Unmarshal(data, &payload)
	}

	order := []string{
		"id",
		"name",
		"type",
		"display_name",
		"ttl",
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

func buildAIGatewayConsumerCredentialChildView(
	credentials []kkComps.AIGatewayConsumerCredential,
) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(credentials))
	for i := range credentials {
		record := aiGatewayConsumerCredentialToRecord(credentials[i])
		tableRows = append(tableRows, table.Row{
			record.ID,
			record.Name,
			record.DisplayName,
			record.Type,
			record.TTL,
		})
	}

	return tableview.ChildView{
		Headers: []string{
			aiGatewayHeaderID,
			aiGatewayHeaderName,
			aiGatewayHeaderDisplayName,
			aiGatewayHeaderType,
			aiGatewayHeaderTTL,
		},
		Rows: tableRows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(credentials) {
				return ""
			}
			return aiGatewayConsumerCredentialDetailView(credentials[index])
		},
		Title:      "AI Gateway Consumer Credentials",
		ParentType: common.ViewParentAIGatewayConsumerCredential,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(credentials) {
				return nil
			}
			return &credentials[index]
		},
	}
}

package eventgateway

import (
	"fmt"
	"net/url"
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
	staticKeysCommandName = "static-keys"
)

type staticKeySummaryRecord struct {
	ID               string
	Name             string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	staticKeysUse = staticKeysCommandName

	staticKeysShort = i18n.T("root.products.konnect.eventgateway.staticKeysShort",
		"Manage static keys for an Event Gateway")
	staticKeysLong = normalizers.LongDesc(
		i18n.T(
			"root.products.konnect.eventgateway.staticKeysLong",
			`Use the static-keys command to list or retrieve static keys for a specific Event Gateway.`,
		),
	)
	staticKeysExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.staticKeysExamples",
			fmt.Sprintf(`
# List static keys for an event gateway by name
%[1]s get event-gateway static-keys --gateway-name my-gateway
# List static keys for an event gateway by ID
%[1]s get event-gateway static-keys --gateway-id <gateway-id>
# Get a specific static key by name (positional argument)
%[1]s get event-gateway static-keys --gateway-name my-gateway my-static-key
# Get a specific static key by ID (positional argument)
%[1]s get event-gateway static-keys --gateway-id <gateway-id> <static-key-id>
# Get a specific static key by name (flag)
%[1]s get event-gateway static-keys --gateway-name my-gateway --static-key-name my-static-key
# Get a specific static key by ID (flag)
%[1]s get event-gateway static-keys --gateway-id <gateway-id> --static-key-id <static-key-id>
`, meta.CLIName)))
)

func newGetEventGatewayStaticKeysCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     staticKeysUse,
		Short:   staticKeysShort,
		Long:    staticKeysLong,
		Example: staticKeysExample,
		Aliases: []string{"static-key", "sk"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			if err := bindEventGatewayChildFlags(cmd, args); err != nil {
				return err
			}
			return bindStaticKeyChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := staticKeysHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(cmd)
	addStaticKeyChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type staticKeysHandler struct {
	cmd *cobra.Command
}

func (h staticKeysHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing static keys requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		skID, skName := getStaticKeyIdentifiers(cfg)
		if skID != "" || skName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					staticKeyIDFlagName,
					staticKeyNameFlagName,
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

	staticKeyAPI := sdk.GetEventGatewayStaticKeyAPI()
	if staticKeyAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Static key client is not available",
			Err: fmt.Errorf("static key client not configured"),
		}
	}

	// Validate mutual exclusivity of static key ID and name flags
	skID, skName := getStaticKeyIdentifiers(cfg)
	if skID != "" && skName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				staticKeyIDFlagName,
				staticKeyNameFlagName,
			),
		}
	}

	var skIdentifier string
	if len(args) == 1 {
		skIdentifier = strings.TrimSpace(args[0])
	} else if skID != "" {
		skIdentifier = skID
	} else if skName != "" {
		skIdentifier = skName
	}

	if skIdentifier != "" {
		return h.getSingleStaticKey(helper, staticKeyAPI, gatewayID, skIdentifier, outType, printer, cfg)
	}

	return h.listStaticKeys(helper, staticKeyAPI, gatewayID, outType, printer, cfg)
}

func (h staticKeysHandler) listStaticKeys(
	helper cmd.Helper,
	staticKeyAPI helpers.EventGatewayStaticKeyAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	keys, err := fetchStaticKeys(helper, staticKeyAPI, gatewayID, cfg, "")
	if err != nil {
		return err
	}

	records := make([]staticKeySummaryRecord, 0, len(keys))
	for _, sk := range keys {
		records = append(records, staticKeyToRecord(sk))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Description})
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		keys,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME", "DESCRIPTION"}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h staticKeysHandler) getSingleStaticKey(
	helper cmd.Helper,
	staticKeyAPI helpers.EventGatewayStaticKeyAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	staticKeyID := identifier
	if !util.IsValidUUID(identifier) {
		// Resolve name to ID by listing and matching exactly
		keys, err := fetchStaticKeys(helper, staticKeyAPI, gatewayID, cfg, identifier)
		if err != nil {
			return err
		}
		match := findStaticKeyByName(keys, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("static key %q not found", identifier),
			}
		}
		staticKeyID = match.ID
	}

	res, err := staticKeyAPI.GetEventGatewayStaticKey(helper.GetContext(), kkOps.GetEventGatewayStaticKeyRequest{
		GatewayID:   gatewayID,
		StaticKeyID: staticKeyID,
	})
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get static key", err, helper.GetCmd(), attrs...)
	}

	if res.GetEventGatewayStaticKey() == nil {
		return &cmd.ExecutionError{
			Msg: "Static key response was empty",
			Err: fmt.Errorf("no static key returned for id %s", staticKeyID),
		}
	}

	sk := res.GetEventGatewayStaticKey()

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		staticKeyToRecord(*sk),
		sk,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchStaticKeys(
	helper cmd.Helper,
	staticKeyAPI helpers.EventGatewayStaticKeyAPI,
	gatewayID string,
	cfg config.Hook,
	nameFilter string,
) ([]kkComps.EventGatewayStaticKey, error) {
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allKeys []kkComps.EventGatewayStaticKey
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayStaticKeysRequest{
			GatewayID: gatewayID,
			PageSize:  new(requestPageSize),
		}

		if nameFilter != "" {
			req.Filter = &kkComps.EventGatewayCommonFilter{
				Name: &kkComps.StringFieldContainsFilter{
					Contains: nameFilter,
				},
			}
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := staticKeyAPI.ListEventGatewayStaticKeys(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list static keys", err, helper.GetCmd(), attrs...)
		}

		if res.GetListEventGatewayStaticKeysResponse() == nil {
			break
		}

		allKeys = append(allKeys, res.GetListEventGatewayStaticKeysResponse().Data...)

		if res.GetListEventGatewayStaticKeysResponse().Meta == nil ||
			res.GetListEventGatewayStaticKeysResponse().Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.GetListEventGatewayStaticKeysResponse().Meta.Page.Next)
		if err != nil {
			return nil, cmd.PrepareExecutionError(
				"Failed to list static keys: invalid cursor",
				err,
				helper.GetCmd(),
			)
		}

		values := u.Query()
		pageAfter = new(values.Get("page[after]"))
	}

	return allKeys, nil
}

func findStaticKeyByName(
	keys []kkComps.EventGatewayStaticKey,
	name string,
) *kkComps.EventGatewayStaticKey {
	lowered := strings.ToLower(name)
	for i := range keys {
		if strings.ToLower(keys[i].Name) == lowered {
			return &keys[i]
		}
	}
	return nil
}

func staticKeyToRecord(sk kkComps.EventGatewayStaticKey) staticKeySummaryRecord {
	id := sk.ID
	if id != "" {
		id = util.AbbreviateUUID(id)
	} else {
		id = valueNA
	}

	name := sk.Name
	if name == "" {
		name = valueNA
	}

	description := valueNA
	if sk.Description != nil && *sk.Description != "" {
		description = *sk.Description
	}

	createdAt := sk.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := sk.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return staticKeySummaryRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func staticKeyDetailView(sk *kkComps.EventGatewayStaticKey) string {
	if sk == nil {
		return ""
	}

	id := strings.TrimSpace(sk.ID)
	if id == "" {
		id = valueNA
	}

	name := sk.Name
	if name == "" {
		name = valueNA
	}

	description := valueNA
	if sk.Description != nil && strings.TrimSpace(*sk.Description) != "" {
		description = strings.TrimSpace(*sk.Description)
	}

	value := valueNA
	if sk.Value != nil && strings.TrimSpace(*sk.Value) != "" {
		value = strings.TrimSpace(*sk.Value)
	}

	createdAt := sk.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := sk.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "value: %s\n", value)
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

func buildStaticKeyChildView(keys []kkComps.EventGatewayStaticKey) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(keys))
	for i := range keys {
		record := staticKeyToRecord(keys[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Description})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(keys) {
			return ""
		}
		return staticKeyDetailView(&keys[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME", "DESCRIPTION"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Static Keys",
		ParentType:     "static-key",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(keys) {
				return nil
			}
			return &keys[index]
		},
	}
}

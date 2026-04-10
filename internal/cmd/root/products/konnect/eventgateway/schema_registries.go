package eventgateway

import (
	"encoding/json"
	"fmt"
	"io"
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
	schemaRegistriesCommandName = "schema-registries"
)

type schemaRegistrySummaryRecord struct {
	ID               string
	Name             string
	Type             string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

// schemaRegistryEntry wraps the SDK SchemaRegistry with raw config data from the API
// response (the SDK Config struct is empty/opaque).
type schemaRegistryEntry struct {
	kkComps.SchemaRegistry
	RawConfig map[string]any
}

var (
	schemaRegistriesUse = schemaRegistriesCommandName

	schemaRegistriesShort = i18n.T("root.products.konnect.eventgateway.schemaRegistriesShort",
		"Manage schema registries for an Event Gateway")
	schemaRegistriesLong = normalizers.LongDesc(
		i18n.T(
			"root.products.konnect.eventgateway.schemaRegistriesLong",
			`Use the schema-registries command to list or retrieve schema registries for a specific Event Gateway.`,
		),
	)
	schemaRegistriesExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.schemaRegistriesExamples",
			fmt.Sprintf(`
# List schema registries for an event gateway by ID
%[1]s get event-gateway schema-registries --gateway-id <gateway-id>
# List schema registries for an event gateway by name
%[1]s get event-gateway schema-registries --gateway-name my-gateway
# Get a specific schema registry by ID (positional argument)
%[1]s get event-gateway schema-registries --gateway-id <gateway-id> <schema-registry-id>
# Get a specific schema registry by name (positional argument)
%[1]s get event-gateway schema-registries --gateway-id <gateway-id> my-registry
# Get a specific schema registry by ID (flag)
%[1]s get event-gateway schema-registries --gateway-id <gateway-id> --schema-registry-id <registry-id>
# Get a specific schema registry by name (flag)
%[1]s get event-gateway schema-registries --gateway-name my-gateway --schema-registry-name my-registry
`, meta.CLIName)))
)

func newGetEventGatewaySchemaRegistriesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     schemaRegistriesUse,
		Short:   schemaRegistriesShort,
		Long:    schemaRegistriesLong,
		Example: schemaRegistriesExample,
		Aliases: []string{"schema-registry", "sr"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			if err := bindEventGatewayChildFlags(cmd, args); err != nil {
				return err
			}
			return bindSchemaRegistryChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := schemaRegistriesHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(cmd)
	addSchemaRegistryChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type schemaRegistriesHandler struct {
	cmd *cobra.Command
}

func (h schemaRegistriesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"too many arguments. Listing schema registries requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		srID, srName := getSchemaRegistryIdentifiers(cfg)
		if srID != "" || srName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					schemaRegistryIDFlagName,
					schemaRegistryNameFlagName,
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

	registryAPI := sdk.GetEventGatewaySchemaRegistryAPI()
	if registryAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Schema registry client is not available",
			Err: fmt.Errorf("schema registry client not configured"),
		}
	}

	// Validate mutual exclusivity of schema registry ID and name flags
	srID, srName := getSchemaRegistryIdentifiers(cfg)
	if srID != "" && srName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				schemaRegistryIDFlagName,
				schemaRegistryNameFlagName,
			),
		}
	}

	var srIdentifier string
	if len(args) == 1 {
		srIdentifier = strings.TrimSpace(args[0])
	} else if srID != "" {
		srIdentifier = srID
	} else if srName != "" {
		srIdentifier = srName
	}

	if srIdentifier != "" {
		return h.getSingleSchemaRegistry(
			helper,
			registryAPI,
			gatewayID,
			srIdentifier,
			outType,
			printer,
			cfg,
		)
	}

	return h.listSchemaRegistries(helper, registryAPI, gatewayID, outType, printer, cfg)
}

func (h schemaRegistriesHandler) listSchemaRegistries(
	helper cmd.Helper,
	registryAPI helpers.EventGatewaySchemaRegistryAPI,
	gatewayID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	registries, err := fetchSchemaRegistries(helper, registryAPI, gatewayID, cfg, "")
	if err != nil {
		return err
	}

	records := make([]schemaRegistrySummaryRecord, 0, len(registries))
	for _, sr := range registries {
		records = append(records, schemaRegistryToRecord(sr))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Type})
	}

	// Extract base SDK types for JSON/YAML serialization.
	rawRegistries := make([]kkComps.SchemaRegistry, 0, len(registries))
	for _, sr := range registries {
		rawRegistries = append(rawRegistries, sr.SchemaRegistry)
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		rawRegistries,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME", "TYPE"}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h schemaRegistriesHandler) getSingleSchemaRegistry(
	helper cmd.Helper,
	registryAPI helpers.EventGatewaySchemaRegistryAPI,
	gatewayID string,
	identifier string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	registryID := identifier
	if !util.IsValidUUID(identifier) {
		// Use name filter to optimize the list query, then match exactly
		registries, err := fetchSchemaRegistries(helper, registryAPI, gatewayID, cfg, identifier)
		if err != nil {
			return err
		}
		match := findSchemaRegistryByName(registries, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("schema registry %q not found", identifier),
			}
		}
		registryID = match.ID
	}

	res, err := registryAPI.GetEventGatewaySchemaRegistry(helper.GetContext(), gatewayID, registryID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get schema registry", err, helper.GetCmd(), attrs...)
	}

	sr := res.GetSchemaRegistry()
	if sr == nil {
		return &cmd.ExecutionError{
			Msg: "Schema registry response was empty",
			Err: fmt.Errorf("no schema registry returned for id %s", registryID),
		}
	}

	// Parse raw response to retrieve the config (SDK Config struct is opaque).
	entry := schemaRegistryEntry{SchemaRegistry: *sr}
	if res.RawResponse != nil && res.RawResponse.Body != nil {
		bodyBytes, readErr := io.ReadAll(res.RawResponse.Body)
		if readErr == nil && len(bodyBytes) > 0 {
			var rawResp struct {
				Config map[string]any `json:"config"`
			}
			if jsonErr := json.Unmarshal(bodyBytes, &rawResp); jsonErr == nil {
				entry.RawConfig = rawResp.Config
			}
		}
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		schemaRegistryToRecord(entry),
		sr,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchSchemaRegistries(
	helper cmd.Helper,
	registryAPI helpers.EventGatewaySchemaRegistryAPI,
	gatewayID string,
	cfg config.Hook,
	nameFilter string,
) ([]schemaRegistryEntry, error) {
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.SchemaRegistry
	rawConfigByID := make(map[string]map[string]any)
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewaySchemaRegistriesRequest{
			GatewayID: gatewayID,
			PageSize:  new(requestPageSize),
		}

		// Apply name filter if provided (fuzzy match; exact match is done client-side)
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

		res, err := registryAPI.ListEventGatewaySchemaRegistries(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError(
				"Failed to list schema registries", err, helper.GetCmd(), attrs...)
		}

		if res.GetListSchemaRegistriesResponse() == nil {
			break
		}

		// Parse the raw response body to extract full config (SDK Config struct is opaque).
		if res.RawResponse != nil && res.RawResponse.Body != nil {
			bodyBytes, readErr := io.ReadAll(res.RawResponse.Body)
			if readErr == nil && len(bodyBytes) > 0 {
				var rawResp struct {
					Data []struct {
						ID     string         `json:"id"`
						Config map[string]any `json:"config"`
					} `json:"data"`
				}
				if jsonErr := json.Unmarshal(bodyBytes, &rawResp); jsonErr == nil {
					for _, item := range rawResp.Data {
						if item.ID != "" && item.Config != nil {
							rawConfigByID[item.ID] = item.Config
						}
					}
				}
			}
		}

		data := res.GetListSchemaRegistriesResponse().Data
		allData = append(allData, data...)

		if res.GetListSchemaRegistriesResponse().Meta == nil ||
			res.GetListSchemaRegistriesResponse().Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.GetListSchemaRegistriesResponse().Meta.Page.Next)
		if err != nil {
			return nil, cmd.PrepareExecutionError(
				"Failed to list schema registries: invalid cursor",
				err,
				helper.GetCmd(),
			)
		}

		values := u.Query()
		pageAfter = new(values.Get("page[after]"))
	}

	entries := make([]schemaRegistryEntry, 0, len(allData))
	for _, sr := range allData {
		entries = append(entries, schemaRegistryEntry{
			SchemaRegistry: sr,
			RawConfig:      rawConfigByID[sr.ID],
		})
	}

	return entries, nil
}

func findSchemaRegistryByName(
	registries []schemaRegistryEntry,
	name string,
) *schemaRegistryEntry {
	lowered := strings.ToLower(name)
	for i := range registries {
		if strings.ToLower(registries[i].Name) == lowered {
			return &registries[i]
		}
	}
	return nil
}

func schemaRegistryToRecord(sr schemaRegistryEntry) schemaRegistrySummaryRecord {
	id := sr.ID
	if id != "" {
		id = util.AbbreviateUUID(id)
	} else {
		id = valueNA
	}

	name := sr.Name
	if name == "" {
		name = valueNA
	}

	srType := sr.Type
	if srType == "" {
		srType = valueNA
	}

	description := valueNA
	if sr.Description != nil && *sr.Description != "" {
		description = *sr.Description
	}

	createdAt := sr.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := sr.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return schemaRegistrySummaryRecord{
		ID:               id,
		Name:             name,
		Type:             srType,
		Description:      description,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

func schemaRegistryDetailView(sr *schemaRegistryEntry) string {
	if sr == nil {
		return ""
	}

	id := strings.TrimSpace(sr.ID)
	if id == "" {
		id = valueNA
	}

	name := sr.Name
	if name == "" {
		name = valueNA
	}

	srType := sr.Type
	if srType == "" {
		srType = valueNA
	}

	description := valueNA
	if sr.Description != nil && strings.TrimSpace(*sr.Description) != "" {
		description = strings.TrimSpace(*sr.Description)
	}

	createdAt := sr.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	updatedAt := sr.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "type: %s\n", srType)
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	if len(sr.RawConfig) > 0 {
		fmt.Fprintf(&b, "config:\n")
		if v, ok := sr.RawConfig["schema_type"]; ok {
			fmt.Fprintf(&b, "  schema_type: %v\n", v)
		}
		if v, ok := sr.RawConfig["endpoint"]; ok {
			fmt.Fprintf(&b, "  endpoint: %v\n", v)
		}
		if v, ok := sr.RawConfig["timeout_seconds"]; ok {
			fmt.Fprintf(&b, "  timeout_seconds: %v\n", v)
		}
		if auth, ok := sr.RawConfig["authentication"].(map[string]any); ok {
			fmt.Fprintf(&b, "  authentication:\n")
			if t, ok := auth["type"]; ok {
				fmt.Fprintf(&b, "    type: %v\n", t)
			}
			if u, ok := auth["username"]; ok {
				fmt.Fprintf(&b, "    username: %v\n", u)
			}
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func buildSchemaRegistryChildView(registries []schemaRegistryEntry) tableview.ChildView {
	tableRows := make([]table.Row, 0, len(registries))
	for i := range registries {
		record := schemaRegistryToRecord(registries[i])
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.Type})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(registries) {
			return ""
		}
		return schemaRegistryDetailView(&registries[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME", "TYPE"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Schema Registries",
		ParentType:     "schema-registry",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(registries) {
				return nil
			}
			return &registries[index]
		},
	}
}

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

// schemaRegistryEntry holds a schema registry with full config from the raw API
// response. The SDK's SchemaRegistryConfig struct is opaque/empty, so we parse
// the raw response body directly, keeping Config as map[string]any.
type schemaRegistryEntry struct {
	ID          string            `json:"id"                    yaml:"id"`
	Name        string            `json:"name"                  yaml:"name"`
	Type        string            `json:"type"                  yaml:"type"`
	Description *string           `json:"description,omitempty" yaml:"description,omitempty"`
	Config      map[string]any    `json:"config"                yaml:"config"`
	Labels      map[string]string `json:"labels,omitempty"      yaml:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"            yaml:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"            yaml:"updated_at"`
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

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		registries,
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

	if res.GetSchemaRegistry() == nil {
		return &cmd.ExecutionError{
			Msg: "Schema registry response was empty",
			Err: fmt.Errorf("no schema registry returned for id %s", registryID),
		}
	}

	// Parse raw response to get full config (SDK SchemaRegistryConfig is opaque/empty).
	var entry schemaRegistryEntry
	if !parseRawPolicyResponse(helper, res, &entry) {
		// Fallback: populate from SDK type (Config will be empty).
		sr := res.GetSchemaRegistry()
		entry = schemaRegistryEntry{
			ID:          sr.ID,
			Name:        sr.Name,
			Type:        sr.Type,
			Description: sr.Description,
			Labels:      sr.Labels,
			CreatedAt:   sr.CreatedAt,
			UpdatedAt:   sr.UpdatedAt,
		}
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		schemaRegistryToRecord(entry),
		&entry,
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

	var allEntries []schemaRegistryEntry
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

		// Parse raw response to get entries with full config (SDK SchemaRegistryConfig is opaque/empty).
		var rawPage struct {
			Data []schemaRegistryEntry `json:"data"`
		}
		if parseRawPolicyResponse(helper, res, &rawPage) && len(rawPage.Data) > 0 {
			allEntries = append(allEntries, rawPage.Data...)
		} else {
			// Fallback: use SDK types (Config will be empty).
			for _, sr := range res.GetListSchemaRegistriesResponse().Data {
				allEntries = append(allEntries, schemaRegistryEntry{
					ID:          sr.ID,
					Name:        sr.Name,
					Type:        sr.Type,
					Description: sr.Description,
					Labels:      sr.Labels,
					CreatedAt:   sr.CreatedAt,
					UpdatedAt:   sr.UpdatedAt,
				})
			}
		}

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

	return allEntries, nil
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
	config := formatJSONValue(sr.Config)

	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)
	fmt.Fprintf(&b, "config: %s\n", config)

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

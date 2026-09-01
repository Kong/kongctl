package aigateway

import (
	"context"
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
	"github.com/kong/kongctl/internal/util/pagination"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type aiGatewayTLSResourceKind int

const (
	aiGatewayCertificateKind aiGatewayTLSResourceKind = iota
	aiGatewayCACertificateKind
	aiGatewaySNIKind
)

type aiGatewayTLSCommandConfig struct {
	kind       aiGatewayTLSResourceKind
	use        string
	short      string
	aliases    []string
	flags      pairedAIGatewayFlags
	resource   string
	headers    []string
	viewParent string
}

type aiGatewayTLSRecord struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	DisplayName string `json:"display_name,omitempty" yaml:"display_name,omitempty"`
	Hostname    string `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	Certificate string `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	Updated     string `json:"updated,omitempty" yaml:"updated,omitempty"`
}

type aiGatewayTLSReader interface {
	List(context.Context, string, int64, *string) ([]any, *string, error)
	Get(context.Context, string, string) (any, error)
}

func newGetAIGatewayCertificatesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	return newGetAIGatewayTLSResourceCmd(aiGatewayTLSCommandConfig{
		kind: aiGatewayCertificateKind, use: "certificates [certificate-id|name]",
		short:   "List or get runtime TLS certificates for a Konnect AI Gateway",
		aliases: []string{"certificate", "cert", "certs"}, flags: aiGatewayCertificateFlags,
		resource: "AI Gateway runtime certificate", headers: []string{"ID", "NAME", "UPDATED"},
		viewParent: common.ViewParentAIGatewayCertificate,
	}, verb, addParentFlags, parentPreRun)
}

func newGetAIGatewayCACertificatesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	return newGetAIGatewayTLSResourceCmd(aiGatewayTLSCommandConfig{
		kind: aiGatewayCACertificateKind, use: "ca-certificates [certificate-id|name]",
		short:   "List or get CA certificates for a Konnect AI Gateway",
		aliases: []string{"ca-certificate", "ca-cert", "ca-certs"}, flags: aiGatewayCACertificateFlags,
		resource: "AI Gateway CA certificate", headers: []string{"ID", "NAME", "UPDATED"},
		viewParent: common.ViewParentAIGatewayCACertificate,
	}, verb, addParentFlags, parentPreRun)
}

func newGetAIGatewaySNIsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	return newGetAIGatewayTLSResourceCmd(aiGatewayTLSCommandConfig{
		kind: aiGatewaySNIKind, use: "snis [sni-id|name]", short: "List or get SNIs for a Konnect AI Gateway",
		aliases: []string{"sni"}, flags: aiGatewaySNIFlags, resource: "AI Gateway SNI",
		headers:    []string{"ID", "NAME", "DISPLAY NAME", "HOSTNAME", "CERTIFICATE", "UPDATED"},
		viewParent: common.ViewParentAIGatewaySNI,
	}, verb, addParentFlags, parentPreRun)
}

func newGetAIGatewayTLSResourceCmd(
	resourceConfig aiGatewayTLSCommandConfig,
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	command := &cobra.Command{
		Use: resourceConfig.use, Short: resourceConfig.short,
		Long: fmt.Sprintf("List or retrieve %s resources using the canonical gateway-scoped API.", resourceConfig.resource),
		Example: fmt.Sprintf("%s get ai-gateway %s --gateway-id <gateway-id>", meta.CLIName,
			strings.Fields(resourceConfig.use)[0]),
		Aliases: resourceConfig.aliases,
		Args:    cobra.MaximumNArgs(1),
		PreRunE: func(command *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(command, args); err != nil {
					return err
				}
			}
			if err := bindAIGatewayChildFlags(command, args); err != nil {
				return err
			}
			return bindAIGatewayFlags(command, args, pairedAIGatewayBindings(resourceConfig.flags)...)
		},
		RunE: func(command *cobra.Command, args []string) error {
			return runAIGatewayTLSResourceCommand(command, args, resourceConfig)
		},
	}
	addAIGatewayChildFlags(command)
	addPairedAIGatewayFlags(command, resourceConfig.flags)
	if addParentFlags != nil {
		addParentFlags(verb, command)
	}
	return command
}

func runAIGatewayTLSResourceCommand(
	command *cobra.Command,
	args []string,
	resourceConfig aiGatewayTLSCommandConfig,
) error {
	helper := cmd.BuildHelper(command, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
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
	gatewayID, err := resolveRequiredAIGatewayID(cfg, sdk.GetAIGatewayAPI(), helper)
	if err != nil {
		return err
	}
	reader, err := newAIGatewayTLSReader(resourceConfig.kind, sdk)
	if err != nil {
		return err
	}
	resourceID, resourceName := getPairedAIGatewayIdentifiers(
		cfg, resourceConfig.flags.idPath, resourceConfig.flags.namePath,
	)
	if len(args) == 1 && (resourceID != "" || resourceName != "") {
		return &cmd.ConfigurationError{Err: fmt.Errorf(
			"cannot specify both a positional argument and --%s or --%s",
			resourceConfig.flags.idFlag, resourceConfig.flags.nameFlag,
		)}
	}
	var identifier string
	if len(args) == 1 {
		identifier = strings.TrimSpace(args[0])
	} else if resourceID != "" {
		identifier = resourceID
	} else {
		identifier = resourceName
	}
	if identifier != "" {
		return renderSingleAIGatewayTLSResource(helper, cfg, reader, gatewayID, identifier, resourceConfig, outType, printer)
	}
	return renderAIGatewayTLSResourceList(helper, cfg, reader, gatewayID, resourceConfig, outType, printer)
}

func renderAIGatewayTLSResourceList(
	helper cmd.Helper,
	cfg config.Hook,
	reader aiGatewayTLSReader,
	gatewayID string,
	resourceConfig aiGatewayTLSCommandConfig,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	items, err := fetchAIGatewayTLSResources(helper, cfg, reader, gatewayID, resourceConfig.resource)
	if err != nil {
		return err
	}
	return renderAIGatewayTLSResources(helper, items, resourceConfig, outType, printer)
}

func renderSingleAIGatewayTLSResource(
	helper cmd.Helper,
	cfg config.Hook,
	reader aiGatewayTLSReader,
	gatewayID, identifier string,
	resourceConfig aiGatewayTLSCommandConfig,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	items, err := fetchAIGatewayTLSResources(helper, cfg, reader, gatewayID, resourceConfig.resource)
	if err != nil {
		return err
	}
	var match any
	for _, item := range items {
		record := aiGatewayTLSResourceRecord(item)
		if record.ID == identifier || record.Name == identifier {
			match = item
			break
		}
	}
	if match == nil {
		return &cmd.ConfigurationError{Err: fmt.Errorf("%s %q not found", resourceConfig.resource, identifier)}
	}
	item, err := reader.Get(helper.GetContext(), gatewayID, aiGatewayTLSResourceRecord(match).ID)
	if err != nil {
		return cmd.PrepareExecutionError("Failed to get "+resourceConfig.resource, err, helper.GetCmd(),
			cmd.TryConvertErrorToAttrs(err)...)
	}
	if item == nil {
		return &cmd.ExecutionError{
			Msg: resourceConfig.resource + " response was empty",
			Err: fmt.Errorf("no resource returned for %s", identifier),
		}
	}
	return renderAIGatewayTLSResources(helper, []any{item}, resourceConfig, outType, printer)
}

func renderAIGatewayTLSResources(
	helper cmd.Helper,
	items []any,
	resourceConfig aiGatewayTLSCommandConfig,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
) error {
	records := make([]aiGatewayTLSRecord, 0, len(items))
	rows := make([]table.Row, 0, len(items))
	for _, item := range items {
		record := aiGatewayTLSResourceRecord(item)
		records = append(records, record)
		if resourceConfig.kind == aiGatewaySNIKind {
			rows = append(rows, table.Row{
				record.ID, record.Name, record.DisplayName, record.Hostname, record.Certificate, record.Updated,
			})
		} else {
			rows = append(rows, table.Row{record.ID, record.Name, record.Updated})
		}
	}
	return tableview.RenderForFormat(
		helper, false, outType, printer, helper.GetStreams(), records, items, "",
		tableview.WithCustomTable(resourceConfig.headers, rows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
		tableview.WithDetailHelper(helper),
		tableview.WithDetailContext(resourceConfig.viewParent, func(index int) any {
			if index < 0 || index >= len(items) {
				return nil
			}
			return items[index]
		}),
	)
}

func fetchAIGatewayTLSResources(
	helper cmd.Helper,
	cfg config.Hook,
	reader aiGatewayTLSReader,
	gatewayID, resourceName string,
) ([]any, error) {
	pageSize := common.ResolveRequestPageSize(cfg)
	var after *string
	var result []any
	for {
		items, next, err := reader.List(helper.GetContext(), gatewayID, pageSize, after)
		if err != nil {
			return nil, cmd.PrepareExecutionError("Failed to list "+resourceName, err, helper.GetCmd(),
				cmd.TryConvertErrorToAttrs(err)...)
		}
		result = append(result, items...)
		if next == nil || *next == "" {
			return result, nil
		}
		after = next
	}
}

func aiGatewayTLSResourceRecord(item any) aiGatewayTLSRecord {
	record := aiGatewayTLSRecord{ID: aiGatewayMissingValue, Name: aiGatewayMissingValue, Updated: aiGatewayMissingValue}
	var updated time.Time
	switch value := item.(type) {
	case kkComps.AIGatewayCertificate:
		record.ID, record.Name, updated = value.ID, value.Name, value.UpdatedAt
	case kkComps.AIGatewayCACertificate:
		record.ID, record.Name, updated = value.ID, value.Name, value.UpdatedAt
	case kkComps.AIGatewaySNI:
		record.ID, record.Name, record.DisplayName = value.ID, value.Name, value.DisplayName
		record.Hostname, _ = value.Hostname.(string)
		record.Certificate, updated = value.Certificate, value.UpdatedAt
	}
	if !updated.IsZero() {
		record.Updated = updated.In(time.Local).Format("2006-01-02 15:04:05")
	}
	return record
}

type aiGatewayCertificateReader struct {
	api helpers.AIGatewayCertificatesAPI
}

func (r aiGatewayCertificateReader) List(
	ctx context.Context, gatewayID string, size int64, after *string,
) ([]any, *string, error) {
	response, err := r.api.ListAiGatewayCertificates(ctx, kkOps.ListAiGatewayCertificatesRequest{
		GatewayID: gatewayID, PageSize: &size, PageAfter: after,
	})
	if err != nil || response == nil || response.GetListAIGatewayCertificatesResponse() == nil {
		return nil, nil, err
	}
	body := response.GetListAIGatewayCertificatesResponse()
	items := make([]any, len(body.Data))
	for i := range body.Data {
		items[i] = body.Data[i]
	}
	return items, pageAfterCursor(body.Meta.Page.Next), nil
}

func (r aiGatewayCertificateReader) Get(ctx context.Context, gatewayID, id string) (any, error) {
	response, err := r.api.GetAiGatewayCertificate(ctx, gatewayID, id)
	if err != nil || response == nil || response.GetAIGatewayCertificate() == nil {
		return nil, err
	}
	return *response.GetAIGatewayCertificate(), nil
}

type aiGatewayCACertificateReader struct {
	api helpers.AIGatewayCACertificatesAPI
}

func (r aiGatewayCACertificateReader) List(
	ctx context.Context, gatewayID string, size int64, after *string,
) ([]any, *string, error) {
	response, err := r.api.ListAiGatewayCaCertificates(ctx, kkOps.ListAiGatewayCaCertificatesRequest{
		GatewayID: gatewayID, PageSize: &size, PageAfter: after,
	})
	if err != nil || response == nil || response.GetListAIGatewayCACertificatesResponse() == nil {
		return nil, nil, err
	}
	body := response.GetListAIGatewayCACertificatesResponse()
	items := make([]any, len(body.Data))
	for i := range body.Data {
		items[i] = body.Data[i]
	}
	return items, pageAfterCursor(body.Meta.Page.Next), nil
}

func (r aiGatewayCACertificateReader) Get(ctx context.Context, gatewayID, id string) (any, error) {
	response, err := r.api.GetAiGatewayCaCertificate(ctx, gatewayID, id)
	if err != nil || response == nil || response.GetAIGatewayCACertificate() == nil {
		return nil, err
	}
	return *response.GetAIGatewayCACertificate(), nil
}

type aiGatewaySNIReader struct{ api helpers.AIGatewaySNIsAPI }

func (r aiGatewaySNIReader) List(
	ctx context.Context, gatewayID string, size int64, after *string,
) ([]any, *string, error) {
	response, err := r.api.ListAiGatewaySnis(ctx, kkOps.ListAiGatewaySnisRequest{
		GatewayID: gatewayID, PageSize: &size, PageAfter: after,
	})
	if err != nil || response == nil || response.GetListAIGatewaySNIsResponse() == nil {
		return nil, nil, err
	}
	body := response.GetListAIGatewaySNIsResponse()
	items := make([]any, len(body.Data))
	for i := range body.Data {
		items[i] = body.Data[i]
	}
	return items, pageAfterCursor(body.Meta.Page.Next), nil
}

func (r aiGatewaySNIReader) Get(ctx context.Context, gatewayID, id string) (any, error) {
	response, err := r.api.GetAiGatewaySni(ctx, gatewayID, id)
	if err != nil || response == nil || response.GetAIGatewaySNI() == nil {
		return nil, err
	}
	return *response.GetAIGatewaySNI(), nil
}

func newAIGatewayTLSReader(kind aiGatewayTLSResourceKind, sdk helpers.SDKAPI) (aiGatewayTLSReader, error) {
	switch kind {
	case aiGatewayCertificateKind:
		if api := sdk.GetAIGatewayCertificatesAPI(); api != nil {
			return aiGatewayCertificateReader{api: api}, nil
		}
	case aiGatewayCACertificateKind:
		if api := sdk.GetAIGatewayCACertificatesAPI(); api != nil {
			return aiGatewayCACertificateReader{api: api}, nil
		}
	case aiGatewaySNIKind:
		if api := sdk.GetAIGatewaySNIsAPI(); api != nil {
			return aiGatewaySNIReader{api: api}, nil
		}
	}
	return nil, &cmd.ExecutionError{
		Msg: "AI Gateway TLS resource client is not available",
		Err: fmt.Errorf("AI Gateway TLS resource client not configured"),
	}
}

func pageAfterCursor(next *string) *string {
	cursor := pagination.ExtractPageAfterCursor(next)
	if cursor == "" {
		return nil
	}
	return &cursor
}

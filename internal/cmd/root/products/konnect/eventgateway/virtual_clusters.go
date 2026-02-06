package eventgateway

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
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
	virtualClustersCommandName = "virtual-clusters"
)

type virtualClusterSummaryRecord struct {
	ID               string
	Name             string
	Description      string
	DNSLabel         string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	virtualClustersUse = virtualClustersCommandName

	virtualClustersShort = i18n.T("root.products.konnect.eventgateway.virtualClustersShort",
		"Manage virtual clusters for an Event Gateway")
	virtualClustersLong = normalizers.LongDesc(i18n.T("root.products.konnect.eventgateway.virtualClustersLong",
		`Use the virtual-clusters command to list or retrieve virtual clusters for a specific Event Gateway.`))
	virtualClustersExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.virtualClustersExamples",
			fmt.Sprintf(`
# List virtual clusters for an event gateway by ID
%[1]s get event-gateway virtual-clusters --gateway-id <gateway-id>
# List virtual clusters for an event gateway by name
%[1]s get event-gateway virtual-clusters --gateway-name my-gateway
# List virtual clusters for a backend cluster
%[1]s get event-gateway virtual-clusters --gateway-id <gateway-id> --backend-cluster-id <cluster-id>
# Get a specific virtual cluster by ID (positional argument)
%[1]s get event-gateway virtual-clusters --gateway-id <gateway-id> <cluster-id>
# Get a specific virtual cluster by name (positional argument)
%[1]s get event-gateway virtual-clusters --gateway-id <gateway-id> my-cluster
# Get a specific virtual cluster by ID (flag)
%[1]s get event-gateway virtual-clusters --gateway-id <gateway-id> --virtual-cluster-id <cluster-id>
# Get a specific virtual cluster by name (flag)
%[1]s get event-gateway virtual-clusters --gateway-name my-gateway --virtual-cluster-name my-cluster
`, meta.CLIName)))
)

func newGetEventGatewayVirtualClustersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     virtualClustersUse,
		Short:   virtualClustersShort,
		Long:    virtualClustersLong,
		Example: virtualClustersExample,
		Aliases: []string{"virtual-cluster", "vc", "vcs"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			if err := bindEventGatewayChildFlags(cmd, args); err != nil {
				return err
			}
			if err := bindBackendClusterChildFlags(cmd, args); err != nil {
				return err
			}
			return bindVirtualClusterChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := virtualClustersHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(cmd)
	addBackendClusterChildFlags(cmd)
	addVirtualClusterChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type virtualClustersHandler struct {
	cmd *cobra.Command
}

func (h virtualClustersHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing virtual clusters requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		clusterID, clusterName := getVirtualClusterIdentifiers(cfg)
		if clusterID != "" || clusterName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					virtualClusterIDFlagName,
					virtualClusterNameFlagName,
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

	clusterAPI := sdk.GetEventGatewayVirtualClusterAPI()
	if clusterAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Virtual clusters client is not available",
			Err: fmt.Errorf("virtual clusters client not configured"),
		}
	}

	// Determine if we're getting a single cluster or listing all
	clusterID, clusterName := getVirtualClusterIdentifiers(cfg)
	var clusterIdentifier string

	if len(args) == 1 {
		clusterIdentifier = strings.TrimSpace(args[0])
	} else if clusterID != "" {
		clusterIdentifier = clusterID
	} else if clusterName != "" {
		clusterIdentifier = clusterName
	}

	// Validate mutual exclusivity of cluster ID and name flags
	if clusterID != "" && clusterName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"only one of --%s or --%s can be provided",
				virtualClusterIDFlagName,
				virtualClusterNameFlagName,
			),
		}
	}

	if clusterIdentifier != "" {
		return h.getSingleCluster(
			helper,
			clusterAPI,
			gatewayID,
			clusterIdentifier,
			outType,
			printer,
			cfg,
		)
	}

	// Check if filtering by backend cluster
	backendClusterID, _ := getBackendClusterIdentifiers(cfg)
	if backendClusterID != "" {
		return h.listClustersByBackendCluster(helper, clusterAPI, gatewayID, backendClusterID, outType, printer, cfg)
	}

	return h.listClusters(helper, clusterAPI, gatewayID, outType, printer, cfg)
}

func (h virtualClustersHandler) listClusters(
	helper cmd.Helper,
	clusterAPI helpers.EventGatewayVirtualClusterAPI,
	gatewayID string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	clusters, err := fetchVirtualClusters(helper, clusterAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]virtualClusterSummaryRecord, 0, len(clusters))
	for _, cluster := range clusters {
		records = append(records, virtualClusterToRecord(cluster))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.DNSLabel})
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		clusters,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME", "DNS LABEL"}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h virtualClustersHandler) listClustersByBackendCluster(
	helper cmd.Helper,
	clusterAPI helpers.EventGatewayVirtualClusterAPI,
	gatewayID string,
	backendClusterID string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	clusters, err := fetchVirtualClusters(helper, clusterAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	// Filter by backend cluster ID
	var filtered []kkComps.VirtualCluster
	for _, cluster := range clusters {
		if cluster.Destination.ID == backendClusterID {
			filtered = append(filtered, cluster)
		}
	}

	records := make([]virtualClusterSummaryRecord, 0, len(filtered))
	for _, cluster := range filtered {
		records = append(records, virtualClusterToRecord(cluster))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.DNSLabel})
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		filtered,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME", "DNS LABEL"}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h virtualClustersHandler) getSingleCluster(
	helper cmd.Helper,
	clusterAPI helpers.EventGatewayVirtualClusterAPI,
	gatewayID string,
	identifier string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	clusterID := identifier
	if !util.IsValidUUID(identifier) {
		clusters, err := fetchVirtualClusters(helper, clusterAPI, gatewayID, cfg)
		if err != nil {
			return err
		}
		match := findVirtualClusterByName(clusters, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("virtual cluster %q not found", identifier),
			}
		}
		if match.ID != "" {
			clusterID = match.ID
		} else {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("virtual cluster %q does not have an ID", identifier),
			}
		}
	}

	res, err := clusterAPI.FetchEventGatewayVirtualCluster(helper.GetContext(), gatewayID, clusterID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get virtual cluster", err, helper.GetCmd(), attrs...)
	}

	cluster := res.GetVirtualCluster()
	if cluster == nil {
		return &cmd.ExecutionError{
			Msg: "Virtual cluster response was empty",
			Err: fmt.Errorf("no cluster returned for id %s", clusterID),
		}
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		virtualClusterToRecord(*cluster),
		cluster,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchVirtualClusters(
	helper cmd.Helper,
	clusterAPI helpers.EventGatewayVirtualClusterAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.VirtualCluster, error) {
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.VirtualCluster
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayVirtualClustersRequest{
			GatewayID: gatewayID,
			PageSize:  kk.Int64(requestPageSize),
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := clusterAPI.ListEventGatewayVirtualClusters(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list virtual clusters", err, helper.GetCmd(), attrs...)
		}

		if res.GetListVirtualClustersResponse() == nil {
			break
		}

		data := res.GetListVirtualClustersResponse().Data
		allData = append(allData, data...)

		if res.GetListVirtualClustersResponse().Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.GetListVirtualClustersResponse().Meta.Page.Next)
		if err != nil {
			return nil, cmd.PrepareExecutionError(
				"Failed to list virtual clusters: invalid cursor",
				err,
				helper.GetCmd(),
			)
		}

		values := u.Query()
		pageAfter = kk.String(values.Get("page[after]"))
	}

	return allData, nil
}

func findVirtualClusterByName(clusters []kkComps.VirtualCluster, identifier string) *kkComps.VirtualCluster {
	lowered := strings.ToLower(identifier)
	for _, cluster := range clusters {
		if cluster.ID != "" && strings.ToLower(cluster.ID) == lowered {
			clusterCopy := cluster
			return &clusterCopy
		}
		if cluster.Name != "" && strings.ToLower(cluster.Name) == lowered {
			clusterCopy := cluster
			return &clusterCopy
		}
	}
	return nil
}

func virtualClusterToRecord(cluster kkComps.VirtualCluster) virtualClusterSummaryRecord {
	id := cluster.ID
	if id != "" {
		id = util.AbbreviateUUID(id)
	} else {
		id = valueNA
	}

	name := cluster.Name
	if name == "" {
		name = valueNA
	}

	description := valueNA
	if cluster.Description != nil && *cluster.Description != "" {
		description = *cluster.Description
	}

	dnsLabel := cluster.DNSLabel
	if dnsLabel == "" {
		dnsLabel = valueNA
	}

	createdAt := cluster.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	updatedAt := cluster.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return virtualClusterSummaryRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		DNSLabel:         dnsLabel,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

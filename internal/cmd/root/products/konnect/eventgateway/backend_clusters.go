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
	backendClustersCommandName = "backend-clusters"
)

type backendClusterSummaryRecord struct {
	ID               string
	Name             string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
}

var (
	backendClustersUse = backendClustersCommandName

	backendClustersShort = i18n.T("root.products.konnect.eventgateway.backendClustersShort",
		"Manage backend clusters for an Event Gateway")
	backendClustersLong = normalizers.LongDesc(i18n.T("root.products.konnect.eventgateway.backendClustersLong",
		`Use the backend-clusters command to list or retrieve backend clusters for a specific Event Gateway.`))
	backendClustersExample = normalizers.Examples(
		i18n.T("root.products.konnect.eventgateway.backendClustersExamples",
			fmt.Sprintf(`
# List backend clusters for an event gateway by ID
%[1]s get event-gateway backend-clusters --gateway-id <gateway-id>
# List backend clusters for an event gateway by name
%[1]s get event-gateway backend-clusters --gateway-name my-gateway
# Get a specific backend cluster by ID (positional argument)
%[1]s get event-gateway backend-clusters --gateway-id <gateway-id> <cluster-id>
# Get a specific backend cluster by name (positional argument)
%[1]s get event-gateway backend-clusters --gateway-id <gateway-id> my-cluster
# Get a specific backend cluster by ID (flag)
%[1]s get event-gateway backend-clusters --gateway-id <gateway-id> --backend-cluster-id <cluster-id>
# Get a specific backend cluster by name (flag)
%[1]s get event-gateway backend-clusters --gateway-name my-gateway --backend-cluster-name my-cluster
`, meta.CLIName)))
)

func newGetEventGatewayBackendClustersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     backendClustersUse,
		Short:   backendClustersShort,
		Long:    backendClustersLong,
		Example: backendClustersExample,
		Aliases: []string{"backend-cluster", "bc", "bcs"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			if err := bindEventGatewayChildFlags(cmd, args); err != nil {
				return err
			}
			return bindBackendClusterChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := backendClustersHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addEventGatewayChildFlags(cmd)
	addBackendClusterChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type backendClustersHandler struct {
	cmd *cobra.Command
}

func (h backendClustersHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing backend clusters requires 0 or 1 arguments (ID or name)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check if positional arg and flags are both provided
	if len(args) == 1 {
		clusterID, clusterName := getBackendClusterIdentifiers(cfg)
		if clusterID != "" || clusterName != "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf(
					"cannot specify both positional argument and --%s or --%s flags",
					backendClusterIDFlagName,
					backendClusterNameFlagName,
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

	clusterAPI := sdk.GetEventGatewayBackendClusterAPI()
	if clusterAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Backend clusters client is not available",
			Err: fmt.Errorf("backend clusters client not configured"),
		}
	}

	// Determine if we're getting a single cluster or listing all
	clusterID, clusterName := getBackendClusterIdentifiers(cfg)
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
				backendClusterIDFlagName,
				backendClusterNameFlagName,
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

	return h.listClusters(helper, clusterAPI, gatewayID, outType, printer, cfg)
}

func (h backendClustersHandler) listClusters(
	helper cmd.Helper,
	clusterAPI helpers.EventGatewayBackendClusterAPI,
	gatewayID string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	clusters, err := fetchBackendClusters(helper, clusterAPI, gatewayID, cfg)
	if err != nil {
		return err
	}

	records := make([]backendClusterSummaryRecord, 0, len(clusters))
	for _, cluster := range clusters {
		records = append(records, backendClusterToRecord(cluster))
	}

	tableRows := make([]table.Row, 0, len(records))
	for _, record := range records {
		tableRows = append(tableRows, table.Row{record.ID, record.Name})
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		clusters,
		"",
		tableview.WithCustomTable([]string{"ID", "NAME"}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func (h backendClustersHandler) getSingleCluster(
	helper cmd.Helper,
	clusterAPI helpers.EventGatewayBackendClusterAPI,
	gatewayID string,
	identifier string, outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	cfg config.Hook,
) error {
	clusterID := identifier
	if !util.IsValidUUID(identifier) {
		clusters, err := fetchBackendClusters(helper, clusterAPI, gatewayID, cfg)
		if err != nil {
			return err
		}
		match := findClusterByName(clusters, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("backend cluster %q not found", identifier),
			}
		}
		if match.ID != "" {
			clusterID = match.ID
		} else {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("backend cluster %q does not have an ID", identifier),
			}
		}
	}

	res, err := clusterAPI.FetchEventGatewayBackendCluster(helper.GetContext(), gatewayID, clusterID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return cmd.PrepareExecutionError("Failed to get backend cluster", err, helper.GetCmd(), attrs...)
	}

	cluster := res.GetBackendCluster()
	if cluster == nil {
		return &cmd.ExecutionError{
			Msg: "Backend cluster response was empty",
			Err: fmt.Errorf("no cluster returned for id %s", clusterID),
		}
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		backendClusterToRecord(*cluster),
		cluster,
		"",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func fetchBackendClusters(
	helper cmd.Helper,
	clusterAPI helpers.EventGatewayBackendClusterAPI,
	gatewayID string,
	cfg config.Hook,
) ([]kkComps.BackendCluster, error) {
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.BackendCluster
	var pageAfter *string

	for {
		req := kkOps.ListEventGatewayBackendClustersRequest{
			GatewayID: gatewayID,
			PageSize:  kk.Int64(requestPageSize),
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := clusterAPI.ListEventGatewayBackendClusters(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list backend clusters", err, helper.GetCmd(), attrs...)
		}

		if res.GetListBackendClustersResponse() == nil {
			break
		}

		data := res.GetListBackendClustersResponse().Data
		allData = append(allData, data...)

		if res.GetListBackendClustersResponse().Meta.Page.Next == nil {
			break
		}

		u, err := url.Parse(*res.GetListBackendClustersResponse().Meta.Page.Next)
		if err != nil {
			return nil, cmd.PrepareExecutionError(
				"Failed to list backend clusters: invalid cursor",
				err,
				helper.GetCmd(),
			)
		}

		values := u.Query()
		pageAfter = kk.String(values.Get("page[after]"))
	}

	return allData, nil
}

func findClusterByName(clusters []kkComps.BackendCluster, identifier string) *kkComps.BackendCluster {
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

func backendClusterToRecord(cluster kkComps.BackendCluster) backendClusterSummaryRecord {
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

	createdAt := cluster.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	updatedAt := cluster.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")

	return backendClusterSummaryRecord{
		ID:               id,
		Name:             name,
		Description:      description,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

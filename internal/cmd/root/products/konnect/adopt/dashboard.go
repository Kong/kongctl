package adopt

import (
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func NewDashboardCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = "dashboard <dashboard-id|dashboard-name>"
	cmd.Short = "Adopt an existing Konnect dashboard into namespace management"
	cmd.Long = "Apply the KONGCTL-namespace label to an existing Konnect dashboard " +
		"that is not currently managed by kongctl."
	cmd.Args = func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one dashboard identifier (name or ID) is required")
		}
		if trimmed := strings.TrimSpace(args[0]); trimmed == "" {
			return fmt.Errorf("dashboard identifier cannot be empty")
		}
		return nil
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	if parentPreRun != nil {
		cmd.PreRunE = parentPreRun
	}

	cmd.RunE = func(cobraCmd *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(cobraCmd, args)

		adoptFlags, err := adoptCommon.ReadAdoptFlags(cobraCmd)
		if err != nil {
			return err
		}

		outType, err := helper.GetOutputFormat()
		if err != nil {
			return err
		}

		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}

		logger, err := helper.GetLogger()
		if err != nil {
			return err
		}

		sdk, err := helper.GetKonnectSDK(cfg, logger)
		if err != nil {
			return err
		}

		result, err := adoptDashboard(
			helper,
			sdk.GetDashboardsAPI(),
			cfg,
			adoptFlags.Namespace,
			adoptFlags.OverwriteNamespace,
			strings.TrimSpace(args[0]),
		)
		if err != nil {
			return err
		}

		streams := helper.GetStreams()
		if outType == cmdCommon.TEXT {
			name := result.Name
			if name == "" {
				name = result.ID
			}
			fmt.Fprintf(streams.Out, "Adopted dashboard %q (%s) into namespace %q\n", name, result.ID, result.Namespace)
			return nil
		}

		printer, err := cli.Format(outType.String(), streams.Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(result)
		return nil
	}

	return cmd, nil
}

func adoptDashboard(
	helper cmdpkg.Helper,
	api helpers.DashboardsAPI,
	cfg config.Hook,
	namespace string,
	overwriteNamespace bool,
	identifier string,
) (*adoptCommon.AdoptResult, error) {
	dashboard, err := resolveDashboard(helper, api, cfg, identifier)
	if err != nil {
		return nil, err
	}

	if existing := dashboard.Labels; existing != nil && !overwriteNamespace {
		if currentNamespace, ok := existing[labels.NamespaceKey]; ok && currentNamespace != "" {
			return nil, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("dashboard %q already has namespace label %q", dashboard.Name, currentNamespace),
			}
		}
	}

	id := getDashboardID(dashboard)
	if id == "" {
		return nil, fmt.Errorf("dashboard %q is missing an ID", dashboard.Name)
	}

	updateReq := kkComps.DashboardUpdateRequest{
		Name:       dashboard.Name,
		Definition: dashboard.Definition,
		Labels:     adoptCommon.StringLabelMap(dashboard.Labels, namespace),
	}

	ctx := adoptCommon.EnsureContext(helper.GetContext())
	resp, err := api.DashboardsUpdate(ctx, id, updateReq)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to update dashboard", err, helper.GetCmd(), attrs...)
	}

	updated := resp.GetDashboardResponse()
	if updated == nil {
		return nil, fmt.Errorf("update dashboard response missing dashboard data")
	}

	ns := namespace
	if updated.Labels != nil {
		if v, ok := updated.Labels[labels.NamespaceKey]; ok && v != "" {
			ns = v
		}
	}

	return &adoptCommon.AdoptResult{
		ResourceType: "dashboard",
		ID:           getDashboardID(updated),
		Name:         updated.Name,
		Namespace:    ns,
	}, nil
}

func resolveDashboard(
	helper cmdpkg.Helper,
	api helpers.DashboardsAPI,
	cfg config.Hook,
	identifier string,
) (*kkComps.DashboardResponse, error) {
	if api == nil {
		return nil, fmt.Errorf("dashboards API client is not configured")
	}

	ctx := adoptCommon.EnsureContext(helper.GetContext())

	if util.IsValidUUID(identifier) {
		resp, err := api.DashboardsGet(ctx, identifier)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to retrieve dashboard", err, helper.GetCmd(), attrs...)
		}
		dashboard := resp.GetDashboardResponse()
		if dashboard == nil {
			return nil, fmt.Errorf("dashboard %s not found", identifier)
		}
		return dashboard, nil
	}

	pageSize := cfg.GetInt(common.RequestPageSizeConfigPath)
	if pageSize < 1 {
		pageSize = common.DefaultRequestPageSize
	}

	var matches []kkComps.DashboardResponse
	var pageNumber int64 = 1
	for {
		pageSize64 := int64(pageSize)
		req := kkOps.DashboardsListRequest{
			PageSize:   &pageSize64,
			PageNumber: &pageNumber,
			Filter: &kkComps.DashboardFilterParameters{
				Name: &kkComps.StringFieldFilter{Eq: &identifier},
			},
		}

		resp, err := api.DashboardsList(ctx, req)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to list dashboards", err, helper.GetCmd(), attrs...)
		}

		if resp.Object == nil || len(resp.Object.Data) == 0 {
			break
		}

		for _, dashboard := range resp.Object.Data {
			if dashboard.Name == identifier {
				matches = append(matches, dashboard)
			}
		}

		if len(resp.Object.Data) < pageSize {
			break
		}
		pageNumber++
	}

	switch len(matches) {
	case 0:
		return nil, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("dashboard %q not found", identifier),
		}
	case 1:
		return &matches[0], nil
	default:
		return nil, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("dashboard name %q matches %d dashboards; use the dashboard ID", identifier, len(matches)),
		}
	}
}

func getDashboardID(dashboard *kkComps.DashboardResponse) string {
	if dashboard == nil || dashboard.ID == nil {
		return ""
	}
	return *dashboard.ID
}

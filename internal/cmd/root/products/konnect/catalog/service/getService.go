package service

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/table"
	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	getServiceShort = i18n.T("root.products.konnect.catalog.getServiceShort",
		"List or get Konnect catalog services")
	getServiceLong = i18n.T("root.products.konnect.catalog.getServiceLong",
		`Use the get verb with the catalog service command to query Konnect Service Catalog services.`)
	getServiceExample = normalizers.Examples(i18n.T("root.products.konnect.catalog.getServiceExamples",
		`  # List catalog services
  kongctl get catalog services
  # Get a specific catalog service by ID
  kongctl get catalog service 123e4567-e89b-12d3-a456-426614174000
  # Get a specific catalog service by name
  kongctl get catalog service payments`))
)

type getServiceCmd struct {
	*cobra.Command
}

func newGetServiceCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getServiceCmd {
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [name|id]", baseCmd.Use),
		Aliases: baseCmd.Aliases,
		Short:   getServiceShort,
		Long:    getServiceLong,
		Example: getServiceExample,
		Args:    cobra.MaximumNArgs(1),
	}

	addParentFlags(verb, cmd)
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if err := parentPreRun(cmd, args); err != nil {
			return err
		}
		return bindCatalogServiceFlags(cmd, args)
	}

	c := &getServiceCmd{Command: cmd}
	cmd.RunE = c.run

	return c
}

func (c *getServiceCmd) validate(helper cmd.Helper) error {
	config, err := helper.GetConfig()
	if err != nil {
		return err
	}

	pageSize := config.GetInt(common.RequestPageSizeConfigPath)
	if pageSize < 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("%s must be greater than 0", common.RequestPageSizeFlagName),
		}
	}

	return nil
}

func (c *getServiceCmd) run(_ *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c.Command, args)

	if err := c.validate(helper); err != nil {
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

	api := sdk.GetCatalogServicesAPI()
	if api == nil {
		return fmt.Errorf("catalog services API not configured")
	}

	pageSize := cfg.GetInt(common.RequestPageSizeConfigPath)
	outFormat, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	ctx := c.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	streams := helper.GetStreams()

	if len(args) == 0 {
		services, listErr := listAllCatalogServices(ctx, api, int64(pageSize))
		if listErr != nil {
			return cmd.PrepareExecutionError("Failed to list catalog services", listErr, helper.GetCmd())
		}
		return renderCatalogServices(helper, outFormat, services)
	}

	service, fetchErr := fetchCatalogService(ctx, api, args[0], int64(pageSize))
	if fetchErr != nil {
		return cmd.PrepareExecutionError("Failed to get catalog service", fetchErr, helper.GetCmd())
	}

	if service == nil {
		fmt.Fprintln(streams.Out, "No catalog service found")
		return nil
	}

	return renderCatalogService(helper, outFormat, *service)
}

func listAllCatalogServices(
	ctx context.Context,
	api helpers.CatalogServicesAPI,
	pageSize int64,
) ([]kkComps.CatalogService, error) {
	result := make([]kkComps.CatalogService, 0)

	pageNumber := int64(1)
	for {
		req := kkOps.ListCatalogServicesRequest{
			PageSize:   new(pageSize),
			PageNumber: new(pageNumber),
		}

		resp, err := api.ListCatalogServices(ctx, req)
		if err != nil {
			return nil, err
		}

		if resp.ListCatalogServicesResponse == nil {
			break
		}

		data := resp.ListCatalogServicesResponse.GetData()
		result = append(result, data...)

		total := resp.ListCatalogServicesResponse.Meta.Page.Total
		if float64(pageNumber*pageSize) >= total || len(data) == 0 {
			break
		}

		pageNumber++
	}

	return result, nil
}

func fetchCatalogService(
	ctx context.Context,
	api helpers.CatalogServicesAPI,
	identifier string,
	pageSize int64,
) (*kkComps.CatalogService, error) {
	// Try ID lookup first
	resp, err := api.FetchCatalogService(ctx, identifier)
	if err == nil && resp != nil && resp.CatalogService != nil {
		return resp.CatalogService, nil
	}

	// Fallback to name lookup using filter
	filter := kkComps.CatalogServiceFilterParameters{
		Name: &kkComps.StringFieldFilter{
			Eq: new(identifier),
		},
	}

	req := kkOps.ListCatalogServicesRequest{
		Filter:     &filter,
		PageSize:   new(pageSize),
		PageNumber: kk.Int64(1),
	}

	listResp, listErr := api.ListCatalogServices(ctx, req)
	if listErr != nil {
		return nil, listErr
	}

	if listResp.ListCatalogServicesResponse != nil {
		data := listResp.ListCatalogServicesResponse.GetData()
		if len(data) > 0 {
			return &data[0], nil
		}
	}

	return nil, nil
}

type catalogServiceDisplay struct {
	Name        string
	DisplayName string
	ID          string
}

func renderCatalogServices(helper cmd.Helper, format cmdCommon.OutputFormat, services []kkComps.CatalogService) error {
	display := make([]catalogServiceDisplay, len(services))
	rows := make([]table.Row, len(services))
	for i, service := range services {
		display[i] = catalogServiceDisplay{
			Name:        service.Name,
			DisplayName: service.DisplayName,
			ID:          service.ID,
		}
		rows[i] = table.Row{service.Name, service.DisplayName, service.ID}
	}
	return renderCatalogServiceTable(helper, format, display, services, rows)
}

func renderCatalogService(helper cmd.Helper, format cmdCommon.OutputFormat, service kkComps.CatalogService) error {
	display := catalogServiceDisplay{Name: service.Name, DisplayName: service.DisplayName, ID: service.ID}
	rows := []table.Row{{service.Name, service.DisplayName, service.ID}}
	return renderCatalogServiceTable(helper, format, display, service, rows)
}

func renderCatalogServiceTable(
	helper cmd.Helper,
	format cmdCommon.OutputFormat,
	display any,
	raw any,
	rows []table.Row,
) error {
	printer, err := cli.Format(format.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	return tableview.RenderForFormat(
		helper,
		false,
		format,
		printer,
		helper.GetStreams(),
		display,
		raw,
		"",
		tableview.WithCustomTable([]string{"NAME", "DISPLAY NAME", "ID"}, rows),
	)
}

// bindCatalogServiceFlags binds Konnect-specific flags to configuration
func bindCatalogServiceFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if f := c.Flags().Lookup(common.BaseURLFlagName); f != nil {
		if err := cfg.BindFlag(common.BaseURLConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.RegionFlagName); f != nil {
		if err := cfg.BindFlag(common.RegionConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.PATFlagName); f != nil {
		if err := cfg.BindFlag(common.PATConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.RequestPageSizeFlagName); f != nil {
		if err := cfg.BindFlag(common.RequestPageSizeConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
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
		return renderCatalogServices(outFormat, services, streams.Out)
	}

	service, fetchErr := fetchCatalogService(ctx, api, args[0], int64(pageSize))
	if fetchErr != nil {
		return cmd.PrepareExecutionError("Failed to get catalog service", fetchErr, helper.GetCmd())
	}

	if service == nil {
		fmt.Fprintln(streams.Out, "No catalog service found")
		return nil
	}

	return renderCatalogService(outFormat, *service, streams.Out)
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
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
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
			Eq: kk.String(identifier),
		},
	}

	req := kkOps.ListCatalogServicesRequest{
		Filter:     &filter,
		PageSize:   kk.Int64(pageSize),
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

func renderCatalogServices(format cmdCommon.OutputFormat, services []kkComps.CatalogService, out io.Writer) error {
	switch format {
	case cmdCommon.JSON:
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(services)
	case cmdCommon.YAML:
		b, err := yaml.Marshal(services)
		if err != nil {
			return err
		}
		_, err = out.Write(b)
		return err
	case cmdCommon.TEXT:
		if len(services) == 0 {
			fmt.Fprintln(out, "No catalog services found")
			return nil
		}

		fmt.Fprintf(out, "%-36s  %-30s  %s\n", "ID", "NAME", "DISPLAY NAME")
		for _, svc := range services {
			fmt.Fprintf(out, "%-36s  %-30s  %s\n",
				util.AbbreviateUUID(svc.ID), svc.Name, svc.DisplayName)
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format")
	}
}

func renderCatalogService(format cmdCommon.OutputFormat, svc kkComps.CatalogService, out io.Writer) error {
	switch format {
	case cmdCommon.JSON:
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(svc)
	case cmdCommon.YAML:
		b, err := yaml.Marshal(svc)
		if err != nil {
			return err
		}
		_, err = out.Write(b)
		return err
	case cmdCommon.TEXT:
		fmt.Fprintf(out, "Name: %s\n", svc.Name)
		fmt.Fprintf(out, "Display Name: %s\n", svc.DisplayName)
		fmt.Fprintf(out, "ID: %s\n", svc.ID)
		if svc.Description != nil {
			fmt.Fprintf(out, "Description: %s\n", *svc.Description)
		} else {
			fmt.Fprintln(out, "Description: ")
		}
		if len(svc.Labels) > 0 {
			fmt.Fprintf(out, "Labels: %v\n", svc.Labels)
		} else {
			fmt.Fprintln(out, "Labels: ")
		}
		if svc.CustomFields != nil {
			fmt.Fprintf(out, "Custom Fields: %v\n", svc.CustomFields)
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format")
	}
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

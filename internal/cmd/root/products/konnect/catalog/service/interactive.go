package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
	"github.com/kong/kongctl/internal/util"
)

// BuildListView returns the interactive catalog services view configuration used by the Konnect
// navigator when the user selects the "catalog-services" resource.
func BuildListView(helper cmd.Helper) (tableview.ChildView, error) {
	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	api := sdk.GetCatalogServicesAPI()
	if api == nil {
		return tableview.ChildView{}, fmt.Errorf("catalog services API not configured")
	}

	pageSize := cfg.GetInt(common.RequestPageSizeConfigPath)
	services, err := listAllCatalogServices(context.Background(), api, int64(pageSize))
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildCatalogServiceChildView(services), nil
}

func buildCatalogServiceChildView(services []kkComps.CatalogService) tableview.ChildView {
	views := make([]catalogServiceView, len(services))
	tableRows := make([]table.Row, 0, len(services))
	for i := range services {
		view := catalogServiceToDisplayView(&services[i])
		views[i] = view
		record := view.DisplayRecord
		tableRows = append(tableRows, table.Row{record.ID, record.Name, record.DisplayName})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(views) {
			return ""
		}
		return catalogServiceDetailView(views[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME", "DISPLAY NAME"},
		Rows:           tableRows,
		DetailRenderer: detailFn,
		Title:          "Catalog Services",
		ParentType:     "catalog_service",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(views) {
				return nil
			}
			return views[index]
		},
	}
}

type catalogServiceView struct {
	DisplayRecord catalogServiceDisplayRecord
	Labels        map[string]string
	CustomFields  []customFieldEntry
	RawCustom     map[string]any
}

type catalogServiceDisplayRecord struct {
	ID          string
	Name        string
	DisplayName string
	Description string
}

type customFieldEntry struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func catalogServiceToDisplayView(svc *kkComps.CatalogService) catalogServiceView {
	id := "n/a"
	if svc.GetID() != "" {
		id = util.AbbreviateUUID(svc.GetID())
	}

	name := svc.GetName()
	if strings.TrimSpace(name) == "" {
		name = "n/a"
	}

	displayName := svc.GetDisplayName()
	if strings.TrimSpace(displayName) == "" {
		displayName = "n/a"
	}

	description := "n/a"
	if svc.GetDescription() != nil && strings.TrimSpace(*svc.GetDescription()) != "" {
		description = *svc.GetDescription()
	}

	rawCustom := map[string]any{}
	if cf, ok := svc.CustomFields.(map[string]any); ok {
		rawCustom = cf
	}

	entries := make([]customFieldEntry, 0, len(rawCustom))
	for k, v := range rawCustom {
		entries = append(entries, customFieldEntry{
			Key:   k,
			Value: v,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	return catalogServiceView{
		DisplayRecord: catalogServiceDisplayRecord{
			ID:          id,
			Name:        name,
			DisplayName: displayName,
			Description: description,
		},
		Labels:       svc.Labels,
		CustomFields: entries,
		RawCustom:    rawCustom,
	}
}

func catalogServiceDetailView(view catalogServiceView) string {
	record := view.DisplayRecord

	var sb strings.Builder
	fmt.Fprintf(&sb, "Name         : %s\n", record.Name)
	fmt.Fprintf(&sb, "Display Name : %s\n", record.DisplayName)
	fmt.Fprintf(&sb, "ID           : %s\n", record.ID)
	fmt.Fprintf(&sb, "Description  : %s\n", record.Description)

	if len(view.Labels) > 0 {
		if data, err := json.MarshalIndent(view.Labels, "", "  "); err == nil {
			fmt.Fprintf(&sb, "Labels       : %s\n", string(data))
		}
	}

	if view.RawCustom != nil {
		if data, err := json.MarshalIndent(view.RawCustom, "", "  "); err == nil {
			fmt.Fprintf(&sb, "Custom Fields: %s\n", string(data))
		}
	}

	return sb.String()
}

func init() {
	navigator.RegisterResource(
		"catalog-services",
		[]string{"catalog-services", "catalog_service", "catalog_services", "catalog"},
		BuildListView,
	)
}

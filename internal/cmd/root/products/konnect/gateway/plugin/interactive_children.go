package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/charmbracelet/bubbles/table"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	kkCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	gatewaycommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
)

func init() {
	tableview.RegisterChildLoader("control-plane", "plugins", loadControlPlanePlugins)
	tableview.RegisterChildLoader("gateway-service", "plugins", loadServicePlugins)
	tableview.RegisterChildLoader("gateway-route", "plugins", loadRoutePlugins)
	tableview.RegisterChildLoader("gateway-consumer", "plugins", loadConsumerPlugins)
	tableview.RegisterChildLoader("consumer-group", "plugins", loadConsumerGroupPlugins)
}

func loadControlPlanePlugins(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	controlPlaneID, err := controlPlaneIDFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return tableview.ChildView{}, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return tableview.ChildView{}, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return tableview.ChildView{}, err
	}

	konnectSDK, ok := sdk.(*helpers.KonnectSDK)
	if !ok || konnectSDK.SDK == nil {
		return tableview.ChildView{}, fmt.Errorf("konnect SDK is not available")
	}

	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))
	plugins, err := helpers.GetAllGatewayPlugins(helper.GetContext(), requestPageSize, controlPlaneID, konnectSDK.SDK)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return tableview.ChildView{}, cmd.PrepareExecutionError(
			"Failed to list Gateway Plugins",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	return buildPluginChildView(plugins), nil
}

func loadServicePlugins(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	ctx, err := gatewaycommon.ServiceContextFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	plugins, err := fetchPluginsForScope(helper, func(
		pageSize int64,
		sdk *helpers.KonnectSDK,
	) ([]kkComps.Plugin, error) {
		return helpers.GetAllGatewayServicePlugins(
			helper.GetContext(),
			pageSize,
			ctx.ControlPlaneID,
			ctx.ServiceID,
			sdk.SDK,
		)
	})
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildPluginChildView(plugins), nil
}

func loadRoutePlugins(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	ctx, err := gatewaycommon.RouteContextFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	plugins, err := fetchPluginsForScope(helper, func(
		pageSize int64,
		sdk *helpers.KonnectSDK,
	) ([]kkComps.Plugin, error) {
		return helpers.GetAllGatewayRoutePlugins(
			helper.GetContext(),
			pageSize,
			ctx.ControlPlaneID,
			ctx.RouteID,
			sdk.SDK,
		)
	})
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildPluginChildView(plugins), nil
}

func loadConsumerPlugins(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	ctx, err := gatewaycommon.ConsumerContextFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	plugins, err := fetchPluginsForScope(helper, func(
		pageSize int64,
		sdk *helpers.KonnectSDK,
	) ([]kkComps.Plugin, error) {
		return helpers.GetAllGatewayConsumerPlugins(
			helper.GetContext(),
			pageSize,
			ctx.ControlPlaneID,
			ctx.ConsumerID,
			sdk.SDK,
		)
	})
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildPluginChildView(plugins), nil
}

func loadConsumerGroupPlugins(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	ctx, err := gatewaycommon.ConsumerGroupContextFromParent(parent)
	if err != nil {
		return tableview.ChildView{}, err
	}

	plugins, err := fetchPluginsForScope(helper, func(
		pageSize int64,
		sdk *helpers.KonnectSDK,
	) ([]kkComps.Plugin, error) {
		return helpers.GetAllGatewayConsumerGroupPlugins(
			helper.GetContext(),
			pageSize,
			ctx.ControlPlaneID,
			ctx.ConsumerGroupID,
			sdk.SDK,
		)
	})
	if err != nil {
		return tableview.ChildView{}, err
	}

	return buildPluginChildView(plugins), nil
}

type pluginDisplayRecord struct {
	ID   string
	Name string
}

func pluginToDisplayRecord(p *kkComps.Plugin) pluginDisplayRecord {
	const missing = "n/a"

	id := missing
	if p.GetID() != nil && *p.GetID() != "" {
		id = util.AbbreviateUUID(*p.GetID())
	}

	name := strings.TrimSpace(p.GetName())
	if name == "" {
		name = missing
	}

	return pluginDisplayRecord{
		ID:   id,
		Name: name,
	}
}

func pluginDetailView(p *kkComps.Plugin) string {
	if p == nil {
		return ""
	}

	const missing = "n/a"

	id := missing
	if p.GetID() != nil && *p.GetID() != "" {
		id = *p.GetID()
	}

	name := strings.TrimSpace(p.GetName())
	if name == "" {
		name = missing
	}

	instanceName := missing
	if p.GetInstanceName() != nil && strings.TrimSpace(*p.GetInstanceName()) != "" {
		instanceName = strings.TrimSpace(*p.GetInstanceName())
	}

	enabled := missing
	if p.GetEnabled() != nil {
		enabled = fmt.Sprintf("%t", *p.GetEnabled())
	}

	protocols := missing
	if values := p.GetProtocols(); len(values) > 0 {
		parts := make([]string, 0, len(values))
		for _, protocol := range values {
			parts = append(parts, string(protocol))
		}
		protocols = strings.Join(parts, ", ")
	}

	tagsLine := missing
	if tags := p.GetTags(); len(tags) > 0 {
		tagsLine = strings.Join(tags, ", ")
	}

	configLine := missing
	if cfg := p.GetConfig(); len(cfg) > 0 {
		if data, err := json.Marshal(cfg); err == nil {
			configLine = string(data)
		}
	}

	serviceID := nestedID(p.GetService())
	routeID := nestedID(p.GetRoute())
	consumerID := nestedID(p.GetConsumer())
	consumerGroupID := nestedID(p.GetConsumerGroup())

	created := missing
	if ts := p.GetCreatedAt(); ts != nil {
		created = time.Unix(0, *ts*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	updated := missing
	if ts := p.GetUpdatedAt(); ts != nil {
		updated = time.Unix(0, *ts*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "instance_name: %s\n", instanceName)
	fmt.Fprintf(&b, "enabled: %s\n", enabled)
	fmt.Fprintf(&b, "protocols: %s\n", protocols)
	fmt.Fprintf(&b, "tags: %s\n", tagsLine)
	fmt.Fprintf(&b, "service_id: %s\n", serviceID)
	fmt.Fprintf(&b, "route_id: %s\n", routeID)
	fmt.Fprintf(&b, "consumer_id: %s\n", consumerID)
	fmt.Fprintf(&b, "consumer_group_id: %s\n", consumerGroupID)
	fmt.Fprintf(&b, "config: %s\n", configLine)
	fmt.Fprintf(&b, "created_at: %s\n", created)
	fmt.Fprintf(&b, "updated_at: %s\n", updated)

	return b.String()
}

func nestedID(value interface{ GetID() *string }) string {
	const missing = "n/a"
	if value == nil || value.GetID() == nil || strings.TrimSpace(*value.GetID()) == "" {
		return missing
	}
	return strings.TrimSpace(*value.GetID())
}

func controlPlaneIDFromParent(parent any) (string, error) {
	if parent == nil {
		return "", fmt.Errorf("control plane parent is nil")
	}

	switch cp := parent.(type) {
	case *kkComps.ControlPlane:
		id := strings.TrimSpace(cp.ID)
		if id == "" {
			return "", fmt.Errorf("control plane identifier is missing")
		}
		return id, nil
	case kkComps.ControlPlane:
		id := strings.TrimSpace(cp.ID)
		if id == "" {
			return "", fmt.Errorf("control plane identifier is missing")
		}
		return id, nil
	default:
		return "", fmt.Errorf("unexpected parent type %T", parent)
	}
}

func buildPluginChildView(plugins []kkComps.Plugin) tableview.ChildView {
	rows := make([]table.Row, 0, len(plugins))
	for i := range plugins {
		record := pluginToDisplayRecord(&plugins[i])
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(plugins) {
			return ""
		}
		return pluginDetailView(&plugins[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Plugins",
		ParentType:     "gateway-plugin",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(plugins) {
				return nil
			}
			return &plugins[index]
		},
	}
}

type pluginScopeFetcher func(pageSize int64, sdk *helpers.KonnectSDK) ([]kkComps.Plugin, error)

func fetchPluginsForScope(
	helper cmd.Helper,
	fetch pluginScopeFetcher,
) ([]kkComps.Plugin, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return nil, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return nil, err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return nil, err
	}

	konnectSDK, ok := sdk.(*helpers.KonnectSDK)
	if !ok || konnectSDK.SDK == nil {
		return nil, fmt.Errorf("konnect SDK is not available")
	}

	requestPageSize := int64(cfg.GetInt(kkCommon.RequestPageSizeConfigPath))
	plugins, err := fetch(requestPageSize, konnectSDK)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError(
			"Failed to list Gateway Plugins",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	return plugins, nil
}

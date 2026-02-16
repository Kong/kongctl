package upstream

import (
	"context"
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
	tableview.RegisterChildLoader("control-plane", "upstreams", loadControlPlaneUpstreams)
	tableview.RegisterChildLoader("upstream", "targets", loadUpstreamTargets)
}

func loadControlPlaneUpstreams(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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
	upstreams, err := helpers.GetAllGatewayUpstreams(
		helper.GetContext(),
		requestPageSize,
		controlPlaneID,
		konnectSDK.SDK,
	)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return tableview.ChildView{}, cmd.PrepareExecutionError(
			"Failed to list Gateway Upstreams",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	rows := make([]table.Row, 0, len(upstreams))
	for i := range upstreams {
		record := upstreamToDisplayRecord(&upstreams[i])
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(upstreams) {
			return ""
		}
		return upstreamDetailView(&upstreams[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Upstreams",
		ParentType:     "upstream",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(upstreams) {
				return nil
			}
			upstreamID := ""
			if upstreams[index].GetID() != nil {
				upstreamID = strings.TrimSpace(*upstreams[index].GetID())
			}
			return &gatewaycommon.UpstreamContext{
				ControlPlaneID: controlPlaneID,
				UpstreamID:     upstreamID,
			}
		},
	}, nil
}

func loadUpstreamTargets(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	ctx, err := gatewaycommon.UpstreamContextFromParent(parent)
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
	targets, err := helpers.GetAllGatewayTargetsForUpstream(
		helper.GetContext(),
		requestPageSize,
		ctx.ControlPlaneID,
		ctx.UpstreamID,
		konnectSDK.SDK,
	)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return tableview.ChildView{}, cmd.PrepareExecutionError(
			"Failed to list Gateway Targets",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	rows := make([]table.Row, 0, len(targets))
	for i := range targets {
		record := targetToDisplayRecord(&targets[i])
		rows = append(rows, table.Row{record.ID, record.Target})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(targets) {
			return ""
		}
		return targetDetailView(&targets[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "TARGET"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Targets",
		ParentType:     "upstream-target",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(targets) {
				return nil
			}
			return &targets[index]
		},
	}, nil
}

type upstreamDisplayRecord struct {
	ID   string
	Name string
}

func upstreamToDisplayRecord(upstream *kkComps.Upstream) upstreamDisplayRecord {
	const missing = "n/a"

	id := missing
	if upstream.GetID() != nil && *upstream.GetID() != "" {
		id = util.AbbreviateUUID(*upstream.GetID())
	}

	name := strings.TrimSpace(upstream.GetName())
	if name == "" {
		name = missing
	}

	return upstreamDisplayRecord{
		ID:   id,
		Name: name,
	}
}

func upstreamDetailView(upstream *kkComps.Upstream) string {
	if upstream == nil {
		return ""
	}

	const missing = "n/a"

	id := missing
	if upstream.GetID() != nil && *upstream.GetID() != "" {
		id = *upstream.GetID()
	}

	name := strings.TrimSpace(upstream.GetName())
	if name == "" {
		name = missing
	}

	algorithm := missing
	if value := upstream.GetAlgorithm(); value != nil {
		algorithm = string(*value)
	}

	slots := missing
	if value := upstream.GetSlots(); value != nil {
		slots = fmt.Sprintf("%d", *value)
	}

	hostHeader := missing
	if value := upstream.GetHostHeader(); value != nil && strings.TrimSpace(*value) != "" {
		hostHeader = strings.TrimSpace(*value)
	}

	useSrvName := missing
	if value := upstream.GetUseSrvName(); value != nil {
		useSrvName = fmt.Sprintf("%t", *value)
	}

	tagsLine := missing
	if tags := upstream.GetTags(); len(tags) > 0 {
		tagsLine = strings.Join(tags, ", ")
	}

	created := missing
	if ts := upstream.GetCreatedAt(); ts != nil {
		created = time.Unix(0, *ts*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	updated := missing
	if ts := upstream.GetUpdatedAt(); ts != nil {
		updated = time.Unix(0, *ts*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "algorithm: %s\n", algorithm)
	fmt.Fprintf(&b, "slots: %s\n", slots)
	fmt.Fprintf(&b, "host_header: %s\n", hostHeader)
	fmt.Fprintf(&b, "use_srv_name: %s\n", useSrvName)
	fmt.Fprintf(&b, "tags: %s\n", tagsLine)
	fmt.Fprintf(&b, "created_at: %s\n", created)
	fmt.Fprintf(&b, "updated_at: %s\n", updated)

	return b.String()
}

type targetDisplayRecord struct {
	ID     string
	Target string
}

func targetToDisplayRecord(target *kkComps.Target) targetDisplayRecord {
	const missing = "n/a"

	id := missing
	if target.GetID() != nil && *target.GetID() != "" {
		id = util.AbbreviateUUID(*target.GetID())
	}

	value := missing
	if target.GetTarget() != nil && strings.TrimSpace(*target.GetTarget()) != "" {
		value = strings.TrimSpace(*target.GetTarget())
	}

	return targetDisplayRecord{
		ID:     id,
		Target: value,
	}
}

func targetDetailView(target *kkComps.Target) string {
	if target == nil {
		return ""
	}

	const missing = "n/a"

	id := missing
	if target.GetID() != nil && *target.GetID() != "" {
		id = *target.GetID()
	}

	value := missing
	if target.GetTarget() != nil && strings.TrimSpace(*target.GetTarget()) != "" {
		value = strings.TrimSpace(*target.GetTarget())
	}

	weight := missing
	if v := target.GetWeight(); v != nil {
		weight = fmt.Sprintf("%d", *v)
	}

	failover := missing
	if v := target.GetFailover(); v != nil {
		failover = fmt.Sprintf("%t", *v)
	}

	tagsLine := missing
	if tags := target.GetTags(); len(tags) > 0 {
		tagsLine = strings.Join(tags, ", ")
	}

	created := missing
	if ts := target.GetCreatedAt(); ts != nil {
		created = time.Unix(0, int64(*ts*float64(time.Millisecond))).In(time.Local).Format("2006-01-02 15:04:05")
	}

	updated := missing
	if ts := target.GetUpdatedAt(); ts != nil {
		updated = time.Unix(0, int64(*ts*float64(time.Millisecond))).In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "target: %s\n", value)
	fmt.Fprintf(&b, "weight: %s\n", weight)
	fmt.Fprintf(&b, "failover: %s\n", failover)
	fmt.Fprintf(&b, "tags: %s\n", tagsLine)
	fmt.Fprintf(&b, "created_at: %s\n", created)
	fmt.Fprintf(&b, "updated_at: %s\n", updated)

	return b.String()
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

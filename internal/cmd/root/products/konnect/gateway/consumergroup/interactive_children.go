package consumergroup

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
	gatewayconsumer "github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/consumer"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
)

func init() {
	tableview.RegisterChildLoader("control-plane", "consumer-groups", loadControlPlaneConsumerGroups)
	tableview.RegisterChildLoader("consumer-group", "consumers", loadConsumerGroupConsumers)
}

func loadControlPlaneConsumerGroups(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
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
	groups, err := helpers.GetAllGatewayConsumerGroups(
		helper.GetContext(),
		requestPageSize,
		controlPlaneID,
		konnectSDK.SDK,
	)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return tableview.ChildView{}, cmd.PrepareExecutionError(
			"Failed to list Gateway Consumer Groups",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	rows := make([]table.Row, 0, len(groups))
	for i := range groups {
		record := consumerGroupToDisplayRecord(&groups[i])
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(groups) {
			return ""
		}
		return consumerGroupDetailView(&groups[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Consumer Groups",
		ParentType:     "consumer-group",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(groups) {
				return nil
			}
			groupID := ""
			if groups[index].GetID() != nil {
				groupID = strings.TrimSpace(*groups[index].GetID())
			}
			return &gatewaycommon.ConsumerGroupContext{
				ControlPlaneID:  controlPlaneID,
				ConsumerGroupID: groupID,
			}
		},
	}, nil
}

func loadConsumerGroupConsumers(_ context.Context, helper cmd.Helper, parent any) (tableview.ChildView, error) {
	ctx, err := gatewaycommon.ConsumerGroupContextFromParent(parent)
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
	consumers, err := helpers.GetAllGatewayConsumerGroupConsumers(
		helper.GetContext(),
		requestPageSize,
		ctx.ControlPlaneID,
		ctx.ConsumerGroupID,
		konnectSDK.SDK,
	)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return tableview.ChildView{}, cmd.PrepareExecutionError(
			"Failed to list Gateway Consumers",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	rows := make([]table.Row, 0, len(consumers))
	for i := range consumers {
		record := consumerListRecord(&consumers[i])
		rows = append(rows, table.Row{record.ID, record.Username})
	}

	detail := func(index int) string {
		if index < 0 || index >= len(consumers) {
			return ""
		}
		return gatewayconsumer.ConsumerDetailView(&consumers[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "USERNAME"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          "Consumers",
		ParentType:     "gateway-consumer",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(consumers) {
				return nil
			}
			return &consumers[index]
		},
	}, nil
}

type consumerGroupDisplayRecord struct {
	ID   string
	Name string
}

func consumerGroupToDisplayRecord(group *kkComps.ConsumerGroup) consumerGroupDisplayRecord {
	const missing = "n/a"

	id := missing
	if group.GetID() != nil && *group.GetID() != "" {
		id = util.AbbreviateUUID(*group.GetID())
	}

	name := strings.TrimSpace(group.GetName())
	if name == "" {
		name = missing
	}

	return consumerGroupDisplayRecord{
		ID:   id,
		Name: name,
	}
}

func consumerGroupDetailView(group *kkComps.ConsumerGroup) string {
	if group == nil {
		return ""
	}

	const missing = "n/a"

	id := missing
	if group.GetID() != nil && *group.GetID() != "" {
		id = *group.GetID()
	}

	name := strings.TrimSpace(group.GetName())
	if name == "" {
		name = missing
	}

	tagsLine := missing
	if tags := group.GetTags(); len(tags) > 0 {
		tagsLine = strings.Join(tags, ", ")
	}

	created := missing
	if ts := group.GetCreatedAt(); ts != nil {
		created = time.Unix(0, *ts*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	updated := missing
	if ts := group.GetUpdatedAt(); ts != nil {
		updated = time.Unix(0, *ts*int64(time.Millisecond)).In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
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

type consumerListDisplayRecord struct {
	ID       string
	Username string
}

func consumerListRecord(consumer *kkComps.Consumer) consumerListDisplayRecord {
	const missing = "n/a"

	id := missing
	if consumer.GetID() != nil && *consumer.GetID() != "" {
		id = util.AbbreviateUUID(*consumer.GetID())
	}

	username := missing
	if consumer.GetUsername() != nil && *consumer.GetUsername() != "" {
		username = *consumer.GetUsername()
	}

	return consumerListDisplayRecord{
		ID:       id,
		Username: username,
	}
}

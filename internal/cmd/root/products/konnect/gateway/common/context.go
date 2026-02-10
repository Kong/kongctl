package common

import (
	"fmt"
	"strings"
)

type ServiceContext struct {
	ControlPlaneID string
	ServiceID      string
}

type RouteContext struct {
	ControlPlaneID string
	RouteID        string
}

type ConsumerContext struct {
	ControlPlaneID string
	ConsumerID     string
}

type ConsumerGroupContext struct {
	ControlPlaneID  string
	ConsumerGroupID string
}

type UpstreamContext struct {
	ControlPlaneID string
	UpstreamID     string
}

func ServiceContextFromParent(parent any) (*ServiceContext, error) {
	ctx, ok := parent.(*ServiceContext)
	if !ok || ctx == nil {
		return nil, fmt.Errorf("unexpected parent type %T", parent)
	}
	controlPlaneID := strings.TrimSpace(ctx.ControlPlaneID)
	if controlPlaneID == "" {
		return nil, fmt.Errorf("control plane identifier is missing")
	}
	serviceID := strings.TrimSpace(ctx.ServiceID)
	if serviceID == "" {
		return nil, fmt.Errorf("service identifier is missing")
	}
	return &ServiceContext{
		ControlPlaneID: controlPlaneID,
		ServiceID:      serviceID,
	}, nil
}

func RouteContextFromParent(parent any) (*RouteContext, error) {
	ctx, ok := parent.(*RouteContext)
	if !ok || ctx == nil {
		return nil, fmt.Errorf("unexpected parent type %T", parent)
	}
	controlPlaneID := strings.TrimSpace(ctx.ControlPlaneID)
	if controlPlaneID == "" {
		return nil, fmt.Errorf("control plane identifier is missing")
	}
	routeID := strings.TrimSpace(ctx.RouteID)
	if routeID == "" {
		return nil, fmt.Errorf("route identifier is missing")
	}
	return &RouteContext{
		ControlPlaneID: controlPlaneID,
		RouteID:        routeID,
	}, nil
}

func ConsumerContextFromParent(parent any) (*ConsumerContext, error) {
	ctx, ok := parent.(*ConsumerContext)
	if !ok || ctx == nil {
		return nil, fmt.Errorf("unexpected parent type %T", parent)
	}
	controlPlaneID := strings.TrimSpace(ctx.ControlPlaneID)
	if controlPlaneID == "" {
		return nil, fmt.Errorf("control plane identifier is missing")
	}
	consumerID := strings.TrimSpace(ctx.ConsumerID)
	if consumerID == "" {
		return nil, fmt.Errorf("consumer identifier is missing")
	}
	return &ConsumerContext{
		ControlPlaneID: controlPlaneID,
		ConsumerID:     consumerID,
	}, nil
}

func ConsumerGroupContextFromParent(parent any) (*ConsumerGroupContext, error) {
	ctx, ok := parent.(*ConsumerGroupContext)
	if !ok || ctx == nil {
		return nil, fmt.Errorf("unexpected parent type %T", parent)
	}
	controlPlaneID := strings.TrimSpace(ctx.ControlPlaneID)
	if controlPlaneID == "" {
		return nil, fmt.Errorf("control plane identifier is missing")
	}
	groupID := strings.TrimSpace(ctx.ConsumerGroupID)
	if groupID == "" {
		return nil, fmt.Errorf("consumer group identifier is missing")
	}
	return &ConsumerGroupContext{
		ControlPlaneID:  controlPlaneID,
		ConsumerGroupID: groupID,
	}, nil
}

func UpstreamContextFromParent(parent any) (*UpstreamContext, error) {
	ctx, ok := parent.(*UpstreamContext)
	if !ok || ctx == nil {
		return nil, fmt.Errorf("unexpected parent type %T", parent)
	}
	controlPlaneID := strings.TrimSpace(ctx.ControlPlaneID)
	if controlPlaneID == "" {
		return nil, fmt.Errorf("control plane identifier is missing")
	}
	upstreamID := strings.TrimSpace(ctx.UpstreamID)
	if upstreamID == "" {
		return nil, fmt.Errorf("upstream identifier is missing")
	}
	return &UpstreamContext{
		ControlPlaneID: controlPlaneID,
		UpstreamID:     upstreamID,
	}, nil
}

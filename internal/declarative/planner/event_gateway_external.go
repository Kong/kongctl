package planner

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func (p *Planner) isEventGatewayExternal(gatewayRef string) bool {
	if p == nil || p.resources == nil {
		return false
	}
	eventGateway := p.resources.GetEventGatewayControlPlaneByRef(gatewayRef)
	return eventGateway != nil && eventGateway.IsExternal()
}

func (p *Planner) isEventGatewayVirtualClusterExternal(virtualClusterRef string) bool {
	if p == nil || p.resources == nil {
		return false
	}
	virtualCluster := p.resources.GetVirtualClusterByRef(virtualClusterRef)
	return virtualCluster != nil && virtualCluster.IsExternal()
}

func matchExternalEventGatewayVirtualCluster(
	virtualCluster *resources.EventGatewayVirtualClusterResource,
	available []state.EventGatewayVirtualCluster,
) (*state.EventGatewayVirtualCluster, error) {
	if virtualCluster == nil || virtualCluster.External == nil {
		return nil, fmt.Errorf("event_gateway_virtual_cluster requires _external")
	}

	if virtualCluster.External.ID != "" {
		for i := range available {
			if available[i].ID == virtualCluster.External.ID {
				return &available[i], nil
			}
		}
		return nil, fmt.Errorf(
			"external event_gateway_virtual_cluster %s: not found with id %s",
			virtualCluster.GetRef(),
			virtualCluster.External.ID,
		)
	}

	if virtualCluster.External.Selector == nil {
		return nil, fmt.Errorf(
			"external event_gateway_virtual_cluster %s: invalid _external configuration",
			virtualCluster.GetRef(),
		)
	}

	matchFields := virtualCluster.External.Selector.MatchFields
	var match *state.EventGatewayVirtualCluster
	for i := range available {
		if virtualCluster.External.Selector.Match(available[i]) {
			if match != nil {
				return nil, fmt.Errorf(
					"external event_gateway_virtual_cluster %s: selector %v matched multiple virtual clusters",
					virtualCluster.GetRef(),
					matchFields,
				)
			}
			match = &available[i]
		}
	}
	if match == nil {
		return nil, fmt.Errorf(
			"external event_gateway_virtual_cluster %s: selector %v did not match any virtual cluster",
			virtualCluster.GetRef(),
			matchFields,
		)
	}

	return match, nil
}

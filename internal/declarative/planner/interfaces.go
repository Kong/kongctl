package planner

import (
	"context"
)

// ResourcePlanner defines the interface that all resource type planners must implement
type ResourcePlanner interface {
	// PlanChanges is the main entry point for planning changes for a resource type
	PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error
}

// PortalPlanner handles planning for portal resources
type PortalPlanner interface {
	ResourcePlanner
	
	// Additional portal-specific methods if needed
}

// AuthStrategyPlanner handles planning for auth strategy resources
type AuthStrategyPlanner interface {
	ResourcePlanner
	
	// Additional auth strategy-specific methods if needed
}

// APIPlanner handles planning for API resources and their child resources
type APIPlanner interface {
	ResourcePlanner
	
	// Additional API-specific methods if needed
}
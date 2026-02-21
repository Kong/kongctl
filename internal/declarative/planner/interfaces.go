package planner

import (
	"context"
)

// ResourcePlanner defines the interface that all resource type planners must implement
type ResourcePlanner interface {
	// PlanChanges is the main entry point for planning changes for a resource type
	PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error
}

// PlannerComponentProvider optionally exposes the workflow component name to use in HTTP logging.
// Implementations should return the canonical resource-style identifier (for example: "portal").
type PlannerComponentProvider interface {
	PlannerComponent() string
}

// PortalPlanner handles planning for portal resources
type PortalPlanner interface {
	ResourcePlanner

	// Additional portal-specific methods if needed
}

// ControlPlanePlanner handles planning for control plane resources
type ControlPlanePlanner interface {
	ResourcePlanner
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

// CatalogServicePlanner handles planning for catalog service resources
type CatalogServicePlanner interface {
	ResourcePlanner
}

// EGWControlPlanePlanner handles planning for Event Gateway Control Plane resources
type EGWControlPlanePlanner interface {
	ResourcePlanner

	// Additional Event Gateway Control Plane-specific methods if needed
}

// TeamPlanner handles planning for team resources
type OrganizationTeamPlanner interface {
	ResourcePlanner

	// Additional Team-specific methods if needed
}

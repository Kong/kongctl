package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// rootPlanner binds a root entry point to its scope and error context. Child
// planning remains owned by that entry point.
type rootPlanner struct {
	resourceType resources.ResourceType
	displayName  string
	planner      ResourcePlanner
	inScope      func(*Plan) bool
}

// rootPlanners is the ordered inventory for root planner construction and
// dispatch. Keep this order stable: planning assigns change IDs and can resolve
// references against changes produced by earlier entries.
func (p *Planner) rootPlanners() []rootPlanner {
	base := NewBasePlanner(p)
	roots := []rootPlanner{
		{
			resourceType: resources.ResourceTypeIdentityAuthServer, displayName: "Kong Identity auth server",
			planner: &identityAuthServerPlanner{BasePlanner: base},
		},
		{

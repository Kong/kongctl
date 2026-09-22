package planner

import (
	"errors"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/state"
)

// optionalPortalSingletonOperations describes an optional, parent-owned singleton.
// Adapters normalize absent observations and retain update/replacement behavior.
type optionalPortalSingletonOperations[D, C any] struct {
	desiredRef func(D) string
	fetch      func() (*C, error)
	create     func(D) string
	reconcile  func(*C, D)
	remove     func(*C)
	// Preserve the supported missing-client fallback without treating other
	// observation errors as absence.
	clientType         string
	description        string
	unavailableWarning string
}

// reconcileOptionalPortalSingleton selects the first unplanned declaration, creates
// under a new parent without reading, and prunes absent declarations only in
// sync mode. A missing API client permits creation with a warning; other read
// errors stop planning. Parent scope remains the caller's responsibility.
func reconcileOptionalPortalSingleton[D, C any](
	resourceType string,
	parentID, parentRef string,
	desired []D,
	ops optionalPortalSingletonOperations[D, C],
	plan *Plan,
) error {
	var selected *D
	for i := range desired {
		if !plan.HasChange(resourceType, ops.desiredRef(desired[i])) {
			selected = &desired[i]
			break
		}
	}

	if parentID == "" {
		if selected != nil {
			ops.create(*selected)
		}
		return nil
	}

	current, err := ops.fetch()
	if err != nil {
		var apiErr *state.APIClientError
		if errors.As(err, &apiErr) && apiErr.ClientType == ops.clientType {
			if selected != nil {
				changeID := ops.create(*selected)
				plan.AddWarning(changeID, ops.unavailableWarning)
			}
			return nil
		}
		identifier := parentRef
		if identifier == "" {
			identifier = parentID
		}
		return fmt.Errorf("failed to get %s for portal %q: %w", ops.description, identifier, err)
	}

	if selected == nil {
		if current != nil && plan.Metadata.Mode == PlanModeSync {
			ops.remove(current)
		}
		return nil
	}
	if current == nil {
		ops.create(*selected)
		return nil
	}
	ops.reconcile(current, *selected)
	return nil
}

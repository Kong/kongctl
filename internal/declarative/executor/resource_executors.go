package executor

import (
	"context"

	"github.com/kong/kongctl/internal/declarative/planner"
)

type resourceWrite func(context.Context, *planner.PlannedChange) (string, error)

// resourceExecutor couples action routing to the payload contract supplied by
// the same typed executor. A nil action retains the unsupported-operation path.
type resourceExecutor struct {
	contract payloadContract
	create   resourceWrite
	update   resourceWrite
	remove   func(context.Context, *planner.PlannedChange) error
}

type createDeleteExecutor interface {
	payloadContract
	Create(context.Context, planner.PlannedChange) (string, error)
	Delete(context.Context, planner.PlannedChange) error
}

type crudExecutor interface {
	createDeleteExecutor
	Update(context.Context, planner.PlannedChange) (string, error)
}

func createDeleteResourceExecutor(base createDeleteExecutor) resourceExecutor {
	return resourceExecutor{
		contract: base,
		create: func(ctx context.Context, change *planner.PlannedChange) (string, error) {
			return base.Create(ctx, *change)
		},
		remove: func(ctx context.Context, change *planner.PlannedChange) error {
			return base.Delete(ctx, *change)
		},
	}
}

func crudResourceExecutor(base crudExecutor) resourceExecutor {
	resource := createDeleteResourceExecutor(base)
	resource.update = func(ctx context.Context, change *planner.PlannedChange) (string, error) {
		return base.Update(ctx, *change)
	}
	return resource
}

// prepareResourceExecutor resolves routing information only for supported
// actions, immediately before their existing executor behavior runs.
func prepareResourceExecutor(
	resource resourceExecutor,
	prepare func(context.Context, *planner.PlannedChange) error,
) resourceExecutor {
	prepareWrite := func(write resourceWrite) resourceWrite {
		if write == nil {
			return nil
		}
		return func(ctx context.Context, change *planner.PlannedChange) (string, error) {
			if err := prepare(ctx, change); err != nil {
				return "", err
			}
			return write(ctx, change)
		}
	}
	resource.create = prepareWrite(resource.create)
	resource.update = prepareWrite(resource.update)
	if remove := resource.remove; remove != nil {
		resource.remove = func(ctx context.Context, change *planner.PlannedChange) error {
			if err := prepare(ctx, change); err != nil {
				return err
			}
			return remove(ctx, change)
		}
	}
	return resource
}

func (e *Executor) registerResourceExecutor(resource resourceExecutor) {
	if resource.contract == nil || resource.contract.ResourceType() == "" {
		panic("resource executor requires a payload contract and resource type")
	}
	if resource.create == nil && resource.update == nil && resource.remove == nil {
		panic("resource executor has no actions for " + resource.contract.ResourceType())
	}
	// This also rejects duplicate registrations, including a conflicting entry
	// in the legacy payload inventory while other families are being migrated.
	e.registerPayloadContracts(resource.contract)
	e.resourceExecutors[resource.contract.ResourceType()] = resource
}

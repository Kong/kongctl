package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
)

// CreateDeleteOperations defines operations for resources that only support create and delete
// (no update operation), such as API versions and API publications
type CreateDeleteOperations[TCreate any] interface {
	// Field mapping
	MapCreateFields(ctx context.Context, execCtx *ExecutionContext, fields map[string]any, create *TCreate) error

	// API calls
	Create(ctx context.Context, req TCreate, namespace string, execCtx *ExecutionContext) (string, error)
	Delete(ctx context.Context, id string, execCtx *ExecutionContext) error
	GetByName(ctx context.Context, name string) (ResourceInfo, error)

	// Resource info
	ResourceType() string
	RequiredFields() []string
}

// SingletonOperations defines operations for singleton resources that always exist
// and only support updates (no create/delete), such as portal customization
type SingletonOperations[TUpdate any] interface {
	// Field mapping
	MapUpdateFields(ctx context.Context, fields map[string]any, update *TUpdate) error

	// API calls - note the special signature for singleton resources
	Update(ctx context.Context, parentID string, req TUpdate) error

	// Resource info
	ResourceType() string
}

// ParentAwareOperations extends ResourceOperations for resources with parent relationships
type ParentAwareOperations[TCreate any, TUpdate any] interface {
	ResourceOperations[TCreate, TUpdate]
	
	// Parent resolution
	ResolveParentID(ctx context.Context, change planner.PlannedChange, executor *Executor) (string, error)
}

// BaseCreateDeleteExecutor provides common operations for create/delete only resources
type BaseCreateDeleteExecutor[TCreate any] struct {
	ops     CreateDeleteOperations[TCreate]
	dryRun  bool
}

// NewBaseCreateDeleteExecutor creates a new executor for create/delete only resources
func NewBaseCreateDeleteExecutor[TCreate any](
	ops CreateDeleteOperations[TCreate],
	dryRun bool,
) *BaseCreateDeleteExecutor[TCreate] {
	return &BaseCreateDeleteExecutor[TCreate]{
		ops:    ops,
		dryRun: dryRun,
	}
}

// Create handles CREATE operations
func (b *BaseCreateDeleteExecutor[TCreate]) Create(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Create ExecutionContext
	execCtx := NewExecutionContext(&change)
	
	// Create request object
	var create TCreate
	if err := b.ops.MapCreateFields(ctx, execCtx, change.Fields, &create); err != nil {
		return "", fmt.Errorf("failed to map fields for %s: %w", b.ops.ResourceType(), err)
	}

	// Handle dry-run
	if b.dryRun {
		return fmt.Sprintf("dry-run-%s-id", b.ops.ResourceType()), nil
	}

	// Create resource
	id, err := b.ops.Create(ctx, create, change.Namespace, execCtx)
	if err != nil {
		return "", fmt.Errorf("failed to create %s: %w", b.ops.ResourceType(), err)
	}

	return id, nil
}

// Delete handles DELETE operations
func (b *BaseCreateDeleteExecutor[TCreate]) Delete(ctx context.Context, change planner.PlannedChange) error {
	// Handle dry-run
	if b.dryRun {
		return nil
	}

	// Create execution context for operations that need parent references
	execCtx := NewExecutionContext(&change)

	// Delete the resource
	err := b.ops.Delete(ctx, change.ResourceID, execCtx)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", b.ops.ResourceType(), err)
	}

	return nil
}

// BaseSingletonExecutor provides common operations for singleton resources
type BaseSingletonExecutor[TUpdate any] struct {
	ops     SingletonOperations[TUpdate]
	dryRun  bool
}

// NewBaseSingletonExecutor creates a new executor for singleton resources
func NewBaseSingletonExecutor[TUpdate any](
	ops SingletonOperations[TUpdate],
	dryRun bool,
) *BaseSingletonExecutor[TUpdate] {
	return &BaseSingletonExecutor[TUpdate]{
		ops:    ops,
		dryRun: dryRun,
	}
}

// Update handles both CREATE and UPDATE operations for singleton resources
func (b *BaseSingletonExecutor[TUpdate]) Update(ctx context.Context, change planner.PlannedChange,
	parentID string) (string, error) {
	// Create update request
	var update TUpdate
	if err := b.ops.MapUpdateFields(ctx, change.Fields, &update); err != nil {
		return "", fmt.Errorf("failed to map fields for %s: %w", b.ops.ResourceType(), err)
	}

	// Handle dry-run
	if b.dryRun {
		return parentID, nil // For singleton resources, we return the parent ID
	}

	// Update resource
	err := b.ops.Update(ctx, parentID, update)
	if err != nil {
		return "", fmt.Errorf("failed to update %s: %w", b.ops.ResourceType(), err)
	}

	return parentID, nil
}
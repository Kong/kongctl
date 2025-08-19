package executor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
)

// Note: Context keys removed - now using explicit ExecutionContext parameter

// ResourceOperations defines the contract for resource-specific operations
type ResourceOperations[TCreate any, TUpdate any] interface {
	// Field mapping
	MapCreateFields(ctx context.Context, execCtx *ExecutionContext, fields map[string]any, create *TCreate) error
	MapUpdateFields(ctx context.Context, execCtx *ExecutionContext, fields map[string]any, update *TUpdate,
		currentLabels map[string]string) error

	// API calls
	Create(ctx context.Context, req TCreate, namespace string, execCtx *ExecutionContext) (string, error)
	Update(ctx context.Context, id string, req TUpdate, namespace string, execCtx *ExecutionContext) (string, error)
	Delete(ctx context.Context, id string, execCtx *ExecutionContext) error
	GetByName(ctx context.Context, name string) (ResourceInfo, error)

	// Resource info
	ResourceType() string
	RequiredFields() []string
	SupportsUpdate() bool
}

// ResourceInfo provides common resource information
type ResourceInfo interface {
	GetID() string
	GetName() string
	GetLabels() map[string]string
	GetNormalizedLabels() map[string]string
}

// BaseExecutor provides common CRUD operations
type BaseExecutor[TCreate any, TUpdate any] struct {
	ops     ResourceOperations[TCreate, TUpdate]
	client  *state.Client
	dryRun  bool
}

// NewBaseExecutor creates a new base executor instance
func NewBaseExecutor[TCreate any, TUpdate any](
	ops ResourceOperations[TCreate, TUpdate],
	client *state.Client,
	dryRun bool,
) *BaseExecutor[TCreate, TUpdate] {
	return &BaseExecutor[TCreate, TUpdate]{
		ops:     ops,
		client:  client,
		dryRun:  dryRun,
	}
}

// Create handles CREATE operations for any resource type
func (b *BaseExecutor[TCreate, TUpdate]) Create(ctx context.Context, change planner.PlannedChange) (string, error) {
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	logger.Debug(fmt.Sprintf("Creating %s", b.ops.ResourceType()),
		slog.Any("fields", change.Fields))

	// Validate required fields
	if err := common.ValidateRequiredFields(change.Fields, b.ops.RequiredFields()); err != nil {
		return "", common.WrapWithResourceContext(err, b.ops.ResourceType(), "")
	}

	// Create execution context
	execCtx := NewExecutionContext(&change)

	// Create request object
	var create TCreate
	if err := b.ops.MapCreateFields(ctx, execCtx, change.Fields, &create); err != nil {
		resourceName := common.ExtractResourceName(change.Fields)
		return "", common.FormatAPIError(b.ops.ResourceType(), resourceName, "create", err)
	}

	// Handle dry-run
	if b.dryRun {
		return fmt.Sprintf("dry-run-%s-id", b.ops.ResourceType()), nil
	}

	// Create resource
	resourceName := common.ExtractResourceName(change.Fields)
	id, err := b.ops.Create(ctx, create, change.Namespace, execCtx)
	if err != nil {
		return "", common.FormatAPIError(b.ops.ResourceType(), resourceName, "create", err)
	}

	return id, nil
}

// Update handles UPDATE operations for any resource type
func (b *BaseExecutor[TCreate, TUpdate]) Update(ctx context.Context, change planner.PlannedChange) (string, error) {
	if !b.ops.SupportsUpdate() {
		return "", fmt.Errorf("%s does not support update operations", b.ops.ResourceType())
	}

	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	logger.Debug(fmt.Sprintf("Updating %s", b.ops.ResourceType()),
		slog.Any("change", change))

	resourceName := common.ExtractResourceName(change.Fields)

	// First, validate protection status at execution time
	resource, err := b.validateResourceForUpdate(ctx, resourceName, change)
	if err != nil {
		return "", fmt.Errorf("failed to validate %s for update: %w", b.ops.ResourceType(), err)
	}
	if resource == nil {
		return "", fmt.Errorf("%s no longer exists", b.ops.ResourceType())
	}

	// Check protection status using common utility
	isProtected := common.GetProtectionStatus(resource.GetNormalizedLabels())
	isProtectionChange := common.IsProtectionChange(change.Protection)

	// Validate protection using common utility
	if err := common.ValidateResourceProtection(
		b.ops.ResourceType(), resourceName, isProtected, change, isProtectionChange,
	); err != nil {
		return err.Error(), err
	}

	// Get current labels for update
	currentLabels := make(map[string]string)
	for k, v := range resource.GetLabels() {
		if !labels.IsKongctlLabel(k) {
			currentLabels[k] = v
		}
	}

	// Create execution context
	execCtx := NewExecutionContext(&change)

	// Create update request
	var update TUpdate
	if err := b.ops.MapUpdateFields(ctx, execCtx, change.Fields, &update, currentLabels); err != nil {
		return "", common.FormatAPIError(b.ops.ResourceType(), resourceName, "update", err)
	}

	// Handle dry-run
	if b.dryRun {
		return change.ResourceID, nil
	}

	// Update resource
	id, err := b.ops.Update(ctx, change.ResourceID, update, change.Namespace, execCtx)
	if err != nil {
		return "", common.FormatAPIError(b.ops.ResourceType(), resourceName, "update", err)
	}

	return id, nil
}

// Delete handles DELETE operations for any resource type
func (b *BaseExecutor[TCreate, TUpdate]) Delete(ctx context.Context, change planner.PlannedChange) error {
	resourceName := common.ExtractResourceName(change.Fields)

	// First, validate protection status at execution time
	resource, err := b.ops.GetByName(ctx, resourceName)
	if err != nil {
		return fmt.Errorf("failed to fetch %s for protection check: %w", b.ops.ResourceType(), err)
	}
	if resource == nil {
		// Resource already deleted, consider this success
		return nil
	}

	// Check if resource is protected
	isProtected := common.GetProtectionStatus(resource.GetNormalizedLabels())
	if isProtected {
		return fmt.Errorf("resource is protected and cannot be deleted")
	}

	// Verify it's a managed resource
	if !labels.IsManagedResource(resource.GetNormalizedLabels()) {
		return fmt.Errorf("cannot delete %s: not a KONGCTL-managed resource", b.ops.ResourceType())
	}

	// Handle dry-run
	if b.dryRun {
		return nil
	}

	// Create execution context for operations that need parent references
	execCtx := NewExecutionContext(&change)

	// Delete the resource
	err = b.ops.Delete(ctx, change.ResourceID, execCtx)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", b.ops.ResourceType(), err)
	}

	return nil
}

// validateResourceForUpdate provides robust resource validation with fallback strategies
func (b *BaseExecutor[TCreate, TUpdate]) validateResourceForUpdate(
	ctx context.Context, resourceName string, change planner.PlannedChange,
) (ResourceInfo, error) {
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	
	// Strategy 1: Standard name-based lookup
	resource, err := b.ops.GetByName(ctx, resourceName)
	if err == nil && resource != nil {
		return resource, nil
	}
	
	// Strategy 2: Try ID-based lookup if available (useful for child resources)
	if change.ResourceID != "" {
		if idLookup, ok := b.ops.(interface{ GetByID(context.Context, string) (ResourceInfo, error) }); ok {
			resource, err := idLookup.GetByID(ctx, change.ResourceID)
			if err == nil && resource != nil {
				logger.Debug("Resource found via ID lookup", 
					"resource_type", b.ops.ResourceType(), 
					"name", resourceName, 
					"id", change.ResourceID)
				return resource, nil
			}
		}
	}
	
	// Strategy 3: For protection changes, try lookup with preserved labels context
	if isProtectionChange(change) && change.Fields != nil {
		if namespace, ok := change.Fields["namespace"].(string); ok {
			// Try namespace-specific lookup
			if nsLookup, ok := b.ops.(interface {
				GetByNameInNamespace(context.Context, string, string) (ResourceInfo, error)
			}); ok {
				resource, err := nsLookup.GetByNameInNamespace(ctx, resourceName, namespace)
				if err == nil && resource != nil {
					logger.Debug("Resource found via namespace lookup during protection change",
						"resource_type", b.ops.ResourceType(),
						"name", resourceName,
						"namespace", namespace)
					return resource, nil
				}
			}
		}
	}
	
	// Return original result if all fallback strategies fail
	return b.ops.GetByName(ctx, resourceName)
}

// Helper function to detect protection changes
func isProtectionChange(change planner.PlannedChange) bool {
	return change.Protection != nil
}
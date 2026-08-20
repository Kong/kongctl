package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/httpclient"
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
	GetByID(ctx context.Context, id string, execCtx *ExecutionContext) (ResourceInfo, error)

	// Resource info
	ResourceType() string
	RequiredFields() []string
	SupportsUpdate() bool
}

// ManagedLabelOperations maps declarative user and kongctl-managed labels onto an update request.
// BaseExecutor invokes it for every update so protection-only changes cannot be dropped when no
// ordinary label fields changed.
type ManagedLabelOperations[TUpdate any] interface {
	MapUpdateLabels(
		execCtx *ExecutionContext,
		update *TUpdate,
		desiredLabels map[string]string,
		currentLabels map[string]string,
	)
}

type ManagedLabelResourceOperations[TCreate any, TUpdate any] interface {
	ResourceOperations[TCreate, TUpdate]
	ManagedLabelOperations[TUpdate]
}

func mapPointerUpdateLabels(
	destination *map[string]*string,
	execCtx *ExecutionContext,
	desiredLabels map[string]string,
	currentLabels map[string]string,
) {
	*destination = labels.BuildUpdateLabels(desiredLabels, currentLabels, execCtx.Namespace, execCtx.Protection)
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
	ops    ResourceOperations[TCreate, TUpdate]
	client *state.Client
	dryRun bool
}

// ResourceType identifies the resource covered by this action-aware payload contract.
func (b *BaseExecutor[TCreate, TUpdate]) ResourceType() string {
	return b.ops.ResourceType()
}

// ValidatePayload maps planner fields into the action-specific SDK request and
// verifies that the mapper did not silently discard planner payload fields.
func (b *BaseExecutor[TCreate, TUpdate]) ValidatePayload(
	ctx context.Context,
	change planner.PlannedChange,
) error {
	execCtx := NewExecutionContext(&change)
	switch change.Action {
	case planner.ActionCreate:
		var create TCreate
		if err := b.ops.MapCreateFields(ctx, execCtx, change.Fields, &create); err != nil {
			return err
		}
		return validateMappedPayload(b.ops.ResourceType(), change.Action, change.Fields, create)
	case planner.ActionUpdate:
		if !b.ops.SupportsUpdate() {
			return fmt.Errorf("action %q is not supported", change.Action)
		}
		var update TUpdate
		if err := b.ops.MapUpdateFields(ctx, execCtx, change.Fields, &update, nil); err != nil {
			return err
		}
		b.mapUpdateLabels(execCtx, change.Fields, &update, nil)
		return validateMappedPayload(b.ops.ResourceType(), change.Action, change.Fields, update)
	case planner.ActionDelete:
		return nil
	case planner.ActionExternalTool:
		return fmt.Errorf("action %q is not supported", change.Action)
	default:
		return fmt.Errorf("action %q is not supported", change.Action)
	}
}

func (b *BaseExecutor[TCreate, TUpdate]) mapUpdateLabels(
	execCtx *ExecutionContext,
	fields map[string]any,
	update *TUpdate,
	currentLabels map[string]string,
) {
	labelOps, ok := b.ops.(ManagedLabelOperations[TUpdate])
	if !ok {
		return
	}
	if rawCurrentLabels, present := fields[planner.FieldCurrentLabels]; present {
		currentLabels = labels.ExtractLabelsFromField(rawCurrentLabels)
		if currentLabels == nil {
			currentLabels = make(map[string]string)
		}
	}
	desiredLabels := currentLabels
	if rawDesiredLabels, labelsChanged := fields[planner.FieldLabels]; labelsChanged {
		desiredLabels = labels.ExtractLabelsFromField(rawDesiredLabels)
		if desiredLabels == nil {
			desiredLabels = make(map[string]string)
		}
	}
	labelOps.MapUpdateLabels(execCtx, update, desiredLabels, currentLabels)
}

// NewBaseExecutor creates a new base executor instance
func NewBaseExecutor[TCreate any, TUpdate any](
	ops ResourceOperations[TCreate, TUpdate],
	client *state.Client,
	dryRun bool,
) *BaseExecutor[TCreate, TUpdate] {
	return &BaseExecutor[TCreate, TUpdate]{
		ops:    ops,
		client: client,
		dryRun: dryRun,
	}
}

func NewManagedLabelBaseExecutor[TCreate any, TUpdate any](
	ops ManagedLabelResourceOperations[TCreate, TUpdate],
	client *state.Client,
	dryRun bool,
) *BaseExecutor[TCreate, TUpdate] {
	return NewBaseExecutor[TCreate, TUpdate](ops, client, dryRun)
}

// Create handles CREATE operations for any resource type
func (b *BaseExecutor[TCreate, TUpdate]) Create(ctx context.Context, change planner.PlannedChange) (string, error) {
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	logger.Debug(fmt.Sprintf("Creating %s", b.ops.ResourceType()),
		slog.Any("fields", httpclient.RedactSensitiveFields(change.Fields)))

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
		slog.Any("change", httpclient.RedactSensitiveFields(change)))

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
	b.mapUpdateLabels(execCtx, change.Fields, &update, currentLabels)

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
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)

	// First, validate protection status at execution time
	var execCtx *ExecutionContext
	var resource ResourceInfo
	var err error
	if change.ResourceID != "" {
		execCtx = NewExecutionContext(&change)
		resource, err = b.ops.GetByID(ctx, change.ResourceID, execCtx)
		if err != nil {
			return fmt.Errorf("failed to fetch %s by ID for protection check: %w", b.ops.ResourceType(), err)
		}
		if resource != nil {
			logger.Debug("Resource resolved via ID lookup before delete",
				slog.String("resource_type", b.ops.ResourceType()),
				slog.String("name", resourceName),
				slog.String("id", change.ResourceID))
		}
	}
	if resource == nil {
		resource, err = b.ops.GetByName(ctx, resourceName)
		if err != nil {
			return fmt.Errorf("failed to fetch %s for protection check: %w", b.ops.ResourceType(), err)
		}
	}

	if resource == nil {
		// Resource already deleted, consider this success
		logger.Warn("Resource not found; treating delete as success",
			slog.String("resource_type", b.ops.ResourceType()),
			slog.String("name", resourceName),
			slog.String("resource_id", change.ResourceID))
		return nil
	}

	// Check if resource is protected
	isProtected := common.GetProtectionStatus(resource.GetNormalizedLabels())
	if isProtected {
		return fmt.Errorf("resource is protected and cannot be deleted")
	}

	// Verify it's a managed resource (child resources rely on parent linkage instead of labels)
	isManaged := labels.IsManagedResource(resource.GetNormalizedLabels())
	if !isManaged && (change.Parent != nil || strings.TrimSpace(change.Namespace) != "") {
		isManaged = true
	}
	if !isManaged {
		return fmt.Errorf("cannot delete %s: not a KONGCTL-managed resource", b.ops.ResourceType())
	}

	// Handle dry-run
	if b.dryRun {
		return nil
	}

	// Create execution context for operations that need parent references
	if execCtx == nil {
		execCtx = NewExecutionContext(&change)
	}

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

	// Strategy 2: Try ID-based lookup
	if change.ResourceID != "" {
		execCtx := NewExecutionContext(&change)
		resource, err := b.ops.GetByID(ctx, change.ResourceID, execCtx)
		if err == nil && resource != nil {
			logger.Debug("Resource found via ID lookup",
				"resource_type", b.ops.ResourceType(),
				"name", resourceName,
				"id", change.ResourceID)
			return resource, nil
		}
	}

	// Strategy 3: For protection changes, try a namespace-specific lookup.
	if isProtectionChange(change) && change.Namespace != "" {
		if nsLookup, ok := b.ops.(interface {
			GetByNameInNamespace(context.Context, string, string) (ResourceInfo, error)
		}); ok {
			resource, err := nsLookup.GetByNameInNamespace(ctx, resourceName, change.Namespace)
			if err == nil && resource != nil {
				logger.Debug("Resource found via namespace lookup during protection change",
					"resource_type", b.ops.ResourceType(),
					"name", resourceName,
					"namespace", change.Namespace)
				return resource, nil
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

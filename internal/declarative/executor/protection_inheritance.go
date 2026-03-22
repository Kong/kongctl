package executor

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/protection"
	"github.com/kong/kongctl/internal/declarative/resources"
	logctx "github.com/kong/kongctl/internal/log"
)

func (e *Executor) validateInheritedProtection(ctx context.Context, change planner.PlannedChange) error {
	if e == nil || e.client == nil || change.ProtectingParent == nil {
		return nil
	}

	protected, err := e.isProtectingParentProtected(ctx, change.ProtectingParent)
	if err != nil {
		return err
	}
	if !protected {
		return nil
	}

	resourceName := common.ExtractResourceName(change.Fields)
	if resourceName == resources.UnknownReferenceID &&
		change.ResourceRef != "" &&
		change.ResourceRef != resources.UnknownReferenceID {
		resourceName = change.ResourceRef
	}

	return fmt.Errorf(
		"resource %q (%s) is protected via parent %q (%s) and cannot be %s",
		resourceName,
		change.ResourceType,
		change.ProtectingParent.ResourceName,
		change.ProtectingParent.ResourceType,
		actionToVerb(change.Action),
	)
}

func (e *Executor) isProtectingParentProtected(
	ctx context.Context,
	info *planner.ProtectingParentInfo,
) (bool, error) {
	if e == nil || info == nil {
		return false, nil
	}
	ctx = withProtectionLookupLogger(ctx)

	return protection.IsManagedResourceProtected(
		ctx,
		e.client,
		resources.ResourceType(info.ResourceType),
		info.ResourceName,
	)
}

func withProtectionLookupLogger(ctx context.Context) context.Context {
	if ctx != nil && ctx.Value(logctx.LoggerKey) != nil {
		return ctx
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return context.WithValue(ctx, logctx.LoggerKey, logger)
}

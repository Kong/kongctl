package log

import (
	"context"
	"log/slog"
	"strings"
)

type httpLogContextKey struct{}

// HTTPLogContext contains contextual metadata emitted with SDK HTTP logs.
type HTTPLogContext struct {
	CommandPath    string
	CommandVerb    string
	CommandMode    string
	CommandProduct string

	Workflow          string
	WorkflowPhase     string
	WorkflowComponent string
	WorkflowMode      string
	WorkflowNamespace string
	WorkflowAction    string
	WorkflowChangeID  string
	WorkflowResource  string
	WorkflowRef       string

	SDKOperationID string
}

var HTTPLogContextKey = httpLogContextKey{}

// WithHTTPLogContext merges non-empty fields from update into ctx.
func WithHTTPLogContext(ctx context.Context, update HTTPLogContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	current := HTTPLogContextFromContext(ctx)
	mergeHTTPLogContext(&current, update)

	return context.WithValue(ctx, HTTPLogContextKey, current)
}

// HTTPLogContextFromContext extracts HTTP logging metadata from ctx.
func HTTPLogContextFromContext(ctx context.Context) HTTPLogContext {
	if ctx == nil {
		return HTTPLogContext{}
	}

	switch value := ctx.Value(HTTPLogContextKey).(type) {
	case HTTPLogContext:
		return value
	case *HTTPLogContext:
		if value != nil {
			return *value
		}
	}

	return HTTPLogContext{}
}

// HTTPLogContextAttrs converts context metadata to slog attributes.
func HTTPLogContextAttrs(ctx context.Context) []slog.Attr {
	meta := HTTPLogContextFromContext(ctx)
	attrs := make([]slog.Attr, 0, 14)

	appendStringAttr(&attrs, "command_path", meta.CommandPath)
	appendStringAttr(&attrs, "command_verb", meta.CommandVerb)
	appendStringAttr(&attrs, "command_mode", meta.CommandMode)
	appendStringAttr(&attrs, "command_product", meta.CommandProduct)

	appendStringAttr(&attrs, "workflow", meta.Workflow)
	appendStringAttr(&attrs, "workflow_phase", meta.WorkflowPhase)
	appendStringAttr(&attrs, "workflow_component", meta.WorkflowComponent)
	appendStringAttr(&attrs, "workflow_mode", meta.WorkflowMode)
	appendStringAttr(&attrs, "workflow_namespace", meta.WorkflowNamespace)
	appendStringAttr(&attrs, "workflow_action", meta.WorkflowAction)
	appendStringAttr(&attrs, "workflow_change_id", meta.WorkflowChangeID)
	appendStringAttr(&attrs, "workflow_resource", meta.WorkflowResource)
	appendStringAttr(&attrs, "workflow_ref", meta.WorkflowRef)

	appendStringAttr(&attrs, "sdk_operation_id", meta.SDKOperationID)

	return attrs
}

func mergeHTTPLogContext(target *HTTPLogContext, update HTTPLogContext) {
	mergeStringField(&target.CommandPath, update.CommandPath)
	mergeStringField(&target.CommandVerb, update.CommandVerb)
	mergeStringField(&target.CommandMode, update.CommandMode)
	mergeStringField(&target.CommandProduct, update.CommandProduct)

	mergeStringField(&target.Workflow, update.Workflow)
	mergeStringField(&target.WorkflowPhase, update.WorkflowPhase)
	mergeStringField(&target.WorkflowComponent, update.WorkflowComponent)
	mergeStringField(&target.WorkflowMode, update.WorkflowMode)
	mergeStringField(&target.WorkflowNamespace, update.WorkflowNamespace)
	mergeStringField(&target.WorkflowAction, update.WorkflowAction)
	mergeStringField(&target.WorkflowChangeID, update.WorkflowChangeID)
	mergeStringField(&target.WorkflowResource, update.WorkflowResource)
	mergeStringField(&target.WorkflowRef, update.WorkflowRef)

	mergeStringField(&target.SDKOperationID, update.SDKOperationID)
}

func mergeStringField(target *string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	*target = trimmed
}

func appendStringAttr(attrs *[]slog.Attr, key, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	*attrs = append(*attrs, slog.String(key, trimmed))
}

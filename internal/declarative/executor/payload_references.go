package executor

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/values"
)

type deferredPayloadPathsKey struct{}

func withDeferredPayloadPaths(ctx context.Context, change *planner.PlannedChange) context.Context {
	paths := make(map[string]bool)
	// This visitor only records paths and cannot fail.
	_ = values.Transform(change.Fields, func(path, value string) (any, error) {
		if tags.IsEnvPlaceholder(value) {
			paths[path] = true
		}
		return value, nil
	})
	for _, intent := range change.SecretWrites {
		paths[intent.Field] = true
	}
	for _, path := range change.LiteralPaths {
		paths[path] = true
	}
	return context.WithValue(ctx, deferredPayloadPathsKey{}, paths)
}

func (e *Executor) hydratePayloadReferences(change *planner.PlannedChange, plan *planner.Plan) error {
	bindings := make(map[string]planner.PayloadReference, len(change.PayloadReferences))
	for _, binding := range change.PayloadReferences {
		if binding.Selector != planner.FieldID && binding.Selector != "ID" {
			return fmt.Errorf("%s %q field %s: unsupported deferred selector %q",
				change.ResourceType, change.ResourceRef, binding.Path, binding.Selector)
		}
		if unresolvedReferenceID(binding.ID) {
			for _, dependency := range change.DependsOn {
				target := findPlannedChangeByID(plan, dependency)
				if target == nil || target.ResourceType != binding.ResourceType || target.ResourceRef != binding.Ref {
					continue
				}
				if id, _, ok := e.getCreatedResource(dependency); ok {
					binding.ID = id
				}
			}
		}
		if unresolvedReferenceID(binding.ID) {
			return fmt.Errorf("%s %q field %s: unresolved reference to %s %q",
				change.ResourceType, change.ResourceRef, binding.Path, binding.ResourceType, binding.Ref)
		}
		bindings[binding.Path] = binding
	}
	if err := values.Transform(change.Fields, func(path, value string) (any, error) {
		binding, ok := bindings[path]
		if !ok {
			return value, nil
		}
		expression := tags.RefPlaceholderPrefix + binding.Ref + "#" + binding.Selector
		if value != expression && value != binding.ID {
			return nil, fmt.Errorf("payload reference binding does not match destination expression")
		}
		delete(bindings, path)
		return binding.ID, nil
	}); err != nil {
		return err
	}
	if len(bindings) != 0 {
		return fmt.Errorf("%s %q: payload reference destination is missing", change.ResourceType, change.ResourceRef)
	}
	return nil
}

// validateResolvedPayload runs on the mapped SDK request immediately before a
// mutation. Environment and secret placeholders have separate resolution rules.
func validateResolvedPayload(request any) error {
	return validateResolvedRequest(context.Background(), request)
}

func validateResolvedRequest(ctx context.Context, request any) error {
	deferred, _ := ctx.Value(deferredPayloadPathsKey{}).(map[string]bool)
	return values.Transform(request, func(path string, value string) (any, error) {
		if deferred[path] {
			return value, nil
		}
		if tags.IsRefPlaceholder(value) || tags.IsExternalPlaceholder(value) {
			return nil, fmt.Errorf("unresolved reference expression in request payload")
		}
		return value, nil
	})
}

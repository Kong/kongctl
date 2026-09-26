package planner

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/values"
)

// Keep the legacy relationship resolver from inferring a type from arbitrary
// payload keys. New bindings and literal provenance already own these values.
func payloadReferenceOwnsField(change PlannedChange, dottedPath string) bool {
	for _, binding := range change.PayloadReferences {
		if strings.Join(decodeJSONPointer(binding.Path), ".") == dottedPath {
			return true
		}
	}
	for _, path := range change.LiteralPaths {
		if strings.Join(decodeJSONPointer(path), ".") == dottedPath {
			return true
		}
	}
	return false
}

// Resolve known values before resource-specific comparison, including identities
// whose declarations need no operation in this plan.
func (p *Planner) resolveKnownPayloadReferences(ctx context.Context, rs *resources.ResourceSet) (bool, error) {
	changed := false
	for _, resource := range rs.AllResources() {
		err := values.Transform(resource, func(path string, value string) (any, error) {
			if rs.GetEnvSources(resource.GetRef())[path] != "" || rs.LiteralSources[resource.GetRef()][path] != "" {
				return value, nil
			}
			if tags.IsExternalPlaceholder(value) && !resources.IsRelationshipPath(resource, path) {
				lookup, ok := tags.ParseExternalPlaceholder(value)
				if !ok {
					return nil, fmt.Errorf("invalid lookup expression")
				}
				return p.resolvePayloadLookup(ctx, rs, lookup,
					fmt.Sprintf("%s %q field %s", resource.GetType(), resource.GetRef(), path))
			}
			if !tags.IsRefPlaceholder(value) {
				return value, nil
			}
			if resources.IsRelationshipPath(resource, path) {
				_, err := rs.ResolvePayloadReference(value)
				return value, err
			}
			if source := rs.PayloadReferenceLiteralSource(value); source != "" {
				rs.AddLiteralSource(resource.GetRef(), path, source)
			}
			if source := rs.PayloadReferenceEnvSource(value); source != "" {
				rs.AddEnvSource(resource.GetRef(), path, source)
			}
			resolved, err := rs.ResolvePayloadReference(value)
			if err == nil && resolved != value {
				changed = true
			}
			return resolved, err
		})
		if err != nil {
			return false, fmt.Errorf("%s %q: %w", resource.GetType(), resource.GetRef(), err)
		}
	}
	return changed, nil
}

func (p *Planner) bindPayloadReferences(plan *Plan, rs *resources.ResourceSet) error {
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Action != ActionCreate && change.Action != ActionUpdate {
			continue
		}
		source, _ := rs.ReferenceTarget(change.ResourceRef)
		err := values.Transform(change.Fields, func(path, expression string) (any, error) {
			if rs.GetEnvSources(change.ResourceRef)[path] != "" {
				return expression, nil
			}
			if rs.LiteralSources[change.ResourceRef][path] != "" {
				change.LiteralPaths = append(change.LiteralPaths, path)
				return expression, nil
			}
			if !tags.IsRefPlaceholder(expression) || (source != nil &&
				(resources.IsRelationshipPath(source, path) || resources.IsEventGatewayReferencePath(source, path))) {
				return expression, nil
			}
			ref, selector, ok := tags.ParseRefPlaceholder(expression)
			if !ok {
				return nil, fmt.Errorf("invalid reference expression")
			}
			target, err := rs.ReferenceTarget(ref)
			if err != nil {
				return nil, err
			}
			resolved, err := rs.ResolvePayloadReference(expression)
			if err != nil {
				return nil, err
			}
			if resolved != expression {
				return resolved, nil
			}
			binding := PayloadReference{Path: path, ResourceType: string(target.GetType()), Ref: ref, Selector: selector}
			creation := slices.IndexFunc(plan.Changes, func(candidate PlannedChange) bool {
				return candidate.ResourceType == binding.ResourceType && candidate.ResourceRef == ref &&
					candidate.Action == ActionCreate
			})
			if creation < 0 {
				return nil, fmt.Errorf("reference %q selector %q has no resolved value or creation operation", ref, selector)
			}
			dependency := plan.Changes[creation].ID
			if !slices.Contains(change.DependsOn, dependency) {
				change.DependsOn = append(change.DependsOn, dependency)
			}
			change.PayloadReferences = append(change.PayloadReferences, binding)
			return expression, nil
		})
		if err != nil {
			return fmt.Errorf("%s %q: %w", change.ResourceType, change.ResourceRef, err)
		}
	}
	return nil
}

// Resolve a parent lookup before its child so each request retains its own scope
// and selector sensitivity in the shared lookup resolver.
func (p *Planner) resolvePayloadLookup(
	ctx context.Context, rs *resources.ResourceSet, lookup tags.ExternalLookup, source string,
) (string, error) {
	if lookup.ResourceType == "" {
		return "", fmt.Errorf("%s: lookup in an arbitrary payload value requires resource_type", source)
	}
	resourceType := resources.ResourceType(lookup.ResourceType)
	capability, ok := resources.ExternalResolutionFor(resourceType)
	if !ok {
		return "", fmt.Errorf("%s: resource type %q does not support external lookup", source, resourceType)
	}
	if lookup.Parent != nil && lookup.ParentRef != "" {
		return "", fmt.Errorf("%s: lookup parent and parent_ref are mutually exclusive", source)
	}
	parentID := ""
	if lookup.Parent != nil {
		if capability.ParentType == "" {
			return "", fmt.Errorf("%s: lookup resource type %s does not accept parent scope", source, resourceType)
		}
		if lookup.Parent.ResourceType != string(capability.ParentType) {
			return "", fmt.Errorf("%s: lookup parent resource_type must be %s", source, capability.ParentType)
		}
		var err error
		parentID, err = p.resolvePayloadLookup(ctx, rs, *lookup.Parent, source+" parent")
		if err != nil {
			return "", err
		}
	}
	if lookup.ParentRef != "" {
		parent, err := rs.ReferenceTarget(lookup.ParentRef)
		if err != nil {
			return "", err
		}
		if parent.GetType() != capability.ParentType {
			return "", fmt.Errorf("lookup parent_ref must identify a %s declaration", capability.ParentType)
		}
		parentID = parent.GetKonnectID()
	}
	return p.externalResolver.resolve(ctx, externalLookupRequest{
		ResourceType: resourceType, MatchFields: lookup.MatchFields,
		SensitiveFields: lookup.SensitiveFields, ParentID: parentID, Source: source,
	})
}

type matchedResourceIdentity struct {
	resourceType resources.ResourceType
	observation  any
}

// Record scoped matches without changing legacy relationship resolution.
func (p *Planner) recordMatchedIdentity(desired, current any) {
	if p == nil || p.matchedIdentities == nil {
		return
	}
	resource, ok := desired.(resources.Resource)
	if !ok {
		return
	}
	p.matchedIdentities[resource.GetRef()] = matchedResourceIdentity{resource.GetType(), current}
}

// Reconciliation uses filtered copies. Copy identities back to original
// declarations only for ad-hoc payload targets, including unchanged targets.
func (p *Planner) resolveMatchedPayloadIdentities(plan *Plan, rs *resources.ResourceSet) {
	for _, change := range plan.Changes {
		if change.Action != ActionCreate && change.Action != ActionUpdate {
			continue
		}
		source, _ := rs.ReferenceTarget(change.ResourceRef)
		// This visitor records known identities; reference validation remains in
		// resolveKnownPayloadReferences and bindPayloadReferences.
		_ = values.Transform(change.Fields, func(path, value string) (any, error) {
			if rs.GetEnvSources(change.ResourceRef)[path] != "" || rs.LiteralSources[change.ResourceRef][path] != "" ||
				(source != nil && (resources.IsRelationshipPath(source, path) ||
					resources.IsEventGatewayReferencePath(source, path))) {
				return value, nil
			}
			ref, selector, ok := tags.ParseRefPlaceholder(value)
			if !ok || (selector != FieldID && selector != "ID") {
				return value, nil
			}
			matched, ok := p.matchedIdentities[ref]
			if !ok {
				return value, nil
			}
			target, err := rs.ReferenceTarget(ref)
			if err == nil && target.GetType() == matched.resourceType {
				matchObservedIdentity(target, matched.observation)
			}
			return value, nil
		})
	}
}

func matchObservedIdentity(target resources.Resource, current any) bool {
	if target.TryMatchKonnectResource(current) {
		return true
	}
	// State observations can embed SDK union types whose resource matcher
	// requires the SDK value itself rather than its normalized state wrapper.
	v := reflect.Indirect(reflect.ValueOf(current))
	if v.Kind() != reflect.Struct {
		return false
	}
	for i := range v.NumField() {
		field := v.Field(i)
		if v.Type().Field(i).Anonymous && field.CanInterface() && matchObservedIdentity(target, field.Interface()) {
			return true
		}
	}
	return false
}

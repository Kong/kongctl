package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/secrets"
)

const deprecatedBareEnvSecretWarning = "write-only field uses deprecated bare !env syntax; wrap it with !secret"

type secretSelector struct {
	resourceType string
	resourceRef  string
	field        string
	matched      bool
}

func (p *Planner) applySecretWriteIntents(
	ctx context.Context,
	plan *Plan,
	rs *resources.ResourceSet,
	opts Options,
) error {
	if plan == nil || rs == nil {
		return nil
	}
	if opts.Mode == PlanModeDelete && (opts.WriteSecrets || len(opts.WriteSecretSelectors) > 0) {
		return fmt.Errorf("secret-write selection is not supported in delete mode")
	}

	selectors, err := parseSecretSelectors(opts.WriteSecretSelectors, rs)
	if err != nil {
		return err
	}

	for resourceRef, declarations := range rs.SecretSources {
		resource, ok := rs.GetResourceByRef(resourceRef)
		if !ok {
			return fmt.Errorf("secret source resource %q was not found", resourceRef)
		}

		change := findPlannedResourceChange(plan, string(resource.GetType()), resourceRef)
		isCreate := change != nil && change.Action == ActionCreate
		existedRemotely := resource.GetKonnectID() != ""
		selected := make([]SecretWriteIntent, 0, len(declarations))
		for field, declaration := range declarations {
			capability, supported := secrets.Match(resource.GetType(), field)
			if !supported {
				return fmt.Errorf("resource %s %q field %s is not a supported write-only field",
					resource.GetType(), resourceRef, field)
			}

			if declaration.DeprecatedBareEnv {
				warningID := changeID(change)
				if warningID == "" {
					warningID = resourceRef
				}
				plan.AddWarning(warningID, fmt.Sprintf(
					"%s %q %s: %s",
					resource.GetType(), resourceRef, field, deprecatedBareEnvSecretWarning,
				))
			}

			requested := opts.WriteSecrets || matchesSecretSelector(selectors, resource, field)
			if !isCreate && !requested {
				continue
			}
			if isCreate && !capability.Create {
				return fmt.Errorf("resource %s %q field %s cannot be written on create",
					resource.GetType(), resourceRef, field)
			}
			if isCreate && existedRemotely && requested && !capability.Update {
				return fmt.Errorf(
					"resource %s %q field %s is create-only and belongs to an existing resource; "+
						"declare a new resource to rotate it",
					resource.GetType(), resourceRef, field,
				)
			}
			if !isCreate && !capability.Update {
				return fmt.Errorf(
					"resource %s %q field %s is create-only; declare a new resource to rotate it",
					resource.GetType(), resourceRef, field,
				)
			}

			selected = append(selected, SecretWriteIntent{Field: field, Expression: declaration.Expression})
		}

		if len(selected) == 0 {
			if change != nil {
				stripDeclaredSecretPaths(change, declarations)
			}
			continue
		}
		slices.SortFunc(selected, func(a, b SecretWriteIntent) int { return strings.Compare(a.Field, b.Field) })

		if change == nil {
			resourceID, err := p.resolveSecretResourceID(ctx, rs, resource)
			if err != nil {
				return err
			}
			if resourceID == "" {
				return fmt.Errorf("resource %s %q could not be resolved for a secret-only update",
					resource.GetType(), resourceRef)
			}
			fields, err := secretResourceFields(resource)
			if err != nil {
				return err
			}
			change = &PlannedChange{
				ID:           p.nextChangeID(ActionUpdate, string(resource.GetType()), resourceRef),
				ResourceType: string(resource.GetType()),
				ResourceRef:  resourceRef,
				ResourceID:   resourceID,
				Action:       ActionUpdate,
				Fields:       fields,
				Namespace:    secretResourceNamespace(rs, resource),
				Parent:       secretResourceParent(rs, resource),
			}
			plan.AddChange(*change)
			change = &plan.Changes[len(plan.Changes)-1]
		} else {
			fields, err := secretResourceFields(resource)
			if err != nil {
				return err
			}
			change.Fields = mergeSecretContextFields(fields, change.Fields)
		}

		change.SecretWrites = append(change.SecretWrites, selected...)
		stripDeclaredSecretPaths(change, declarations)
	}

	for _, selector := range selectors {
		if !selector.matched {
			return fmt.Errorf("--write-secret selector %q did not match a configured write-only field",
				formatSecretSelector(selector))
		}
	}
	plan.UpdateSummary()
	return nil
}

func parseSecretSelectors(raw []string, rs *resources.ResourceSet) ([]*secretSelector, error) {
	selectors := make([]*secretSelector, 0, len(raw))
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("--write-secret selector cannot be empty")
		}
		resourcePart, field, _ := strings.Cut(value, "#")
		resourceType, resourceRef := "", resourcePart
		if before, after, found := strings.Cut(resourcePart, ":"); found {
			resourceType, resourceRef = strings.TrimSpace(before), strings.TrimSpace(after)
		}
		resourceRef = strings.TrimSpace(resourceRef)
		field = strings.TrimSpace(field)
		resource, ok := rs.GetResourceByRef(resourceRef)
		if !ok {
			return nil, fmt.Errorf("--write-secret resource %q was not found in the configuration", resourceRef)
		}
		if resourceType != "" && resourceType != string(resource.GetType()) {
			return nil, fmt.Errorf("--write-secret resource %q has type %s, not %s",
				resourceRef, resource.GetType(), resourceType)
		}
		selectors = append(selectors, &secretSelector{
			resourceType: resourceType,
			resourceRef:  resourceRef,
			field:        field,
		})
	}
	return selectors, nil
}

func matchesSecretSelector(selectors []*secretSelector, resource resources.Resource, field string) bool {
	matched := false
	for _, selector := range selectors {
		if selector.resourceRef != resource.GetRef() {
			continue
		}
		if selector.resourceType != "" && selector.resourceType != string(resource.GetType()) {
			continue
		}
		if selector.field != "" && !secretFieldSelectorMatches(selector.field, field) {
			continue
		}
		selector.matched = true
		matched = true
	}
	return matched
}

var indexedSecretField = regexp.MustCompile(`\[\d+\]`)

func secretFieldSelectorMatches(selector, pointer string) bool {
	if strings.HasPrefix(selector, "/") {
		return selector == pointer
	}
	dotted := pointerToSecretField(pointer)
	return selector == dotted || selector == indexedSecretField.ReplaceAllString(dotted, "[]")
}

func pointerToSecretField(pointer string) string {
	segments := decodeJSONPointer(pointer)
	var result strings.Builder
	for _, segment := range segments {
		if _, err := strconv.Atoi(segment); err == nil {
			fmt.Fprintf(&result, "[%s]", segment)
			continue
		}
		if result.Len() > 0 {
			result.WriteByte('.')
		}
		result.WriteString(segment)
	}
	return result.String()
}

func formatSecretSelector(selector *secretSelector) string {
	prefix := selector.resourceRef
	if selector.resourceType != "" {
		prefix = selector.resourceType + ":" + prefix
	}
	if selector.field != "" {
		prefix += "#" + selector.field
	}
	return prefix
}

func findPlannedResourceChange(plan *Plan, resourceType, resourceRef string) *PlannedChange {
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.ResourceType == resourceType && change.ResourceRef == resourceRef && change.Action != ActionDelete {
			return change
		}
	}
	return nil
}

func changeID(change *PlannedChange) string {
	if change == nil {
		return ""
	}
	return change.ID
}

func secretResourceFields(resource resources.Resource) (map[string]any, error) {
	data, err := json.Marshal(resource)
	if err != nil {
		return nil, fmt.Errorf("failed to build secret write context for %s %q: %w",
			resource.GetType(), resource.GetRef(), err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("failed to decode secret write context for %s %q: %w",
			resource.GetType(), resource.GetRef(), err)
	}
	delete(fields, "ref")
	delete(fields, "kongctl")
	if resource.GetType() == resources.ResourceTypeDCRProvider {
		if providerType, ok := fields[FieldDCRProviderProviderType]; ok {
			fields[FieldDCRProviderUpdateType] = providerType
		}
	}
	return fields, nil
}

func mergeSecretContextFields(contextFields, plannedFields map[string]any) map[string]any {
	merged := make(map[string]any)
	maps.Copy(merged, contextFields)
	maps.Copy(merged, plannedFields)
	return merged
}

func stripDeclaredSecretPaths(
	change *PlannedChange,
	declarations map[string]resources.SecretSourceDeclaration,
) {
	for field := range declarations {
		removeSecretPath(change.Fields, decodeJSONPointer(field))
		removeSecretChangedPath(change.ChangedFields, decodeJSONPointer(field))
	}
}

func removeSecretChangedPath(changed map[string]FieldChange, segments []string) {
	if len(segments) == 0 {
		return
	}
	fieldChange, ok := changed[segments[0]]
	if !ok {
		return
	}
	if len(segments) == 1 {
		delete(changed, segments[0])
		return
	}
	removeSecretPathValue(fieldChange.New, segments[1:])
	changed[segments[0]] = fieldChange
}

func removeSecretPath(fields map[string]any, segments []string) {
	if len(segments) == 0 {
		return
	}
	if len(segments) == 1 {
		delete(fields, segments[0])
		return
	}
	removeSecretPathValue(fields[segments[0]], segments[1:])
}

func removeSecretPathValue(value any, segments []string) {
	if len(segments) == 0 {
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		if len(segments) == 1 {
			delete(typed, segments[0])
			return
		}
		removeSecretPathValue(typed[segments[0]], segments[1:])
	case []any:
		index, err := strconv.Atoi(segments[0])
		if err != nil || index < 0 || index >= len(typed) {
			return
		}
		if len(segments) == 1 {
			typed[index] = nil
			return
		}
		removeSecretPathValue(typed[index], segments[1:])
	}
}

func secretResourceNamespace(rs *resources.ResourceSet, resource resources.Resource) string {
	switch typed := resource.(type) {
	case *resources.DCRProviderResource:
		return resources.GetNamespace(typed.Kongctl)
	case *resources.PortalResource:
		return resources.GetNamespace(typed.Kongctl)
	case *resources.AIGatewayResource:
		return resources.GetNamespace(typed.Kongctl)
	case *resources.EventGatewayControlPlaneResource:
		return resources.GetNamespace(typed.Kongctl)
	case resources.ResourceWithParent:
		parent := typed.GetParentRef()
		if parent != nil {
			if parentResource, ok := rs.GetResourceByRef(parent.Ref); ok {
				return secretResourceNamespace(rs, parentResource)
			}
		}
	}
	return DefaultNamespace
}

func secretResourceParent(rs *resources.ResourceSet, resource resources.Resource) *ParentInfo {
	child, ok := resource.(resources.ResourceWithParent)
	if !ok || child.GetParentRef() == nil {
		return nil
	}
	parentRef := child.GetParentRef()
	parent, ok := rs.GetResourceByRef(parentRef.Ref)
	if !ok {
		return &ParentInfo{Ref: parentRef.Ref}
	}
	return &ParentInfo{Ref: parentRef.Ref, ID: parent.GetKonnectID()}
}

func (p *Planner) resolveSecretResourceID(
	ctx context.Context,
	rs *resources.ResourceSet,
	resource resources.Resource,
) (string, error) {
	if id := resource.GetKonnectID(); id != "" {
		return id, nil
	}
	switch typed := resource.(type) {
	case *resources.DCRProviderResource:
		current, err := p.client.GetDCRProviderByName(ctx, typed.Name)
		if err != nil || current == nil {
			return "", err
		}
		return current.ID, nil
	case *resources.PortalIdentityProviderResource:
		parent := secretResourceParent(rs, resource)
		if parent == nil || parent.ID == "" {
			return "", fmt.Errorf("portal identity provider %q has no resolved portal", typed.Ref)
		}
		current, err := p.client.ListPortalIdentityProviders(ctx, parent.ID)
		if err != nil {
			return "", err
		}
		for _, candidate := range current {
			if typed.Type != nil && candidate.Type == *typed.Type {
				return candidate.ID, nil
			}
		}
	case *resources.AIGatewayProviderResource:
		return p.resolveAIGatewayProviderSecretID(ctx, rs, resource, typed.Name)
	case *resources.AIGatewayIdentityProviderResource:
		parent := secretResourceParent(rs, resource)
		if parent == nil || parent.ID == "" {
			return "", fmt.Errorf("AI Gateway identity provider %q has no resolved gateway", typed.Ref)
		}
		current, err := p.client.ListAIGatewayIdentityProviders(ctx, parent.ID)
		if err != nil {
			return "", err
		}
		for _, candidate := range current {
			if candidate.Name == typed.Name {
				return candidate.ID, nil
			}
		}
	case *resources.AIGatewayVaultResource:
		parent := secretResourceParent(rs, resource)
		if parent == nil || parent.ID == "" {
			return "", fmt.Errorf("AI Gateway vault %q has no resolved gateway", typed.Ref)
		}
		current, err := p.client.ListAIGatewayVaults(ctx, parent.ID)
		if err != nil {
			return "", err
		}
		for _, candidate := range current {
			if resources.AIGatewayVaultName(candidate.AIGatewayVault) == typed.Name() {
				return resources.AIGatewayVaultID(candidate.AIGatewayVault), nil
			}
		}
	case *resources.EventGatewaySchemaRegistryResource:
		parent := secretResourceParent(rs, resource)
		if parent == nil || parent.ID == "" {
			return "", fmt.Errorf("event gateway schema registry %q has no resolved gateway", typed.Ref)
		}
		current, err := p.client.ListEventGatewaySchemaRegistries(ctx, parent.ID)
		if err != nil {
			return "", err
		}
		for _, candidate := range current {
			if candidate.Name == typed.GetMoniker() {
				return candidate.ID, nil
			}
		}
	case *resources.AIGatewayConsumerCredentialResource:
		parent := secretResourceParent(rs, resource)
		consumer := rs.GetAIGatewayConsumerByRef(typed.AIGatewayConsumer)
		if parent == nil || parent.ID == "" || consumer == nil {
			return "", fmt.Errorf("AI Gateway consumer credential %q has no resolved consumer", typed.Ref)
		}
		gateway := rs.GetAIGatewayByRef(consumer.AIGateway)
		if gateway == nil || gateway.GetKonnectID() == "" {
			return "", fmt.Errorf("AI Gateway consumer credential %q has no resolved gateway", typed.Ref)
		}
		current, err := p.client.ListAIGatewayConsumerCredentials(ctx, gateway.GetKonnectID(), parent.ID)
		if err != nil {
			return "", err
		}
		for _, candidate := range current {
			if resources.AIGatewayConsumerCredentialName(candidate.AIGatewayConsumerCredential) == typed.Name {
				return resources.AIGatewayConsumerCredentialID(candidate.AIGatewayConsumerCredential), nil
			}
		}
	}
	return "", nil
}

func (p *Planner) resolveAIGatewayProviderSecretID(
	ctx context.Context,
	rs *resources.ResourceSet,
	resource resources.Resource,
	name string,
) (string, error) {
	parent := secretResourceParent(rs, resource)
	if parent == nil || parent.ID == "" {
		return "", fmt.Errorf("AI Gateway model provider %q has no resolved gateway", resource.GetRef())
	}
	current, err := p.client.ListAIGatewayProviders(ctx, parent.ID)
	if err != nil {
		return "", err
	}
	for _, candidate := range current {
		if candidate.Name == name {
			return candidate.ID, nil
		}
	}
	return "", nil
}

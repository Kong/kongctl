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
	skipped, err := validateSecretWriteSelection(plan, rs, opts, selectors)
	if err != nil {
		return err
	}

	for _, resourceRef := range slices.Sorted(maps.Keys(rs.SecretSources)) {
		declarations := rs.SecretSources[resourceRef]
		resource, ok := rs.GetResourceByRef(resourceRef)
		if !ok {
			return fmt.Errorf("secret source resource %q was not found", resourceRef)
		}

		change := findPlannedResourceChange(plan, string(resource.GetType()), resourceRef)
		isCreate := change != nil && change.Action == ActionCreate
		selected := make([]SecretWriteIntent, 0, len(declarations))
		for _, field := range slices.Sorted(maps.Keys(declarations)) {
			declaration := declarations[field]
			if _, skip := skipped[secretWriteTarget{resourceRef: resourceRef, field: field}]; skip {
				continue
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

			selected = append(selected, SecretWriteIntent{Field: field, Expression: declaration.Expression})
		}

		if len(selected) == 0 {
			if change != nil {
				if err := prepareDeclaredSecretPaths(change, declarations, nil); err != nil {
					return err
				}
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
			fields, err := secretResourceFields(resource, ActionUpdate)
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
			fields, err := secretResourceFields(resource, change.Action)
			if err != nil {
				return err
			}
			change.Fields = mergeSecretContextFields(fields, change.Fields)
		}

		if err := prepareDeclaredSecretPaths(change, declarations, selected); err != nil {
			return fmt.Errorf("resource %s %q: %w", resource.GetType(), resourceRef, err)
		}
		change.SecretWrites = append(change.SecretWrites, selected...)
	}

	plan.UpdateSummary()
	if opts.WriteSecrets && plan.Summary.SecretWrites == 0 {
		plan.AddWarning("", "--write-secrets did not select any writable secret fields")
	}
	return nil
}

type pendingSecretWriteWarning struct {
	changeID string
	message  string
}

type secretWriteTarget struct {
	resourceRef string
	field       string
}

func validateSecretWriteSelection(
	plan *Plan,
	rs *resources.ResourceSet,
	opts Options,
	selectors []*secretSelector,
) (map[secretWriteTarget]struct{}, error) {
	problems := make([]string, 0)
	warnings := make([]pendingSecretWriteWarning, 0)
	skipped := make(map[secretWriteTarget]struct{})
	for resourceRef, declarations := range rs.SecretSources {
		resource, ok := rs.GetResourceByRef(resourceRef)
		if !ok {
			problems = append(problems, fmt.Sprintf("secret source resource %q was not found", resourceRef))
			continue
		}

		change := findPlannedResourceChange(plan, string(resource.GetType()), resourceRef)
		isCreate := change != nil && change.Action == ActionCreate
		existedRemotely := resource.GetKonnectID() != ""
		for field := range declarations {
			capability, supported := secrets.Match(resource.GetType(), field)
			if !supported {
				problems = append(problems, fmt.Sprintf(
					"resource %s %q field %s is not a supported write-only field",
					resource.GetType(), resourceRef, field,
				))
				continue
			}

			explicitlyRequested := matchesSecretSelector(selectors, resource, field)
			requested := opts.WriteSecrets || explicitlyRequested
			if !isCreate && !requested {
				continue
			}
			switch {
			case isCreate && !capability.Create:
				problems = append(problems, fmt.Sprintf(
					"resource %s %q field %s cannot be written on create",
					resource.GetType(), resourceRef, field,
				))
			case isCreate && existedRemotely && requested && !capability.Update:
				message := fmt.Sprintf(
					"resource %s %q field %s is create-only and belongs to an existing resource; "+
						"declare a new resource to rotate it",
					resource.GetType(), resourceRef, field,
				)
				if opts.WriteSecrets && !explicitlyRequested {
					skipped[secretWriteTarget{resourceRef: resourceRef, field: field}] = struct{}{}
					warnings = append(warnings, pendingSecretWriteWarning{
						changeID: secretWriteWarningID(change, resourceRef),
						message:  "--write-secrets skipped " + message,
					})
				} else {
					problems = append(problems, message)
				}
			case !isCreate && !capability.Update:
				message := fmt.Sprintf(
					"resource %s %q field %s is create-only; declare a new resource to rotate it",
					resource.GetType(), resourceRef, field,
				)
				if opts.WriteSecrets && !explicitlyRequested {
					skipped[secretWriteTarget{resourceRef: resourceRef, field: field}] = struct{}{}
					warnings = append(warnings, pendingSecretWriteWarning{
						changeID: secretWriteWarningID(change, resourceRef),
						message:  "--write-secrets skipped " + message,
					})
				} else {
					problems = append(problems, message)
				}
			}
		}
	}

	for _, selector := range selectors {
		if !selector.matched {
			problems = append(problems, fmt.Sprintf(
				"--write-secret selector %q did not match a configured write-only field",
				formatSecretSelector(selector),
			))
		}
	}
	if len(problems) > 0 {
		slices.Sort(problems)
		return nil, fmt.Errorf("secret write selection cannot be completed:\n  - %s", strings.Join(problems, "\n  - "))
	}

	slices.SortFunc(warnings, func(a, b pendingSecretWriteWarning) int {
		return strings.Compare(a.message, b.message)
	})
	for _, warning := range warnings {
		plan.AddWarning(warning.changeID, warning.message)
	}
	return skipped, nil
}

func secretWriteWarningID(change *PlannedChange, resourceRef string) string {
	if id := changeID(change); id != "" {
		return id
	}
	return resourceRef
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

func secretResourceFields(resource resources.Resource, action ActionType) (map[string]any, error) {
	var fields map[string]any
	if payloadResource, ok := resource.(interface {
		MutablePayloadMap() (map[string]any, error)
	}); ok {
		var err error
		fields, err = payloadResource.MutablePayloadMap()
		if err != nil {
			return nil, fmt.Errorf("failed to build secret write context for %s %q: %w",
				resource.GetType(), resource.GetRef(), err)
		}
	} else {
		data, err := json.Marshal(resource)
		if err != nil {
			return nil, fmt.Errorf("failed to build secret write context for %s %q: %w",
				resource.GetType(), resource.GetRef(), err)
		}
		if err := json.Unmarshal(data, &fields); err != nil {
			return nil, fmt.Errorf("failed to decode secret write context for %s %q: %w",
				resource.GetType(), resource.GetRef(), err)
		}
	}

	delete(fields, resources.SchemaFieldRef)
	delete(fields, resources.SchemaFieldKongctl)
	for _, descriptor := range resources.RelationshipDescriptorsForType(resource.GetType()) {
		if descriptor.Kind == resources.RelationshipKindKongctlParentSelector {
			removeSecretContextPath(fields, strings.Split(descriptor.FieldPath, "."))
		}
	}
	if resource.GetType() == resources.ResourceTypeDCRProvider && action == ActionUpdate {
		if providerType, ok := fields[FieldDCRProviderProviderType]; ok {
			fields[FieldDCRProviderUpdateType] = providerType
		}
	}
	if resource.GetType() == resources.ResourceTypePortalIdentityProvider && action == ActionUpdate {
		delete(fields, FieldType)
	}
	return fields, nil
}

func removeSecretContextPath(fields map[string]any, path []string) {
	if len(path) == 0 {
		return
	}
	if len(path) == 1 {
		delete(fields, path[0])
		return
	}
	child, ok := fields[path[0]].(map[string]any)
	if !ok {
		return
	}
	removeSecretContextPath(child, path[1:])
}

func mergeSecretContextFields(contextFields, plannedFields map[string]any) map[string]any {
	merged := make(map[string]any)
	maps.Copy(merged, contextFields)
	maps.Copy(merged, plannedFields)
	return merged
}

func prepareDeclaredSecretPaths(
	change *PlannedChange,
	declarations map[string]resources.SecretSourceDeclaration,
	selected []SecretWriteIntent,
) error {
	selectedFields := make(map[string]struct{}, len(selected))
	for _, intent := range selected {
		selectedFields[intent.Field] = struct{}{}
	}

	type arrayGroup struct {
		prefix   []string
		paths    map[string][]string
		selected int
	}
	groups := make(map[string]*arrayGroup)
	paths := make(map[string][]string, len(declarations))
	for field := range declarations {
		path := decodeJSONPointer(field)
		paths[field] = path
		prefix, ok := secretArrayPrefix(path)
		if !ok {
			continue
		}
		key := strings.Join(prefix, "\x00")
		group := groups[key]
		if group == nil {
			group = &arrayGroup{prefix: prefix, paths: make(map[string][]string)}
			groups[key] = group
		}
		group.paths[field] = path
		if _, ok := selectedFields[field]; ok {
			group.selected++
		}
	}

	groupedFields := make(map[string]struct{})
	for _, key := range slices.Sorted(maps.Keys(groups)) {
		group := groups[key]
		for field := range group.paths {
			groupedFields[field] = struct{}{}
		}
		switch {
		case group.selected == 0:
			removeSecretPath(change.Fields, group.prefix)
			removeSecretChangedPath(change.ChangedFields, group.prefix)
		case group.selected != len(group.paths):
			return fmt.Errorf(
				"write-only array field /%s must be written as a complete group; select all configured values",
				strings.Join(group.prefix, "/"),
			)
		default:
			for _, field := range slices.Sorted(maps.Keys(group.paths)) {
				path := group.paths[field]
				if !maskDirectSecretArrayElement(change.Fields, path) {
					removeSecretPath(change.Fields, path)
				}
			}
			removeSecretChangedPath(change.ChangedFields, group.prefix)
		}
	}

	nonArrayPaths := make([][]string, 0, len(paths)-len(groupedFields))
	for field, path := range paths {
		if _, grouped := groupedFields[field]; !grouped {
			nonArrayPaths = append(nonArrayPaths, path)
		}
	}
	slices.SortFunc(nonArrayPaths, compareSecretRemovalPaths)
	for _, path := range nonArrayPaths {
		removeSecretPath(change.Fields, path)
		removeSecretChangedPath(change.ChangedFields, path)
	}
	return nil
}

func secretArrayPrefix(segments []string) ([]string, bool) {
	for i, segment := range segments {
		if _, err := strconv.Atoi(segment); err == nil {
			return slices.Clone(segments[:i]), true
		}
	}
	return nil, false
}

func maskDirectSecretArrayElement(fields map[string]any, segments []string) bool {
	var current any = fields
	for i, segment := range segments {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return false
			}
			if i == len(segments)-1 {
				typed[index] = nil
				return true
			}
			current = typed[index]
		default:
			return false
		}
	}
	return false
}

func projectPublicVaultReferences(value any) (any, bool) {
	switch typed := value.(type) {
	case string:
		if secrets.IsVaultReference(typed) {
			return typed, true
		}
	case []string:
		projected := make([]any, len(typed))
		found := false
		for i, entry := range typed {
			if secrets.IsVaultReference(entry) {
				projected[i] = entry
				found = true
			}
		}
		return projected, found
	case []any:
		projected := make([]any, len(typed))
		found := false
		for i, entry := range typed {
			if reference, ok := projectPublicVaultReferences(entry); ok {
				projected[i] = reference
				found = true
			}
		}
		return projected, found
	}
	return nil, false
}

// Array elements must be removed from the highest index down so earlier
// removals do not shift the indexes of fields that have not been removed yet.
func compareSecretRemovalPaths(a, b []string) int {
	for i := range min(len(a), len(b)) {
		if a[i] == b[i] {
			continue
		}
		aIndex, aErr := strconv.Atoi(a[i])
		bIndex, bErr := strconv.Atoi(b[i])
		if aErr == nil && bErr == nil {
			return bIndex - aIndex
		}
		return strings.Compare(a[i], b[i])
	}
	return len(b) - len(a)
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
	updated, keep := removeSecretPathValue(fieldChange.New, segments[1:])
	if !keep {
		delete(changed, segments[0])
		return
	}
	fieldChange.New = updated
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
	value, ok := fields[segments[0]]
	if !ok {
		return
	}
	updated, keep := removeSecretPathValue(value, segments[1:])
	if !keep {
		delete(fields, segments[0])
		return
	}
	fields[segments[0]] = updated
}

func removeSecretPathValue(value any, segments []string) (any, bool) {
	if len(segments) == 0 {
		return value, true
	}
	switch typed := value.(type) {
	case map[string]any:
		if len(segments) == 1 {
			delete(typed, segments[0])
			return typed, true
		}
		child, ok := typed[segments[0]]
		if !ok {
			return typed, true
		}
		updated, keep := removeSecretPathValue(child, segments[1:])
		if !keep {
			delete(typed, segments[0])
		} else {
			typed[segments[0]] = updated
		}
		return typed, true
	case []any:
		index, err := strconv.Atoi(segments[0])
		if err != nil || index < 0 || index >= len(typed) {
			return typed, true
		}
		if len(segments) == 1 {
			typed = slices.Delete(typed, index, index+1)
			return typed, len(typed) > 0
		}
		updated, keep := removeSecretPathValue(typed[index], segments[1:])
		if !keep {
			typed = slices.Delete(typed, index, index+1)
		} else {
			typed[index] = updated
		}
		return typed, len(typed) > 0
	}
	return value, true
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

package planner

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

type externalLookupRequest struct {
	ResourceType    resources.ResourceType
	MatchFields     map[string]string
	SensitiveFields []string
	ParentID        string
	Source          string
}

type externalLookupCacheEntry struct {
	id  string
	err error
}

type externalLookupAdapter func(context.Context, externalLookupRequest) (string, error)

type inlineExternalParent struct {
	resourceType resources.ResourceType
	id           string
	ref          string
	parentID     string
	parentRef    string
}

type inlineLookupParent struct {
	id  string
	ref string
}

// externalLookupResolver owns all remote identity lookups for one plan generation.
type externalLookupResolver struct {
	planner          *Planner
	cache            map[string]externalLookupCacheEntry
	adapters         map[resources.ResourceType]externalLookupAdapter
	hasInlineParents bool
}

func newExternalLookupResolver(planner *Planner) *externalLookupResolver {
	r := &externalLookupResolver{
		planner: planner,
		cache:   make(map[string]externalLookupCacheEntry),
	}
	r.adapters = map[resources.ResourceType]externalLookupAdapter{
		resources.ResourceTypeAPI:                        r.lookupAPI,
		resources.ResourceTypeApplicationAuthStrategy:    r.lookupApplicationAuthStrategy,
		resources.ResourceTypePortal:                     r.lookupPortal,
		resources.ResourceTypeControlPlane:               r.lookupControlPlane,
		resources.ResourceTypeGatewayService:             r.lookupGatewayService,
		resources.ResourceTypeAIGateway:                  r.lookupAIGateway,
		resources.ResourceTypeAuditLogWebhookDestination: r.lookupAuditLogWebhookDestination,
		resources.ResourceTypeOrganizationTeam:           r.lookupOrganizationTeam,
		resources.ResourceTypeEventGatewayControlPlane:   r.lookupEventGatewayControlPlane,
		resources.ResourceTypeEventGatewayVirtualCluster: r.lookupEventGatewayVirtualCluster,
	}
	return r
}

func (r *externalLookupResolver) validateRegistry() error {
	for _, resourceType := range resources.ExternalResolvableTypes() {
		if r.adapters[resourceType] == nil {
			return fmt.Errorf("externally resolvable resource type %s has no planner lookup adapter", resourceType)
		}
	}
	for resourceType := range r.adapters {
		if _, ok := resources.ExternalResolutionFor(resourceType); !ok {
			return fmt.Errorf(
				"external lookup adapter registered for resource type %s without capability",
				resourceType,
			)
		}
	}
	return nil
}

func (r *externalLookupResolver) resolve(ctx context.Context, req externalLookupRequest) (string, error) {
	capability, ok := resources.ExternalResolutionFor(req.ResourceType)
	if !ok {
		return "", fmt.Errorf("%s: resource type %s does not support external lookup", req.Source, req.ResourceType)
	}
	if len(req.MatchFields) == 0 {
		return "", fmt.Errorf("%s: external lookup requires at least one selector", req.Source)
	}
	if id, hasID := req.MatchFields[FieldID]; hasID {
		if len(req.MatchFields) != 1 {
			return "", fmt.Errorf("%s: external lookup id cannot be combined with other selectors", req.Source)
		}
		if strings.TrimSpace(id) == "" {
			return "", fmt.Errorf("%s: external lookup id cannot be empty", req.Source)
		}
		return id, nil
	}
	for field := range req.MatchFields {
		if !capability.AllowAnyStringSelector && !slices.Contains(capability.Selectors, field) {
			return "", fmt.Errorf(
				"%s: external %s lookup does not support selector %q (supported: %s)",
				req.Source,
				req.ResourceType,
				field,
				strings.Join(capability.Selectors, ", "),
			)
		}
	}
	if capability.ParentType != "" && req.ParentID == "" {
		return "", fmt.Errorf(
			"%s: external %s lookup requires resolved %s scope",
			req.Source,
			req.ResourceType,
			capability.ParentType,
		)
	}

	key := string(req.ResourceType) + "|" + req.ParentID + "|" + tags.ExternalLookupKey(req.MatchFields)
	if cached, ok := r.cache[key]; ok {
		return cached.id, cached.err
	}

	adapter := r.adapters[req.ResourceType]
	if adapter == nil {
		return "", fmt.Errorf("%s: no external lookup adapter for %s", req.Source, req.ResourceType)
	}
	id, err := adapter(ctx, req)
	r.cache[key] = externalLookupCacheEntry{id: id, err: err}
	return id, err
}

func externalRequest(
	resourceType resources.ResourceType,
	external *resources.ExternalBlock,
	parentID string,
	source string,
) externalLookupRequest {
	matchFields := make(map[string]string)
	if external != nil {
		if external.ID != "" {
			matchFields[FieldID] = external.ID
		} else if external.Selector != nil {
			maps.Copy(matchFields, external.Selector.MatchFields)
		}
	}
	return externalLookupRequest{
		ResourceType: resourceType,
		MatchFields:  matchFields,
		ParentID:     parentID,
		Source:       source,
	}
}

func (r *externalLookupResolver) resolveDeclarations(ctx context.Context, rs *resources.ResourceSet) error {
	if err := r.validateRegistry(); err != nil {
		return err
	}

	// Resolve unscoped resources first so scoped resources can consume their IDs.
	for _, resourceType := range resources.ExternalResolvableTypesByScope(false) {
		for _, item := range rs.AllResourcesByType(resourceType) {
			external, ok := item.(resources.ExternallyResolvableResource)
			if !ok || external.GetExternalBlock() == nil || item.GetKonnectID() != "" {
				continue
			}
			id, err := r.resolve(ctx, externalRequest(
				item.GetType(), external.GetExternalBlock(), "", externalDeclarationSource(item),
			))
			if err != nil {
				return err
			}
			external.SetKonnectID(id)
		}
	}

	return nil
}

func externalDeclarationSource(resource resources.Resource) string {
	return fmt.Sprintf("%s %q _external", resource.GetType(), resource.GetRef())
}

func (r *externalLookupResolver) resolveScopedDeclarations(ctx context.Context, rs *resources.ResourceSet) error {
	controlPlaneByRef := make(map[string]*resources.ControlPlaneResource, len(rs.ControlPlanes))
	for i := range rs.ControlPlanes {
		controlPlaneByRef[rs.ControlPlanes[i].GetRef()] = &rs.ControlPlanes[i]
	}
	deckControlPlanes := deckControlPlaneRefs(rs.ControlPlanes)
	for _, resourceType := range resources.ExternalResolvableTypesByScope(true) {
		capability, _ := resources.ExternalResolutionFor(resourceType)
		for _, item := range rs.AllResourcesByType(resourceType) {
			external, ok := item.(resources.ExternallyResolvableResource)
			if !ok || external.GetExternalBlock() == nil || item.GetKonnectID() != "" {
				continue
			}

			parentID, deferred, err := r.scopedDeclarationParentID(
				rs, item, capability, controlPlaneByRef, deckControlPlanes,
			)
			if err != nil {
				return err
			}
			if deferred {
				continue
			}
			id, err := r.resolve(ctx, externalRequest(
				item.GetType(), external.GetExternalBlock(), parentID, externalDeclarationSource(item),
			))
			if err != nil {
				if service, ok := item.(*resources.GatewayServiceResource); ok &&
					controlPlaneHasDeck(service, deckControlPlanes) {
					continue
				}
				return err
			}
			external.SetKonnectID(id)
			if service, ok := item.(*resources.GatewayServiceResource); ok {
				service.SetResolvedControlPlaneID(parentID)
			}
		}
	}
	return nil
}

func (r *externalLookupResolver) scopedDeclarationParentID(
	rs *resources.ResourceSet,
	item resources.Resource,
	capability resources.ExternalResolutionRegistration,
	controlPlaneByRef map[string]*resources.ControlPlaneResource,
	deckControlPlanes map[string]bool,
) (parentID string, deferred bool, err error) {
	if service, ok := item.(*resources.GatewayServiceResource); ok {
		parentID, err := r.planner.resolveGatewayServiceControlPlaneID(service, controlPlaneByRef)
		if err != nil {
			return "", false, err
		}
		return parentID, parentID == "" && controlPlaneHasDeck(service, deckControlPlanes), nil
	}

	parentValue, err := stringFieldByPath(item, capability.ParentFieldPath)
	if err != nil {
		return "", false, fmt.Errorf("%s %q: resolve parent scope: %w", item.GetType(), item.GetRef(), err)
	}
	if tags.IsRefPlaceholder(parentValue) {
		ref, field, ok := tags.ParseRefPlaceholder(parentValue)
		if !ok || field != FieldID {
			return "", false, fmt.Errorf("%s %q: invalid parent reference %q", item.GetType(), item.GetRef(), parentValue)
		}
		parentValue = ref
	}
	if util.IsValidUUID(parentValue) {
		return parentValue, false, nil
	}
	parent, ok := rs.GetResourceByRef(parentValue)
	if !ok || parent.GetType() != capability.ParentType {
		return "", false, fmt.Errorf(
			"%s %q: referenced %s parent %q not found",
			item.GetType(), item.GetRef(), capability.ParentType, parentValue,
		)
	}
	if parent.GetKonnectID() == "" {
		return "", false, fmt.Errorf(
			"%s %q: parent %s %q does not have a resolved Konnect ID",
			item.GetType(), item.GetRef(), capability.ParentType, parentValue,
		)
	}
	return parent.GetKonnectID(), false, nil
}

func (r *externalLookupResolver) resolveInlineLookups(
	ctx context.Context,
	rs *resources.ResourceSet,
	targetTypes ...resources.ResourceType,
) error {
	targetSet := make(map[resources.ResourceType]struct{}, len(targetTypes))
	for _, targetType := range targetTypes {
		targetSet[targetType] = struct{}{}
	}

	var resolutionErr error
	inlineParents := make(map[string]inlineExternalParent)
	rs.ForEachResource(func(resource resources.Resource) bool {
		for _, relationship := range resources.RelationshipDescriptorsFor(resource) {
			fieldPath := relationship.FieldPath
			targetType := relationship.TargetType
			if relationship.TargetTypeResolver != nil {
				discriminator, err := stringFieldByPath(resource, relationship.TargetDiscriminatorFieldPath)
				if err != nil {
					resolutionErr = fmt.Errorf(
						"%s %q field %s: resolve target discriminator: %w",
						resource.GetType(), resource.GetRef(), fieldPath, err,
					)
					return false
				}
				var ok bool
				targetType, ok = relationship.TargetTypeResolver(discriminator)
				if !ok {
					continue
				}
			}
			if _, selected := targetSet[targetType]; !selected {
				continue
			}
			err := visitStringFieldsByPath(resource, fieldPath, func(value string, set func(string)) error {
				lookup, isLookup := tags.ParseExternalPlaceholder(value)
				if !isLookup {
					return nil
				}

				parent, err := r.inlineLookupParent(rs, resource, relationship)
				if err != nil {
					return err
				}
				source := fmt.Sprintf("%s %q field %s", resource.GetType(), resource.GetRef(), fieldPath)
				if lookup.Line > 0 {
					source += fmt.Sprintf(" (line %d, column %d)", lookup.Line, lookup.Column)
				}
				id, err := r.resolve(ctx, externalLookupRequest{
					ResourceType:    targetType,
					MatchFields:     lookup.MatchFields,
					SensitiveFields: lookup.SensitiveFields,
					ParentID:        parent.id,
					Source:          source,
				})
				if err != nil {
					return err
				}
				resolvedRef := id
				if relationship.ResultField == resources.RelationshipResultFieldRef {
					resolvedRef = inlineExternalResourceRef(rs, targetType, id)
				}
				set(resolvedRef)
				if relationship.ResultField == resources.RelationshipResultFieldRef {
					r.hasInlineParents = true
					if rs.SyncScope != nil {
						rs.SyncScope.RebindChildParent(targetType, value, resolvedRef)
					}
					key := string(targetType) + "|" + resolvedRef
					inlineParents[key] = inlineExternalParent{
						resourceType: targetType,
						id:           id,
						ref:          resolvedRef,
						parentID:     parent.id,
						parentRef:    parent.ref,
					}
				}
				return nil
			})
			if err != nil {
				resolutionErr = fmt.Errorf("%s %q field %s: %w", resource.GetType(), resource.GetRef(), fieldPath, err)
				return false
			}
		}
		return true
	})
	if resolutionErr != nil {
		return resolutionErr
	}
	for _, parent := range inlineParents {
		if err := ensureInlineExternalTraversal(rs, parent); err != nil {
			return err
		}
	}
	return nil
}

func ensureInlineExternalParent(rs *resources.ResourceSet, parent inlineExternalParent) error {
	if parent.ref == "" {
		parent.ref = parent.id
	}
	if existing, ok := rs.GetResourceByRef(parent.ref); ok {
		if existing.GetType() != parent.resourceType {
			return fmt.Errorf(
				"resolved external ref %q is already used by %s, expected %s",
				parent.ref,
				existing.GetType(),
				parent.resourceType,
			)
		}
		if existing.GetKonnectID() != "" && existing.GetKonnectID() != parent.id {
			return fmt.Errorf(
				"resolved external %s %q has Konnect ID %q, expected %q",
				parent.resourceType,
				parent.ref,
				existing.GetKonnectID(),
				parent.id,
			)
		}
		return nil
	}

	_, err := resources.MaterializeExternalResource(rs, parent.resourceType, parent.ref, parent.id, parent.parentRef)
	return err
}

func ensureInlineExternalTraversal(rs *resources.ResourceSet, parent inlineExternalParent) error {
	capability, ok := resources.ExternalResolutionFor(parent.resourceType)
	if !ok {
		return fmt.Errorf("cannot materialize inline external parent for unregistered type %s", parent.resourceType)
	}
	if capability.ParentType != "" {
		if parent.parentID == "" || parent.parentRef == "" {
			return fmt.Errorf(
				"cannot materialize inline external %s %q without resolved %s scope",
				parent.resourceType,
				parent.ref,
				capability.ParentType,
			)
		}
		ancestor, ok := rs.GetResourceByRef(parent.parentRef)
		if ok && ancestor.GetType() != capability.ParentType {
			return fmt.Errorf(
				"inline external %s %q has %s ancestor %q, expected %s",
				parent.resourceType,
				parent.ref,
				ancestor.GetType(),
				parent.parentRef,
				capability.ParentType,
			)
		}
		if ok && ancestor.GetKonnectID() != "" && ancestor.GetKonnectID() != parent.parentID {
			return fmt.Errorf(
				"inline external %s %q ancestor %q has Konnect ID %q, expected %q",
				parent.resourceType,
				parent.ref,
				parent.parentRef,
				ancestor.GetKonnectID(),
				parent.parentID,
			)
		}
		if !ok {
			if err := ensureInlineExternalParent(rs, inlineExternalParent{
				resourceType: capability.ParentType,
				id:           parent.parentID,
				ref:          parent.parentRef,
			}); err != nil {
				return fmt.Errorf("materialize inline external ancestor: %w", err)
			}
		}
	}

	if err := ensureInlineExternalParent(rs, parent); err != nil {
		return err
	}
	if capability.ParentType != "" && rs.SyncScope != nil {
		rs.SyncScope.AddChild(capability.ParentType, parent.parentRef, parent.resourceType)
	}
	return validateInlineExternalTraversal(rs, parent, capability)
}

func validateInlineExternalTraversal(
	rs *resources.ResourceSet,
	parent inlineExternalParent,
	capability resources.ExternalResolutionRegistration,
) error {
	target, ok := rs.GetResourceByRef(parent.ref)
	if !ok || target.GetType() != parent.resourceType || target.GetKonnectID() != parent.id {
		return fmt.Errorf(
			"inline external %s %q was not materialized with Konnect ID %q",
			parent.resourceType,
			parent.ref,
			parent.id,
		)
	}
	if capability.ParentType == "" {
		return nil
	}
	ancestor, ok := rs.GetResourceByRef(parent.parentRef)
	if !ok || ancestor.GetType() != capability.ParentType || ancestor.GetKonnectID() != parent.parentID {
		return fmt.Errorf(
			"inline external %s %q has no materialized %s ancestor %q",
			parent.resourceType,
			parent.ref,
			capability.ParentType,
			parent.parentRef,
		)
	}
	if rs.SyncScope != nil && !rs.SyncScope.ChildInScope(
		capability.ParentType,
		parent.parentRef,
		parent.resourceType,
	) {
		return fmt.Errorf(
			"inline external %s %q is not reachable from %s %q in sync scope",
			parent.resourceType,
			parent.ref,
			capability.ParentType,
			parent.parentRef,
		)
	}
	return nil
}

func inlineExternalResourceRef(
	rs *resources.ResourceSet,
	resourceType resources.ResourceType,
	id string,
) string {
	var ref string
	rs.ForEachResource(func(resource resources.Resource) bool {
		if resource.GetType() == resourceType && resource.GetKonnectID() == id {
			ref = resource.GetRef()
			return false
		}
		return true
	})
	if ref != "" {
		return ref
	}
	return id
}

func (r *externalLookupResolver) inlineLookupParent(
	rs *resources.ResourceSet,
	resource resources.Resource,
	relationship resources.RelationshipDescriptor,
) (inlineLookupParent, error) {
	targetType := relationship.TargetType
	capability, _ := resources.ExternalResolutionFor(targetType)
	if capability.ParentType == "" {
		return inlineLookupParent{}, nil
	}

	if relationship.ScopeFieldPath == "" {
		return inlineLookupParent{}, fmt.Errorf("no parent-scope field registered for %s", targetType)
	}
	parentValue, err := stringFieldByPath(resource, relationship.ScopeFieldPath)
	if err != nil {
		return inlineLookupParent{}, fmt.Errorf("lookup requires companion %s: %w", relationship.ScopeFieldPath, err)
	}

	if tags.IsExternalPlaceholder(parentValue) {
		return inlineLookupParent{}, fmt.Errorf("parent lookup must be resolved before child lookup")
	}
	if util.IsValidUUID(parentValue) {
		return inlineLookupParent{
			id:  parentValue,
			ref: inlineExternalResourceRef(rs, capability.ParentType, parentValue),
		}, nil
	}
	parentRef := parentValue
	if tags.IsRefPlaceholder(parentValue) {
		ref, field, ok := tags.ParseRefPlaceholder(parentValue)
		if !ok || field != FieldID {
			return inlineLookupParent{}, fmt.Errorf("invalid parent reference %q", parentValue)
		}
		parentRef = ref
	}
	parent, ok := rs.GetResourceByRef(parentRef)
	if !ok {
		return inlineLookupParent{}, fmt.Errorf("parent resource %q not found", parentValue)
	}
	if parent.GetType() != capability.ParentType {
		return inlineLookupParent{}, fmt.Errorf(
			"parent %q is %s, expected %s",
			parentValue,
			parent.GetType(),
			capability.ParentType,
		)
	}
	if parent.GetKonnectID() == "" {
		return inlineLookupParent{}, fmt.Errorf("parent resource %q does not have a resolved Konnect ID", parentValue)
	}
	return inlineLookupParent{id: parent.GetKonnectID(), ref: parent.GetRef()}, nil
}

func deckControlPlaneRefs(controlPlanes []resources.ControlPlaneResource) map[string]bool {
	result := make(map[string]bool)
	for i := range controlPlanes {
		if controlPlanes[i].HasDeckConfig() {
			result[controlPlanes[i].GetRef()] = true
		}
	}
	return result
}

func matchExternalCandidates[T any](
	req externalLookupRequest,
	candidates []T,
	id func(T) string,
) (string, error) {
	selector := &resources.ExternalSelector{MatchFields: req.MatchFields}
	matches := make([]string, 0, 1)
	for _, candidate := range candidates {
		if selector.Match(candidate) {
			matches = append(matches, id(candidate))
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf(
			"%s: no %s matched selector {%s}",
			req.Source,
			req.ResourceType,
			tags.ExternalLookupDisplayKey(req.MatchFields, req.SensitiveFields),
		)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf(
			"%s: selector {%s} matched %d %s resources",
			req.Source,
			tags.ExternalLookupDisplayKey(req.MatchFields, req.SensitiveFields),
			len(matches),
			req.ResourceType,
		)
	}
	return matches[0], nil
}

func (r *externalLookupResolver) lookupPortal(ctx context.Context, req externalLookupRequest) (string, error) {
	items, err := r.planner.client.ListAllPortals(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: list portals: %w", req.Source, err)
	}
	return matchExternalCandidates(req, items, func(item state.Portal) string { return item.ID })
}

func (r *externalLookupResolver) lookupAPI(ctx context.Context, req externalLookupRequest) (string, error) {
	items, err := r.planner.client.ListAllAPIs(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: list APIs: %w", req.Source, err)
	}
	return matchExternalCandidates(req, items, func(item state.API) string { return item.ID })
}

func (r *externalLookupResolver) lookupApplicationAuthStrategy(
	ctx context.Context,
	req externalLookupRequest,
) (string, error) {
	items, err := r.planner.client.ListAllApplicationAuthStrategies(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: list application auth strategies: %w", req.Source, err)
	}
	return matchExternalCandidates(req, items, func(item state.ApplicationAuthStrategy) string { return item.ID })
}

func (r *externalLookupResolver) lookupControlPlane(ctx context.Context, req externalLookupRequest) (string, error) {
	items, err := r.planner.client.ListAllControlPlanes(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: list control planes: %w", req.Source, err)
	}
	return matchExternalCandidates(req, items, func(item state.ControlPlane) string { return item.ID })
}

func (r *externalLookupResolver) lookupGatewayService(ctx context.Context, req externalLookupRequest) (string, error) {
	items, err := r.planner.client.ListGatewayServices(ctx, req.ParentID)
	if err != nil {
		return "", fmt.Errorf("%s: list gateway services: %w", req.Source, err)
	}
	return matchExternalCandidates(req, items, func(item state.GatewayService) string { return item.ID })
}

func (r *externalLookupResolver) lookupAIGateway(ctx context.Context, req externalLookupRequest) (string, error) {
	items, err := r.planner.client.ListAllAIGateways(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: list AI Gateways: %w", req.Source, err)
	}
	return matchExternalCandidates(req, items, func(item state.AIGateway) string { return item.ID })
}

func (r *externalLookupResolver) lookupAuditLogWebhookDestination(
	ctx context.Context,
	req externalLookupRequest,
) (string, error) {
	items, err := r.planner.client.ListAuditLogWebhookDestinations(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: list audit-log webhook destinations: %w", req.Source, err)
	}
	return matchExternalCandidates(req, items, func(item state.AuditLogWebhookDestination) string { return item.ID })
}

func (r *externalLookupResolver) lookupOrganizationTeam(
	ctx context.Context,
	req externalLookupRequest,
) (string, error) {
	teams, err := r.planner.client.ListAllOrganizationTeams(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: list organization teams: %w", req.Source, err)
	}
	teams = slices.DeleteFunc(teams, func(team state.OrganizationTeam) bool { return team.ID == "" })
	return matchExternalCandidates(req, teams, func(item state.OrganizationTeam) string { return item.ID })
}

func (r *externalLookupResolver) lookupEventGatewayControlPlane(
	ctx context.Context,
	req externalLookupRequest,
) (string, error) {
	items, err := r.planner.client.ListAllEventGatewayControlPlanes(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: list Event Gateways: %w", req.Source, err)
	}
	return matchExternalCandidates(req, items, func(item state.EventGatewayControlPlane) string { return item.ID })
}

func (r *externalLookupResolver) lookupEventGatewayVirtualCluster(
	ctx context.Context,
	req externalLookupRequest,
) (string, error) {
	items, err := r.planner.client.ListEventGatewayVirtualClusters(ctx, req.ParentID)
	if err != nil {
		return "", fmt.Errorf("%s: list Event Gateway virtual clusters: %w", req.Source, err)
	}
	return matchExternalCandidates(req, items, func(item state.EventGatewayVirtualCluster) string { return item.ID })
}

func setStringFieldByPath(resource resources.Resource, path, value string) error {
	if implementation, ok := resource.(*resources.APIImplementationResource); ok {
		switch path {
		case "service.id", "service.control_plane_id":
			if implementation.ServiceReference == nil {
				return fmt.Errorf("service is not configured")
			}
			service := implementation.ServiceReference.GetService()
			if service == nil {
				return fmt.Errorf("service is not configured")
			}
			if path == "service.id" {
				service.ID = value
			} else {
				service.ControlPlaneID = value
			}
			return nil
		case "control_plane.control_plane_id":
			controlPlane := implementation.ControlPlaneReference.GetControlPlane()
			if controlPlane == nil {
				return fmt.Errorf("control_plane is not configured")
			}
			controlPlane.ID = value
			return nil
		}
	}

	current, err := resolveFieldPath(resource, path)
	if err != nil {
		return err
	}
	if current.Kind() != reflect.String || !current.CanSet() {
		return fmt.Errorf("field %s is not a settable string", path)
	}
	current.SetString(value)
	return nil
}

// resolveFieldPath walks a dotted field path through a resource using the
// yaml/json struct tags, unwrapping pointers along the way, and returns the
// addressable value at the end of the path.
func resolveFieldPath(resource resources.Resource, path string) (reflect.Value, error) {
	current := reflect.ValueOf(resource)
	for part := range strings.SplitSeq(path, ".") {
		for current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return reflect.Value{}, fmt.Errorf("field %s is nil", path)
			}
			current = current.Elem()
		}
		current = findSettableTaggedField(current, part)
		if !current.IsValid() {
			return reflect.Value{}, fmt.Errorf("field %s not found", path)
		}
	}
	for current.Kind() == reflect.Pointer {
		if current.IsNil() {
			return reflect.Value{}, fmt.Errorf("field %s is nil", path)
		}
		current = current.Elem()
	}
	return current, nil
}

func stringFieldByPath(resource resources.Resource, path string) (string, error) {
	if implementation, ok := resource.(*resources.APIImplementationResource); ok {
		switch path {
		case "service.id", "service.control_plane_id":
			if implementation.ServiceReference == nil || implementation.ServiceReference.GetService() == nil {
				return "", nil
			}
			service := implementation.ServiceReference.GetService()
			if path == "service.id" {
				return service.ID, nil
			}
			return service.ControlPlaneID, nil
		case "control_plane.control_plane_id":
			controlPlane := implementation.ControlPlaneReference.GetControlPlane()
			if controlPlane == nil {
				return "", nil
			}
			return controlPlane.ID, nil
		}
	}

	current, err := resolveFieldPath(resource, path)
	if err != nil {
		return "", err
	}
	if current.Kind() != reflect.String {
		return "", fmt.Errorf("field %s is not a string", path)
	}
	return current.String(), nil
}

func visitStringFieldsByPath(resource resources.Resource, path string, visit func(string, func(string)) error) error {
	if implementation, ok := resource.(*resources.APIImplementationResource); ok {
		value, err := stringFieldByPath(resource, path)
		if err != nil || value == "" {
			return err
		}
		return visit(value, func(resolved string) {
			_ = setStringFieldByPath(implementation, path, resolved)
		})
	}

	current := reflect.ValueOf(resource)
	for part := range strings.SplitSeq(path, ".") {
		for current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return nil
			}
			current = current.Elem()
		}
		current = findSettableTaggedField(current, part)
		if !current.IsValid() {
			return fmt.Errorf("field %s not found", path)
		}
	}
	for current.Kind() == reflect.Pointer {
		if current.IsNil() {
			return nil
		}
		current = current.Elem()
	}

	//exhaustive:ignore reflect.Kind is validated by the default branch.
	switch current.Kind() {
	case reflect.String:
		return visit(current.String(), func(resolved string) { current.SetString(resolved) })
	case reflect.Slice, reflect.Array:
		for i := range current.Len() {
			item := current.Index(i)
			if item.Kind() != reflect.String || !item.CanSet() {
				return fmt.Errorf("field %s item %d is not a settable string", path, i)
			}
			if err := visit(item.String(), func(resolved string) { item.SetString(resolved) }); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("field %s is not a string or string list", path)
	}
}

func findSettableTaggedField(value reflect.Value, name string) reflect.Value {
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	typeOfValue := value.Type()
	for i := range value.NumField() {
		fieldInfo := typeOfValue.Field(i)
		for _, tagName := range []string{"yaml", "json"} {
			tag, _, _ := strings.Cut(fieldInfo.Tag.Get(tagName), ",")
			if tag == name {
				return value.Field(i)
			}
		}
	}
	for i := range value.NumField() {
		fieldInfo := typeOfValue.Field(i)
		if !fieldInfo.Anonymous {
			continue
		}
		field := value.Field(i)
		for field.Kind() == reflect.Pointer && !field.IsNil() {
			field = field.Elem()
		}
		if result := findSettableTaggedField(field, name); result.IsValid() {
			return result
		}
	}
	return reflect.Value{}
}

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
	ResourceType resources.ResourceType
	MatchFields  map[string]string
	ParentID     string
	Source       string
}

type externalLookupCacheEntry struct {
	id  string
	err error
}

type externalLookupAdapter func(context.Context, externalLookupRequest) (string, error)

type inlineExternalParent struct {
	resourceType resources.ResourceType
	id           string
	parentID     string
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
	for _, resourceType := range []resources.ResourceType{
		resources.ResourceTypeControlPlane,
		resources.ResourceTypeEventGatewayControlPlane,
		resources.ResourceTypePortal,
		resources.ResourceTypeAIGateway,
		resources.ResourceTypeOrganizationTeam,
	} {
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

	if rs.AuditLogs != nil {
		for i := range rs.AuditLogs.Destinations {
			destination := &rs.AuditLogs.Destinations[i]
			if destination.GetKonnectID() != "" {
				continue
			}
			id, err := r.resolve(ctx, externalRequest(
				destination.GetType(), destination.External, "", externalDeclarationSource(destination),
			))
			if err != nil {
				return err
			}
			destination.SetKonnectID(id)
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
	for i := range rs.GatewayServices {
		service := &rs.GatewayServices[i]
		if !service.IsExternal() || service.GetKonnectID() != "" {
			continue
		}
		parentID, err := r.planner.resolveGatewayServiceControlPlaneID(service, controlPlaneByRef)
		if err != nil {
			return err
		}
		usesDeck := controlPlaneHasDeck(service, deckControlPlanes)
		if parentID == "" && usesDeck {
			continue
		}
		id, err := r.resolve(ctx, externalRequest(
			service.GetType(), service.External, parentID, externalDeclarationSource(service),
		))
		if err != nil {
			if usesDeck {
				continue
			}
			return err
		}
		service.SetKonnectID(id)
		service.SetResolvedControlPlaneID(parentID)
	}

	eventGatewayByRef := make(map[string]*resources.EventGatewayControlPlaneResource, len(rs.EventGatewayControlPlanes))
	for i := range rs.EventGatewayControlPlanes {
		eventGatewayByRef[rs.EventGatewayControlPlanes[i].GetRef()] = &rs.EventGatewayControlPlanes[i]
	}
	for i := range rs.EventGatewayVirtualClusters {
		cluster := &rs.EventGatewayVirtualClusters[i]
		if !cluster.IsExternal() || cluster.GetKonnectID() != "" {
			continue
		}
		parentID, err := resolveScopedParentID(cluster.EventGateway, eventGatewayByRef)
		if err != nil {
			return fmt.Errorf("event_gateway_virtual_cluster %q: %w", cluster.GetRef(), err)
		}
		id, err := r.resolve(ctx, externalRequest(
			cluster.GetType(), cluster.External, parentID, externalDeclarationSource(cluster),
		))
		if err != nil {
			return err
		}
		cluster.SetKonnectID(id)
	}
	return nil
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
			if _, selected := targetSet[targetType]; !selected {
				continue
			}
			value, err := stringFieldByPath(resource, fieldPath)
			if err != nil {
				resolutionErr = fmt.Errorf("%s %q field %s: %w", resource.GetType(), resource.GetRef(), fieldPath, err)
				return false
			}
			lookup, isLookup := tags.ParseExternalPlaceholder(value)
			if !isLookup {
				continue
			}

			parentID, err := r.inlineLookupParentID(rs, resource, relationship)
			if err != nil {
				resolutionErr = fmt.Errorf("%s %q field %s: %w", resource.GetType(), resource.GetRef(), fieldPath, err)
				return false
			}
			source := fmt.Sprintf("%s %q field %s", resource.GetType(), resource.GetRef(), fieldPath)
			if lookup.Line > 0 {
				source += fmt.Sprintf(" (line %d, column %d)", lookup.Line, lookup.Column)
			}
			id, err := r.resolve(ctx, externalLookupRequest{
				ResourceType: targetType,
				MatchFields:  lookup.MatchFields,
				ParentID:     parentID,
				Source:       source,
			})
			if err != nil {
				resolutionErr = err
				return false
			}
			if err := setStringFieldByPath(resource, fieldPath, id); err != nil {
				resolutionErr = fmt.Errorf("%s: bind resolved ID: %w", source, err)
				return false
			}
			if relationship.Kind == resources.RelationshipKindKongctlParentSelector {
				r.hasInlineParents = true
				if rs.SyncScope != nil {
					rs.SyncScope.RebindChildParent(targetType, value, id)
				}
				key := string(targetType) + "|" + id
				inlineParents[key] = inlineExternalParent{
					resourceType: targetType,
					id:           id,
					parentID:     parentID,
				}
			}
		}
		return true
	})
	if resolutionErr != nil {
		return resolutionErr
	}
	for _, parent := range inlineParents {
		if err := ensureInlineExternalParent(rs, parent); err != nil {
			return err
		}
	}
	return nil
}

func ensureInlineExternalParent(rs *resources.ResourceSet, parent inlineExternalParent) error {
	if existing, ok := rs.GetResourceByRef(parent.id); ok {
		if existing.GetType() != parent.resourceType {
			return fmt.Errorf(
				"resolved external ID %q is already used as ref by %s, expected %s",
				parent.id,
				existing.GetType(),
				parent.resourceType,
			)
		}
		return nil
	}

	external := &resources.ExternalBlock{ID: parent.id}
	// Only registered parent-capable external types can reach this switch.
	//nolint:exhaustive
	switch parent.resourceType {
	case resources.ResourceTypePortal:
		rs.Portals = append(rs.Portals, resources.PortalResource{
			BaseResource: resources.BaseResource{Ref: parent.id},
			External:     external,
		})
		rs.Portals[len(rs.Portals)-1].SetKonnectID(parent.id)
	case resources.ResourceTypeControlPlane:
		rs.ControlPlanes = append(rs.ControlPlanes, resources.ControlPlaneResource{
			BaseResource: resources.BaseResource{Ref: parent.id},
			External:     external,
		})
		rs.ControlPlanes[len(rs.ControlPlanes)-1].SetKonnectID(parent.id)
	case resources.ResourceTypeAIGateway:
		rs.AIGateways = append(rs.AIGateways, resources.AIGatewayResource{
			BaseResource: resources.BaseResource{Ref: parent.id},
			External:     external,
		})
		rs.AIGateways[len(rs.AIGateways)-1].SetKonnectID(parent.id)
	case resources.ResourceTypeOrganizationTeam:
		rs.OrganizationTeams = append(rs.OrganizationTeams, resources.OrganizationTeamResource{
			BaseResource: resources.BaseResource{Ref: parent.id},
			External:     external,
		})
		rs.OrganizationTeams[len(rs.OrganizationTeams)-1].SetKonnectID(parent.id)
	case resources.ResourceTypeEventGatewayControlPlane:
		rs.EventGatewayControlPlanes = append(
			rs.EventGatewayControlPlanes,
			resources.EventGatewayControlPlaneResource{
				BaseResource: resources.BaseResource{Ref: parent.id},
				External:     external,
			},
		)
		rs.EventGatewayControlPlanes[len(rs.EventGatewayControlPlanes)-1].SetKonnectID(parent.id)
	case resources.ResourceTypeEventGatewayVirtualCluster:
		rs.EventGatewayVirtualClusters = append(
			rs.EventGatewayVirtualClusters,
			resources.EventGatewayVirtualClusterResource{
				Ref:          parent.id,
				EventGateway: parent.parentID,
				External:     external,
			},
		)
		rs.EventGatewayVirtualClusters[len(rs.EventGatewayVirtualClusters)-1].SetKonnectID(parent.id)
	default:
		return fmt.Errorf("cannot materialize inline external parent for %s", parent.resourceType)
	}
	return nil
}

func (r *externalLookupResolver) inlineLookupParentID(
	rs *resources.ResourceSet,
	resource resources.Resource,
	relationship resources.RelationshipDescriptor,
) (string, error) {
	targetType := relationship.TargetType
	capability, _ := resources.ExternalResolutionFor(targetType)
	if capability.ParentType == "" {
		return "", nil
	}

	if relationship.ScopeFieldPath == "" {
		return "", fmt.Errorf("no parent-scope field registered for %s", targetType)
	}
	parentValue, err := stringFieldByPath(resource, relationship.ScopeFieldPath)
	if err != nil {
		return "", fmt.Errorf("lookup requires companion %s: %w", relationship.ScopeFieldPath, err)
	}

	if tags.IsExternalPlaceholder(parentValue) {
		return "", fmt.Errorf("parent lookup must be resolved before child lookup")
	}
	if util.IsValidUUID(parentValue) {
		return parentValue, nil
	}
	parentRef := parentValue
	if tags.IsRefPlaceholder(parentValue) {
		ref, field, ok := tags.ParseRefPlaceholder(parentValue)
		if !ok || field != FieldID {
			return "", fmt.Errorf("invalid parent reference %q", parentValue)
		}
		parentRef = ref
	}
	parent, ok := rs.GetResourceByRef(parentRef)
	if !ok {
		return "", fmt.Errorf("parent resource %q not found", parentValue)
	}
	if parent.GetType() != capability.ParentType {
		return "", fmt.Errorf("parent %q is %s, expected %s", parentValue, parent.GetType(), capability.ParentType)
	}
	if parent.GetKonnectID() == "" {
		return "", fmt.Errorf("parent resource %q does not have a resolved Konnect ID", parentValue)
	}
	return parent.GetKonnectID(), nil
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

func resolveScopedParentID[T resources.Resource](value string, byRef map[string]T) (string, error) {
	if tags.IsRefPlaceholder(value) {
		ref, field, ok := tags.ParseRefPlaceholder(value)
		if !ok || field != FieldID {
			return "", fmt.Errorf("invalid parent reference %q", value)
		}
		value = ref
	}
	if util.IsValidUUID(value) {
		return value, nil
	}
	parent, ok := byRef[value]
	if !ok {
		return "", fmt.Errorf("parent resource %q not found", value)
	}
	if parent.GetKonnectID() == "" {
		return "", fmt.Errorf("parent resource %q does not have a resolved Konnect ID", value)
	}
	return parent.GetKonnectID(), nil
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
			"%s: no %s matched selector {%s}", req.Source, req.ResourceType, tags.ExternalLookupKey(req.MatchFields),
		)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf(
			"%s: selector {%s} matched %d %s resources",
			req.Source, tags.ExternalLookupKey(req.MatchFields), len(matches), req.ResourceType,
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
	matches := make([]string, 0, 1)
	for _, item := range items {
		matched := true
		for field, value := range req.MatchFields {
			switch field {
			case FieldName:
				matched = matched && item.Name == value
			case FieldDisplayName:
				matched = matched && item.DisplayName == value
			}
		}
		if matched {
			matches = append(matches, item.ID)
		}
	}
	return singleExternalID(req, matches)
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
	matches := make([]string, 0, 1)
	for _, team := range teams {
		if team.Name != nil && *team.Name == req.MatchFields[FieldName] && team.ID != nil && *team.ID != "" {
			matches = append(matches, *team.ID)
		}
	}
	return singleExternalID(req, matches)
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

func singleExternalID(req externalLookupRequest, matches []string) (string, error) {
	if len(matches) == 0 {
		return "", fmt.Errorf(
			"%s: no %s matched selector {%s}", req.Source, req.ResourceType, tags.ExternalLookupKey(req.MatchFields),
		)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf(
			"%s: selector {%s} matched %d %s resources",
			req.Source, tags.ExternalLookupKey(req.MatchFields), len(matches), req.ResourceType,
		)
	}
	return matches[0], nil
}

func setStringFieldByPath(resource resources.Resource, path, value string) error {
	if implementation, ok := resource.(*resources.APIImplementationResource); ok {
		if implementation.ServiceReference == nil {
			return fmt.Errorf("service is not configured")
		}
		service := implementation.ServiceReference.GetService()
		if service == nil {
			return fmt.Errorf("service is not configured")
		}
		switch path {
		case "service.id":
			service.ID = value
			return nil
		case "service.control_plane_id":
			service.ControlPlaneID = value
			return nil
		}
	}

	current := reflect.ValueOf(resource)
	for part := range strings.SplitSeq(path, ".") {
		for current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return fmt.Errorf("field %s is nil", path)
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
			return fmt.Errorf("field %s is nil", path)
		}
		current = current.Elem()
	}
	if current.Kind() != reflect.String || !current.CanSet() {
		return fmt.Errorf("field %s is not a settable string", path)
	}
	current.SetString(value)
	return nil
}

func stringFieldByPath(resource resources.Resource, path string) (string, error) {
	if implementation, ok := resource.(*resources.APIImplementationResource); ok {
		switch path {
		case "service.id":
			if implementation.ServiceReference == nil || implementation.ServiceReference.GetService() == nil {
				return "", nil
			}
			service := implementation.ServiceReference.GetService()
			return service.ID, nil
		case "service.control_plane_id":
			if implementation.ServiceReference == nil || implementation.ServiceReference.GetService() == nil {
				return "", nil
			}
			service := implementation.ServiceReference.GetService()
			return service.ControlPlaneID, nil
		}
	}

	current := reflect.ValueOf(resource)
	for part := range strings.SplitSeq(path, ".") {
		for current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return "", fmt.Errorf("field %s is nil", path)
			}
			current = current.Elem()
		}
		current = findSettableTaggedField(current, part)
		if !current.IsValid() {
			return "", fmt.Errorf("field %s not found", path)
		}
	}
	for current.Kind() == reflect.Pointer {
		if current.IsNil() {
			return "", fmt.Errorf("field %s is nil", path)
		}
		current = current.Elem()
	}
	if current.Kind() != reflect.String {
		return "", fmt.Errorf("field %s is not a string", path)
	}
	return current.String(), nil
}

func findSettableTaggedField(value reflect.Value, name string) reflect.Value {
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	typeOfValue := value.Type()
	for i := 0; i < value.NumField(); i++ {
		fieldInfo := typeOfValue.Field(i)
		for _, tagName := range []string{"yaml", "json"} {
			tag := strings.Split(fieldInfo.Tag.Get(tagName), ",")[0]
			if tag == name {
				return value.Field(i)
			}
		}
	}
	for i := 0; i < value.NumField(); i++ {
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

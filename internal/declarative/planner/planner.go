package planner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

// Options configures plan generation behavior
type Options struct {
	Mode      PlanMode
	Generator string
}

const defaultGenerator = "kongctl/dev"

var errGatewayServiceNotFound = errors.New("gateway service not found")

// Planner generates execution plans
type Planner struct {
	client      *state.Client
	logger      *slog.Logger
	resolver    *ReferenceResolver
	depResolver *DependencyResolver
	changeCount int

	// Generic planner for common operations
	genericPlanner *GenericPlanner

	// Resource-specific planners
	portalPlanner                   PortalPlanner
	controlPlanePlanner             ControlPlanePlanner
	authStrategyPlanner             AuthStrategyPlanner
	apiPlanner                      APIPlanner
	catalogServicePlanner           CatalogServicePlanner
	eventGatewayControlPlanePlanner EGWControlPlanePlanner

	// ResourceSet containing all desired resources
	resources *resources.ResourceSet

	// Legacy field access for backward compatibility (provides global access)
	desiredPortals              []resources.PortalResource
	desiredPortalPages          []resources.PortalPageResource
	desiredPortalSnippets       []resources.PortalSnippetResource
	desiredPortalTeams          []resources.PortalTeamResource
	desiredPortalTeamRoles      []resources.PortalTeamRoleResource
	desiredPortalCustomizations []resources.PortalCustomizationResource
	desiredPortalAuthSettings   []resources.PortalAuthSettingsResource
	desiredPortalCustomDomains  []resources.PortalCustomDomainResource
	desiredPortalAssetLogos     []resources.PortalAssetLogoResource
	desiredPortalAssetFavicons  []resources.PortalAssetFaviconResource
	desiredPortalEmailConfigs   []resources.PortalEmailConfigResource
	desiredPortalEmailTemplates []resources.PortalEmailTemplateResource
}

// NewPlanner creates a new planner
func NewPlanner(client *state.Client, logger *slog.Logger) *Planner {
	p := &Planner{
		client: client,
		logger: logger,
		// resolver will be initialized with ResourceSet during planning
		depResolver: NewDependencyResolver(),
		changeCount: 0,
	}

	// Initialize generic planner
	p.genericPlanner = NewGenericPlanner(p)

	// Initialize resource-specific planners
	base := NewBasePlanner(p)
	p.portalPlanner = NewPortalPlanner(base)
	p.eventGatewayControlPlanePlanner = NewEGWControlPlanePlanner(base, p.resources)
	p.controlPlanePlanner = NewControlPlanePlanner(base)
	p.authStrategyPlanner = NewAuthStrategyPlanner(base)
	p.catalogServicePlanner = NewCatalogServicePlanner(base)
	p.apiPlanner = NewAPIPlanner(base)

	return p
}

// GeneratePlan creates a plan from declarative configuration
func (p *Planner) GeneratePlan(ctx context.Context, rs *resources.ResourceSet, opts Options) (*Plan, error) {
	generator := opts.Generator
	if generator == "" {
		generator = defaultGenerator
	}

	// Create base plan
	basePlan := NewPlan("1.0", generator, opts.Mode)

	// Pre-resolution phase: Resolve resource identities before planning
	if err := p.resolveResourceIdentities(ctx, rs); err != nil {
		return nil, fmt.Errorf("failed to resolve resource identities: %w", err)
	}

	// Initialize resolver with populated ResourceSet
	p.resolver = NewReferenceResolver(p.client, rs)

	// Extract all unique namespaces from desired resources
	namespaces := p.getResourceNamespaces(rs)

	// Sync mode needs to account for namespaces supplied via _defaults even when other
	// resources are present in the input set (e.g., multi-file inputs where one file
	// is defaults-only). When no resources are present, fall back to the provided
	// default namespace(s) (or the implicit "default").
	if opts.Mode == PlanModeSync {
		defaultNamespaces := rs.DefaultNamespaces
		if len(defaultNamespaces) == 0 && rs.DefaultNamespace != "" {
			defaultNamespaces = []string{rs.DefaultNamespace}
		}

		if len(namespaces) == 0 {
			if len(defaultNamespaces) > 0 {
				namespaces = append(namespaces, defaultNamespaces...)
			} else {
				namespaces = []string{DefaultNamespace}
			}
		} else {
			for _, ns := range defaultNamespaces {
				if ns != "" && !containsString(namespaces, ns) {
					namespaces = append(namespaces, ns)
				}
			}
			sort.Strings(namespaces)
		}
	}

	// Log namespace processing
	p.logger.Debug("Processing namespaces",
		slog.Int("count", len(namespaces)),
		slog.Any("namespaces", namespaces))

	// Process each namespace independently
	for _, namespace := range namespaces {
		// Create a namespace-specific planner context
		namespacePlanner := &Planner{
			client:      p.client,
			logger:      p.logger,
			resolver:    p.resolver,
			depResolver: p.depResolver,
			changeCount: p.changeCount,
		}

		// Initialize generic planner for namespace-specific planner
		namespacePlanner.genericPlanner = NewGenericPlanner(namespacePlanner)

		// Create new sub-planners for this namespace to ensure they reference
		// the namespace-specific resources, not the parent's empty lists
		base := NewBasePlanner(namespacePlanner)
		namespacePlanner.portalPlanner = NewPortalPlanner(base)
		namespacePlanner.controlPlanePlanner = NewControlPlanePlanner(base)
		namespacePlanner.authStrategyPlanner = NewAuthStrategyPlanner(base)
		namespacePlanner.catalogServicePlanner = NewCatalogServicePlanner(base)
		namespacePlanner.apiPlanner = NewAPIPlanner(base)
		namespacePlanner.eventGatewayControlPlanePlanner = NewEGWControlPlanePlanner(base, rs)

		// Store full ResourceSet for access by planners (enables both filtered views and global lookups)
		namespacePlanner.resources = rs

		// Populate legacy field access for backward compatibility
		namespacePlanner.desiredPortals = rs.Portals
		namespacePlanner.desiredPortalPages = rs.PortalPages
		namespacePlanner.desiredPortalSnippets = rs.PortalSnippets
		namespacePlanner.desiredPortalTeams = rs.PortalTeams
		namespacePlanner.desiredPortalTeamRoles = rs.PortalTeamRoles
		namespacePlanner.desiredPortalCustomizations = rs.PortalCustomizations
		namespacePlanner.desiredPortalAuthSettings = rs.PortalAuthSettings
		namespacePlanner.desiredPortalCustomDomains = rs.PortalCustomDomains
		namespacePlanner.desiredPortalAssetLogos = rs.PortalAssetLogos
		namespacePlanner.desiredPortalAssetFavicons = rs.PortalAssetFavicons
		namespacePlanner.desiredPortalEmailConfigs = rs.PortalEmailConfigs
		namespacePlanner.desiredPortalEmailTemplates = rs.PortalEmailTemplates

		// Create a plan for this namespace
		namespacePlan := NewPlan("1.0", generator, opts.Mode)

		// Generate changes using interface-based planners
		// Pass the specific namespace to planners instead of wildcard
		actualNamespace := namespace
		if namespace == "*" {
			// For sync mode with empty config, we still need to query all namespaces
			actualNamespace = "*"
		}

		// Create planner context with namespace
		plannerCtx := NewConfig(actualNamespace)

		if err := namespacePlanner.authStrategyPlanner.PlanChanges(ctx, plannerCtx, namespacePlan); err != nil {
			return nil, fmt.Errorf("failed to plan auth strategy changes for namespace %s: %w", namespace, err)
		}

		if err := namespacePlanner.controlPlanePlanner.PlanChanges(ctx, plannerCtx, namespacePlan); err != nil {
			return nil, fmt.Errorf("failed to plan control plane changes for namespace %s: %w", namespace, err)
		}

		if err := namespacePlanner.portalPlanner.PlanChanges(ctx, plannerCtx, namespacePlan); err != nil {
			return nil, fmt.Errorf("failed to plan portal changes for namespace %s: %w", namespace, err)
		}

		if err := namespacePlanner.catalogServicePlanner.PlanChanges(ctx, plannerCtx, namespacePlan); err != nil {
			return nil, fmt.Errorf("failed to plan catalog service changes for namespace %s: %w", namespace, err)
		}

		// Plan API changes (includes child resources)
		if err := namespacePlanner.apiPlanner.PlanChanges(ctx, plannerCtx, namespacePlan); err != nil {
			return nil, fmt.Errorf("failed to plan API changes for namespace %s: %w", namespace, err)
		}

		if err := namespacePlanner.eventGatewayControlPlanePlanner.PlanChanges(ctx, plannerCtx, namespacePlan); err != nil {
			return nil, fmt.Errorf("failed to plan Event Gateway Control Plane changes for namespace %s: %w", namespace, err)
		}

		// Merge namespace plan into base plan
		basePlan.Changes = append(basePlan.Changes, namespacePlan.Changes...)
		basePlan.Warnings = append(basePlan.Warnings, namespacePlan.Warnings...)

		// Update change count
		p.changeCount = namespacePlanner.changeCount
	}

	if err := p.planDeckDependencies(rs, basePlan); err != nil {
		return nil, err
	}

	// Update the base plan summary after merging all namespace changes
	basePlan.UpdateSummary()

	// Note: Orphan portal child resources (those referencing non-existent portals)
	// are now handled within each namespace's processing using the namespace-filtered
	// resource access methods.

	// Resolve references for all changes
	resolveResult, err := p.resolver.ResolveReferences(ctx, basePlan.Changes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve references: %w", err)
	}

	// Apply resolved references to changes
	for changeID, refs := range resolveResult.ChangeReferences {
		for i := range basePlan.Changes {
			if basePlan.Changes[i].ID == changeID {
				// Preserve existing references and merge with resolver results
				if basePlan.Changes[i].References == nil {
					basePlan.Changes[i].References = make(map[string]ReferenceInfo)
				}
				for field, ref := range refs {
					basePlan.Changes[i].References[field] = ReferenceInfo{
						Ref: ref.Ref,
						ID:  ref.ID,
					}
				}
				break
			}
		}
	}

	// Ensure portal team roles depend on referenced APIs created in the same plan
	adjustPortalTeamRoleDependencies(basePlan)

	// Resolve dependencies and calculate execution order
	// Inject additional dependency constraints that span resource planners
	adjustAuthStrategyDeleteDependencies(basePlan.Changes)

	executionOrder, err := p.depResolver.ResolveDependencies(basePlan.Changes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}
	basePlan.SetExecutionOrder(executionOrder)

	// Reassign change IDs to match execution order
	p.reassignChangeIDs(basePlan, executionOrder)

	// Add warnings for unresolved references
	for _, change := range basePlan.Changes {
		for field, ref := range change.References {
			if ref.ID == "[unknown]" {
				basePlan.AddWarning(change.ID, fmt.Sprintf(
					"Reference %s=%s will be resolved during execution",
					field, ref.Ref))
			}
		}
	}

	return basePlan, nil
}

// adjustAuthStrategyDeleteDependencies ensures auth strategy DELETE changes execute only
// after their dependent API and API publication DELETE operations. Without this wiring,
// the planner can schedule auth strategy removals before the dependent resources are
// cleaned up, which triggers 409 conflicts from Konnect.
func adjustAuthStrategyDeleteDependencies(changes []PlannedChange) {
	var apiDeletes []*PlannedChange
	var publicationDeletes []*PlannedChange

	for i := range changes {
		change := &changes[i]
		if change.Action != ActionDelete {
			continue
		}

		switch change.ResourceType {
		case "api":
			apiDeletes = append(apiDeletes, change)
		case "api_publication":
			publicationDeletes = append(publicationDeletes, change)
		}
	}

	for i := range changes {
		change := &changes[i]
		if change.Action != ActionDelete || change.ResourceType != "application_auth_strategy" {
			continue
		}

		for _, dep := range apiDeletes {
			if shouldLinkAuthStrategy(change, dep) {
				change.DependsOn = appendDependsOn(change.DependsOn, dep.ID)
			}
		}

		for _, dep := range publicationDeletes {
			if shouldLinkAuthStrategy(change, dep) {
				change.DependsOn = appendDependsOn(change.DependsOn, dep.ID)
			}
		}
	}
}

func appendDependsOn(existing []string, id string) []string {
	for _, dep := range existing {
		if dep == id {
			return existing
		}
	}
	return append(existing, id)
}

// shouldLinkAuthStrategy determines if the auth strategy DELETE should depend on the
// provided change. During sync planning some legacy resources may surface without a
// namespace (empty string). In that case we conservatively treat the namespace as a
// wildcard so that dependencies are not skipped.
func shouldLinkAuthStrategy(authDelete, dep *PlannedChange) bool {
	if authDelete.Namespace != "" && dep.Namespace != "" {
		return authDelete.Namespace == dep.Namespace
	}

	if dep.ResourceType == "api_publication" {
		return publicationReferencesAuthStrategy(dep, authDelete)
	}

	// Fallback: if we only have one namespace provided (or both empty), fall back to equality
	return authDelete.Namespace == dep.Namespace
}

func publicationReferencesAuthStrategy(publication, authDelete *PlannedChange) bool {
	if len(publication.Fields) == 0 {
		return false
	}

	rawIDs, ok := publication.Fields["auth_strategy_ids"]
	if !ok {
		return false
	}

	ids, ok := asStringSlice(rawIDs)
	if !ok {
		return false
	}

	if authDelete.ResourceID != "" && containsString(ids, authDelete.ResourceID) {
		return true
	}

	if authDelete.ResourceRef != "" && containsString(ids, authDelete.ResourceRef) {
		return true
	}

	return false
}

func asStringSlice(value any) ([]string, bool) {
	switch v := value.(type) {
	case []string:
		return v, true
	case []any:
		result := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, false
			}
			result[i] = str
		}
		return result, true
	default:
		return nil, false
	}
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

// nextChangeID generates temporary change IDs during planning phase
func (p *Planner) nextChangeID(action ActionType, resourceType string, ref string) string {
	p.changeCount++
	actionChar := "?"
	switch action {
	case ActionCreate:
		actionChar = "c"
	case ActionUpdate:
		actionChar = "u"
	case ActionDelete:
		actionChar = "d"
	case ActionExternalTool:
		actionChar = "e"
	}
	// Use temporary IDs that will be reassigned based on execution order
	return fmt.Sprintf("temp-%d:%s:%s:%s", p.changeCount, actionChar, resourceType, ref)
}

// reassignChangeIDs updates change IDs to match execution order
func (p *Planner) reassignChangeIDs(plan *Plan, executionOrder []string) {
	// Create mapping from old IDs to new IDs based on execution order
	idMapping := make(map[string]string)
	for newPos, oldID := range executionOrder {
		// Extract components from old ID (format: "temp-N:action:type:ref")
		// We need to parse out the action, type, and ref parts
		parts := strings.SplitN(oldID, ":", 4)
		if len(parts) == 4 && strings.HasPrefix(parts[0], "temp-") {
			// Reconstruct with new position
			newID := fmt.Sprintf("%d:%s:%s:%s", newPos+1, parts[1], parts[2], parts[3])
			idMapping[oldID] = newID
		}
	}

	// Update change IDs
	for i := range plan.Changes {
		if newID, ok := idMapping[plan.Changes[i].ID]; ok {
			plan.Changes[i].ID = newID
		}

		// Update DependsOn references
		for j := range plan.Changes[i].DependsOn {
			if newID, ok := idMapping[plan.Changes[i].DependsOn[j]]; ok {
				plan.Changes[i].DependsOn[j] = newID
			}
		}
	}

	// Update execution order with new IDs
	for i := range plan.ExecutionOrder {
		if newID, ok := idMapping[plan.ExecutionOrder[i]]; ok {
			plan.ExecutionOrder[i] = newID
		}
	}

	// Update warnings
	for i := range plan.Warnings {
		if newID, ok := idMapping[plan.Warnings[i].ChangeID]; ok {
			plan.Warnings[i].ChangeID = newID
		}
	}
}

// validateProtection checks if a protected resource would be modified or deleted
func (p *Planner) validateProtection(
	resourceType, resourceName string,
	currentProtected bool,
	action ActionType,
) error {
	if action == ActionUpdate || action == ActionDelete {
		if currentProtected {
			var actionVerb string
			switch action { //nolint:exhaustive // ActionCreate is not possible here due to outer if condition
			case ActionDelete:
				actionVerb = "deleted"
			case ActionUpdate:
				actionVerb = "updated"
			default:
				actionVerb = "modified"
			}
			return fmt.Errorf("%s %q is protected and cannot be %s",
				resourceType, resourceName, actionVerb)
		}
	}
	return nil
}

// validateProtectionWithChange checks if a protected resource would be modified or deleted,
// but allows protection-only removal
func (p *Planner) validateProtectionWithChange(
	resourceType, resourceName string,
	currentProtected bool,
	action ActionType,
	protectionChange *ProtectionChange,
	hasOtherFieldChanges bool,
) error {
	if action == ActionUpdate && currentProtected {
		// Allow if only removing protection (no other field changes)
		if protectionChange != nil && !protectionChange.New && !hasOtherFieldChanges {
			return nil
		}
		// Block all other updates to protected resources
		return fmt.Errorf("%s %q is protected and cannot be updated",
			resourceType, resourceName)
	}
	if action == ActionDelete && currentProtected {
		return fmt.Errorf("%s %q is protected and cannot be deleted",
			resourceType, resourceName)
	}
	return nil
}

// getString dereferences string pointer or returns empty
func getString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Legacy methods for backward compatibility - delegate to ResourceSet methods
// These search across all namespaces since the callers expect global access

// GetDesiredAPIs returns all desired API resources (across all namespaces)
func (p *Planner) GetDesiredAPIs() []resources.APIResource {
	if p.resources == nil {
		return nil
	}
	return p.resources.APIs
}

// GetDesiredPortalCustomizations returns all desired portal customization resources (across all namespaces)
func (p *Planner) GetDesiredPortalCustomizations() []resources.PortalCustomizationResource {
	if p.resources == nil {
		return nil
	}
	return p.resources.PortalCustomizations
}

// GetDesiredPortalCustomDomains returns all desired portal custom domain resources (across all namespaces)
func (p *Planner) GetDesiredPortalCustomDomains() []resources.PortalCustomDomainResource {
	if p.resources == nil {
		return nil
	}
	return p.resources.PortalCustomDomains
}

// GetDesiredPortalPages returns all desired portal page resources (across all namespaces)
func (p *Planner) GetDesiredPortalPages() []resources.PortalPageResource {
	if p.resources == nil {
		return nil
	}
	return p.resources.PortalPages
}

// GetDesiredPortalSnippets returns all desired portal snippet resources (across all namespaces)
func (p *Planner) GetDesiredPortalSnippets() []resources.PortalSnippetResource {
	if p.resources == nil {
		return nil
	}
	return p.resources.PortalSnippets
}

// resolveResourceIdentities pre-resolves Konnect IDs for all resources
func (p *Planner) resolveResourceIdentities(ctx context.Context, rs *resources.ResourceSet) error {
	// Resolve Control Plane identities
	if err := p.resolveControlPlaneIdentities(ctx, rs.ControlPlanes); err != nil {
		return fmt.Errorf("failed to resolve control plane identities: %w", err)
	}

	if err := p.resolveGatewayServiceIdentities(ctx, rs.GatewayServices, rs.ControlPlanes); err != nil {
		return fmt.Errorf("failed to resolve gateway service identities: %w", err)
	}

	if err := p.resolveAPIImplementationServiceReferences(rs); err != nil {
		return fmt.Errorf("failed to resolve API implementation services: %w", err)
	}

	if err := p.resolveCatalogServiceIdentities(ctx, rs.CatalogServices); err != nil {
		return fmt.Errorf("failed to resolve catalog service identities: %w", err)
	}

	// Resolve API identities
	if err := p.resolveAPIIdentities(ctx, rs.APIs); err != nil {
		return fmt.Errorf("failed to resolve API identities: %w", err)
	}

	// Resolve Portal identities
	if err := p.resolvePortalIdentities(ctx, rs.Portals); err != nil {
		return fmt.Errorf("failed to resolve portal identities: %w", err)
	}

	// Resolve Auth Strategy identities
	if err := p.resolveAuthStrategyIdentities(ctx, rs.ApplicationAuthStrategies); err != nil {
		return fmt.Errorf("failed to resolve auth strategy identities: %w", err)
	}

	// API child resources are resolved through their parent APIs
	// so we don't need to resolve them separately here

	return nil
}

// resolveCatalogServiceIdentities resolves Konnect IDs for catalog services
func (p *Planner) resolveCatalogServiceIdentities(
	ctx context.Context,
	services []resources.CatalogServiceResource,
) error {
	for i := range services {
		svc := &services[i]

		if svc.GetKonnectID() != "" {
			continue
		}

		if svc.Name == "" {
			continue
		}

		konnectSvc, err := p.client.GetCatalogServiceByName(ctx, svc.Name)
		if err != nil {
			return fmt.Errorf("failed to lookup catalog service %s: %w", svc.GetRef(), err)
		}

		if konnectSvc != nil {
			svc.TryMatchKonnectResource(konnectSvc)
		}
	}

	return nil
}

// resolveAPIIdentities resolves Konnect IDs for API resources
func (p *Planner) resolveAPIIdentities(ctx context.Context, apis []resources.APIResource) error {
	for i := range apis {
		api := &apis[i]

		// Skip if already resolved
		if api.GetKonnectID() != "" {
			continue
		}

		// Try to find the API using filter
		filter := api.GetKonnectMonikerFilter()
		if filter == "" {
			continue
		}

		konnectAPI, err := p.client.GetAPIByFilter(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to lookup API %s: %w", api.GetRef(), err)
		}

		if konnectAPI != nil {
			// Match found, update the resource
			api.TryMatchKonnectResource(konnectAPI)
		}
	}

	return nil
}

// resolveControlPlaneIdentities resolves Konnect IDs for control plane resources
func (p *Planner) resolveControlPlaneIdentities(
	ctx context.Context,
	controlPlanes []resources.ControlPlaneResource,
) error {
	var (
		cpByID               map[string]*state.ControlPlane
		cpByName             map[string][]*state.ControlPlane
		allControlPlanesInit bool
	)

	loadAllControlPlanes := func() error {
		if allControlPlanesInit {
			return nil
		}

		cps, err := p.client.ListAllControlPlanes(ctx)
		if err != nil {
			return fmt.Errorf("failed to list control planes for external lookup: %w", err)
		}

		cpByID = make(map[string]*state.ControlPlane, len(cps))
		cpByName = make(map[string][]*state.ControlPlane)

		for i := range cps {
			cp := &cps[i]
			cpByID[cp.ID] = cp
			cpByName[cp.Name] = append(cpByName[cp.Name], cp)
		}

		allControlPlanesInit = true
		return nil
	}

	for i := range controlPlanes {
		cp := &controlPlanes[i]

		if cp.GetKonnectID() != "" {
			continue
		}

		if cp.IsExternal() {
			if err := loadAllControlPlanes(); err != nil {
				return err
			}

			var match *state.ControlPlane

			if cp.External.ID != "" {
				match = cpByID[cp.External.ID]
				if match == nil {
					return fmt.Errorf("external control_plane %s: not found with id %s", cp.GetRef(), cp.External.ID)
				}
			} else if cp.External.Selector != nil {
				name, ok := cp.External.Selector.MatchFields["name"]
				if !ok {
					return fmt.Errorf("external control_plane %s: selector currently supports 'name' field", cp.GetRef())
				}

				matches := cpByName[name]
				if len(matches) == 0 {
					return fmt.Errorf("external control_plane %s: no control plane found with name %q", cp.GetRef(), name)
				}
				if len(matches) > 1 {
					return fmt.Errorf(
						"external control_plane %s: selector matched %d control planes for name %q",
						cp.GetRef(),
						len(matches),
						name,
					)
				}
				match = matches[0]
			} else {
				return fmt.Errorf("external control_plane %s: invalid _external configuration", cp.GetRef())
			}

			if !cp.TryMatchKonnectResource(match) {
				return fmt.Errorf("external control_plane %s: failed to bind Konnect resource", cp.GetRef())
			}
			if cp.Name == "" && match.Name != "" {
				cp.Name = match.Name
			}

			p.logger.Debug("Resolved external control plane",
				slog.String("ref", cp.GetRef()),
				slog.String("id", cp.GetKonnectID()),
			)
			continue
		}

		filter := cp.GetKonnectMonikerFilter()
		if filter == "" {
			continue
		}

		konnectCP, err := p.client.GetControlPlaneByFilter(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to lookup control plane %s: %w", cp.GetRef(), err)
		}

		if konnectCP != nil {
			cp.TryMatchKonnectResource(konnectCP)
		}
	}

	return nil
}

func (p *Planner) resolveGatewayServiceIdentities(
	ctx context.Context,
	services []resources.GatewayServiceResource,
	controlPlanes []resources.ControlPlaneResource,
) error {
	if len(services) == 0 {
		return nil
	}

	cpByRef := make(map[string]*resources.ControlPlaneResource, len(controlPlanes))
	for i := range controlPlanes {
		cp := &controlPlanes[i]
		cpByRef[cp.GetRef()] = cp
	}

	serviceCache := make(map[string][]state.GatewayService)

	for i := range services {
		service := &services[i]

		cpID, err := p.resolveGatewayServiceControlPlaneID(service, cpByRef)
		if err != nil {
			return err
		}

		service.SetResolvedControlPlaneID(cpID)

		if !service.IsExternal() {
			// Managed services will be resolved when supported; for now record CP ID only.
			continue
		}

		if service.HasDeckRequires() {
			if cpID == "" {
				p.logger.Debug("Skipping gateway service lookup; control plane ID not resolved",
					slog.String("ref", service.GetRef()),
				)
				continue
			}

			available, ok := serviceCache[cpID]
			if !ok {
				list, err := p.client.ListGatewayServices(ctx, cpID)
				if err != nil {
					return fmt.Errorf("failed to list gateway services for control plane %s: %w", cpID, err)
				}
				available = list
				serviceCache[cpID] = available
			}

			match, err := p.matchGatewayService(service, available)
			if err != nil {
				if errors.Is(err, errGatewayServiceNotFound) {
					p.logger.Debug("External gateway service not found; continuing due to deck requirements",
						slog.String("ref", service.GetRef()),
						slog.String("control_plane_id", cpID),
					)
					continue
				}
				return err
			}

			if !service.TryMatchKonnectResource(match) {
				return fmt.Errorf("gateway_service %s: failed to bind Konnect resource", service.GetRef())
			}

			service.SetResolvedControlPlaneID(match.ControlPlaneID)

			p.logger.Debug("Resolved external gateway service",
				slog.String("ref", service.GetRef()),
				slog.String("service_id", service.GetKonnectID()),
				slog.String("control_plane_id", match.ControlPlaneID),
			)
			continue
		}

		available, ok := serviceCache[cpID]
		if !ok {
			list, err := p.client.ListGatewayServices(ctx, cpID)
			if err != nil {
				return fmt.Errorf("failed to list gateway services for control plane %s: %w", cpID, err)
			}
			available = list
			serviceCache[cpID] = available
		}

		match, err := p.matchGatewayService(service, available)
		if err != nil {
			return err
		}

		if !service.TryMatchKonnectResource(match) {
			return fmt.Errorf("gateway_service %s: failed to bind Konnect resource", service.GetRef())
		}

		service.SetResolvedControlPlaneID(match.ControlPlaneID)

		p.logger.Debug("Resolved external gateway service",
			slog.String("ref", service.GetRef()),
			slog.String("service_id", service.GetKonnectID()),
			slog.String("control_plane_id", match.ControlPlaneID),
		)
	}

	return nil
}

func (p *Planner) resolveGatewayServiceControlPlaneID(
	service *resources.GatewayServiceResource,
	cpByRef map[string]*resources.ControlPlaneResource,
) (string, error) {
	value := service.ControlPlane
	if value == "" {
		return "", fmt.Errorf("gateway_service %s: control_plane is required", service.GetRef())
	}

	if tags.IsRefPlaceholder(value) {
		ref, field, ok := tags.ParseRefPlaceholder(value)
		if !ok {
			return "", fmt.Errorf("gateway_service %s: invalid control_plane reference", service.GetRef())
		}
		if field != "id" {
			return "", fmt.Errorf("gateway_service %s: control_plane references support '#id' only", service.GetRef())
		}
		value = ref
	}

	if util.IsValidUUID(value) {
		return value, nil
	}

	cpResource, ok := cpByRef[value]
	if !ok {
		return "", fmt.Errorf("gateway_service %s: referenced control_plane %s not found", service.GetRef(), value)
	}

	if cpResource.GetKonnectID() == "" {
		if service.HasDeckRequires() {
			return "", nil
		}
		return "", fmt.Errorf(
			"gateway_service %s: control_plane %s does not have a resolved Konnect ID",
			service.GetRef(),
			cpResource.GetRef(),
		)
	}

	return cpResource.GetKonnectID(), nil
}

func (p *Planner) matchGatewayService(
	service *resources.GatewayServiceResource,
	available []state.GatewayService,
) (*state.GatewayService, error) {
	var match *state.GatewayService

	if service.External == nil {
		return nil, fmt.Errorf("gateway_service %s: _external block required for external resolution", service.GetRef())
	}

	if service.External.ID != "" {
		for i := range available {
			if available[i].ID == service.External.ID {
				match = &available[i]
				break
			}
		}
		if match == nil {
			return nil, fmt.Errorf(
				"%w: external gateway_service %s: no service found with id %s",
				errGatewayServiceNotFound,
				service.GetRef(),
				service.External.ID,
			)
		}
		return match, nil
	}

	if service.External.Selector != nil {
		matchFields := service.External.Selector.MatchFields
		for i := range available {
			candidate := available[i]
			if service.External.Selector.Match(candidate) {
				if match != nil {
					return nil, fmt.Errorf(
						"external gateway_service %s: selector %v matched multiple services",
						service.GetRef(),
						matchFields,
					)
				}
				match = &available[i]
			}
		}

		if match == nil {
			return nil, fmt.Errorf(
				"%w: external gateway_service %s: selector %v did not match any services",
				errGatewayServiceNotFound,
				service.GetRef(),
				matchFields,
			)
		}

		return match, nil
	}

	return nil, fmt.Errorf("external gateway_service %s: invalid _external configuration", service.GetRef())
}

func (p *Planner) resolveAPIImplementationServiceReferences(rs *resources.ResourceSet) error {
	if len(rs.APIImplementations) == 0 {
		return nil
	}

	serviceByRef := make(map[string]*resources.GatewayServiceResource, len(rs.GatewayServices))
	for i := range rs.GatewayServices {
		svc := &rs.GatewayServices[i]
		serviceByRef[svc.GetRef()] = svc
	}

	controlPlaneByRef := make(map[string]*resources.ControlPlaneResource, len(rs.ControlPlanes))
	for i := range rs.ControlPlanes {
		cp := &rs.ControlPlanes[i]
		controlPlaneByRef[cp.GetRef()] = cp
	}

	for i := range rs.APIImplementations {
		impl := &rs.APIImplementations[i]
		if err := p.normalizeAPIImplementationService(impl, serviceByRef, controlPlaneByRef); err != nil {
			return err
		}
	}

	return nil
}

func (p *Planner) normalizeAPIImplementationService(
	impl *resources.APIImplementationResource,
	serviceByRef map[string]*resources.GatewayServiceResource,
	controlPlaneByRef map[string]*resources.ControlPlaneResource,
) error {
	if impl.ServiceReference == nil {
		return nil
	}

	service := impl.ServiceReference.GetService()
	if service == nil {
		return nil
	}

	implRef := impl.GetRef()
	if implRef == "" && impl.API != "" {
		implRef = fmt.Sprintf("%s implementation", impl.API)
	}

	serviceID := strings.TrimSpace(service.ID)
	if serviceID == "" {
		return fmt.Errorf("api_implementation %s: service.id is required", implRef)
	}

	resolvedServiceID, linkedService, err := p.resolveGatewayServiceReference(serviceID, serviceByRef, implRef)
	if err != nil {
		return err
	}
	service.ID = resolvedServiceID

	resolvedControlPlaneID, err := p.resolveImplementationControlPlaneID(
		strings.TrimSpace(service.ControlPlaneID),
		linkedService,
		controlPlaneByRef,
		implRef,
	)
	if err != nil {
		return err
	}
	service.ControlPlaneID = resolvedControlPlaneID

	return nil
}

func (p *Planner) resolveGatewayServiceReference(
	value string,
	serviceByRef map[string]*resources.GatewayServiceResource,
	implRef string,
) (string, *resources.GatewayServiceResource, error) {
	if tags.IsRefPlaceholder(value) {
		ref, field, ok := tags.ParseRefPlaceholder(value)
		if !ok {
			return "", nil, fmt.Errorf("api_implementation %s: invalid service.id reference", implRef)
		}
		if field != "id" {
			return "", nil, fmt.Errorf("api_implementation %s: service.id references support '#id' only", implRef)
		}
		svc, ok := serviceByRef[ref]
		if !ok {
			return "", nil, fmt.Errorf("api_implementation %s: gateway_service %s not found", implRef, ref)
		}
		if svc.GetKonnectID() == "" {
			if svc.HasDeckRequires() {
				return value, svc, nil
			}
			return "", nil, fmt.Errorf(
				"api_implementation %s: gateway_service %s does not have a resolved ID",
				implRef,
				svc.GetRef(),
			)
		}
		return svc.GetKonnectID(), svc, nil
	}

	if util.IsValidUUID(value) {
		return value, nil, nil
	}

	svc, ok := serviceByRef[value]
	if !ok {
		return "", nil, fmt.Errorf("api_implementation %s: gateway_service %s not found", implRef, value)
	}

	if svc.GetKonnectID() == "" {
		if svc.HasDeckRequires() {
			placeholder := fmt.Sprintf("%s%s#id", tags.RefPlaceholderPrefix, svc.GetRef())
			return placeholder, svc, nil
		}
		return "", nil, fmt.Errorf(
			"api_implementation %s: gateway_service %s does not have a resolved ID",
			implRef,
			svc.GetRef(),
		)
	}

	return svc.GetKonnectID(), svc, nil
}

func (p *Planner) resolveImplementationControlPlaneID(
	value string,
	linkedService *resources.GatewayServiceResource,
	controlPlaneByRef map[string]*resources.ControlPlaneResource,
	implRef string,
) (string, error) {
	if tags.IsRefPlaceholder(value) {
		ref, field, ok := tags.ParseRefPlaceholder(value)
		if !ok {
			return "", fmt.Errorf("api_implementation %s: invalid control_plane reference", implRef)
		}
		if field != "id" {
			return "", fmt.Errorf("api_implementation %s: control_plane references support '#id' only", implRef)
		}
		value = ref
	}

	if value == "" && linkedService != nil && linkedService.ResolvedControlPlaneID() != "" {
		return linkedService.ResolvedControlPlaneID(), nil
	}

	if util.IsValidUUID(value) {
		return value, nil
	}

	if value == "" {
		return "", fmt.Errorf("api_implementation %s: service.control_plane_id is required", implRef)
	}

	cp, ok := controlPlaneByRef[value]
	if !ok {
		return "", fmt.Errorf("api_implementation %s: control_plane %s not found", implRef, value)
	}

	if cp.GetKonnectID() == "" {
		if linkedService != nil && linkedService.HasDeckRequires() {
			return value, nil
		}
		return "", fmt.Errorf(
			"api_implementation %s: control_plane %s does not have a resolved Konnect ID",
			implRef,
			cp.GetRef(),
		)
	}

	resolved := cp.GetKonnectID()
	if linkedService != nil && linkedService.ResolvedControlPlaneID() != "" {
		resolved = linkedService.ResolvedControlPlaneID()
	}

	return resolved, nil
}

// resolvePortalIdentities resolves Konnect IDs for Portal resources
func (p *Planner) resolvePortalIdentities(ctx context.Context, portals []resources.PortalResource) error {
	// First pass: resolve external portals
	for i := range portals {
		portal := &portals[i]

		// Skip if not external
		if !portal.IsExternal() {
			continue
		}

		// Skip if already resolved
		if portal.GetKonnectID() != "" {
			continue
		}

		// Resolve external portal
		var konnectPortal *state.Portal
		var err error

		if portal.External.ID != "" {
			// Direct ID lookup - need to find the portal by ID
			// For now, we'll use ListAllPortals and filter by ID
			allPortals, err := p.client.ListAllPortals(ctx)
			if err != nil {
				return fmt.Errorf("failed to list portals for external lookup: %w", err)
			}

			for _, p := range allPortals {
				if p.ID == portal.External.ID {
					konnectPortal = &p
					break
				}
			}

			if konnectPortal == nil {
				return fmt.Errorf("external portal not found with ID: %s", portal.External.ID)
			}
		} else if portal.External.Selector != nil {
			// Selector-based lookup
			if name, ok := portal.External.Selector.MatchFields["name"]; ok {
				// Name-based lookup
				allPortals, err := p.client.ListAllPortals(ctx)
				if err != nil {
					return fmt.Errorf("failed to list portals for external lookup: %w", err)
				}

				for _, p := range allPortals {
					if p.Name == name {
						konnectPortal = &p
						break
					}
				}

				if konnectPortal == nil {
					return fmt.Errorf("external portal not found with name: %s", name)
				}
			} else {
				return fmt.Errorf("external portal %s: only 'name' field selector is currently supported", portal.GetRef())
			}
		}

		if err != nil {
			return fmt.Errorf("failed to resolve external portal %s: %w", portal.GetRef(), err)
		}

		if konnectPortal != nil {
			// Set the ID directly for external portals
			// We use reflection via TryMatchKonnectResource to set the konnectID
			if portal.TryMatchKonnectResource(konnectPortal) {
				// Align the desired portal name with Konnect resource to avoid mismatches in planning.
				if konnectPortal.Name != "" {
					portal.Name = konnectPortal.Name
				}
			}

			p.logger.Debug("Resolved external portal",
				slog.String("ref", portal.GetRef()),
				slog.String("id", portal.GetKonnectID()),
				slog.String("name", konnectPortal.Name),
			)
		}
	}

	// Second pass: resolve managed portals (existing logic)
	for i := range portals {
		portal := &portals[i]

		// Skip external portals (already resolved)
		if portal.IsExternal() {
			continue
		}

		// Skip if already resolved
		if portal.GetKonnectID() != "" {
			continue
		}

		// Try to find the portal using filter
		filter := portal.GetKonnectMonikerFilter()
		if filter == "" {
			continue
		}

		konnectPortal, err := p.client.GetPortalByFilter(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to lookup portal %s: %w", portal.GetRef(), err)
		}

		if konnectPortal != nil {
			// Match found, update the resource
			portal.TryMatchKonnectResource(konnectPortal)
		}
	}

	return nil
}

// resolveAuthStrategyIdentities resolves Konnect IDs for Auth Strategy resources
func (p *Planner) resolveAuthStrategyIdentities(
	ctx context.Context, strategies []resources.ApplicationAuthStrategyResource,
) error {
	for i := range strategies {
		strategy := &strategies[i]

		// Skip if already resolved
		if strategy.GetKonnectID() != "" {
			continue
		}

		// Try to find the strategy using filter
		filter := strategy.GetKonnectMonikerFilter()
		if filter == "" {
			continue
		}

		konnectStrategy, err := p.client.GetAuthStrategyByFilter(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to lookup auth strategy %s: %w", strategy.GetRef(), err)
		}

		if konnectStrategy != nil {
			// Match found, update the resource
			strategy.TryMatchKonnectResource(konnectStrategy)
		}
	}

	return nil
}

// getResourceNamespaces extracts all unique namespaces from the desired resources
func (p *Planner) getResourceNamespaces(rs *resources.ResourceSet) []string {
	namespaceSet := make(map[string]bool)
	hasExternalPortals := false

	// Extract namespaces from parent resources
	for _, portal := range rs.Portals {
		if portal.IsExternal() {
			hasExternalPortals = true
			continue
		}
		ns := resources.GetNamespace(portal.Kongctl)
		namespaceSet[ns] = true
	}

	for _, cp := range rs.ControlPlanes {
		ns := resources.GetNamespace(cp.Kongctl)
		namespaceSet[ns] = true
	}

	for _, svc := range rs.CatalogServices {
		ns := resources.GetNamespace(svc.Kongctl)
		namespaceSet[ns] = true
	}

	for _, api := range rs.APIs {
		ns := resources.GetNamespace(api.Kongctl)
		namespaceSet[ns] = true
	}

	for _, strategy := range rs.ApplicationAuthStrategies {
		ns := resources.GetNamespace(strategy.Kongctl)
		namespaceSet[ns] = true
	}

	for _, cp := range rs.EventGatewayControlPlanes {
		ns := resources.GetNamespace(cp.Kongctl)
		namespaceSet[ns] = true
	}

	// Convert set to sorted slice for consistent ordering
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	// Sort for consistent processing order
	sort.Strings(namespaces)

	if hasExternalPortals {
		namespaces = append(namespaces, resources.NamespaceExternal)
	}

	return namespaces
}

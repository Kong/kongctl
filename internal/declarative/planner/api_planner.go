package planner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

// apiPlannerImpl implements planning logic for API resources
type apiPlannerImpl struct {
	*BasePlanner
}

// NewAPIPlanner creates a new API planner
func NewAPIPlanner(base *BasePlanner) APIPlanner {
	return &apiPlannerImpl{
		BasePlanner: base,
	}
}

// PlanChanges generates changes for API resources and their child resources
func (a *apiPlannerImpl) PlanChanges(ctx context.Context, plan *Plan) error {
	// Debug logging
	desiredAPIs := a.GetDesiredAPIs()
	a.planner.logger.Debug("apiPlannerImpl.PlanChanges called", "desiredAPIs", len(desiredAPIs))
	
	// Plan API resources
	if err := a.planner.planAPIChanges(ctx, desiredAPIs, plan); err != nil {
		return err
	}
	
	// Plan child resources that are defined separately
	if err := a.planner.planAPIVersionsChanges(ctx, a.GetDesiredAPIVersions(), plan); err != nil {
		return err
	}
	
	if err := a.planner.planAPIPublicationsChanges(ctx, a.GetDesiredAPIPublications(), plan); err != nil {
		return err
	}
	
	if err := a.planner.planAPIImplementationsChanges(ctx, a.GetDesiredAPIImplementations(), plan); err != nil {
		return err
	}
	
	if err := a.planner.planAPIDocumentsChanges(ctx, a.GetDesiredAPIDocuments(), plan); err != nil {
		return err
	}
	
	return nil
}

// planAPIChanges generates changes for API resources and their child resources
func (p *Planner) planAPIChanges(ctx context.Context, desired []resources.APIResource, plan *Plan) error {
	// Debug logging
	p.logger.Debug("planAPIChanges called", "desiredCount", len(desired))
	
	// Skip if no API resources to plan and not in sync mode
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		p.logger.Debug("Skipping API planning - no desired APIs")
		return nil
	}

	// Get namespace from context
	namespace, ok := ctx.Value(NamespaceContextKey).(string)
	if !ok {
		// Default to all namespaces for backward compatibility
		namespace = "*"
	}
	
	// Fetch current managed APIs from the specific namespace
	namespaceFilter := []string{namespace}
	currentAPIs, err := p.client.ListManagedAPIs(ctx, namespaceFilter)
	if err != nil {
		// If API client is not configured, skip API planning
		if err.Error() == "API client not configured" {
			return nil
		}
		return fmt.Errorf("failed to list current APIs: %w", err)
	}

	// Index current APIs by name
	currentByName := make(map[string]state.API)
	for _, api := range currentAPIs {
		currentByName[api.Name] = api
	}

	// Collect protection validation errors
	var protectionErrors []error

	// Compare each desired API
	for _, desiredAPI := range desired {
		current, exists := currentByName[desiredAPI.Name]

		if !exists {
			// CREATE action
			apiChangeID := p.planAPICreate(desiredAPI, plan)
			// Plan child resources after API creation
			p.planAPIChildResourcesCreate(desiredAPI, apiChangeID, plan)
		} else {
			// Check if update needed
			isProtected := labels.IsProtectedResource(current.NormalizedLabels)

			// Get protection status from desired configuration
			shouldProtect := false
			if desiredAPI.Kongctl != nil && desiredAPI.Kongctl.Protected != nil && *desiredAPI.Kongctl.Protected {
				shouldProtect = true
			}

			// Handle protection changes
			if isProtected != shouldProtect {
				// When changing protection status, include any other field updates too
				_, updateFields := p.shouldUpdateAPI(current, desiredAPI)
				p.planAPIProtectionChangeWithFields(current, desiredAPI, isProtected, shouldProtect, updateFields, plan)
			} else {
				// Check if update needed based on configuration
				needsUpdate, updateFields := p.shouldUpdateAPI(current, desiredAPI)
				if needsUpdate {
					// Regular update - check protection
					if err := p.validateProtection("api", desiredAPI.Name, isProtected, ActionUpdate); err != nil {
						protectionErrors = append(protectionErrors, err)
					} else {
						p.planAPIUpdateWithFields(current, desiredAPI, updateFields, plan)
					}
				}
			}

			// Plan child resource changes
			if err := p.planAPIChildResourceChanges(ctx, current, desiredAPI, plan); err != nil {
				return err
			}
		}
	}

	// Check for managed resources to delete (sync mode only)
	if plan.Metadata.Mode == PlanModeSync {
		// Build set of desired API names
		desiredNames := make(map[string]bool)
		for _, api := range desired {
			desiredNames[api.Name] = true
		}

		// Find managed APIs not in desired state
		for name, current := range currentByName {
			if !desiredNames[name] {
				// Validate protection before adding DELETE
				isProtected := labels.IsProtectedResource(current.NormalizedLabels)
				if err := p.validateProtection("api", name, isProtected, ActionDelete); err != nil {
					protectionErrors = append(protectionErrors, err)
				} else {
					p.planAPIDelete(current, plan)
				}
			}
		}
	}

	// Fail fast if any protected resources would be modified
	if len(protectionErrors) > 0 {
		errMsg := "Cannot generate plan due to protected resources:\n"
		for _, err := range protectionErrors {
			errMsg += fmt.Sprintf("- %s\n", err.Error())
		}
		errMsg += "\nTo proceed, first update these resources to set protected: false"
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// extractAPIFields extracts fields from an API resource for planner operations
func extractAPIFields(resource interface{}) map[string]interface{} {
	fields := make(map[string]interface{})
	
	api, ok := resource.(resources.APIResource)
	if !ok {
		return fields
	}
	
	fields["name"] = api.Name
	if api.Description != nil {
		fields["description"] = *api.Description
	}
	
	// Copy user-defined labels only (protection label will be added during execution)
	if len(api.Labels) > 0 {
		labelsMap := make(map[string]interface{})
		for k, v := range api.Labels {
			labelsMap[k] = v
		}
		fields["labels"] = labelsMap
	}
	
	return fields
}

// planAPICreate creates a CREATE change for an API
func (p *Planner) planAPICreate(api resources.APIResource, plan *Plan) string {
	generic := p.genericPlanner
	
	// Extract protection status
	var protection interface{}
	if api.Kongctl != nil && api.Kongctl.Protected != nil {
		protection = *api.Kongctl.Protected
	}
	
	// Extract namespace
	namespace := DefaultNamespace
	if api.Kongctl != nil && api.Kongctl.Namespace != nil {
		namespace = *api.Kongctl.Namespace
	}
	
	config := CreateConfig{
		ResourceType:   "api",
		ResourceName:   api.Name,
		ResourceRef:    api.GetRef(),
		RequiredFields: []string{"name"},
		FieldExtractor: func(_ interface{}) map[string]interface{} {
			return extractAPIFields(api)
		},
		Namespace:      namespace,
		DependsOn:      []string{},
	}
	
	change, err := generic.PlanCreate(context.Background(), config)
	if err != nil {
		// This shouldn't happen with valid configuration
		p.logger.Error("Failed to plan API create", slog.String("error", err.Error()))
		return ""
	}
	
	// Set protection after creation
	change.Protection = protection
	
	plan.AddChange(change)
	return change.ID
}

// shouldUpdateAPI checks if API needs update based on configured fields only
func (p *Planner) shouldUpdateAPI(
	current state.API,
	desired resources.APIResource,
) (bool, map[string]interface{}) {
	updates := make(map[string]interface{})

	// Only compare fields present in desired configuration
	if desired.Description != nil {
		currentDesc := getString(current.Description)
		if currentDesc != *desired.Description {
			updates["description"] = *desired.Description
		}
	}

	// Check if labels are defined in the desired state
	if desired.Labels != nil {
		// Compare only user labels to determine if update is needed
		if labels.CompareUserLabels(current.NormalizedLabels, desired.Labels) {
			// User labels differ, include all labels in update
			labelsMap := make(map[string]interface{})
			for k, v := range desired.Labels {
				labelsMap[k] = v
			}
			updates["labels"] = labelsMap
		}
	}

	return len(updates) > 0, updates
}

// planAPIUpdateWithFields creates an UPDATE change with specific fields
func (p *Planner) planAPIUpdateWithFields(
	current state.API,
	desired resources.APIResource,
	updateFields map[string]interface{},
	plan *Plan,
) {
	// Pass current labels so executor can properly handle removals
	if _, hasLabels := updateFields["labels"]; hasLabels {
		updateFields[FieldCurrentLabels] = current.NormalizedLabels
	}
	
	// Extract namespace
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}
	
	config := UpdateConfig{
		ResourceType:   "api",
		ResourceName:   desired.Name,
		ResourceRef:    desired.GetRef(),
		ResourceID:     current.ID,
		CurrentFields:  nil, // Not needed for direct update
		DesiredFields:  updateFields,
		RequiredFields: []string{"name"},
		Namespace:      namespace,
	}
	
	change, err := p.genericPlanner.PlanUpdate(context.Background(), config)
	if err != nil {
		// This shouldn't happen with valid configuration
		p.logger.Error("Failed to plan API update", slog.String("error", err.Error()))
		return
	}
	
	// Check if already protected
	if labels.IsProtectedResource(current.NormalizedLabels) {
		change.Protection = true
	}
	
	plan.AddChange(change)
}

// planAPIProtectionChangeWithFields creates an UPDATE for protection status with optional field updates
func (p *Planner) planAPIProtectionChangeWithFields(
	current state.API,
	desired resources.APIResource,
	wasProtected, shouldProtect bool,
	updateFields map[string]interface{},
	plan *Plan,
) {
	// Extract namespace
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}
	
	// Use generic protection change planner
	config := ProtectionChangeConfig{
		ResourceType:  "api",
		ResourceName:  desired.Name,
		ResourceRef:   desired.GetRef(),
		ResourceID:    current.ID,
		OldProtected:  wasProtected,
		NewProtected:  shouldProtect,
		Namespace:     namespace,
	}
	
	change := p.genericPlanner.PlanProtectionChange(context.Background(), config)
	
	// Include any field updates if unprotecting
	if wasProtected && !shouldProtect && len(updateFields) > 0 {
		fields := make(map[string]interface{})
		for field, newValue := range updateFields {
			fields[field] = newValue
		}
		// Always include name for identification
		fields["name"] = current.Name
		change.Fields = fields
	}
	
	plan.AddChange(change)
}

// planAPIDelete creates a DELETE change for an API
func (p *Planner) planAPIDelete(api state.API, plan *Plan) {
	// Extract namespace from labels (for existing resources being deleted)
	namespace := DefaultNamespace
	if ns, ok := api.NormalizedLabels[labels.NamespaceKey]; ok {
		namespace = ns
	}
	
	config := DeleteConfig{
		ResourceType: "api",
		ResourceName: api.Name,
		ResourceRef:  api.Name,
		ResourceID:   api.ID,
		Namespace:    namespace,
	}
	
	change := p.genericPlanner.PlanDelete(context.Background(), config)
	// Add the name field for backward compatibility
	change.Fields = map[string]interface{}{"name": api.Name}
	
	plan.AddChange(change)
}

// planAPIChildResourcesCreate plans creation of child resources for a new API
func (p *Planner) planAPIChildResourcesCreate(api resources.APIResource, apiChangeID string, plan *Plan) {
	// Plan version creation - API ID is not yet known
	for _, version := range api.Versions {
		p.planAPIVersionCreate(api.GetRef(), "", version, []string{apiChangeID}, plan)
	}

	// Plan publication creation - API ID is not yet known
	for _, publication := range api.Publications {
		p.planAPIPublicationCreate(api.GetRef(), "", publication, []string{apiChangeID}, plan)
	}

	// Plan implementation creation - API ID is not yet known
	for _, implementation := range api.Implementations {
		p.planAPIImplementationCreate(api.GetRef(), "", implementation, []string{apiChangeID}, plan)
	}

	// Plan document creation - API ID is not yet known
	for _, document := range api.Documents {
		p.planAPIDocumentCreate(api.GetRef(), "", document, []string{apiChangeID}, plan)
	}
}

// planAPIChildResourceChanges plans changes for child resources of an existing API
func (p *Planner) planAPIChildResourceChanges(
	ctx context.Context, current state.API, desired resources.APIResource, plan *Plan,
) error {
	// Plan version changes
	if err := p.planAPIVersionChanges(ctx, current.ID, desired.GetRef(), desired.Versions, plan); err != nil {
		return fmt.Errorf("failed to plan API version changes: %w", err)
	}

	// Plan publication changes
	if err := p.planAPIPublicationChanges(ctx, current.ID, desired.GetRef(), desired.Publications, plan); err != nil {
		return fmt.Errorf("failed to plan API publication changes: %w", err)
	}

	// Plan implementation changes
	if err := p.planAPIImplementationChanges(
		ctx, current.ID, desired.GetRef(), desired.Implementations, plan); err != nil {
		return fmt.Errorf("failed to plan API implementation changes: %w", err)
	}

	// Plan document changes
	if err := p.planAPIDocumentChanges(ctx, current.ID, desired.GetRef(), desired.Documents, plan); err != nil {
		return fmt.Errorf("failed to plan API document changes: %w", err)
	}

	return nil
}

// API Version planning

func (p *Planner) planAPIVersionChanges(
	ctx context.Context, apiID string, apiRef string, desired []resources.APIVersionResource, plan *Plan,
) error {
	// List current versions
	currentVersions, err := p.client.ListAPIVersions(ctx, apiID)
	if err != nil {
		return fmt.Errorf("failed to list current API versions: %w", err)
	}

	// Index current versions by version string
	currentByVersion := make(map[string]state.APIVersion)
	for _, v := range currentVersions {
		currentByVersion[v.Version] = v
	}

	// Compare desired versions
	for _, desiredVersion := range desired {
		versionStr := ""
		if desiredVersion.Version != nil {
			versionStr = *desiredVersion.Version
		}

		if _, exists := currentByVersion[versionStr]; !exists {
			// CREATE - versions don't support update
			p.planAPIVersionCreate(apiRef, apiID, desiredVersion, []string{}, plan)
		}
		// Note: API versions don't support update operations
	}

	// In sync mode, delete unmanaged versions
	if plan.Metadata.Mode == PlanModeSync {
		// Check if there are extracted versions for this API that will be processed later
		hasExtractedVersions := false
		for _, ver := range p.desiredAPIVersions {
			if ver.API == apiRef {
				hasExtractedVersions = true
				break
			}
		}
		
		// If there are extracted versions, skip deletion during child resource planning
		if hasExtractedVersions && len(desired) == 0 {
			p.logger.Debug("Skipping version deletion - extracted versions exist",
				slog.String("api", apiRef),
				slog.Int("current_count", len(currentByVersion)),
			)
			return nil
		}
		
		desiredVersions := make(map[string]bool)
		for _, ver := range desired {
			if ver.Version != nil {
				desiredVersions[*ver.Version] = true
			}
		}

		p.logger.Debug("Sync mode: checking for versions to delete",
			slog.String("api", apiRef),
			slog.Int("current_count", len(currentByVersion)),
			slog.Int("desired_count", len(desiredVersions)),
		)

		for versionStr, current := range currentByVersion {
			if !desiredVersions[versionStr] {
				p.logger.Debug("Marking version for deletion",
					slog.String("api", apiRef),
					slog.String("version", versionStr),
					slog.String("version_id", current.ID),
				)
				p.planAPIVersionDelete(apiRef, apiID, current.ID, versionStr, plan)
			}
		}
	}

	return nil
}

func (p *Planner) planAPIVersionCreate(
	apiRef string, apiID string, version resources.APIVersionResource, dependsOn []string, plan *Plan,
) {
	fields := make(map[string]interface{})
	if version.Version != nil {
		fields["version"] = *version.Version
	}
	if version.Spec != nil && version.Spec.Content != nil {
		// Store spec as a map with content field for proper JSON serialization
		fields["spec"] = map[string]interface{}{
			"content": *version.Spec.Content,
		}
	}
	// Note: PublishStatus, Deprecated, SunsetDate are not supported by the SDK create operation

	parentInfo := &ParentInfo{Ref: apiRef}
	if apiID != "" {
		parentInfo.ID = apiID
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, "api_version", version.GetRef()),
		ResourceType: "api_version",
		ResourceRef:  version.GetRef(),
		Parent:       parentInfo,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependsOn,
	}

	plan.AddChange(change)
}

func (p *Planner) planAPIVersionDelete(apiRef string, apiID string, versionID string, versionStr string, plan *Plan) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, "api_version", versionID),
		ResourceType: "api_version",
		ResourceRef:  "[unknown]",
		ResourceID:   versionID,
		ResourceMonikers: map[string]string{
			"version":    versionStr,
			"parent_api": apiRef,
		},
		Parent:    &ParentInfo{Ref: apiRef, ID: apiID},
		Action:    ActionDelete,
		Fields:    map[string]interface{}{"version": versionStr},
		DependsOn: []string{},
	}

	plan.AddChange(change)
}

// API Publication planning

func (p *Planner) planAPIPublicationChanges(
	ctx context.Context, apiID string, apiRef string, desired []resources.APIPublicationResource, plan *Plan,
) error {
	// Get namespace from context
	namespace, ok := ctx.Value(NamespaceContextKey).(string)
	if !ok {
		// Default to all namespaces for backward compatibility
		namespace = "*"
	}
	namespaceFilter := []string{namespace}
	
	// List current publications
	currentPublications, err := p.client.ListAPIPublications(ctx, apiID)
	if err != nil {
		return fmt.Errorf("failed to list current API publications: %w", err)
	}

	// Index current publications by portal ID
	currentByPortal := make(map[string]state.APIPublication)
	for _, p := range currentPublications {
		currentByPortal[p.PortalID] = p
	}

	// Build portal ref to ID mapping (following portal pages pattern)
	portalRefToID := make(map[string]string)
	portalIDToRef := make(map[string]string) // Reverse mapping for deletion display
	
	p.logger.Debug("Building portal reference mapping",
		slog.Int("desired_portals", len(p.desiredPortals)),
	)
	
	// First, add desired portals to the mapping
	for _, portal := range p.desiredPortals {
		if resolvedID := portal.GetKonnectID(); resolvedID != "" {
			portalRefToID[portal.Ref] = resolvedID
			portalIDToRef[resolvedID] = portal.Ref
			p.logger.Debug("Added desired portal to mapping",
				slog.String("ref", portal.Ref),
				slog.String("id", resolvedID),
			)
		}
	}
	
	// Also fetch managed portals from the same namespace to ensure complete mapping
	// This handles cases where publications exist for portals not in current desired state
	allPortals, err := p.client.ListManagedPortals(ctx, namespaceFilter)
	if err == nil {
		p.logger.Debug("Fetched all managed portals",
			slog.Int("count", len(allPortals)),
		)
		// Add any portals not already in the mapping
		for _, portal := range allPortals {
			if _, exists := portalIDToRef[portal.ID]; !exists {
				// Try to find the ref by matching name with desired portals
				// If not found, use the portal name as a fallback ref
				portalRef := portal.Name
				for _, desiredPortal := range p.desiredPortals {
					if desiredPortal.Name == portal.Name {
						portalRef = desiredPortal.Ref
						break
					}
				}
				portalIDToRef[portal.ID] = portalRef
				// Also add to portalRefToID if the ref isn't already mapped
				if _, exists := portalRefToID[portalRef]; !exists {
					portalRefToID[portalRef] = portal.ID
				}
				p.logger.Debug("Added existing portal to mapping",
					slog.String("name", portal.Name),
					slog.String("ref", portalRef),
					slog.String("id", portal.ID),
				)
			}
		}
	} else {
		p.logger.Debug("Failed to fetch managed portals",
			slog.String("error", err.Error()),
		)
	}

	// Compare desired publications
	for _, desiredPub := range desired {
		// Resolve portal reference to ID before comparing
		resolvedPortalID := desiredPub.PortalID
		if id, ok := portalRefToID[desiredPub.PortalID]; ok {
			resolvedPortalID = id
		}

		_, exists := currentByPortal[resolvedPortalID]
		
		p.logger.Debug("Checking publication existence",
			slog.String("api", apiRef),
			slog.String("portal_ref", desiredPub.PortalID),
			slog.String("resolved_portal_id", resolvedPortalID),
			slog.Bool("exists", exists),
		)

		if !exists {
			// CREATE - publications don't support update
			p.planAPIPublicationCreate(apiRef, apiID, desiredPub, []string{}, plan)
		}
		// Note: Publications are identified by portal ID, not a separate ID
	}

	// In sync mode, delete unmanaged publications
	if plan.Metadata.Mode == PlanModeSync {
		// Check if there are extracted publications for this API that will be processed later
		hasExtractedPublications := false
		for _, pub := range p.desiredAPIPublications {
			if pub.API == apiRef {
				hasExtractedPublications = true
				break
			}
		}
		
		// If there are extracted publications, skip deletion during child resource planning
		// The extracted publications will handle deletion properly when they are processed
		if hasExtractedPublications && len(desired) == 0 {
			p.logger.Debug("Skipping publication deletion - extracted publications exist",
				slog.String("api", apiRef),
				slog.Int("current_count", len(currentByPortal)),
			)
			return nil
		}
		
		desiredPortals := make(map[string]bool)
		for _, pub := range desired {
			// Use resolved portal ID for sync mode comparison
			resolvedPortalID := pub.PortalID
			if id, ok := portalRefToID[pub.PortalID]; ok {
				resolvedPortalID = id
			}
			desiredPortals[resolvedPortalID] = true
			p.logger.Debug("Added to desired portals for sync",
				slog.String("api", apiRef),
				slog.String("portal_ref", pub.PortalID),
				slog.String("resolved_portal_id", resolvedPortalID),
			)
		}

		p.logger.Debug("Sync mode: checking for publications to delete",
			slog.String("api", apiRef),
			slog.Int("current_count", len(currentByPortal)),
			slog.Int("desired_count", len(desiredPortals)),
		)

		for portalID := range currentByPortal {
			if !desiredPortals[portalID] {
				p.logger.Debug("Marking publication for deletion",
					slog.String("api", apiRef),
					slog.String("portal_id", portalID),
					slog.Bool("in_desired", desiredPortals[portalID]),
				)
				// Get portal ref for better display
				portalRef := portalIDToRef[portalID]
				if portalRef == "" {
					portalRef = portalID // Fallback to ID if ref not found
				}
				p.planAPIPublicationDelete(apiRef, apiID, portalID, portalRef, plan)
			}
		}
	}

	return nil
}

func (p *Planner) planAPIPublicationCreate(
	apiRef string, apiID string, publication resources.APIPublicationResource, dependsOn []string, plan *Plan,
) {
	fields := make(map[string]interface{})
	fields["portal_id"] = publication.PortalID
	if publication.AuthStrategyIds != nil {
		fields["auth_strategy_ids"] = publication.AuthStrategyIds
		// Warn if multiple auth strategies are specified (Kong limitation)
		if len(publication.AuthStrategyIds) > 1 {
			plan.AddWarning(
				p.nextChangeID(ActionCreate, "api_publication", publication.GetRef()),
				"Kong currently only supports 1 auth strategy per API publication. Only the first auth strategy will be used.",
			)
		}
	}
	if publication.AutoApproveRegistrations != nil {
		fields["auto_approve_registrations"] = *publication.AutoApproveRegistrations
	}
	if publication.Visibility != nil {
		fields["visibility"] = string(*publication.Visibility)
	}

	parentInfo := &ParentInfo{Ref: apiRef}
	if apiID != "" {
		parentInfo.ID = apiID
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, "api_publication", publication.GetRef()),
		ResourceType: "api_publication",
		ResourceRef:  publication.GetRef(),
		Parent:       parentInfo,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependsOn,
	}

	// Look up portal name for reference resolution
	var portalName string
	for _, portal := range p.desiredPortals {
		if portal.Ref == publication.PortalID {
			portalName = portal.Name
			break
		}
	}

	// Set up reference with lookup fields
	if publication.PortalID != "" {
		change.References = map[string]ReferenceInfo{
			"portal_id": {
				Ref: publication.PortalID,
				LookupFields: map[string]string{
					"name": portalName,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planAPIPublicationDelete(apiRef string, apiID string, portalID string, portalRef string, plan *Plan) {
	// Create a composite reference that includes both API and portal for clarity
	compositeRef := fmt.Sprintf("%s-to-%s", apiRef, portalRef)
	
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, "api_publication", compositeRef),
		ResourceType: "api_publication",
		ResourceRef:  compositeRef,
		ResourceID:   fmt.Sprintf("%s:%s", apiID, portalID), // Composite ID for API publication
		Parent:       &ParentInfo{Ref: apiRef, ID: apiID},
		Action:       ActionDelete,
		Fields: map[string]interface{}{
			"api_id":    apiID,
			"portal_id": portalID,
		},
		DependsOn: []string{},
	}

	plan.AddChange(change)
}

// API Implementation planning

func (p *Planner) planAPIImplementationChanges(
	ctx context.Context, apiID string, _ string, desired []resources.APIImplementationResource, plan *Plan,
) error {
	// List current implementations
	currentImplementations, err := p.client.ListAPIImplementations(ctx, apiID)
	if err != nil {
		return fmt.Errorf("failed to list current API implementations: %w", err)
	}

	// Index current implementations by service ID + control plane ID
	currentByService := make(map[string]state.APIImplementation)
	for _, i := range currentImplementations {
		if i.Service != nil {
			key := fmt.Sprintf("%s:%s", i.Service.ID, i.Service.ControlPlaneID)
			currentByService[key] = i
		}
	}

	// Compare desired implementations
	for _, desiredImpl := range desired {
		if desiredImpl.Service != nil {
			key := fmt.Sprintf("%s:%s", desiredImpl.Service.ID, desiredImpl.Service.ControlPlaneID)
			if _, exists := currentByService[key]; !exists {
				// Skip CREATE - SDK doesn't support implementation creation yet
				// TODO: Enable when SDK adds support
				// p.planAPIImplementationCreate(apiRef, apiID, desiredImpl, []string{}, plan)
				_ = desiredImpl // Acknowledge we'd process this when SDK supports it
			}
			// Note: Implementation IDs are managed by the SDK
		}
	}

	// In sync mode, delete unmanaged implementations
	if plan.Metadata.Mode == PlanModeSync {
		desiredServices := make(map[string]bool)
		for _, impl := range desired {
			if impl.Service != nil {
				key := fmt.Sprintf("%s:%s", impl.Service.ID, impl.Service.ControlPlaneID)
				desiredServices[key] = true
			}
		}

		for serviceKey, current := range currentByService {
			if !desiredServices[serviceKey] {
				// Skip DELETE - SDK doesn't support implementation deletion yet
				// TODO: Enable when SDK adds support
				// p.planAPIImplementationDelete(apiRef, current.ID, plan)
				_ = current // suppress unused variable warning
			}
		}
	}

	return nil
}

func (p *Planner) planAPIImplementationCreate(
	apiRef string, apiID string, implementation resources.APIImplementationResource, dependsOn []string, plan *Plan,
) {
	fields := make(map[string]interface{})
	// APIImplementation only has Service field in the SDK
	if implementation.Service != nil {
		fields["service"] = map[string]interface{}{
			"id":               implementation.Service.ID,
			"control_plane_id": implementation.Service.ControlPlaneID,
		}
	}

	parentInfo := &ParentInfo{Ref: apiRef}
	if apiID != "" {
		parentInfo.ID = apiID
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, "api_implementation", implementation.GetRef()),
		ResourceType: "api_implementation",
		ResourceRef:  implementation.GetRef(),
		Parent:       parentInfo,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependsOn,
	}

	plan.AddChange(change)
}

// API Document planning

func (p *Planner) planAPIDocumentChanges(
	ctx context.Context, apiID string, apiRef string, desired []resources.APIDocumentResource, plan *Plan,
) error {
	// List current documents
	currentDocuments, err := p.client.ListAPIDocuments(ctx, apiID)
	if err != nil {
		return fmt.Errorf("failed to list current API documents: %w", err)
	}

	// Index current documents by slug
	// Normalize slugs by stripping leading slash for consistent matching
	currentBySlug := make(map[string]state.APIDocument)
	for _, d := range currentDocuments {
		normalizedSlug := strings.TrimPrefix(d.Slug, "/")
		currentBySlug[normalizedSlug] = d
	}

	// Compare desired documents
	for _, desiredDoc := range desired {
		slug := ""
		if desiredDoc.Slug != nil {
			slug = *desiredDoc.Slug
		}

		// Normalize desired slug for matching
		normalizedSlug := strings.TrimPrefix(slug, "/")
		current, exists := currentBySlug[normalizedSlug]

		if !exists {
			// CREATE
			p.planAPIDocumentCreate(apiRef, apiID, desiredDoc, []string{}, plan)
		} else {
			// UPDATE - documents support update
			// Fetch full document to get content for comparison
			fullDoc, err := p.client.GetAPIDocument(ctx, apiID, current.ID)
			if err != nil {
				return fmt.Errorf("failed to fetch document %s: %w", current.Slug, err)
			}
			if fullDoc != nil {
				current = *fullDoc
			}
			
			// Now compare with full content
			if p.shouldUpdateAPIDocument(current, desiredDoc) {
				p.planAPIDocumentUpdate(apiRef, apiID, current.ID, desiredDoc, plan)
			}
		}
	}

	// In sync mode, delete unmanaged documents
	if plan.Metadata.Mode == PlanModeSync {
		desiredSlugs := make(map[string]bool)
		for _, doc := range desired {
			if doc.Slug != nil {
				// Normalize desired slug
				normalizedSlug := strings.TrimPrefix(*doc.Slug, "/")
				desiredSlugs[normalizedSlug] = true
			}
		}

		for slug, current := range currentBySlug {
			if !desiredSlugs[slug] {
				p.planAPIDocumentDelete(apiRef, apiID, current.ID, slug, plan)
			}
		}
	}

	return nil
}

func (p *Planner) shouldUpdateAPIDocument(current state.APIDocument, desired resources.APIDocumentResource) bool {
	// Normalize content for comparison (trim whitespace)
	currentContent := strings.TrimSpace(current.Content)
	desiredContent := strings.TrimSpace(desired.Content)
	if currentContent != desiredContent {
		return true
	}
	if desired.Title != nil && current.Title != *desired.Title {
		return true
	}
	if desired.Status != nil && current.Status != string(*desired.Status) {
		return true
	}
	if desired.ParentDocumentID != "" && current.ParentDocumentID != desired.ParentDocumentID {
		return true
	}
	return false
}

func (p *Planner) planAPIDocumentCreate(
	apiRef string, apiID string, document resources.APIDocumentResource, dependsOn []string, plan *Plan,
) {
	fields := make(map[string]interface{})
	fields["content"] = document.Content
	if document.Title != nil {
		fields["title"] = *document.Title
	}
	if document.Slug != nil {
		fields["slug"] = *document.Slug
	}
	if document.Status != nil {
		fields["status"] = string(*document.Status)
	}
	if document.ParentDocumentID != "" {
		fields["parent_document_id"] = document.ParentDocumentID
	}

	parentInfo := &ParentInfo{Ref: apiRef}
	if apiID != "" {
		parentInfo.ID = apiID
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, "api_document", document.GetRef()),
		ResourceType: "api_document",
		ResourceRef:  document.GetRef(),
		Parent:       parentInfo,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependsOn,
	}

	// Set API reference for executor
	if apiRef != "" {
		// Find the API to get its name for lookup
		var apiName string
		for _, api := range p.desiredAPIs {
			if api.Ref == apiRef {
				apiName = api.Name
				break
			}
		}
		
		change.References = map[string]ReferenceInfo{
			"api_id": {
				Ref: apiRef,
				ID:  apiID, // May be empty if API doesn't exist yet
				LookupFields: map[string]string{
					"name": apiName,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planAPIDocumentUpdate(
	apiRef string, apiID string, documentID string, document resources.APIDocumentResource, plan *Plan,
) {
	fields := make(map[string]interface{})
	fields["content"] = document.Content
	if document.Title != nil {
		fields["title"] = *document.Title
	}
	if document.Slug != nil {
		fields["slug"] = *document.Slug
	}
	if document.Status != nil {
		fields["status"] = string(*document.Status)
	}
	if document.ParentDocumentID != "" {
		fields["parent_document_id"] = document.ParentDocumentID
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, "api_document", document.GetRef()),
		ResourceType: "api_document",
		ResourceRef:  document.GetRef(),
		ResourceID:   documentID,
		Parent:       &ParentInfo{Ref: apiRef, ID: apiID},
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Set API reference for executor
	if apiRef != "" {
		// Find the API to get its name for lookup
		var apiName string
		for _, api := range p.desiredAPIs {
			if api.Ref == apiRef {
				apiName = api.Name
				break
			}
		}
		
		change.References = map[string]ReferenceInfo{
			"api_id": {
				Ref: apiRef,
				ID:  apiID,
				LookupFields: map[string]string{
					"name": apiName,
				},
			},
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planAPIDocumentDelete(apiRef string, apiID string, documentID string, slug string, plan *Plan) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, "api_document", documentID),
		ResourceType: "api_document",
		ResourceRef:  "[unknown]",
		ResourceID:   documentID,
		ResourceMonikers: map[string]string{
			"slug":       slug,
			"parent_api": apiRef,
		},
		Parent:    &ParentInfo{Ref: apiRef, ID: apiID},
		Action:    ActionDelete,
		Fields:    map[string]interface{}{"slug": slug},
		DependsOn: []string{},
	}

	// Set API reference for executor
	if apiRef != "" {
		// Find the API to get its name for lookup
		var apiName string
		for _, api := range p.desiredAPIs {
			if api.Ref == apiRef {
				apiName = api.Name
				break
			}
		}
		
		change.References = map[string]ReferenceInfo{
			"api_id": {
				Ref: apiRef,
				ID:  apiID,
				LookupFields: map[string]string{
					"name": apiName,
				},
			},
		}
	}

	plan.AddChange(change)
}

// planAPIVersionsChanges plans changes for extracted API version resources
func (p *Planner) planAPIVersionsChanges(
	ctx context.Context, desired []resources.APIVersionResource, plan *Plan,
) error {
	// Group versions by parent API
	versionsByAPI := make(map[string][]resources.APIVersionResource)
	for _, version := range desired {
		versionsByAPI[version.API] = append(versionsByAPI[version.API], version)
	}

	// For each API, plan version changes
	for apiRef, versions := range versionsByAPI {
		// Find the API ID from existing changes or state
		apiID := ""
		for _, change := range plan.Changes {
			if change.ResourceType == "api" && change.ResourceRef == apiRef {
				if change.Action == ActionCreate {
					// API is being created, use dependency
					for _, v := range versions {
						p.planAPIVersionCreate(apiRef, "", v, []string{change.ID}, plan)
					}
					continue
				}
				apiID = change.ResourceID
				break
			}
		}

		// If API not in changes, use the resolved ID from pre-resolution phase
		if apiID == "" {
			// Find the API resource by ref to get its resolved ID
			for _, api := range p.GetDesiredAPIs() {
				if api.GetRef() == apiRef {
					resolvedID := api.GetKonnectID()
					if resolvedID != "" {
						apiID = resolvedID
					}
					break
				}
			}
		}

		if apiID != "" {
			// Plan version changes for existing API
			if err := p.planAPIVersionChanges(ctx, apiID, apiRef, versions, plan); err != nil {
				return err
			}
		}
	}

	return nil
}

// planAPIPublicationsChanges plans changes for extracted API publication resources
func (p *Planner) planAPIPublicationsChanges(
	ctx context.Context, desired []resources.APIPublicationResource, plan *Plan,
) error {
	// Group publications by parent API
	publicationsByAPI := make(map[string][]resources.APIPublicationResource)
	for _, pub := range desired {
		publicationsByAPI[pub.API] = append(publicationsByAPI[pub.API], pub)
	}

	// For each API, plan publication changes
	for apiRef, publications := range publicationsByAPI {
		// Find the API ID from existing changes or state
		apiID := ""
		for _, change := range plan.Changes {
			if change.ResourceType == "api" && change.ResourceRef == apiRef {
				if change.Action == ActionCreate {
					// API is being created, use dependency
					for _, pub := range publications {
						p.planAPIPublicationCreate(apiRef, "", pub, []string{change.ID}, plan)
					}
					continue
				}
				apiID = change.ResourceID
				break
			}
		}

		// If API not in changes, use the resolved ID from pre-resolution phase
		if apiID == "" {
			// Find the API resource by ref to get its resolved ID
			for _, api := range p.GetDesiredAPIs() {
				if api.GetRef() == apiRef {
					resolvedID := api.GetKonnectID()
					if resolvedID != "" {
						apiID = resolvedID
					}
					break
				}
			}
		}

		if apiID != "" {
			// Plan publication changes for existing API
			if err := p.planAPIPublicationChanges(ctx, apiID, apiRef, publications, plan); err != nil {
				return err
			}
		}
	}

	return nil
}

// planAPIImplementationsChanges plans changes for extracted API implementation resources
func (p *Planner) planAPIImplementationsChanges(
	ctx context.Context, desired []resources.APIImplementationResource, plan *Plan,
) error {
	// Group implementations by parent API
	implementationsByAPI := make(map[string][]resources.APIImplementationResource)
	for _, impl := range desired {
		implementationsByAPI[impl.API] = append(implementationsByAPI[impl.API], impl)
	}

	// For each API, plan implementation changes
	for apiRef, implementations := range implementationsByAPI {
		// Find the API ID from existing changes or state
		apiID := ""
		for _, change := range plan.Changes {
			if change.ResourceType == "api" && change.ResourceRef == apiRef {
				if change.Action == ActionCreate {
					// Skip CREATE - SDK doesn't support implementation creation yet
					// TODO: Enable when SDK adds support
					// for _, impl := range implementations {
					//	p.planAPIImplementationCreate(apiRef, "", impl, []string{change.ID}, plan)
					// }
					continue
				}
				apiID = change.ResourceID
				break
			}
		}

		// If API not in changes, use the resolved ID from pre-resolution phase
		if apiID == "" {
			// Find the API resource by ref to get its resolved ID
			for _, api := range p.GetDesiredAPIs() {
				if api.GetRef() == apiRef {
					resolvedID := api.GetKonnectID()
					if resolvedID != "" {
						apiID = resolvedID
					}
					break
				}
			}
		}

		if apiID != "" {
			// Plan implementation changes for existing API
			if err := p.planAPIImplementationChanges(ctx, apiID, apiRef, implementations, plan); err != nil {
				return err
			}
		}
	}

	return nil
}

// planAPIDocumentsChanges plans changes for extracted API document resources
func (p *Planner) planAPIDocumentsChanges(
	ctx context.Context, desired []resources.APIDocumentResource, plan *Plan,
) error {
	// Group documents by parent API
	documentsByAPI := make(map[string][]resources.APIDocumentResource)
	for _, doc := range desired {
		documentsByAPI[doc.API] = append(documentsByAPI[doc.API], doc)
	}

	// For each API, plan document changes
	for apiRef, documents := range documentsByAPI {
		// Find the API ID from existing changes or state
		apiID := ""
		for _, change := range plan.Changes {
			if change.ResourceType == "api" && change.ResourceRef == apiRef {
				if change.Action == ActionCreate {
					// API is being created, use dependency
					for _, doc := range documents {
						p.planAPIDocumentCreate(apiRef, "", doc, []string{change.ID}, plan)
					}
					continue
				}
				apiID = change.ResourceID
				break
			}
		}

		// If API not in changes, use the resolved ID from pre-resolution phase
		if apiID == "" {
			// Find the API resource by ref to get its resolved ID
			for _, api := range p.GetDesiredAPIs() {
				if api.GetRef() == apiRef {
					resolvedID := api.GetKonnectID()
					if resolvedID != "" {
						apiID = resolvedID
					}
					break
				}
			}
		}

		if apiID != "" {
			// Plan document changes for existing API
			if err := p.planAPIDocumentChanges(ctx, apiID, apiRef, documents, plan); err != nil {
				return err
			}
		}
	}

	return nil
}

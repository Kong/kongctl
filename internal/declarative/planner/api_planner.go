package planner

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"

	"github.com/kong/kongctl/internal/declarative/attributes"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util/normalizers"
)

// NoRequiredFields is an explicitly empty slice for operations that don't require field validation
var NoRequiredFields = []string{}

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
func (a *apiPlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	// Get namespace from planner context
	namespace := plannerCtx.Namespace

	// Debug logging
	desiredAPIs := a.GetDesiredAPIs(namespace)
	a.planner.logger.Debug("apiPlannerImpl.PlanChanges called", "desiredAPIs", len(desiredAPIs))

	// Plan API resources
	if err := a.planner.planAPIChanges(ctx, plannerCtx, desiredAPIs, plan); err != nil {
		return err
	}

	// Plan child resources that are defined separately
	if err := a.planner.planAPIVersionsChanges(ctx, plannerCtx, a.GetDesiredAPIVersions(namespace), plan); err != nil {
		return err
	}

	if err := a.planner.planAPIPublicationsChanges(
		ctx, plannerCtx, a.GetDesiredAPIPublications(namespace), plan,
	); err != nil {
		return err
	}

	if err := a.planner.planAPIImplementationsChanges(
		ctx, plannerCtx, a.GetDesiredAPIImplementations(namespace), plan,
	); err != nil {
		return err
	}

	if err := a.planner.planAPIDocumentsChanges(ctx, plannerCtx, a.GetDesiredAPIDocuments(namespace), plan); err != nil {
		return err
	}

	return nil
}

// planAPIChanges generates changes for API resources and their child resources
func (p *Planner) planAPIChanges(
	ctx context.Context, plannerCtx *Config, desired []resources.APIResource, plan *Plan,
) error {
	// Debug logging
	p.logger.Debug("planAPIChanges called", "desiredCount", len(desired))

	// Skip if no API resources to plan and not in sync mode
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		p.logger.Debug("Skipping API planning - no desired APIs")
		return nil
	}

	// Get namespace from planner context
	namespace := plannerCtx.Namespace

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
			// Extract namespace for child resources
			parentNamespace := DefaultNamespace
			if desiredAPI.Kongctl != nil && desiredAPI.Kongctl.Namespace != nil {
				parentNamespace = *desiredAPI.Kongctl.Namespace
			}
			// Plan child resources after API creation
			p.planAPIChildResourcesCreate(parentNamespace, desiredAPI, apiChangeID, plan)
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
				needsUpdate, updateFields := p.shouldUpdateAPI(current, desiredAPI)

				// Create protection change object
				protectionChange := &ProtectionChange{
					Old: isProtected,
					New: shouldProtect,
				}

				// Validate protection change
				err := p.validateProtectionWithChange("api", desiredAPI.Name, isProtected, ActionUpdate,
					protectionChange, needsUpdate)
				if err != nil {
					protectionErrors = append(protectionErrors, err)
				} else {
					p.planAPIProtectionChangeWithFields(current, desiredAPI, isProtected, shouldProtect, updateFields, plan)
				}
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
			if err := p.planAPIChildResourceChanges(ctx, plannerCtx, current, desiredAPI, plan); err != nil {
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
func extractAPIFields(resource any) map[string]any {
	fields := make(map[string]any)

	api, ok := resource.(resources.APIResource)
	if !ok {
		return fields
	}

	fields["name"] = api.Name
	if api.Description != nil {
		fields["description"] = *api.Description
	}
	if api.Version != nil {
		fields["version"] = *api.Version
	}
	if api.Slug != nil {
		fields["slug"] = *api.Slug
	}

	// Copy user-defined labels only (protection label will be added during execution)
	if len(api.Labels) > 0 {
		labelsMap := make(map[string]any)
		for k, v := range api.Labels {
			labelsMap[k] = v
		}
		fields["labels"] = labelsMap
	}

	if api.Attributes != nil {
		if normalized, ok := attributes.NormalizeAPIAttributes(api.Attributes); ok {
			fields["attributes"] = normalized
		} else {
			fields["attributes"] = api.Attributes
		}
	}

	return fields
}

// planAPICreate creates a CREATE change for an API
func (p *Planner) planAPICreate(api resources.APIResource, plan *Plan) string {
	generic := p.genericPlanner

	// Extract protection status
	var protection any
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
		FieldExtractor: func(_ any) map[string]any {
			return extractAPIFields(api)
		},
		Namespace: namespace,
		DependsOn: []string{},
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
) (bool, map[string]any) {
	updates := make(map[string]any)

	// Only compare fields present in desired configuration
	if desired.Description != nil {
		currentDesc := getString(current.Description)
		if currentDesc != *desired.Description {
			updates["description"] = *desired.Description
		}
	}

	// Check version if present in desired configuration
	if desired.Version != nil {
		currentVersion := getString(current.Version)
		if currentVersion != *desired.Version {
			updates["version"] = *desired.Version
		}
	}

	// Check if labels are defined in the desired state
	if desired.Labels != nil {
		// Compare only user labels to determine if update is needed
		if labels.CompareUserLabels(current.NormalizedLabels, desired.Labels) {
			// User labels differ, include all labels in update
			labelsMap := make(map[string]any)
			for k, v := range desired.Labels {
				labelsMap[k] = v
			}
			updates["labels"] = labelsMap
		}
	}

	// Check slug differences when configured
	if desired.Slug != nil {
		currentSlug := getString(current.Slug)
		if currentSlug != *desired.Slug {
			updates["slug"] = *desired.Slug
		}
	}

	// Check attributes when provided in desired state
	if desired.Attributes != nil {
		desiredAttrs := desired.Attributes
		if normalized, ok := attributes.NormalizeAPIAttributes(desired.Attributes); ok {
			desiredAttrs = normalized
		}
		currentAttrs := current.Attributes
		if normalized, ok := attributes.NormalizeAPIAttributes(current.Attributes); ok {
			currentAttrs = normalized
		}
		if !attributesEqual(currentAttrs, desiredAttrs) {
			updates["attributes"] = desiredAttrs
		}
	}

	return len(updates) > 0, updates
}

// planAPIUpdateWithFields creates an UPDATE change with specific fields
func (p *Planner) planAPIUpdateWithFields(
	current state.API,
	desired resources.APIResource,
	updateFields map[string]any,
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
		RequiredFields: NoRequiredFields, // No required fields for updates - we already have the resource ID
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

func attributesEqual(current, desired any) bool {
	if normalized, ok := attributes.NormalizeAPIAttributes(current); ok {
		current = normalized
	}
	if normalized, ok := attributes.NormalizeAPIAttributes(desired); ok {
		desired = normalized
	}

	if current == nil && desired == nil {
		return true
	}

	currentJSON, err := normalizers.SpecToJSON(current)
	if err != nil {
		return reflect.DeepEqual(current, desired)
	}

	desiredJSON, err := normalizers.SpecToJSON(desired)
	if err != nil {
		return reflect.DeepEqual(current, desired)
	}

	return currentJSON == desiredJSON
}

// planAPIProtectionChangeWithFields creates an UPDATE for protection status with optional field updates
func (p *Planner) planAPIProtectionChangeWithFields(
	current state.API,
	desired resources.APIResource,
	wasProtected, shouldProtect bool,
	updateFields map[string]any,
	plan *Plan,
) {
	// Extract namespace
	namespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		namespace = *desired.Kongctl.Namespace
	}

	// Use generic protection change planner
	config := ProtectionChangeConfig{
		ResourceType: "api",
		ResourceName: desired.Name,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		OldProtected: wasProtected,
		NewProtected: shouldProtect,
		Namespace:    namespace,
	}

	change := p.genericPlanner.PlanProtectionChange(context.Background(), config)

	// Always include essential fields for protection changes
	fields := make(map[string]any)

	// Include any field updates if present
	for field, newValue := range updateFields {
		fields[field] = newValue
	}

	// ALWAYS include essential identification fields for protection changes
	fields["name"] = current.Name
	fields["id"] = current.ID

	// Preserve namespace context for execution phase
	if current.Labels != nil {
		if namespace, exists := current.Labels[labels.NamespaceKey]; exists {
			fields["namespace"] = namespace
		}
	}

	// Preserve other critical labels that identify managed resources
	if current.Labels != nil {
		preservedLabels := make(map[string]string)
		for key, value := range current.Labels {
			// Preserve all KONGCTL- prefixed labels except protected (which will be updated)
			if strings.HasPrefix(key, "KONGCTL-") && key != labels.ProtectedKey {
				preservedLabels[key] = value
			}
		}
		if len(preservedLabels) > 0 {
			fields["preserved_labels"] = preservedLabels
		}
	}

	change.Fields = fields

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
	change.Fields = map[string]any{"name": api.Name}

	plan.AddChange(change)
}

// planAPIChildResourcesCreate plans creation of child resources for a new API
func (p *Planner) planAPIChildResourcesCreate(
	parentNamespace string, api resources.APIResource, apiChangeID string, plan *Plan,
) {
	// Plan version creation - API ID is not yet known
	for _, version := range p.getAPIVersionsForAPI(api) {
		if plan.HasChange("api_version", version.GetRef()) {
			continue
		}
		p.planAPIVersionCreate(parentNamespace, api.GetRef(), "", version, []string{apiChangeID}, plan)
	}

	// Plan publication creation - API ID is not yet known
	for _, publication := range p.getAPIPublicationsForAPI(api) {
		if plan.HasChange("api_publication", publication.GetRef()) {
			continue
		}
		p.planAPIPublicationCreate(parentNamespace, api.GetRef(), "", publication, []string{apiChangeID}, plan)
	}

	// Plan implementation creation - API ID is not yet known
	for _, implementation := range p.getAPIImplementationsForAPI(api) {
		if plan.HasChange("api_implementation", implementation.GetRef()) {
			continue
		}
		p.planAPIImplementationCreate(parentNamespace, api.GetRef(), "", implementation, []string{apiChangeID}, plan)
	}

	// Plan document creation - API ID is not yet known
	for _, document := range p.getAPIDocumentsForAPI(api) {
		if plan.HasChange("api_document", document.GetRef()) {
			continue
		}
		p.planAPIDocumentCreate(
			parentNamespace,
			api.GetRef(),
			"",
			document,
			[]string{apiChangeID},
			apiDocumentLookup{},
			plan,
		)
	}
}

func (p *Planner) getAPIVersionsForAPI(api resources.APIResource) []resources.APIVersionResource {
	result := make([]resources.APIVersionResource, 0)
	seen := make(map[string]struct{})
	for _, version := range api.Versions {
		result = append(result, version)
		seen[version.GetRef()] = struct{}{}
	}
	for _, version := range p.resources.APIVersions {
		if version.API != api.GetRef() {
			continue
		}
		if _, ok := seen[version.GetRef()]; ok {
			continue
		}
		result = append(result, version)
		seen[version.GetRef()] = struct{}{}
	}
	return result
}

func (p *Planner) getAPIPublicationsForAPI(api resources.APIResource) []resources.APIPublicationResource {
	result := make([]resources.APIPublicationResource, 0)
	seen := make(map[string]struct{})
	for _, pub := range api.Publications {
		result = append(result, pub)
		seen[pub.GetRef()] = struct{}{}
	}
	for _, pub := range p.resources.APIPublications {
		if pub.API != api.GetRef() {
			continue
		}
		if _, ok := seen[pub.GetRef()]; ok {
			continue
		}
		result = append(result, pub)
		seen[pub.GetRef()] = struct{}{}
	}
	return result
}

func (p *Planner) getAPIImplementationsForAPI(api resources.APIResource) []resources.APIImplementationResource {
	result := make([]resources.APIImplementationResource, 0)
	seen := make(map[string]struct{})
	for _, impl := range api.Implementations {
		result = append(result, impl)
		seen[impl.GetRef()] = struct{}{}
	}
	for _, impl := range p.resources.APIImplementations {
		if impl.API != api.GetRef() {
			continue
		}
		if _, ok := seen[impl.GetRef()]; ok {
			continue
		}
		result = append(result, impl)
		seen[impl.GetRef()] = struct{}{}
	}
	return result
}

func (p *Planner) getAPIDocumentsForAPI(api resources.APIResource) []resources.APIDocumentResource {
	result := make([]resources.APIDocumentResource, 0)
	seen := make(map[string]struct{})
	for _, doc := range api.Documents {
		result = append(result, doc)
		seen[doc.GetRef()] = struct{}{}
	}
	for _, doc := range p.resources.APIDocuments {
		if doc.API != api.GetRef() {
			continue
		}
		if _, ok := seen[doc.GetRef()]; ok {
			continue
		}
		result = append(result, doc)
		seen[doc.GetRef()] = struct{}{}
	}
	return result
}

// planAPIChildResourceChanges plans changes for child resources of an existing API
func (p *Planner) planAPIChildResourceChanges(
	ctx context.Context, plannerCtx *Config, current state.API, desired resources.APIResource, plan *Plan,
) error {
	// Extract parent namespace for child resources
	parentNamespace := DefaultNamespace
	if desired.Kongctl != nil && desired.Kongctl.Namespace != nil {
		parentNamespace = *desired.Kongctl.Namespace
	}

	// Plan version changes
	if err := p.planAPIVersionChanges(
		ctx, plannerCtx, parentNamespace, current.ID, desired.GetRef(), desired.Versions, plan,
	); err != nil {
		return fmt.Errorf("failed to plan API version changes: %w", err)
	}

	// Plan publication changes
	if err := p.planAPIPublicationChanges(
		ctx, plannerCtx, parentNamespace, current.ID, desired.GetRef(), desired.Publications, plan,
	); err != nil {
		return fmt.Errorf("failed to plan API publication changes: %w", err)
	}

	// Plan implementation changes
	if err := p.planAPIImplementationChanges(
		ctx, plannerCtx, parentNamespace, current.ID, desired.GetRef(), desired.Implementations, plan); err != nil {
		return fmt.Errorf("failed to plan API implementation changes: %w", err)
	}

	// Plan document changes
	if err := p.planAPIDocumentChanges(
		ctx, plannerCtx, parentNamespace, current.ID, desired.GetRef(), desired.Documents, plan,
	); err != nil {
		return fmt.Errorf("failed to plan API document changes: %w", err)
	}

	return nil
}

// API Version planning

func (p *Planner) planAPIVersionChanges(
	ctx context.Context, _ *Config, parentNamespace string, apiID string, apiRef string,
	desired []resources.APIVersionResource, plan *Plan,
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
		if plan.HasChange("api_version", desiredVersion.GetRef()) {
			continue
		}
		versionStr := ""
		if desiredVersion.Version != nil {
			versionStr = *desiredVersion.Version
		}

		if current, exists := currentByVersion[versionStr]; !exists {
			// CREATE new version
			p.planAPIVersionCreate(parentNamespace, apiRef, apiID, desiredVersion, []string{}, plan)
		} else {
			// CHECK FOR UPDATES - API versions now support updates
			// Fetch full version to get spec content for comparison
			fullVersion, err := p.client.FetchAPIVersion(ctx, apiID, current.ID)
			if err != nil {
				return fmt.Errorf("failed to fetch version %s: %w", versionStr, err)
			}
			if fullVersion != nil {
				current = *fullVersion
			}

			// Now compare with full content
			if p.shouldUpdateAPIVersion(current, desiredVersion) {
				p.planAPIVersionUpdate(parentNamespace, apiRef, apiID, current.ID, desiredVersion, plan)
			}
		}
	}

	// In sync mode, delete unmanaged versions
	if plan.Metadata.Mode == PlanModeSync {
		// Check if there are extracted versions for this API that will be processed later
		hasExtractedVersions := false
		for _, ver := range p.resources.GetAPIVersionsByNamespace(parentNamespace) {
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
	parentNamespace string, apiRef string, apiID string, version resources.APIVersionResource,
	dependsOn []string, plan *Plan,
) {
	fields := make(map[string]any)
	if version.Version != nil {
		fields["version"] = *version.Version
	}
	if version.Spec != nil && version.Spec.Content != nil {
		// Store spec as a map with content field for proper JSON serialization
		fields["spec"] = map[string]any{
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
		Namespace:    parentNamespace,
	}

	// Set API reference for executor - find the API name for lookup
	if apiRef != "" {
		var apiName string
		if api := p.resources.GetAPIByRef(apiRef); api != nil {
			apiName = api.Name
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
		Fields:    map[string]any{"version": versionStr},
		DependsOn: []string{},
	}

	plan.AddChange(change)
}

// API Publication planning

func (p *Planner) planAPIPublicationChanges(
	ctx context.Context, plannerCtx *Config, parentNamespace string, apiID string, apiRef string,
	desired []resources.APIPublicationResource, plan *Plan,
) error {
	// Get namespace from planner context
	namespace := plannerCtx.Namespace
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
		slog.Int("desired_portals", len(p.resources.Portals)),
	)

	// First, add desired portals to the mapping (search all namespaces)
	for _, portal := range p.resources.Portals {
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
				for _, desiredPortal := range p.resources.Portals {
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
		if plan.HasChange("api_publication", desiredPub.GetRef()) {
			continue
		}
		// Resolve portal reference to ID before comparing
		resolvedPortalID := desiredPub.PortalID
		// Parse __REF__ format if present
		lookupRef := desiredPub.PortalID
		if strings.HasPrefix(lookupRef, tags.RefPlaceholderPrefix) {
			if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok {
				lookupRef = parsedRef
			}
		}
		if id, ok := portalRefToID[lookupRef]; ok {
			resolvedPortalID = id
		}

		current, exists := currentByPortal[resolvedPortalID]

		p.logger.Debug("Checking publication existence",
			slog.String("api", apiRef),
			slog.String("portal_ref", desiredPub.PortalID),
			slog.String("resolved_portal_id", resolvedPortalID),
			slog.Bool("exists", exists),
		)

		if !exists {
			// CREATE new publication
			p.planAPIPublicationCreate(parentNamespace, apiRef, apiID, desiredPub, []string{}, plan)
		} else {
			// Check if update needed - publications use PUT which supports both create/update
			needsUpdate, updateFields := p.shouldUpdateAPIPublication(current, desiredPub)
			if needsUpdate {
				p.logger.Debug("API publication needs update",
					slog.String("api", apiRef),
					slog.String("portal", desiredPub.PortalID),
					slog.Any("fields", updateFields),
				)
				p.planAPIPublicationUpdate(parentNamespace, apiRef, apiID, current, desiredPub, updateFields, plan)
			}
		}
		// Note: Publications are identified by portal ID, not a separate ID
	}

	// In sync mode, delete unmanaged publications
	if plan.Metadata.Mode == PlanModeSync {
		// Check if there are extracted publications for this API that will be processed later
		hasExtractedPublications := false
		for _, pub := range p.resources.GetAPIPublicationsByNamespace(parentNamespace) {
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
			// Parse __REF__ format if present
			lookupRef := pub.PortalID
			if strings.HasPrefix(lookupRef, tags.RefPlaceholderPrefix) {
				if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok {
					lookupRef = parsedRef
				}
			}
			if id, ok := portalRefToID[lookupRef]; ok {
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
	parentNamespace string, apiRef string, apiID string, publication resources.APIPublicationResource,
	dependsOn []string, plan *Plan,
) {
	fields := make(map[string]any)
	fields["portal_id"] = publication.PortalID
	if publication.AuthStrategyIds != nil {
		fields["auth_strategy_ids"] = publication.AuthStrategyIds
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
		Namespace:    parentNamespace,
	}

	// Look up portal name for reference resolution using global lookup
	var portalName string
	if portal := p.resources.GetPortalByRef(publication.PortalID); portal != nil {
		portalName = portal.Name
	}

	// Set up references with lookup fields
	change.References = make(map[string]ReferenceInfo)

	// Set API reference for executor - find the API name for lookup
	if apiRef != "" {
		var apiName string
		if api := p.resources.GetAPIByRef(apiRef); api != nil {
			apiName = api.Name
		}

		change.References["api_id"] = ReferenceInfo{
			Ref: apiRef,
			ID:  apiID, // May be empty if API doesn't exist yet
			LookupFields: map[string]string{
				"name": apiName,
			},
		}
	}

	// Set portal reference
	if publication.PortalID != "" {
		change.References["portal_id"] = ReferenceInfo{
			Ref: publication.PortalID,
			LookupFields: map[string]string{
				"name": portalName,
			},
		}
	}

	// Set up auth_strategy_ids references (array)
	if len(publication.AuthStrategyIds) > 0 {
		// Look up names for each auth strategy reference
		var authStrategyNames []string
		for _, ref := range publication.AuthStrategyIds {
			// Find the auth strategy in desired state using global lookup
			var strategyName string
			if strategy := p.resources.GetAuthStrategyByRef(ref); strategy != nil {
				strategyName = p.getAuthStrategyName(*strategy)
			}
			authStrategyNames = append(authStrategyNames, strategyName)
		}

		// Set up array reference with lookup names
		change.References["auth_strategy_ids"] = ReferenceInfo{
			Refs:    publication.AuthStrategyIds,
			IsArray: true,
			LookupArrays: map[string][]string{
				"names": authStrategyNames,
			},
		}
	}

	plan.AddChange(change)
}

// getAuthStrategyName extracts the name from an auth strategy resource (handles union type)
func (p *Planner) getAuthStrategyName(strategy resources.ApplicationAuthStrategyResource) string {
	// Handle the union type - check which strategy type is populated
	if strategy.AppAuthStrategyKeyAuthRequest != nil {
		return strategy.AppAuthStrategyKeyAuthRequest.Name
	}
	if strategy.AppAuthStrategyOpenIDConnectRequest != nil {
		return strategy.AppAuthStrategyOpenIDConnectRequest.Name
	}
	// Return empty string if no known type is found
	return ""
}

func (p *Planner) planAPIPublicationDelete(apiRef string, apiID string, portalID string, portalRef string, plan *Plan) {
	// Parse __REF__ format if present in portalRef
	cleanPortalRef := portalRef
	if strings.HasPrefix(portalRef, tags.RefPlaceholderPrefix) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(portalRef); ok {
			cleanPortalRef = parsedRef
		}
	}
	// Create a composite reference that includes both API and portal for clarity
	compositeRef := fmt.Sprintf("%s-to-%s", apiRef, cleanPortalRef)

	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, "api_publication", compositeRef),
		ResourceType: "api_publication",
		ResourceRef:  compositeRef,
		ResourceID:   fmt.Sprintf("%s:%s", apiID, portalID), // Composite ID for API publication
		Parent:       &ParentInfo{Ref: apiRef, ID: apiID},
		Action:       ActionDelete,
		Fields: map[string]any{
			"api_id":    apiID,
			"portal_id": portalID,
		},
		DependsOn: []string{},
	}

	plan.AddChange(change)
}

// planAPIPublicationUpdate plans an update to an existing API publication
func (p *Planner) planAPIPublicationUpdate(
	parentNamespace string, apiRef string, apiID string,
	current state.APIPublication, desired resources.APIPublicationResource,
	updateFields map[string]any, plan *Plan,
) {
	// Update fields with resolved portal ID
	updateFields["portal_id"] = current.PortalID

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, "api_publication", desired.GetRef()),
		ResourceType: "api_publication",
		ResourceRef:  desired.GetRef(),
		ResourceID:   fmt.Sprintf("%s:%s", apiID, current.PortalID), // Composite ID
		Parent:       &ParentInfo{Ref: apiRef, ID: apiID},
		Action:       ActionUpdate,
		Fields:       updateFields,
		DependsOn:    []string{},
		Namespace:    parentNamespace,
	}

	// Look up portal name for reference resolution using global lookup
	var portalName string
	if portal := p.resources.GetPortalByRef(desired.PortalID); portal != nil {
		portalName = portal.Name
	}

	// Set up references with lookup fields
	change.References = make(map[string]ReferenceInfo)

	// Set API reference
	if apiRef != "" {
		var apiName string
		if api := p.resources.GetAPIByRef(apiRef); api != nil {
			apiName = api.Name
		}

		change.References["api_id"] = ReferenceInfo{
			Ref: apiRef,
			ID:  apiID,
			LookupFields: map[string]string{
				"name": apiName,
			},
		}
	}

	// Set portal reference
	if desired.PortalID != "" {
		change.References["portal_id"] = ReferenceInfo{
			Ref: desired.PortalID,
			ID:  current.PortalID, // Use the resolved ID
			LookupFields: map[string]string{
				"name": portalName,
			},
		}
	}

	// Handle auth strategy references if they are being updated
	if authStrategyIDs, ok := updateFields["auth_strategy_ids"].([]string); ok && len(authStrategyIDs) > 0 {
		// Extract auth strategy names for lookup
		authStrategyNames := make([]string, 0, len(authStrategyIDs))
		for _, strategyRef := range authStrategyIDs {
			// Find the auth strategy by ref to get its name using global lookup
			if strategy := p.resources.GetAuthStrategyByRef(strategyRef); strategy != nil {
				authStrategyNames = append(authStrategyNames, p.getAuthStrategyName(*strategy))
			}
		}

		// Set auth strategy array reference
		change.References["auth_strategy_ids"] = ReferenceInfo{
			Refs:    authStrategyIDs,
			IsArray: true,
			LookupArrays: map[string][]string{
				"names": authStrategyNames,
			},
		}
	}

	plan.AddChange(change)
}

// shouldUpdateAPIPublication compares current and desired API publication to determine if update is needed
func (p *Planner) shouldUpdateAPIPublication(
	current state.APIPublication,
	desired resources.APIPublicationResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

	// Compare auth strategy IDs (order-independent comparison)
	if !p.compareStringSlices(current.AuthStrategyIDs, desired.AuthStrategyIds) {
		updates["auth_strategy_ids"] = desired.AuthStrategyIds
	}

	// Compare auto approve registrations
	desiredAutoApprove := false
	if desired.AutoApproveRegistrations != nil {
		desiredAutoApprove = *desired.AutoApproveRegistrations
	}
	if current.AutoApproveRegistrations != desiredAutoApprove {
		updates["auto_approve_registrations"] = desiredAutoApprove
	}

	// Compare visibility - only update if explicitly specified and different
	if desired.Visibility != nil {
		desiredVisibility := string(*desired.Visibility)
		if current.Visibility != desiredVisibility {
			updates["visibility"] = desiredVisibility
		}
	}

	return len(updates) > 0, updates
}

// compareStringSlices compares two string slices for equality (order-independent)
func (p *Planner) compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps for efficient comparison
	aMap := make(map[string]bool)
	for _, s := range a {
		aMap[s] = true
	}

	// Check if all elements in b exist in a
	for _, s := range b {
		if !aMap[s] {
			return false
		}
	}

	return true
}

// API Implementation planning

func (p *Planner) planAPIImplementationChanges(
	ctx context.Context, _ *Config, parentNamespace string, apiID string, apiRef string,
	desired []resources.APIImplementationResource, plan *Plan,
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
		if plan.HasChange("api_implementation", desiredImpl.GetRef()) {
			continue
		}
		if desiredImpl.Service != nil {
			key := fmt.Sprintf("%s:%s", desiredImpl.Service.ID, desiredImpl.Service.ControlPlaneID)
			if _, exists := currentByService[key]; !exists {
				p.planAPIImplementationCreate(parentNamespace, apiRef, apiID, desiredImpl, []string{}, plan)
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
				p.planAPIImplementationDelete(parentNamespace, apiRef, apiID, current, plan)
			}
		}
	}

	return nil
}

func (p *Planner) planAPIImplementationCreate(
	parentNamespace string, apiRef string, apiID string,
	implementation resources.APIImplementationResource, dependsOn []string, plan *Plan,
) {
	fields := make(map[string]any)
	// APIImplementation only has Service field in the SDK
	if implementation.Service != nil {
		fields["service"] = map[string]any{
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
		Namespace:    parentNamespace,
	}

	// Set API reference for executor - find the API name for lookup
	if apiRef != "" {
		var apiName string
		if api := p.resources.GetAPIByRef(apiRef); api != nil {
			apiName = api.Name
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

func (p *Planner) planAPIImplementationDelete(
	parentNamespace string, apiRef string, apiID string,
	implementation state.APIImplementation, plan *Plan,
) {
	ref := implementation.ID
	if ref == "" && implementation.Service != nil {
		ref = fmt.Sprintf("%s:%s", implementation.Service.ID, implementation.Service.ControlPlaneID)
	}

	fields := map[string]any{
		"api_id": apiID,
	}
	if implementation.Service != nil {
		fields["service"] = map[string]any{
			"id":               implementation.Service.ID,
			"control_plane_id": implementation.Service.ControlPlaneID,
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, "api_implementation", fmt.Sprintf("%s:%s", apiRef, ref)),
		ResourceType: "api_implementation",
		ResourceRef:  fmt.Sprintf("%s:%s", apiRef, ref),
		ResourceID:   implementation.ID,
		Parent:       &ParentInfo{Ref: apiRef, ID: apiID},
		Action:       ActionDelete,
		Fields:       fields,
		Namespace:    parentNamespace,
	}

	plan.AddChange(change)
}

// API Document planning

type apiDocumentLookup struct {
	paths map[string]string
	slugs map[string]string
}

func (p *Planner) planAPIDocumentChanges(
	ctx context.Context, _ *Config, parentNamespace string, apiID string, apiRef string,
	desired []resources.APIDocumentResource, plan *Plan,
) error {
	// List current documents
	currentDocuments, err := p.client.ListAPIDocuments(ctx, apiID)
	if err != nil {
		return fmt.Errorf("failed to list current API documents: %w", err)
	}

	lookup := p.buildAPIDocumentLookup(desired)
	desiredPaths := lookup.paths
	stateIndex := newAPIDocumentStateIndex(currentDocuments)

	// Compare desired documents
	for _, desiredDoc := range desired {
		if plan.HasChange("api_document", desiredDoc.GetRef()) {
			continue
		}
		desiredPath := desiredPaths[desiredDoc.Ref]
		if desiredPath == "" && desiredDoc.Slug != nil {
			desiredPath = strings.TrimPrefix(*desiredDoc.Slug, "/")
		}
		current, exists := stateIndex.getByPath(desiredPath)

		if !exists {
			// CREATE
			p.planAPIDocumentCreate(parentNamespace, apiRef, apiID, desiredDoc, []string{}, lookup, plan)
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
				p.planAPIDocumentUpdate(parentNamespace, apiRef, apiID, current.ID, desiredDoc, lookup, plan)
			}

			stateIndex.markProcessed(desiredPath)
		}
	}

	// In sync mode, delete unmanaged documents
	if plan.Metadata.Mode == PlanModeSync {
		remaining := stateIndex.unprocessed()
		for path, current := range remaining {
			p.planAPIDocumentDelete(apiRef, apiID, current.ID, path, plan)
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

func (p *Planner) shouldUpdateAPIVersion(current state.APIVersion, desired resources.APIVersionResource) bool {
	// Check if version string changed
	if desired.Version != nil && current.Version != *desired.Version {
		return true
	}

	// Check if spec content changed
	if desired.Spec != nil && desired.Spec.Content != nil {
		// Both should already be normalized JSON, but ensure consistency
		currentSpec := strings.TrimSpace(current.Spec)
		desiredSpec := strings.TrimSpace(*desired.Spec.Content)

		// Re-normalize both sides to ensure consistent comparison
		// This handles any edge cases where normalization wasn't applied
		normalizedCurrent, err := normalizers.SpecToJSON(currentSpec)
		if err != nil {
			// Fallback to direct comparison if normalization fails
			normalizedCurrent = currentSpec
		}
		normalizedDesired, err := normalizers.SpecToJSON(desiredSpec)
		if err != nil {
			// Fallback to direct comparison if normalization fails
			normalizedDesired = desiredSpec
		}

		if normalizedCurrent != normalizedDesired {
			return true
		}
	}

	return false
}

func (p *Planner) planAPIVersionUpdate(
	parentNamespace string, apiRef string, apiID string, versionID string,
	version resources.APIVersionResource, plan *Plan,
) {
	fields := make(map[string]any)

	// Add fields that can be updated
	if version.Version != nil {
		fields["version"] = *version.Version
	}
	if version.Spec != nil && version.Spec.Content != nil {
		// Store spec as a map with content field for proper JSON serialization
		fields["spec"] = map[string]any{
			"content": *version.Spec.Content,
		}
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, "api_version", version.GetRef()),
		ResourceType: "api_version",
		ResourceRef:  version.GetRef(),
		ResourceID:   versionID,
		Parent:       &ParentInfo{Ref: apiRef, ID: apiID},
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    []string{},
		Namespace:    parentNamespace,
	}

	// Set API reference for executor
	if apiRef != "" {
		// Find the API to get its name for lookup
		var apiName string
		if api := p.resources.GetAPIByRef(apiRef); api != nil {
			apiName = api.Name
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

func (p *Planner) planAPIDocumentCreate(
	parentNamespace string, apiRef string, apiID string, document resources.APIDocumentResource,
	dependsOn []string, lookup apiDocumentLookup, plan *Plan,
) {
	fields := make(map[string]any)
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
		Namespace:    parentNamespace,
	}

	// Set API reference for executor
	if apiRef != "" {
		// Find the API to get its name for lookup
		var apiName string
		if api := p.resources.GetAPIByRef(apiRef); api != nil {
			apiName = api.Name
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

	// Handle parent document references
	if document.ParentDocumentRef != "" {
		lookupFields := make(map[string]string)
		if lookup.paths != nil {
			if parentPath := lookup.paths[document.ParentDocumentRef]; parentPath != "" {
				lookupFields["slug_path"] = parentPath
			}
		}
		if lookup.slugs != nil {
			if parentSlug := lookup.slugs[document.ParentDocumentRef]; parentSlug != "" {
				lookupFields["slug"] = parentSlug
			}
		}

		if change.References == nil {
			change.References = make(map[string]ReferenceInfo)
		}
		change.References["parent_document_id"] = ReferenceInfo{
			Ref:          document.ParentDocumentRef,
			LookupFields: lookupFields,
		}

		// Ensure the parent document change executes first if present in the plan
		for _, depChange := range plan.Changes {
			if depChange.ResourceType == "api_document" && depChange.ResourceRef == document.ParentDocumentRef {
				change.DependsOn = append(change.DependsOn, depChange.ID)
				break
			}
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planAPIDocumentUpdate(
	parentNamespace string, apiRef string, apiID string, documentID string,
	document resources.APIDocumentResource, lookup apiDocumentLookup, plan *Plan,
) {
	fields := make(map[string]any)
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
		Namespace:    parentNamespace,
	}

	// Set API reference for executor
	if apiRef != "" {
		// Find the API to get its name for lookup
		var apiName string
		if api := p.resources.GetAPIByRef(apiRef); api != nil {
			apiName = api.Name
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

	if document.ParentDocumentRef != "" {
		lookupFields := make(map[string]string)
		if lookup.paths != nil {
			if parentPath := lookup.paths[document.ParentDocumentRef]; parentPath != "" {
				lookupFields["slug_path"] = parentPath
			}
		}
		if lookup.slugs != nil {
			if parentSlug := lookup.slugs[document.ParentDocumentRef]; parentSlug != "" {
				lookupFields["slug"] = parentSlug
			}
		}

		if change.References == nil {
			change.References = make(map[string]ReferenceInfo)
		}
		change.References["parent_document_id"] = ReferenceInfo{
			Ref:          document.ParentDocumentRef,
			LookupFields: lookupFields,
		}

		// If parent document change exists, ensure it runs before this update
		for _, depChange := range plan.Changes {
			if depChange.ResourceType == "api_document" && depChange.ResourceRef == document.ParentDocumentRef {
				change.DependsOn = append(change.DependsOn, depChange.ID)
				break
			}
		}
	}

	plan.AddChange(change)
}

func (p *Planner) planAPIDocumentDelete(apiRef string, apiID string, documentID string, path string, plan *Plan) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, "api_document", documentID),
		ResourceType: "api_document",
		ResourceRef:  "[unknown]",
		ResourceID:   documentID,
		ResourceMonikers: map[string]string{
			"slug":       path,
			"parent_api": apiRef,
		},
		Parent:    &ParentInfo{Ref: apiRef, ID: apiID},
		Action:    ActionDelete,
		Fields:    map[string]any{"slug": path},
		DependsOn: []string{},
	}

	// Set API reference for executor
	if apiRef != "" {
		// Find the API to get its name for lookup
		var apiName string
		if api := p.resources.GetAPIByRef(apiRef); api != nil {
			apiName = api.Name
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
	ctx context.Context, plannerCtx *Config, desired []resources.APIVersionResource, plan *Plan,
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
					// Get parent namespace from the API change
					parentNamespace := change.Namespace
					if parentNamespace == "" {
						parentNamespace = DefaultNamespace
					}
					for _, v := range versions {
						if plan.HasChange("api_version", v.GetRef()) {
							continue
						}
						p.planAPIVersionCreate(parentNamespace, apiRef, "", v, []string{change.ID}, plan)
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
			// Find parent namespace from the API resource
			parentNamespace := DefaultNamespace
			for _, api := range p.GetDesiredAPIs() {
				if api.GetRef() == apiRef {
					if api.Kongctl != nil && api.Kongctl.Namespace != nil {
						parentNamespace = *api.Kongctl.Namespace
					}
					break
				}
			}
			if err := p.planAPIVersionChanges(ctx, plannerCtx, parentNamespace, apiID, apiRef, versions, plan); err != nil {
				return err
			}
		}
	}

	return nil
}

// planAPIPublicationsChanges plans changes for extracted API publication resources
func (p *Planner) planAPIPublicationsChanges(
	ctx context.Context, plannerCtx *Config, desired []resources.APIPublicationResource, plan *Plan,
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
					// Get parent namespace from the API change
					parentNamespace := change.Namespace
					if parentNamespace == "" {
						parentNamespace = DefaultNamespace
					}
					for _, pub := range publications {
						if plan.HasChange("api_publication", pub.GetRef()) {
							continue
						}
						p.planAPIPublicationCreate(parentNamespace, apiRef, "", pub, []string{change.ID}, plan)
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
			// Find parent namespace from the API resource
			parentNamespace := DefaultNamespace
			for _, api := range p.GetDesiredAPIs() {
				if api.GetRef() == apiRef {
					if api.Kongctl != nil && api.Kongctl.Namespace != nil {
						parentNamespace = *api.Kongctl.Namespace
					}
					break
				}
			}
			if err := p.planAPIPublicationChanges(
				ctx, plannerCtx, parentNamespace, apiID, apiRef, publications, plan,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// planAPIImplementationsChanges plans changes for extracted API implementation resources
func (p *Planner) planAPIImplementationsChanges(
	ctx context.Context, plannerCtx *Config, desired []resources.APIImplementationResource, plan *Plan,
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
					parentNamespace := change.Namespace
					if parentNamespace == "" {
						parentNamespace = DefaultNamespace
					}
					for _, impl := range implementations {
						if plan.HasChange("api_implementation", impl.GetRef()) {
							continue
						}
						p.planAPIImplementationCreate(parentNamespace, apiRef, "", impl, []string{change.ID}, plan)
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
			// Plan implementation changes for existing API
			// Find parent namespace from the API resource
			parentNamespace := DefaultNamespace
			for _, api := range p.GetDesiredAPIs() {
				if api.GetRef() == apiRef {
					if api.Kongctl != nil && api.Kongctl.Namespace != nil {
						parentNamespace = *api.Kongctl.Namespace
					}
					break
				}
			}
			if err := p.planAPIImplementationChanges(
				ctx, plannerCtx, parentNamespace, apiID, apiRef, implementations, plan,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// planAPIDocumentsChanges plans changes for extracted API document resources
func (p *Planner) planAPIDocumentsChanges(
	ctx context.Context, plannerCtx *Config, desired []resources.APIDocumentResource, plan *Plan,
) error {
	// Group documents by parent API
	documentsByAPI := make(map[string][]resources.APIDocumentResource)
	for _, doc := range desired {
		documentsByAPI[doc.API] = append(documentsByAPI[doc.API], doc)
	}

	// For each API, plan document changes
	for apiRef, documents := range documentsByAPI {
		lookup := p.buildAPIDocumentLookup(documents)
		// Find the API ID from existing changes or state
		apiID := ""
		for _, change := range plan.Changes {
			if change.ResourceType == "api" && change.ResourceRef == apiRef {
				if change.Action == ActionCreate {
					// API is being created, use dependency
					// Get parent namespace from the API change
					parentNamespace := change.Namespace
					if parentNamespace == "" {
						parentNamespace = DefaultNamespace
					}
					for _, doc := range documents {
						if plan.HasChange("api_document", doc.GetRef()) {
							continue
						}
						p.planAPIDocumentCreate(parentNamespace, apiRef, "", doc, []string{change.ID}, lookup, plan)
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
			// Find parent namespace from the API resource
			parentNamespace := DefaultNamespace
			for _, api := range p.GetDesiredAPIs() {
				if api.GetRef() == apiRef {
					if api.Kongctl != nil && api.Kongctl.Namespace != nil {
						parentNamespace = *api.Kongctl.Namespace
					}
					break
				}
			}
			if err := p.planAPIDocumentChanges(ctx, plannerCtx, parentNamespace, apiID, apiRef, documents, plan); err != nil {
				return err
			}
		}
	}

	return nil
}

// buildAPIDocumentLookup constructs helper metadata for desired API documents using their parent references.
func (p *Planner) buildAPIDocumentLookup(docs []resources.APIDocumentResource) apiDocumentLookup {
	docByRef := make(map[string]resources.APIDocumentResource)
	for _, doc := range docs {
		docByRef[doc.Ref] = doc
	}

	paths := make(map[string]string)
	slugs := make(map[string]string)
	visited := make(map[string]bool)

	var resolve func(ref string) string
	resolve = func(ref string) string {
		if path, ok := paths[ref]; ok {
			return path
		}
		if visited[ref] {
			return ""
		}
		visited[ref] = true

		doc, ok := docByRef[ref]
		if !ok {
			visited[ref] = false
			return ""
		}

		slug := ""
		if doc.Slug != nil {
			slug = strings.Trim(strings.TrimPrefix(*doc.Slug, "/"), "/")
		}
		slugs[ref] = slug

		parentRef := doc.ParentDocumentRef
		if parentRef == "" {
			paths[ref] = slug
			visited[ref] = false
			return slug
		}

		parentPath := resolve(parentRef)
		visited[ref] = false

		if parentPath == "" {
			paths[ref] = slug
			return slug
		}

		if slug == "" {
			paths[ref] = parentPath
			return parentPath
		}

		combined := parentPath
		if combined != "" {
			combined = combined + "/" + slug
		} else {
			combined = slug
		}

		paths[ref] = combined
		return combined
	}

	for ref := range docByRef {
		resolve(ref)
	}

	return apiDocumentLookup{
		paths: paths,
		slugs: slugs,
	}
}

type apiDocumentStateIndex struct {
	byPath   map[string]state.APIDocument
	idToPath map[string]string
}

func newAPIDocumentStateIndex(docs []state.APIDocument) *apiDocumentStateIndex {
	docByID := make(map[string]state.APIDocument)
	for _, doc := range docs {
		docByID[doc.ID] = doc
	}

	idToPath := make(map[string]string)
	visited := make(map[string]bool)

	var resolve func(id string) string
	resolve = func(id string) string {
		if path, ok := idToPath[id]; ok {
			return path
		}
		if visited[id] {
			return ""
		}
		visited[id] = true

		doc, ok := docByID[id]
		if !ok {
			visited[id] = false
			return ""
		}

		slug := strings.Trim(strings.TrimPrefix(doc.Slug, "/"), "/")
		if doc.ParentDocumentID == "" {
			idToPath[id] = slug
			visited[id] = false
			return slug
		}

		parentPath := resolve(doc.ParentDocumentID)
		visited[id] = false

		if parentPath == "" {
			idToPath[id] = slug
			return slug
		}

		if slug == "" {
			idToPath[id] = parentPath
			return parentPath
		}

		combined := parentPath
		if combined != "" {
			combined = combined + "/" + slug
		} else {
			combined = slug
		}

		idToPath[id] = combined
		return combined
	}

	byPath := make(map[string]state.APIDocument)
	for id, doc := range docByID {
		path := resolve(id)
		byPath[path] = doc
	}

	return &apiDocumentStateIndex{
		byPath:   byPath,
		idToPath: idToPath,
	}
}

func (i *apiDocumentStateIndex) getByPath(path string) (state.APIDocument, bool) {
	if path == "" {
		return state.APIDocument{}, false
	}
	doc, ok := i.byPath[path]
	return doc, ok
}

func (i *apiDocumentStateIndex) markProcessed(path string) {
	if path == "" {
		return
	}
	delete(i.byPath, path)
}

func (i *apiDocumentStateIndex) unprocessed() map[string]state.APIDocument {
	remaining := make(map[string]state.APIDocument, len(i.byPath))
	for path, doc := range i.byPath {
		remaining[path] = doc
	}
	return remaining
}

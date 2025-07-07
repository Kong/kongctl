package planner

import (
	"context"
	"fmt"
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
	// Plan API resources
	if err := a.planner.planAPIChanges(ctx, a.GetDesiredAPIs(), plan); err != nil {
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
	// Skip if no API resources to plan
	if len(desired) == 0 {
		return nil
	}

	// Fetch current managed APIs
	currentAPIs, err := p.client.ListManagedAPIs(ctx)
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
			if desiredAPI.Kongctl != nil && desiredAPI.Kongctl.Protected {
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

// planAPICreate creates a CREATE change for an API
func (p *Planner) planAPICreate(api resources.APIResource, plan *Plan) string {
	fields := make(map[string]interface{})
	fields["name"] = api.Name
	if api.Description != nil {
		fields["description"] = *api.Description
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionCreate, api.GetRef()),
		ResourceType: "api",
		ResourceRef:  api.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	// Always set protection status explicitly
	if api.Kongctl != nil && api.Kongctl.Protected {
		change.Protection = true
	} else {
		change.Protection = false
	}

	// Copy user-defined labels only (protection label will be added during execution)
	if len(api.Labels) > 0 {
		labelsMap := make(map[string]interface{})
		for k, v := range api.Labels {
			labelsMap[k] = v
		}
		fields["labels"] = labelsMap
	}

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
	fields := make(map[string]interface{})

	// Store the fields that need updating
	for field, newValue := range updateFields {
		fields[field] = newValue
	}
	
	// Pass current labels so executor can properly handle removals
	if _, hasLabels := updateFields["labels"]; hasLabels {
		fields[FieldCurrentLabels] = current.NormalizedLabels
	}

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "api",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    []string{},
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
	fields := make(map[string]interface{})

	// Include any field updates if unprotecting
	if wasProtected && !shouldProtect && len(updateFields) > 0 {
		for field, newValue := range updateFields {
			fields[field] = newValue
		}
	}

	// Always include name for identification
	fields["name"] = current.Name

	// Don't add protection label here - it will be added during execution
	// based on the Protection field

	change := PlannedChange{
		ID:           p.nextChangeID(ActionUpdate, desired.GetRef()),
		ResourceType: "api",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		Protection: ProtectionChange{
			Old: wasProtected,
			New: shouldProtect,
		},
		DependsOn: []string{},
	}

	plan.AddChange(change)
}

// planAPIDelete creates a DELETE change for an API
func (p *Planner) planAPIDelete(api state.API, plan *Plan) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, api.Name),
		ResourceType: "api",
		ResourceRef:  api.Name,
		ResourceID:   api.ID,
		Action:       ActionDelete,
		Fields:       map[string]interface{}{"name": api.Name},
		DependsOn:    []string{},
	}

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

	// Note: We don't delete versions in sync mode as they may be in use

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
		ID:           p.nextChangeID(ActionCreate, version.GetRef()),
		ResourceType: "api_version",
		ResourceRef:  version.GetRef(),
		Parent:       parentInfo,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependsOn,
	}

	plan.AddChange(change)
}

// API Publication planning

func (p *Planner) planAPIPublicationChanges(
	ctx context.Context, apiID string, apiRef string, desired []resources.APIPublicationResource, plan *Plan,
) error {
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

	// Compare desired publications
	for _, desiredPub := range desired {
		_, exists := currentByPortal[desiredPub.PortalID]

		if !exists {
			// CREATE - publications don't support update
			p.planAPIPublicationCreate(apiRef, apiID, desiredPub, []string{}, plan)
		}
		// Note: Publications are identified by portal ID, not a separate ID
	}

	// In sync mode, delete unmanaged publications
	if plan.Metadata.Mode == PlanModeSync {
		desiredPortals := make(map[string]bool)
		for _, pub := range desired {
			desiredPortals[pub.PortalID] = true
		}

		for portalID := range currentByPortal {
			if !desiredPortals[portalID] {
				p.planAPIPublicationDelete(apiRef, portalID, plan)
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
				p.nextChangeID(ActionCreate, publication.GetRef()),
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
		ID:           p.nextChangeID(ActionCreate, publication.GetRef()),
		ResourceType: "api_publication",
		ResourceRef:  publication.GetRef(),
		Parent:       parentInfo,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependsOn,
	}

	plan.AddChange(change)
}

func (p *Planner) planAPIPublicationDelete(apiRef string, portalID string, plan *Plan) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, portalID),
		ResourceType: "api_publication",
		ResourceRef:  portalID,
		ResourceID:   portalID, // For publications, we use portal ID for deletion
		Parent:       &ParentInfo{Ref: apiRef},
		Action:       ActionDelete,
		Fields:       map[string]interface{}{},
		DependsOn:    []string{},
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
		ID:           p.nextChangeID(ActionCreate, implementation.GetRef()),
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
	currentBySlug := make(map[string]state.APIDocument)
	for _, d := range currentDocuments {
		currentBySlug[d.Slug] = d
	}

	// Compare desired documents
	for _, desiredDoc := range desired {
		slug := ""
		if desiredDoc.Slug != nil {
			slug = *desiredDoc.Slug
		}

		current, exists := currentBySlug[slug]

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
				p.planAPIDocumentUpdate(apiRef, current.ID, desiredDoc, plan)
			}
		}
	}

	// In sync mode, delete unmanaged documents
	if plan.Metadata.Mode == PlanModeSync {
		desiredSlugs := make(map[string]bool)
		for _, doc := range desired {
			if doc.Slug != nil {
				desiredSlugs[*doc.Slug] = true
			}
		}

		for slug, current := range currentBySlug {
			if !desiredSlugs[slug] {
				p.planAPIDocumentDelete(apiRef, current.ID, plan)
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
		ID:           p.nextChangeID(ActionCreate, document.GetRef()),
		ResourceType: "api_document",
		ResourceRef:  document.GetRef(),
		Parent:       parentInfo,
		Action:       ActionCreate,
		Fields:       fields,
		DependsOn:    dependsOn,
	}

	plan.AddChange(change)
}

func (p *Planner) planAPIDocumentUpdate(
	apiRef string, documentID string, document resources.APIDocumentResource, plan *Plan,
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
		ID:           p.nextChangeID(ActionUpdate, document.GetRef()),
		ResourceType: "api_document",
		ResourceRef:  document.GetRef(),
		ResourceID:   documentID,
		Parent:       &ParentInfo{Ref: apiRef},
		Action:       ActionUpdate,
		Fields:       fields,
		DependsOn:    []string{},
	}

	plan.AddChange(change)
}

func (p *Planner) planAPIDocumentDelete(apiRef string, documentID string, plan *Plan) {
	change := PlannedChange{
		ID:           p.nextChangeID(ActionDelete, documentID),
		ResourceType: "api_document",
		ResourceRef:  documentID,
		ResourceID:   documentID,
		Parent:       &ParentInfo{Ref: apiRef},
		Action:       ActionDelete,
		Fields:       map[string]interface{}{},
		DependsOn:    []string{},
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

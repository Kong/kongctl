package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/deck"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/util/normalizers"
)

// Executor handles the execution of declarative configuration plans
type Executor struct {
	client   *state.Client
	reporter ProgressReporter
	dryRun   bool
	// Track created resources during execution
	createdResources map[string]string // changeID -> resourceID
	// Track resource refs to IDs for reference resolution
	refToID map[string]map[string]string // resourceType -> ref -> resourceID
	// Unified state cache
	stateCache *state.Cache

	// Resource executors
	portalExecutor       *BaseExecutor[kkComps.CreatePortal, kkComps.UpdatePortal]
	controlPlaneExecutor *BaseExecutor[kkComps.CreateControlPlaneRequest, kkComps.UpdateControlPlaneRequest]
	apiExecutor          *BaseExecutor[kkComps.CreateAPIRequest, kkComps.UpdateAPIRequest]
	authStrategyExecutor *BaseExecutor[
		kkComps.CreateAppAuthStrategyRequest,
		kkComps.UpdateAppAuthStrategyRequest,
	]
	catalogServiceExecutor           *BaseExecutor[kkComps.CreateCatalogService, kkComps.UpdateCatalogService]
	eventGatewayControlPlaneExecutor *BaseExecutor[kkComps.CreateGatewayRequest, kkComps.UpdateGatewayRequest]
	organizationTeamExecutor         *BaseExecutor[kkComps.CreateTeam, kkComps.UpdateTeam]

	// Event Gateway child resource executors
	eventGatewayBackendClusterExecutor *BaseExecutor[
		kkComps.CreateBackendClusterRequest, kkComps.UpdateBackendClusterRequest]
	eventGatewayVirtualClusterExecutor *BaseExecutor[
		kkComps.CreateVirtualClusterRequest, kkComps.UpdateVirtualClusterRequest]

	// Portal child resource executors
	portalCustomizationExecutor *BaseSingletonExecutor[kkComps.PortalCustomization]
	portalAuthSettingsExecutor  *BaseSingletonExecutor[kkComps.PortalAuthenticationSettingsUpdateRequest]
	portalAssetLogoExecutor     *BaseSingletonExecutor[kkComps.ReplacePortalImageAsset]
	portalAssetFaviconExecutor  *BaseSingletonExecutor[kkComps.ReplacePortalImageAsset]
	portalDomainExecutor        *BaseExecutor[kkComps.CreatePortalCustomDomainRequest,
		kkComps.UpdatePortalCustomDomainRequest]
	portalPageExecutor          *BaseExecutor[kkComps.CreatePortalPageRequest, kkComps.UpdatePortalPageRequest]
	portalSnippetExecutor       *BaseExecutor[kkComps.CreatePortalSnippetRequest, kkComps.UpdatePortalSnippetRequest]
	portalTeamExecutor          *BaseExecutor[kkComps.PortalCreateTeamRequest, kkComps.PortalUpdateTeamRequest]
	portalTeamRoleExecutor      *BaseExecutor[kkComps.PortalAssignRoleRequest, kkComps.PortalAssignRoleRequest]
	portalEmailConfigExecutor   *BaseExecutor[kkComps.PostPortalEmailConfig, kkComps.PatchPortalEmailConfig]
	portalEmailTemplateExecutor *BaseExecutor[kkOps.UpdatePortalCustomEmailTemplateRequest,
		kkOps.UpdatePortalCustomEmailTemplateRequest]

	// API child resource executors
	apiVersionExecutor     *BaseExecutor[kkComps.CreateAPIVersionRequest, kkComps.APIVersion]
	apiPublicationExecutor *BaseCreateDeleteExecutor[kkComps.APIPublication]
	apiDocumentExecutor    *BaseExecutor[kkComps.CreateAPIDocumentRequest, kkComps.APIDocument]
	// API implementation is not yet supported by SDK but we include adapter for completeness
	apiImplementationExecutor *BaseCreateDeleteExecutor[kkComps.APIImplementation]

	deckRunner     deck.Runner
	konnectToken   string
	konnectBaseURL string
	executionMode  planner.PlanMode
	planBaseDir    string
}

// Options configures executor behavior.
type Options struct {
	DeckRunner     deck.Runner
	KonnectToken   string
	KonnectBaseURL string
	Mode           planner.PlanMode
	PlanBaseDir    string
}

// New creates a new Executor instance with default options.
func New(client *state.Client, reporter ProgressReporter, dryRun bool) *Executor {
	return NewWithOptions(client, reporter, dryRun, Options{})
}

// NewWithOptions creates a new Executor instance.
func NewWithOptions(client *state.Client, reporter ProgressReporter, dryRun bool, opts Options) *Executor {
	deckRunner := opts.DeckRunner
	if deckRunner == nil {
		deckRunner = deck.NewRunner()
	}
	e := &Executor{
		client:           client,
		reporter:         reporter,
		dryRun:           dryRun,
		createdResources: make(map[string]string),
		refToID:          make(map[string]map[string]string),
		stateCache:       state.NewCache(),
		deckRunner:       deckRunner,
		konnectToken:     opts.KonnectToken,
		konnectBaseURL:   opts.KonnectBaseURL,
		executionMode:    opts.Mode,
		planBaseDir:      strings.TrimSpace(opts.PlanBaseDir),
	}

	// Initialize resource executors
	e.portalExecutor = NewBaseExecutor[kkComps.CreatePortal, kkComps.UpdatePortal](
		NewPortalAdapter(client),
		client,
		dryRun,
	)
	e.controlPlaneExecutor = NewBaseExecutor[kkComps.CreateControlPlaneRequest, kkComps.UpdateControlPlaneRequest](
		NewControlPlaneAdapter(client),
		client,
		dryRun,
	)
	e.apiExecutor = NewBaseExecutor[kkComps.CreateAPIRequest, kkComps.UpdateAPIRequest](
		NewAPIAdapter(client),
		client,
		dryRun,
	)
	e.authStrategyExecutor = NewBaseExecutor[kkComps.CreateAppAuthStrategyRequest, kkComps.UpdateAppAuthStrategyRequest](
		NewAuthStrategyAdapter(client),
		client,
		dryRun,
	)
	e.catalogServiceExecutor = NewBaseExecutor[kkComps.CreateCatalogService, kkComps.UpdateCatalogService](
		NewCatalogServiceAdapter(client),
		client,
		dryRun,
	)
	e.eventGatewayControlPlaneExecutor = NewBaseExecutor[kkComps.CreateGatewayRequest, kkComps.UpdateGatewayRequest](
		NewEventGatewayControlPlaneControlPlaneAdapter(client),
		client,
		dryRun,
	)
	e.organizationTeamExecutor = NewBaseExecutor[kkComps.CreateTeam, kkComps.UpdateTeam](
		NewOrganizationTeamAdapter(client),
		client,
		dryRun,
	)

	// Initialize event gateway child resource executors
	e.eventGatewayBackendClusterExecutor = NewBaseExecutor[
		kkComps.CreateBackendClusterRequest, kkComps.UpdateBackendClusterRequest](
		NewEventGatewayBackendClusterAdapter(client),
		client,
		dryRun,
	)

	e.eventGatewayVirtualClusterExecutor = NewBaseExecutor[
		kkComps.CreateVirtualClusterRequest, kkComps.UpdateVirtualClusterRequest](
		NewEventGatewayVirtualClusterAdapter(client),
		client,
		dryRun,
	)

	// Initialize portal child resource executors
	e.portalCustomizationExecutor = NewBaseSingletonExecutor[kkComps.PortalCustomization](
		NewPortalCustomizationAdapter(client),
		dryRun,
	)
	e.portalAuthSettingsExecutor = NewBaseSingletonExecutor[kkComps.PortalAuthenticationSettingsUpdateRequest](
		NewPortalAuthSettingsAdapter(client),
		dryRun,
	)
	e.portalAssetLogoExecutor = NewBaseSingletonExecutor[kkComps.ReplacePortalImageAsset](
		NewPortalAssetLogoAdapter(client),
		dryRun,
	)
	e.portalAssetFaviconExecutor = NewBaseSingletonExecutor[kkComps.ReplacePortalImageAsset](
		NewPortalAssetFaviconAdapter(client),
		dryRun,
	)
	e.portalDomainExecutor = NewBaseExecutor[kkComps.CreatePortalCustomDomainRequest,
		kkComps.UpdatePortalCustomDomainRequest](
		NewPortalDomainAdapter(client),
		client,
		dryRun,
	)
	e.portalPageExecutor = NewBaseExecutor[kkComps.CreatePortalPageRequest, kkComps.UpdatePortalPageRequest](
		NewPortalPageAdapter(client),
		client,
		dryRun,
	)
	e.portalSnippetExecutor = NewBaseExecutor[kkComps.CreatePortalSnippetRequest, kkComps.UpdatePortalSnippetRequest](
		NewPortalSnippetAdapter(client),
		client,
		dryRun,
	)
	e.portalTeamExecutor = NewBaseExecutor[kkComps.PortalCreateTeamRequest, kkComps.PortalUpdateTeamRequest](
		NewPortalTeamAdapter(client),
		client,
		dryRun,
	)
	e.portalTeamRoleExecutor = NewBaseExecutor[kkComps.PortalAssignRoleRequest, kkComps.PortalAssignRoleRequest](
		NewPortalTeamRoleAdapter(client),
		client,
		dryRun,
	)
	e.portalEmailConfigExecutor = NewBaseExecutor[kkComps.PostPortalEmailConfig, kkComps.PatchPortalEmailConfig](
		NewPortalEmailConfigAdapter(client),
		client,
		dryRun,
	)
	e.portalEmailTemplateExecutor = NewBaseExecutor[kkOps.UpdatePortalCustomEmailTemplateRequest,
		kkOps.UpdatePortalCustomEmailTemplateRequest](
		NewPortalEmailTemplateAdapter(client),
		client,
		dryRun,
	)

	// Initialize API child resource executors
	e.apiVersionExecutor = NewBaseExecutor[kkComps.CreateAPIVersionRequest, kkComps.APIVersion](
		NewAPIVersionAdapter(client),
		client,
		dryRun,
	)
	e.apiPublicationExecutor = NewBaseCreateDeleteExecutor[kkComps.APIPublication](
		NewAPIPublicationAdapter(client),
		dryRun,
	)
	e.apiDocumentExecutor = NewBaseExecutor[kkComps.CreateAPIDocumentRequest, kkComps.APIDocument](
		NewAPIDocumentAdapter(client),
		client,
		dryRun,
	)

	e.apiImplementationExecutor = NewBaseCreateDeleteExecutor[kkComps.APIImplementation](
		NewAPIImplementationAdapter(client),
		dryRun,
	)

	return e
}

// Execute runs the plan and returns the execution result
func (e *Executor) Execute(ctx context.Context, plan *planner.Plan) *ExecutionResult {
	result := &ExecutionResult{
		DryRun: e.dryRun,
	}

	// Notify reporter of execution start
	if e.reporter != nil {
		e.reporter.StartExecution(plan)
	}

	// Execute changes in order
	for i, changeID := range plan.ExecutionOrder {
		// Find the change by ID
		var change *planner.PlannedChange
		for j := range plan.Changes {
			if plan.Changes[j].ID == changeID {
				change = &plan.Changes[j]
				break
			}
		}

		if change == nil {
			// This shouldn't happen, but handle gracefully
			err := fmt.Errorf("change with ID %s not found in plan", changeID)
			result.Errors = append(result.Errors, ExecutionError{
				ChangeID: changeID,
				Error:    err.Error(),
			})
			result.FailureCount++
			continue
		}

		// Execute the change, the error will be captured in result
		_ = e.executeChange(ctx, result, change, plan, i)
	}

	// Notify reporter of execution completion
	if e.reporter != nil {
		e.reporter.FinishExecution(result)
	}

	return result
}

// executeChange executes a single change from the plan
func (e *Executor) executeChange(ctx context.Context, result *ExecutionResult, change *planner.PlannedChange,
	plan *planner.Plan, changeIndex int,
) error {
	// Notify reporter of change start
	if e.reporter != nil {
		e.reporter.StartChange(*change)
	}

	// Extract resource name from fields
	resourceName := getResourceName(change.Fields)

	// Pre-execution validation (always performed, even in dry-run)
	if err := e.validateChangePreExecution(ctx, *change); err != nil {
		// Record error
		execError := ExecutionError{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Error:        err.Error(),
		}
		result.Errors = append(result.Errors, execError)
		result.FailureCount++

		// In dry-run, also record validation result
		if e.dryRun {
			result.ValidationResults = append(result.ValidationResults, ValidationResult{
				ChangeID:     change.ID,
				ResourceType: change.ResourceType,
				ResourceName: resourceName,
				ResourceRef:  change.ResourceRef,
				Action:       string(change.Action),
				Status:       "would_fail",
				Validation:   "failed",
				Message:      err.Error(),
			})
		}

		// Notify reporter
		if e.reporter != nil {
			e.reporter.CompleteChange(*change, err)
		}

		return err
	}

	// If dry-run, skip actual execution
	if e.dryRun {
		result.SkippedCount++
		result.ValidationResults = append(result.ValidationResults, ValidationResult{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Status:       "would_succeed",
			Validation:   "passed",
		})

		if e.reporter != nil {
			e.reporter.SkipChange(*change, "dry-run mode")
		}

		return nil
	}

	// Execute the actual change
	var err error
	var resourceID string

	switch change.Action {
	case planner.ActionCreate:
		if change.ResourceType == planner.ResourceTypeDeck {
			err = e.executeDeckStep(ctx, change, plan)
		} else {
			resourceID, err = e.createResource(ctx, change)
		}
	case planner.ActionExternalTool:
		if change.ResourceType != planner.ResourceTypeDeck {
			err = fmt.Errorf("external tool action is only supported for %s resources", planner.ResourceTypeDeck)
		} else {
			err = e.executeDeckStep(ctx, change, plan)
		}
	case planner.ActionUpdate:
		resourceID, err = e.updateResource(ctx, change)
	case planner.ActionDelete:
		err = e.deleteResource(ctx, change)
		resourceID = change.ResourceID
	default:
		err = fmt.Errorf("unknown action: %s", change.Action)
	}

	// Record result
	if err != nil {
		execError := ExecutionError{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			Error:        err.Error(),
		}
		result.Errors = append(result.Errors, execError)
		result.FailureCount++
	} else {
		result.SuccessCount++
		result.ChangesApplied = append(result.ChangesApplied, AppliedChange{
			ChangeID:     change.ID,
			ResourceType: change.ResourceType,
			ResourceName: resourceName,
			ResourceRef:  change.ResourceRef,
			Action:       string(change.Action),
			ResourceID:   resourceID,
		})

		// Track created resources for dependencies
		if change.Action == planner.ActionCreate && resourceID != "" {
			e.createdResources[change.ID] = resourceID

			// Also track by resource type and ref for reference resolution
			if e.refToID[change.ResourceType] == nil {
				e.refToID[change.ResourceType] = make(map[string]string)
			}
			e.refToID[change.ResourceType][change.ResourceRef] = resourceID

			// Propagate the created resource ID to any pending changes that reference it
			if changeIndex+1 < len(plan.ExecutionOrder) {
				// Update remaining changes directly in plan.Changes
				for i := changeIndex + 1; i < len(plan.ExecutionOrder); i++ {
					changeID := plan.ExecutionOrder[i]
					for j := range plan.Changes {
						if plan.Changes[j].ID == changeID {
							// Check all references in this change
							for refKey, refInfo := range plan.Changes[j].References {
								// Match based on resource type from the reference key
								refResourceType := strings.TrimSuffix(refKey, "_id")

								// Extract the actual ref from __REF__ format if present
								actualRef := refInfo.Ref
								if strings.HasPrefix(refInfo.Ref, tags.RefPlaceholderPrefix) {
									parsedRef, _, ok := tags.ParseRefPlaceholder(refInfo.Ref)
									if ok {
										actualRef = parsedRef
									}
								}

								if refResourceType == change.ResourceType && actualRef == change.ResourceRef {
									// Update the reference with the created resource ID
									refInfo.ID = resourceID
									plan.Changes[j].References[refKey] = refInfo
									slog.Debug("Propagated created resource ID to dependent change",
										"change_id", plan.Changes[j].ID,
										"ref_key", refKey,
										"resource_type", change.ResourceType,
										"resource_ref", change.ResourceRef,
										"resource_id", resourceID,
									)
								}
							}
							break
						}
					}
				}
			}
		}
	}

	// Notify reporter
	if e.reporter != nil {
		e.reporter.CompleteChange(*change, err)
	}

	return err
}

// validateChangePreExecution performs validation before executing a change
func (e *Executor) validateChangePreExecution(ctx context.Context, change planner.PlannedChange) error {
	switch change.Action {
	case planner.ActionExternalTool:
		return nil
	case planner.ActionUpdate, planner.ActionDelete:
		// For update/delete, verify resource still exists and check protection
		// Special case: singleton portal children without their own ID
		if change.ResourceID == "" &&
			change.ResourceType != "portal_customization" &&
			change.ResourceType != "portal_auth_settings" &&
			change.ResourceType != "portal_asset_logo" &&
			change.ResourceType != "portal_asset_favicon" {
			return fmt.Errorf("resource ID required for %s operation", change.Action)
		}

		// Skip validation for updates/deletes with ResourceID - planner already verified existence
		// and actual operations handle protection checks
		if change.ResourceID != "" {
			return nil
		}

		// Perform resource-specific validation for updates/deletes without ResourceID
		// (This is mainly for portal_customization which is a singleton)
		switch change.ResourceType {
		case "portal":
			if e.client != nil {
				portal, err := e.client.GetPortalByName(ctx, getResourceName(change.Fields))
				if err != nil {
					return fmt.Errorf("failed to fetch portal: %w", err)
				}
				if portal == nil {
					return fmt.Errorf("portal no longer exists")
				}

				// Check protection status using common utility
				isProtected := common.GetProtectionStatus(portal.NormalizedLabels)
				isProtectionChange := common.IsProtectionChange(change.Protection)

				// Validate protection using common utility
				resourceName := common.ExtractResourceName(change.Fields)
				if err := common.ValidateResourceProtection(
					"portal", resourceName, isProtected, change, isProtectionChange,
				); err != nil {
					return err
				}
			}
		case "api":
			if e.client != nil {
				api, err := e.client.GetAPIByName(ctx, getResourceName(change.Fields))
				if err != nil {
					return fmt.Errorf("failed to fetch API: %w", err)
				}
				if api == nil {
					return fmt.Errorf("API no longer exists")
				}

				// Check protection status
				isProtected := api.NormalizedLabels[labels.ProtectedKey] == "true"

				// For updates, check if this is a protection change (which is allowed)
				isProtectionChange := false
				if change.Action == planner.ActionUpdate {
					// Check if this is a protection change
					switch p := change.Protection.(type) {
					case planner.ProtectionChange:
						isProtectionChange = true
					case map[string]any:
						// From JSON deserialization
						if _, hasOld := p["old"].(bool); hasOld {
							if _, hasNew := p["new"].(bool); hasNew {
								isProtectionChange = true
							}
						}
					}
				}

				// Block protected resources unless it's a protection change
				if isProtected && !isProtectionChange &&
					(change.Action == planner.ActionUpdate || change.Action == planner.ActionDelete) {
					return fmt.Errorf("resource is protected and cannot be %s",
						actionToVerb(change.Action))
				}
			}
		}

	case planner.ActionCreate:
		// For create, verify resource doesn't already exist
		switch change.ResourceType {
		case "portal":
			if e.client != nil {
				resourceName := getResourceName(change.Fields)
				portal, err := e.client.GetPortalByName(ctx, resourceName)
				if err != nil {
					// API error is acceptable here - might mean not found
					// Only fail if it's a real API error (not 404)
					if !strings.Contains(err.Error(), "not found") {
						return fmt.Errorf("failed to check existing portal: %w", err)
					}
				}
				if portal != nil {
					// Portal already exists - this is an error for CREATE
					return fmt.Errorf("portal '%s' already exists", resourceName)
				}
			}
		case "control_plane":
			if e.client != nil {
				resourceName := common.ExtractResourceName(change.Fields)
				cp, err := e.client.GetControlPlaneByName(ctx, resourceName)
				if err != nil {
					if !strings.Contains(err.Error(), "not found") {
						return fmt.Errorf("failed to check existing control plane: %w", err)
					}
				}
				if cp != nil {
					return fmt.Errorf("control_plane '%s' already exists", resourceName)
				}
			}
		case "api":
			if e.client != nil {
				resourceName := common.ExtractResourceName(change.Fields)
				api, err := e.client.GetAPIByName(ctx, resourceName)
				if err != nil {
					// API error is acceptable here - might mean not found
					// Only fail if it's a real API error (not 404)
					if !strings.Contains(err.Error(), "not found") {
						return common.FormatAPIError("api", resourceName, "check existence", err)
					}
				}
				if api != nil {
					// API already exists - this is an error for CREATE
					return common.FormatResourceExistsError("api", resourceName)
				}
			}
		}
	}

	return nil
}

// resolveAuthStrategyRef resolves an auth strategy reference to its ID
func (e *Executor) resolveAuthStrategyRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	// First check if it was created in this execution
	if authStrategies, ok := e.refToID["application_auth_strategy"]; ok {
		if id, found := authStrategies[lookupRef]; found {
			return id, nil
		}
		// Fallback to original ref in case older executions stored placeholders
		if lookupRef != refInfo.Ref {
			if id, found := authStrategies[refInfo.Ref]; found {
				return id, nil
			}
		}
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields["name"]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Otherwise, look it up from the API
	strategy, err := e.client.GetAuthStrategyByName(ctx, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get auth strategy by name: %w", err)
	}
	if strategy == nil {
		return "", fmt.Errorf("auth strategy not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupValue)
	}

	return strategy.ID, nil
}

// resolvePortalRef resolves a portal reference to its ID
func (e *Executor) resolvePortalRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	// First check if the reference already has an ID (resolved from dependency)
	if refInfo.ID != "" {
		return refInfo.ID, nil
	}

	// Check if it was created in this execution
	if portals, ok := e.refToID["portal"]; ok {
		if id, found := portals[refInfo.Ref]; found {
			return id, nil
		}
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := refInfo.Ref
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields["name"]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Otherwise, look it up from the API
	portal, err := e.client.GetPortalByName(ctx, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get portal by name: %w", err)
	}
	if portal == nil {
		return "", fmt.Errorf("portal not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupValue)
	}

	return portal.ID, nil
}

func (e *Executor) resolvePortalTeamRef(
	ctx context.Context,
	portalID string,
	refInfo planner.ReferenceInfo,
) (string, error) {
	if portalID == "" {
		return "", fmt.Errorf("portal ID is required to resolve portal team")
	}

	if teams, ok := e.refToID["portal_team"]; ok {
		if id, found := teams[refInfo.Ref]; found && id != "" {
			return id, nil
		}
	}

	lookupValue := refInfo.Ref
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields["name"]; hasName && name != "" {
			lookupValue = name
		}
	}

	portalTeams, err := e.client.ListPortalTeams(ctx, portalID)
	if err != nil {
		return "", fmt.Errorf("failed to list portal teams: %w", err)
	}

	for _, team := range portalTeams {
		if team.Name == lookupValue {
			return team.ID, nil
		}
	}

	return "", fmt.Errorf("portal team not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupValue)
}

func (e *Executor) resolveControlPlaneRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	if controlPlanes, ok := e.refToID["control_plane"]; ok {
		if id, found := controlPlanes[lookupRef]; found && id != "" && id != "[unknown]" {
			return id, nil
		}
	}

	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, ok := refInfo.LookupFields["name"]; ok && name != "" {
			lookupValue = name
		}
	}

	cp, err := e.client.GetControlPlaneByName(ctx, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get control plane by name: %w", err)
	}
	if cp == nil {
		return "", fmt.Errorf("control plane not found: ref=%s, lookup=%s", refInfo.Ref, lookupValue)
	}

	return cp.ID, nil
}

func (e *Executor) syncControlPlaneGroupMembers(
	ctx context.Context,
	change *planner.PlannedChange,
	controlPlaneID string,
) error {
	field, ok := change.Fields["members"]
	if !ok {
		return nil
	}

	desiredIDs, err := extractMemberIDsFromField(field)
	if err != nil {
		return fmt.Errorf("failed to extract control plane group members: %w", err)
	}
	if desiredIDs == nil {
		return nil
	}

	resolved := make([]string, len(desiredIDs))
	copy(resolved, desiredIDs)

	refInfo, hasRefs := change.References["members"]
	for idx, id := range desiredIDs {
		if !tags.IsRefPlaceholder(id) {
			continue
		}

		if !hasRefs || !refInfo.IsArray {
			return fmt.Errorf("missing reference information for control plane group member %q", id)
		}

		resolvedID, err := e.resolveMemberReference(ctx, id, refInfo, idx)
		if err != nil {
			return err
		}
		resolved[idx] = resolvedID
	}

	for _, id := range resolved {
		if tags.IsRefPlaceholder(id) {
			return fmt.Errorf("unable to resolve control plane group member reference %q", id)
		}
	}

	normalized := normalizers.NormalizeMemberIDs(resolved)
	if e.dryRun {
		return nil
	}

	return e.client.UpsertControlPlaneGroupMemberships(ctx, controlPlaneID, normalized)
}

func (e *Executor) resolveMemberReference(
	ctx context.Context,
	placeholder string,
	refInfo planner.ReferenceInfo,
	index int,
) (string, error) {
	targetIndex := -1
	if index < len(refInfo.Refs) && refInfo.Refs[index] == placeholder {
		targetIndex = index
	} else {
		for i, ref := range refInfo.Refs {
			if ref == placeholder {
				targetIndex = i
				break
			}
		}
	}

	if targetIndex == -1 {
		return "", fmt.Errorf("control plane membership reference %q not found", placeholder)
	}

	lookupFields := buildLookupFieldsForIndex(refInfo, targetIndex)
	lookupRef := planner.ReferenceInfo{
		Ref:          refInfo.Refs[targetIndex],
		LookupFields: lookupFields,
	}

	return e.resolveControlPlaneRef(ctx, lookupRef)
}

func buildLookupFieldsForIndex(refInfo planner.ReferenceInfo, index int) map[string]string {
	if refInfo.LookupArrays == nil {
		return nil
	}

	fields := make(map[string]string)
	if names, ok := refInfo.LookupArrays["names"]; ok {
		if index < len(names) && names[index] != "" {
			fields["name"] = names[index]
		}
	}

	if len(fields) == 0 {
		return nil
	}
	return fields
}

func extractMemberIDsFromField(field any) ([]string, error) {
	switch v := field.(type) {
	case []map[string]string:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			ids = append(ids, item["id"])
		}
		return ids, nil
	case []map[string]any:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			id, ok := item["id"].(string)
			if !ok {
				return nil, fmt.Errorf("control plane member entry missing id")
			}
			ids = append(ids, id)
		}
		return ids, nil
	case []any:
		ids := make([]string, 0, len(v))
		for _, item := range v {
			switch entry := item.(type) {
			case map[string]string:
				ids = append(ids, entry["id"])
			case map[string]any:
				id, ok := entry["id"].(string)
				if !ok {
					return nil, fmt.Errorf("control plane member entry missing id")
				}
				ids = append(ids, id)
			default:
				return nil, fmt.Errorf("unsupported control plane member entry type %T", item)
			}
		}
		return ids, nil
	default:
		return nil, nil
	}
}

// resolveAPIRef resolves an API reference to its ID
func (e *Executor) resolveAPIRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	// First check if it was created in this execution
	if apis, ok := e.refToID["api"]; ok {
		if id, found := apis[lookupRef]; found {
			slog.Debug("Resolved API reference from created resources",
				"api_ref", lookupRef,
				"api_id", id,
			)
			return id, nil
		}
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields["name"]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Try to find the API in Konnect with retry for eventual consistency
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		api, err := e.client.GetAPIByName(ctx, lookupValue)
		if err == nil && api != nil {
			apiID := api.ID
			slog.Debug("Resolved API reference from Konnect",
				"api_ref", refInfo.Ref,
				"lookup_value", lookupValue,
				"api_id", apiID,
				"attempt", attempt+1,
			)

			// Cache this resolution
			if apis, ok := e.refToID["api"]; ok {
				apis[refInfo.Ref] = apiID
			}
			return apiID, nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("failed to resolve API reference %s (lookup: %s) after 3 attempts: %w",
		refInfo.Ref, lookupValue, lastErr)
}

// resolveEventGatewayRef resolves an event gateway reference to its ID
func (e *Executor) resolveEventGatewayRef(ctx context.Context, refInfo planner.ReferenceInfo) (string, error) {
	lookupRef := refInfo.Ref
	if tags.IsRefPlaceholder(lookupRef) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(lookupRef); ok && parsedRef != "" {
			lookupRef = parsedRef
		}
	}

	// First check if it was created in this execution
	if gateways, ok := e.refToID["event_gateway"]; ok {
		if id, found := gateways[lookupRef]; found {
			slog.Debug("Resolved event gateway reference from created resources",
				"gateway_ref", lookupRef,
				"gateway_id", id,
			)
			return id, nil
		}
	}

	// Determine the lookup value - use name from lookup fields if available
	lookupValue := lookupRef
	if refInfo.LookupFields != nil {
		if name, hasName := refInfo.LookupFields["name"]; hasName && name != "" {
			lookupValue = name
		}
	}

	// Try to find the event gateway in Konnect
	gateway, err := e.client.GetEventGatewayControlPlaneByName(ctx, lookupValue)
	if err != nil {
		return "", fmt.Errorf("failed to get event gateway by name: %w", err)
	}
	if gateway == nil {
		return "", fmt.Errorf("event gateway not found: ref=%s, looked up by name=%s", refInfo.Ref, lookupValue)
	}

	gatewayID := gateway.ID
	slog.Debug("Resolved event gateway reference from Konnect",
		"gateway_ref", refInfo.Ref,
		"lookup_value", lookupValue,
		"gateway_id", gatewayID,
	)

	// Cache this resolution
	if gateways, ok := e.refToID["event_gateway"]; ok {
		gateways[refInfo.Ref] = gatewayID
	}

	return gatewayID, nil
}

// populatePortalPages fetches and caches all pages for a portal
func (e *Executor) populatePortalPages(ctx context.Context, portalID string) error {
	portal, exists := e.stateCache.Portals[portalID]
	if !exists {
		portal = &state.CachedPortal{
			Pages: make(map[string]*state.CachedPortalPage),
		}
		e.stateCache.Portals[portalID] = portal
	}

	// Fetch all pages
	pages, err := e.client.ListManagedPortalPages(ctx, portalID)
	if err != nil {
		return fmt.Errorf("failed to list portal pages: %w", err)
	}

	// First pass: create all pages
	pageMap := make(map[string]*state.CachedPortalPage)
	for _, page := range pages {
		cachedPage := &state.CachedPortalPage{
			PortalPage: page,
			Children:   make(map[string]*state.CachedPortalPage),
		}
		pageMap[page.ID] = cachedPage
	}

	// Second pass: establish parent-child relationships
	for _, page := range pages {
		cachedPage := pageMap[page.ID]

		if page.ParentPageID == "" {
			// Root page
			portal.Pages[page.ID] = cachedPage
		} else if parent, ok := pageMap[page.ParentPageID]; ok {
			// Child page
			parent.Children[page.ID] = cachedPage
		}
	}

	return nil
}

// populateAPIDocuments fetches and caches all documents for an API
func (e *Executor) populateAPIDocuments(ctx context.Context, apiID string) error {
	if apiID == "" {
		return fmt.Errorf("API ID is required to populate documents")
	}

	cachedAPI, exists := e.stateCache.APIs[apiID]
	if !exists {
		cachedAPI = &state.CachedAPI{
			Documents:       make(map[string]*state.CachedAPIDocument),
			Versions:        make(map[string]*state.APIVersion),
			Publications:    make(map[string]*state.APIPublication),
			Implementations: make(map[string]*state.APIImplementation),
		}
		e.stateCache.APIs[apiID] = cachedAPI
	}

	if cachedAPI.Documents == nil {
		cachedAPI.Documents = make(map[string]*state.CachedAPIDocument)
	}

	if len(cachedAPI.Documents) > 0 {
		return nil
	}

	documents, err := e.client.ListAPIDocuments(ctx, apiID)
	if err != nil {
		return fmt.Errorf("failed to list API documents: %w", err)
	}

	docMap := make(map[string]*state.CachedAPIDocument)
	for _, doc := range documents {
		cachedDoc := &state.CachedAPIDocument{
			APIDocument: doc,
			Children:    make(map[string]*state.CachedAPIDocument),
		}
		docMap[doc.ID] = cachedDoc
	}

	for _, cachedDoc := range docMap {
		if cachedDoc.ParentDocumentID == "" {
			cachedAPI.Documents[cachedDoc.ID] = cachedDoc
			continue
		}

		parent, ok := docMap[cachedDoc.ParentDocumentID]
		if !ok {
			cachedAPI.Documents[cachedDoc.ID] = cachedDoc
			continue
		}

		if parent.Children == nil {
			parent.Children = make(map[string]*state.CachedAPIDocument)
		}
		parent.Children[cachedDoc.ID] = cachedDoc
	}

	return nil
}

// resolvePortalPageRef resolves a portal page reference to its ID
func (e *Executor) resolvePortalPageRef(
	ctx context.Context, portalID string, pageRef string, lookupFields map[string]string,
) (string, error) {
	// First check if it was created in this execution
	if pages, ok := e.refToID["portal_page"]; ok {
		if id, found := pages[pageRef]; found {
			return id, nil
		}
	}

	// Ensure portal pages are cached
	if _, exists := e.stateCache.Portals[portalID]; !exists ||
		e.stateCache.Portals[portalID].Pages == nil {
		if err := e.populatePortalPages(ctx, portalID); err != nil {
			return "", err
		}
	}

	portal := e.stateCache.Portals[portalID]

	// If we have a parent path, use it for more accurate matching
	if lookupFields != nil && lookupFields["parent_path"] != "" {
		targetPath := lookupFields["parent_path"]

		if page := portal.FindPageBySlugPath(targetPath); page != nil {
			return page.ID, nil
		}
	}

	// Fallback: search all pages for matching slug
	var searchPages func(pages map[string]*state.CachedPortalPage) string
	searchPages = func(pages map[string]*state.CachedPortalPage) string {
		for _, page := range pages {
			normalizedSlug := strings.TrimPrefix(page.Slug, "/")
			if normalizedSlug == pageRef {
				return page.ID
			}
			// Search children
			if childID := searchPages(page.Children); childID != "" {
				return childID
			}
		}
		return ""
	}

	if pageID := searchPages(portal.Pages); pageID != "" {
		return pageID, nil
	}

	return "", fmt.Errorf("portal page not found: ref=%s in portal=%s", pageRef, portalID)
}

// resolveAPIDocumentRef resolves an API document reference to its ID
func (e *Executor) resolveAPIDocumentRef(
	ctx context.Context, apiID string, refInfo planner.ReferenceInfo,
) (string, error) {
	if refInfo.ID != "" && refInfo.ID != "[unknown]" {
		return refInfo.ID, nil
	}

	actualRef := refInfo.Ref
	if strings.HasPrefix(actualRef, tags.RefPlaceholderPrefix) {
		if parsedRef, _, ok := tags.ParseRefPlaceholder(actualRef); ok {
			actualRef = parsedRef
		}
	}

	if docs, ok := e.refToID["api_document"]; ok {
		if id, found := docs[actualRef]; found {
			return id, nil
		}
	}

	if apiID == "" {
		return "", fmt.Errorf("API ID is required to resolve document reference")
	}

	if err := e.populateAPIDocuments(ctx, apiID); err != nil {
		return "", err
	}

	cachedAPI, ok := e.stateCache.APIs[apiID]
	if !ok {
		return "", fmt.Errorf("API %s not found in cache", apiID)
	}

	if refInfo.LookupFields != nil {
		if path, ok := refInfo.LookupFields["slug_path"]; ok && path != "" {
			if doc := findCachedAPIDocumentByPath(cachedAPI.Documents, path); doc != nil {
				return doc.ID, nil
			}
		}
		if slug, ok := refInfo.LookupFields["slug"]; ok && slug != "" {
			if doc := findCachedAPIDocumentByPath(cachedAPI.Documents, slug); doc != nil {
				return doc.ID, nil
			}
		}
	}

	return "", fmt.Errorf("failed to resolve API document reference %q", actualRef)
}

func findCachedAPIDocumentByPath(
	documents map[string]*state.CachedAPIDocument, path string,
) *state.CachedAPIDocument {
	cleanPath := strings.Trim(path, "/")
	if cleanPath == "" {
		return nil
	}

	segments := strings.Split(cleanPath, "/")
	for _, doc := range documents {
		if found := traverseCachedAPIDocument(doc, segments); found != nil {
			return found
		}
	}

	return nil
}

func traverseCachedAPIDocument(
	doc *state.CachedAPIDocument, segments []string,
) *state.CachedAPIDocument {
	if doc == nil || len(segments) == 0 {
		return nil
	}

	slug := strings.Trim(strings.TrimPrefix(doc.Slug, "/"), "/")
	if slug != segments[0] {
		return nil
	}

	if len(segments) == 1 {
		return doc
	}

	for _, child := range doc.Children {
		if found := traverseCachedAPIDocument(child, segments[1:]); found != nil {
			return found
		}
	}

	return nil
}

// Resource operations

func (e *Executor) createResource(ctx context.Context, change *planner.PlannedChange) (string, error) {
	// Note: ExecutionContext is now passed explicitly to executors instead of using context.WithValue

	switch change.ResourceType {
	case "portal":
		// Resolve auth strategy reference if present
		if authStrategyRef, ok := change.References["default_application_auth_strategy_id"]; ok &&
			authStrategyRef.ID == "" {
			authStrategyID, err := e.resolveAuthStrategyRef(ctx, authStrategyRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve auth strategy reference: %w", err)
			}
			// Update the reference with the resolved ID
			authStrategyRef.ID = authStrategyID
			change.References["default_application_auth_strategy_id"] = authStrategyRef

			// Also update the field value to use the resolved ID instead of the placeholder
			change.Fields["default_application_auth_strategy_id"] = authStrategyID
		}
		return e.portalExecutor.Create(ctx, *change)
	case "control_plane":
		id, err := e.controlPlaneExecutor.Create(ctx, *change)
		if err != nil {
			return "", err
		}
		if err := e.syncControlPlaneGroupMembers(ctx, change, id); err != nil {
			return "", err
		}
		return id, nil
	case "api":
		// No references to resolve for api
		return e.apiExecutor.Create(ctx, *change)
	case "catalog_service":
		return e.catalogServiceExecutor.Create(ctx, *change)
	case "api_version":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		return e.apiVersionExecutor.Create(ctx, *change)
	case "api_publication":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		// Also resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		// Resolve auth_strategy_ids array references if needed
		if authStrategyRefs, ok := change.References["auth_strategy_ids"]; ok && authStrategyRefs.IsArray {
			resolvedIDs := make([]string, 0, len(authStrategyRefs.Refs))

			for i, ref := range authStrategyRefs.Refs {
				var resolvedID string
				var err error

				// Check if already resolved
				if authStrategyRefs.ResolvedIDs != nil && i < len(authStrategyRefs.ResolvedIDs) &&
					authStrategyRefs.ResolvedIDs[i] != "" {
					resolvedID = authStrategyRefs.ResolvedIDs[i]
				} else {
					// Construct ReferenceInfo for the auth strategy
					refInfo := planner.ReferenceInfo{
						Ref: ref,
					}
					// Add lookup fields if available
					if names, ok := authStrategyRefs.LookupArrays["names"]; ok && i < len(names) {
						refInfo.LookupFields = map[string]string{
							"name": names[i],
						}
					}

					resolvedID, err = e.resolveAuthStrategyRef(ctx, refInfo)
					if err != nil {
						return "", fmt.Errorf("failed to resolve auth strategy reference %q: %w", ref, err)
					}
				}

				if resolvedID == "" {
					return "", fmt.Errorf("failed to resolve auth strategy reference %q", ref)
				}

				resolvedIDs = append(resolvedIDs, resolvedID)
			}

			// Update the reference with resolved IDs
			authStrategyRefs.ResolvedIDs = resolvedIDs
			change.References["auth_strategy_ids"] = authStrategyRefs
		}
		return e.apiPublicationExecutor.Create(ctx, *change)
	case "api_implementation":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		return e.apiImplementationExecutor.Create(ctx, *change)
	case "api_document":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		if parentRef, ok := change.References["parent_document_id"]; ok && parentRef.Ref != "" && parentRef.ID == "" {
			apiID := ""
			if apiInfo, exists := change.References["api_id"]; exists {
				apiID = apiInfo.ID
			}
			if apiID == "" && change.Parent != nil {
				apiID = change.Parent.ID
			}
			resolvedParentID, err := e.resolveAPIDocumentRef(ctx, apiID, parentRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent document reference: %w", err)
			}
			parentRef.ID = resolvedParentID
			change.References["parent_document_id"] = parentRef
		}
		return e.apiDocumentExecutor.Create(ctx, *change)
	case "application_auth_strategy":
		// No references to resolve for application_auth_strategy
		return e.authStrategyExecutor.Create(ctx, *change)
	case "portal_customization":
		// Portal customization is a singleton resource - always exists, so we update instead
		portalID, err := e.resolvePortalRef(ctx, change.References["portal_id"])
		if err != nil {
			return "", err
		}
		return e.portalCustomizationExecutor.Update(ctx, *change, portalID)
	case "portal_auth_settings":
		portalID, err := e.resolvePortalRef(ctx, change.References["portal_id"])
		if err != nil {
			return "", err
		}
		return e.portalAuthSettingsExecutor.Update(ctx, *change, portalID)
	case "portal_asset_logo":
		// Portal asset logo is a singleton resource - always exists, so we update instead
		portalID, err := e.resolvePortalRef(ctx, change.References["portal_id"])
		if err != nil {
			return "", err
		}
		return e.portalAssetLogoExecutor.Update(ctx, *change, portalID)
	case "portal_asset_favicon":
		// Portal asset favicon is a singleton resource - always exists, so we update instead
		portalID, err := e.resolvePortalRef(ctx, change.References["portal_id"])
		if err != nil {
			return "", err
		}
		return e.portalAssetFaviconExecutor.Update(ctx, *change, portalID)
	case "portal_custom_domain":
		// Resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalDomainExecutor.Create(ctx, *change)
	case "portal_page":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		// Handle parent page reference resolution if needed
		if parentPageRef, ok := change.References["parent_page_id"]; ok && parentPageRef.ID == "" {
			portalID := change.References["portal_id"].ID
			parentPageID, err := e.resolvePortalPageRef(ctx, portalID, parentPageRef.Ref, parentPageRef.LookupFields)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent page reference: %w", err)
			}
			// Create a new reference with the resolved ID
			parentPageRef.ID = parentPageID
			change.References["parent_page_id"] = parentPageRef
		}
		return e.portalPageExecutor.Create(ctx, *change)
	case "portal_snippet":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalSnippetExecutor.Create(ctx, *change)
	case "portal_team":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalTeamExecutor.Create(ctx, *change)
	case "portal_team_role":
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}

		if teamRef, ok := change.References["team_id"]; ok && teamRef.ID == "" {
			portalID := ""
			if portalInfo, exists := change.References["portal_id"]; exists {
				portalID = portalInfo.ID
			}
			if portalID == "" && change.Parent != nil {
				portalID = change.Parent.ID
			}

			teamID, err := e.resolvePortalTeamRef(ctx, portalID, teamRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal team reference: %w", err)
			}
			teamRef.ID = teamID
			change.References["team_id"] = teamRef
		}

		if entityRef, ok := change.References["entity_id"]; ok && (entityRef.ID == "" || entityRef.ID == "[unknown]") {
			apiID, err := e.resolveAPIRef(ctx, entityRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve entity reference: %w", err)
			}
			entityRef.ID = apiID
			change.References["entity_id"] = entityRef
		}

		return e.portalTeamRoleExecutor.Create(ctx, *change)
	case "portal_email_config":
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalEmailConfigExecutor.Create(ctx, *change)
	case "portal_email_template":
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalEmailTemplateExecutor.Create(ctx, *change)
	case "event_gateway":
		return e.eventGatewayControlPlaneExecutor.Create(ctx, *change)
	case "event_gateway_backend_cluster":
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			// Update the reference with the resolved ID
			gatewayRef.ID = gatewayID
			change.References["event_gateway_id"] = gatewayRef
		}
		return e.eventGatewayBackendClusterExecutor.Create(ctx, *change)
	case "event_gateway_virtual_cluster":
		// Resolve event gateway reference if needed
		if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			// Update the reference with the resolved ID
			gatewayRef.ID = gatewayID
			change.References["event_gateway_id"] = gatewayRef
		}
		return e.eventGatewayVirtualClusterExecutor.Create(ctx, *change)
	case "team":
		return e.organizationTeamExecutor.Create(ctx, *change)
	default:
		return "", fmt.Errorf("create operation not yet implemented for %s", change.ResourceType)
	}
}

func (e *Executor) updateResource(ctx context.Context, change *planner.PlannedChange) (string, error) {
	// Note: ExecutionContext is now passed explicitly to executors instead of using context.WithValue

	switch change.ResourceType {
	case "portal":
		return e.portalExecutor.Update(ctx, *change)
	case "control_plane":
		id, err := e.controlPlaneExecutor.Update(ctx, *change)
		if err != nil {
			return "", err
		}
		if err := e.syncControlPlaneGroupMembers(ctx, change, id); err != nil {
			return "", err
		}
		return id, nil
	case "api":
		return e.apiExecutor.Update(ctx, *change)
	case "catalog_service":
		return e.catalogServiceExecutor.Update(ctx, *change)
	case "api_document":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		if parentRef, ok := change.References["parent_document_id"]; ok && parentRef.Ref != "" && parentRef.ID == "" {
			apiID := ""
			if apiInfo, exists := change.References["api_id"]; exists {
				apiID = apiInfo.ID
			}
			if apiID == "" && change.Parent != nil {
				apiID = change.Parent.ID
			}
			resolvedParentID, err := e.resolveAPIDocumentRef(ctx, apiID, parentRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent document reference: %w", err)
			}
			parentRef.ID = resolvedParentID
			change.References["parent_document_id"] = parentRef
		}
		return e.apiDocumentExecutor.Update(ctx, *change)
	case "api_publication":
		// API publications use PUT for both create and update
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		// Also resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		// Resolve auth strategy references if present
		if authStrategyRefs, ok := change.References["auth_strategy_ids"]; ok && authStrategyRefs.IsArray {
			resolvedIDs := make([]string, 0, len(authStrategyRefs.Refs))
			for _, ref := range authStrategyRefs.Refs {
				strategyRef := planner.ReferenceInfo{
					Ref:          ref,
					LookupFields: make(map[string]string),
				}
				// Copy lookup fields if available
				if authStrategyRefs.LookupArrays != nil && len(authStrategyRefs.LookupArrays["names"]) > 0 {
					// Find corresponding name for this ref
					for i, r := range authStrategyRefs.Refs {
						if r == ref && i < len(authStrategyRefs.LookupArrays["names"]) {
							strategyRef.LookupFields["name"] = authStrategyRefs.LookupArrays["names"][i]
							break
						}
					}
				}
				resolvedID, err := e.resolveAuthStrategyRef(ctx, strategyRef)
				if err != nil {
					return "", fmt.Errorf("failed to resolve auth strategy reference %q: %w", ref, err)
				}
				resolvedIDs = append(resolvedIDs, resolvedID)
			}
			// Update the reference with resolved IDs
			authStrategyRefs.ResolvedIDs = resolvedIDs
			change.References["auth_strategy_ids"] = authStrategyRefs
		}
		// Use Create method which handles PUT (both create and update)
		return e.apiPublicationExecutor.Create(ctx, *change)
	case "application_auth_strategy":
		return e.authStrategyExecutor.Update(ctx, *change)
	case "portal_customization":
		portalID, err := e.resolvePortalRef(ctx, change.References["portal_id"])
		if err != nil {
			return "", err
		}
		return e.portalCustomizationExecutor.Update(ctx, *change, portalID)
	case "portal_auth_settings":
		portalID, err := e.resolvePortalRef(ctx, change.References["portal_id"])
		if err != nil {
			return "", err
		}
		return e.portalAuthSettingsExecutor.Update(ctx, *change, portalID)
	case "portal_email_config":
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalEmailConfigExecutor.Update(ctx, *change)
	case "portal_email_template":
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalEmailTemplateExecutor.Update(ctx, *change)
	case "portal_asset_logo":
		portalID, err := e.resolvePortalRef(ctx, change.References["portal_id"])
		if err != nil {
			return "", err
		}
		return e.portalAssetLogoExecutor.Update(ctx, *change, portalID)
	case "portal_asset_favicon":
		portalID, err := e.resolvePortalRef(ctx, change.References["portal_id"])
		if err != nil {
			return "", err
		}
		return e.portalAssetFaviconExecutor.Update(ctx, *change, portalID)
	case "portal_custom_domain":
		return e.portalDomainExecutor.Update(ctx, *change)
	case "portal_page":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		// Handle parent page reference resolution if needed
		if parentPageRef, ok := change.References["parent_page_id"]; ok && parentPageRef.ID == "" {
			portalID := change.References["portal_id"].ID
			parentPageID, err := e.resolvePortalPageRef(ctx, portalID, parentPageRef.Ref, parentPageRef.LookupFields)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent page reference: %w", err)
			}
			// Create a new reference with the resolved ID
			parentPageRef.ID = parentPageID
			change.References["parent_page_id"] = parentPageRef
		}
		return e.portalPageExecutor.Update(ctx, *change)
	case "portal_snippet":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalSnippetExecutor.Update(ctx, *change)
	case "portal_team":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalTeamExecutor.Update(ctx, *change)
	case "api_version":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		return e.apiVersionExecutor.Update(ctx, *change)
	// Note: api_publication and api_implementation don't support update
	case "event_gateway":
		return e.eventGatewayControlPlaneExecutor.Update(ctx, *change)
	case "event_gateway_backend_cluster":
		// Resolve event gateway reference if needed (typically should already be in Parent)
		if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References["event_gateway_id"] = gatewayRef
		}
		return e.eventGatewayBackendClusterExecutor.Update(ctx, *change)
	case "event_gateway_virtual_cluster":
		// Resolve event gateway reference if needed (typically should already be in Parent)
		if gatewayRef, ok := change.References["event_gateway_id"]; ok && gatewayRef.ID == "" {
			gatewayID, err := e.resolveEventGatewayRef(ctx, gatewayRef)
			if err != nil {
				return "", fmt.Errorf("failed to resolve event gateway reference: %w", err)
			}
			gatewayRef.ID = gatewayID
			change.References["event_gateway_id"] = gatewayRef
		}
		return e.eventGatewayVirtualClusterExecutor.Update(ctx, *change)
	case "team":
		return e.organizationTeamExecutor.Update(ctx, *change)
	default:
		return "", fmt.Errorf("update operation not yet implemented for %s", change.ResourceType)
	}
}

func (e *Executor) deleteResource(ctx context.Context, change *planner.PlannedChange) error {
	// Note: ExecutionContext is now passed explicitly to executors instead of using context.WithValue

	switch change.ResourceType {
	case "portal":
		// No references to resolve for portal
		return e.portalExecutor.Delete(ctx, *change)
	case "control_plane":
		return e.controlPlaneExecutor.Delete(ctx, *change)
	case "api":
		// No references to resolve for api
		return e.apiExecutor.Delete(ctx, *change)
	case "catalog_service":
		return e.catalogServiceExecutor.Delete(ctx, *change)
	case "api_version":
		// No references to resolve for api_version delete
		return e.apiVersionExecutor.Delete(ctx, *change)
	case "api_publication":
		// No references to resolve for api_publication delete
		return e.apiPublicationExecutor.Delete(ctx, *change)
	case "api_implementation":
		// No references to resolve for api_implementation delete
		return e.apiImplementationExecutor.Delete(ctx, *change)
	case "api_document":
		// First resolve API reference if needed
		if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
			apiID, err := e.resolveAPIRef(ctx, apiRef)
			if err != nil {
				return fmt.Errorf("failed to resolve API reference: %w", err)
			}
			// Update the reference with the resolved ID
			apiRef.ID = apiID
			change.References["api_id"] = apiRef
		}
		return e.apiDocumentExecutor.Delete(ctx, *change)
	case "application_auth_strategy":
		// No references to resolve for application_auth_strategy
		return e.authStrategyExecutor.Delete(ctx, *change)
	case "portal_custom_domain":
		// No references to resolve for portal_custom_domain
		return e.portalDomainExecutor.Delete(ctx, *change)
	case "portal_page":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalPageExecutor.Delete(ctx, *change)
	case "portal_snippet":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalSnippetExecutor.Delete(ctx, *change)
	case "portal_team":
		// First resolve portal reference if needed
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			// Update the reference with the resolved ID
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalTeamExecutor.Delete(ctx, *change)
	case "portal_team_role":
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		if teamRef, ok := change.References["team_id"]; ok && teamRef.ID == "" {
			portalID := ""
			if portalInfo, exists := change.References["portal_id"]; exists {
				portalID = portalInfo.ID
			}
			if portalID == "" && change.Parent != nil {
				portalID = change.Parent.ID
			}
			teamID, err := e.resolvePortalTeamRef(ctx, portalID, teamRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal team reference: %w", err)
			}
			teamRef.ID = teamID
			change.References["team_id"] = teamRef
		}
		return e.portalTeamRoleExecutor.Delete(ctx, *change)
	case "portal_email_config":
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalEmailConfigExecutor.Delete(ctx, *change)
	case "portal_email_template":
		if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
			portalID, err := e.resolvePortalRef(ctx, portalRef)
			if err != nil {
				return fmt.Errorf("failed to resolve portal reference: %w", err)
			}
			portalRef.ID = portalID
			change.References["portal_id"] = portalRef
		}
		return e.portalEmailTemplateExecutor.Delete(ctx, *change)
	// Note: portal_customization is a singleton resource and cannot be deleted
	case "event_gateway":
		return e.eventGatewayControlPlaneExecutor.Delete(ctx, *change)
	case "event_gateway_backend_cluster":
		// No need to resolve event gateway reference for delete - parent ID should be in Parent field
		return e.eventGatewayBackendClusterExecutor.Delete(ctx, *change)
	// I think this should be organization_team
	case "team":
		return e.organizationTeamExecutor.Delete(ctx, *change)
	case "event_gateway_virtual_cluster":
		// No need to resolve event gateway reference for delete - parent ID should be in Parent field
		return e.eventGatewayVirtualClusterExecutor.Delete(ctx, *change)
	default:
		return fmt.Errorf("delete operation not yet implemented for %s", change.ResourceType)
	}
}

// Helper functions

// getResourceName is deprecated, use common.ExtractResourceName instead
// Kept for backward compatibility with existing code
func getResourceName(fields map[string]any) string {
	return common.ExtractResourceName(fields)
}

// actionToVerb is deprecated, use common utilities instead
// Kept for backward compatibility with existing code
func actionToVerb(action planner.ActionType) string {
	switch action {
	case planner.ActionCreate:
		return "created"
	case planner.ActionUpdate:
		return "updated"
	case planner.ActionDelete:
		return "deleted"
	case planner.ActionExternalTool:
		return "executed"
	default:
		return string(action)
	}
}

// getParentAPIID resolves the parent API ID for child resources
//
//nolint:unused // kept for backward compatibility, will be removed in Phase 2 cleanup
func (e *Executor) getParentAPIID(ctx context.Context, change planner.PlannedChange) (string, error) {
	// Add debug logging
	logger := ctx.Value(log.LoggerKey).(*slog.Logger)
	logger.Debug("getParentAPIID called",
		slog.String("change_id", change.ID),
		slog.String("resource_type", change.ResourceType),
		slog.String("resource_ref", change.ResourceRef),
		slog.Any("parent", change.Parent),
	)

	if change.Parent == nil {
		return "", fmt.Errorf("parent API reference required")
	}

	// Log parent details
	logger.Debug("Parent details",
		slog.String("parent_ref", change.Parent.Ref),
		slog.String("parent_id", change.Parent.ID),
		slog.Bool("parent_id_empty", change.Parent.ID == ""),
		slog.Int("parent_id_length", len(change.Parent.ID)),
	)

	// Use the parent ID if it was already resolved
	if change.Parent.ID != "" {
		logger.Debug("Using resolved parent ID", slog.String("parent_id", change.Parent.ID))
		return change.Parent.ID, nil
	}

	// Check if parent was created in this execution
	logger.Debug("Checking dependencies", slog.Int("dep_count", len(change.DependsOn)))
	for _, dep := range change.DependsOn {
		if resourceID, ok := e.createdResources[dep]; ok {
			logger.Debug("Found parent in created resources",
				slog.String("dependency", dep),
				slog.String("resource_id", resourceID),
			)
			return resourceID, nil
		}
	}

	// Otherwise look up by name
	logger.Debug("Falling back to API lookup by name", slog.String("api_ref", change.Parent.Ref))
	parentAPI, err := e.client.GetAPIByName(ctx, change.Parent.Ref)
	if err != nil {
		return "", fmt.Errorf("failed to get parent API: %w", err)
	}
	if parentAPI == nil {
		return "", fmt.Errorf("parent API not found: %s", change.Parent.Ref)
	}

	logger.Debug("Found parent API by name",
		slog.String("api_name", parentAPI.Name),
		slog.String("api_id", parentAPI.ID),
	)

	return parentAPI.ID, nil
}

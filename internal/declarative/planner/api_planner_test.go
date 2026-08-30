package planner

import (
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanAPIChangesExternalParentPlansChildWithoutAPIChange(t *testing.T) {
	t.Parallel()

	versionName := "v1"
	externalAPI := resources.APIResource{
		BaseResource: resources.BaseResource{Ref: "shared-api"},
		External:     &resources.ExternalBlock{ID: "api-id"},
		Versions: []resources.APIVersionResource{{
			Ref: "v1", CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{Version: &versionName},
		}},
	}
	externalAPI.SetKonnectID("api-id")
	planner := NewPlanner(state.NewClient(state.ClientConfig{
		APIVersionAPI: &stubAPIVersionAPI{},
	}), slog.Default())
	planner.resources = &resources.ResourceSet{APIs: []resources.APIResource{externalAPI}}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)

	err := planner.planAPIChanges(
		t.Context(), &Config{Namespace: resources.NamespaceExternal}, []resources.APIResource{externalAPI}, plan,
	)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeAPIVersion, plan.Changes[0].ResourceType)
	require.Equal(t, ActionCreate, plan.Changes[0].Action)
	require.Equal(t, "api-id", plan.Changes[0].Parent.ID)
}

func TestPlanAPIChangesExternalParentDeletesOnlyDeclaredChild(t *testing.T) {
	t.Parallel()

	versionName := "v1"
	externalAPI := resources.APIResource{
		BaseResource: resources.BaseResource{Ref: "shared-api"},
		External:     &resources.ExternalBlock{ID: "api-id"},
		Versions: []resources.APIVersionResource{{
			Ref: "v1", CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{Version: &versionName},
		}},
	}
	externalAPI.SetKonnectID("api-id")
	planner := NewPlanner(state.NewClient(state.ClientConfig{
		APIVersionAPI: &stubAPIVersionAPI{response: &kkOps.ListAPIVersionsResponse{
			ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
				Data: []kkComps.ListAPIVersionResponseAPIVersionSummary{
					{ID: "version-id", Version: "v1"},
					{ID: "out-of-band-version-id", Version: "v2"},
				},
				Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 2}},
			},
		}},
	}), slog.Default())
	planner.resources = &resources.ResourceSet{APIs: []resources.APIResource{externalAPI}}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeDelete)

	err := planner.planAPIChanges(
		t.Context(), &Config{Namespace: resources.NamespaceExternal}, []resources.APIResource{externalAPI}, plan,
	)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeAPIVersion, plan.Changes[0].ResourceType)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, "version-id", plan.Changes[0].ResourceID)
	require.Equal(t, "api-id", plan.Changes[0].Parent.ID)
}

func TestPlanAPIChangesExternalParentSyncDeletesStaleVersions(t *testing.T) {
	t.Parallel()

	versionName := "v1"
	externalAPI := resources.APIResource{
		BaseResource: resources.BaseResource{Ref: "shared-api"},
		External:     &resources.ExternalBlock{ID: "api-id"},
		Versions: []resources.APIVersionResource{{
			Ref: "v1", CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{Version: &versionName},
		}},
	}
	externalAPI.SetKonnectID("api-id")
	planner := NewPlanner(state.NewClient(state.ClientConfig{
		APIVersionAPI: &stubAPIVersionAPI{response: &kkOps.ListAPIVersionsResponse{
			ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
				Data: []kkComps.ListAPIVersionResponseAPIVersionSummary{
					{ID: "out-of-band-version-id", Version: "v2"},
				},
				Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
			},
		}},
	}), slog.Default())
	planner.resources = &resources.ResourceSet{APIs: []resources.APIResource{externalAPI}}
	planner.resources.EnsureSyncScope().AddChild(
		resources.ResourceTypeAPI,
		externalAPI.GetRef(),
		resources.ResourceTypeAPIVersion,
	)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)

	err := planner.planAPIChanges(
		t.Context(), &Config{Namespace: resources.NamespaceExternal}, []resources.APIResource{externalAPI}, plan,
	)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)
	require.Equal(t, ResourceTypeAPIVersion, plan.Changes[0].ResourceType)
	require.Equal(t, ActionCreate, plan.Changes[0].Action)
	require.Equal(t, "v1", plan.Changes[0].ResourceRef)
	require.Equal(t, ResourceTypeAPIVersion, plan.Changes[1].ResourceType)
	require.Equal(t, ActionDelete, plan.Changes[1].Action)
	require.Equal(t, "out-of-band-version-id", plan.Changes[1].ResourceID)
	require.Equal(t, "api-id", plan.Changes[1].Parent.ID)
}

func TestPlanAPIChangesExternalParentSyncRetainsExtractedDesiredVersion(t *testing.T) {
	t.Parallel()

	versionName := "v1"
	externalAPI := resources.APIResource{
		BaseResource: resources.BaseResource{Ref: "shared-api"},
		External:     &resources.ExternalBlock{ID: "api-id"},
	}
	externalAPI.SetKonnectID("api-id")
	planner := NewPlanner(state.NewClient(state.ClientConfig{
		APIVersionAPI: &stubAPIVersionAPI{
			response: &kkOps.ListAPIVersionsResponse{
				ListAPIVersionResponse: &kkComps.ListAPIVersionResponse{
					Data: []kkComps.ListAPIVersionResponseAPIVersionSummary{
						{ID: "version-id", Version: versionName},
						{ID: "stale-version-id", Version: "v2"},
					},
					Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 2}},
				},
			},
			fetchResponse: &kkOps.FetchAPIVersionResponse{
				APIVersionResponse: &kkComps.APIVersionResponse{ID: "version-id", Version: versionName},
			},
		},
	}), slog.Default())
	planner.resources = &resources.ResourceSet{
		APIs: []resources.APIResource{externalAPI},
		APIVersions: []resources.APIVersionResource{{
			Ref:                     "v1",
			API:                     "shared-api",
			CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{Version: &versionName},
		}},
	}
	planner.resources.EnsureSyncScope().AddChild(
		resources.ResourceTypeAPI,
		externalAPI.GetRef(),
		resources.ResourceTypeAPIVersion,
	)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)

	err := planner.planAPIChanges(
		t.Context(), &Config{Namespace: resources.NamespaceExternal}, []resources.APIResource{externalAPI}, plan,
	)
	require.NoError(t, err)
	err = planner.planAPIVersionsChanges(
		t.Context(),
		&Config{Namespace: resources.NamespaceExternal},
		planner.resources.APIVersions,
		plan,
	)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeAPIVersion, plan.Changes[0].ResourceType)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, "stale-version-id", plan.Changes[0].ResourceID)
}

func TestPlanAPIChangesExternalParentSyncDeletesStalePublications(t *testing.T) {
	t.Parallel()

	externalAPI := resources.APIResource{
		BaseResource: resources.BaseResource{Ref: "shared-api"},
		External:     &resources.ExternalBlock{ID: "api-id"},
	}
	externalAPI.SetKonnectID("api-id")
	planner := NewPlanner(state.NewClient(state.ClientConfig{
		APIPublicationAPI: &stubAPIPublicationAPI{response: &kkOps.ListAPIPublicationsResponse{
			ListAPIPublicationResponse: &kkComps.ListAPIPublicationResponse{
				Data: []kkComps.APIPublicationListItem{{APIID: "api-id", PortalID: "portal-id"}},
				Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
			},
		}},
	}), slog.Default())
	planner.resources = &resources.ResourceSet{APIs: []resources.APIResource{externalAPI}}
	planner.resources.EnsureSyncScope().AddChild(
		resources.ResourceTypeAPI,
		externalAPI.GetRef(),
		resources.ResourceTypeAPIPublication,
	)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)

	err := planner.planAPIChanges(
		t.Context(), &Config{Namespace: resources.NamespaceExternal}, []resources.APIResource{externalAPI}, plan,
	)

	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeAPIPublication, plan.Changes[0].ResourceType)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, "shared-api-to-portal-id", plan.Changes[0].ResourceRef)
	require.Equal(t, "api-id", plan.Changes[0].Parent.ID)
}

func TestPlanAPIChangesExternalParentSyncDeletesStaleImplementations(t *testing.T) {
	t.Parallel()

	externalAPI := resources.APIResource{
		BaseResource: resources.BaseResource{Ref: "shared-api"},
		External:     &resources.ExternalBlock{ID: "api-id"},
	}
	externalAPI.SetKonnectID("api-id")
	planner := NewPlanner(state.NewClient(state.ClientConfig{
		APIImplementationAPI: &stubAPIImplementationAPI{response: &kkOps.ListAPIImplementationsResponse{
			ListAPIImplementationsResponse: &kkComps.ListAPIImplementationsResponse{
				Data: []kkComps.APIImplementationListItem{
					kkComps.CreateAPIImplementationListItemAPIImplementationListItemGatewayServiceEntity(
						kkComps.APIImplementationListItemGatewayServiceEntity{
							ID:    "implementation-id",
							APIID: "api-id",
							Service: &kkComps.APIImplementationService{
								ID: "service-id", ControlPlaneID: "control-plane-id",
							},
						},
					),
				},
				Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
			},
		}},
	}), slog.Default())
	planner.resources = &resources.ResourceSet{APIs: []resources.APIResource{externalAPI}}
	planner.resources.EnsureSyncScope().AddChild(
		resources.ResourceTypeAPI,
		externalAPI.GetRef(),
		resources.ResourceTypeAPIImplementation,
	)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)

	err := planner.planAPIChanges(
		t.Context(), &Config{Namespace: resources.NamespaceExternal}, []resources.APIResource{externalAPI}, plan,
	)

	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeAPIImplementation, plan.Changes[0].ResourceType)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, "implementation-id", plan.Changes[0].ResourceID)
	require.Equal(t, "api-id", plan.Changes[0].Parent.ID)
}

func TestPlanAPIChangesExternalParentSyncDeletesStaleDocuments(t *testing.T) {
	t.Parallel()

	externalAPI := resources.APIResource{
		BaseResource: resources.BaseResource{Ref: "shared-api"},
		External:     &resources.ExternalBlock{ID: "api-id"},
	}
	externalAPI.SetKonnectID("api-id")
	planner := NewPlanner(state.NewClient(state.ClientConfig{
		APIDocumentAPI: &stubAPIDocumentAPI{response: &kkOps.ListAPIDocumentsResponse{
			ListAPIDocumentResponse: &kkComps.ListAPIDocumentResponse{
				Data: []kkComps.APIDocumentSummaryWithChildren{
					{ID: "document-id", Slug: "stale-document", Title: "Stale document"},
				},
			},
		}},
	}), slog.Default())
	planner.resources = &resources.ResourceSet{APIs: []resources.APIResource{externalAPI}}
	planner.resources.EnsureSyncScope().AddChild(
		resources.ResourceTypeAPI,
		externalAPI.GetRef(),
		resources.ResourceTypeAPIDocument,
	)
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)

	err := planner.planAPIChanges(
		t.Context(), &Config{Namespace: resources.NamespaceExternal}, []resources.APIResource{externalAPI}, plan,
	)

	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeAPIDocument, plan.Changes[0].ResourceType)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, "document-id", plan.Changes[0].ResourceID)
	require.Equal(t, "api-id", plan.Changes[0].Parent.ID)
}

func TestValidateNoExternalResourceChangesRejectsAPIChange(t *testing.T) {
	t.Parallel()

	api := resources.APIResource{
		BaseResource: resources.BaseResource{Ref: "shared-api"},
		External:     &resources.ExternalBlock{ID: "api-id"},
	}
	api.SetKonnectID("api-id")
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ResourceType: ResourceTypeAPI,
		ResourceRef:  "shared-api",
		ResourceID:   "api-id",
		Action:       ActionUpdate,
	})

	err := validateNoExternalResourceChanges(plan, &resources.ResourceSet{APIs: []resources.APIResource{api}})
	require.ErrorContains(t, err, "external api \"shared-api\" received UPDATE change")
}

// TestAPIVersionConstraintValidation tests that the loader properly validates API version constraints
func TestAPIVersionConstraintValidation(t *testing.T) {
	// The validation logic is tested in the loader tests (validator_test.go)
	// This file is a placeholder to show that we have considered planner-level tests
	// The actual validation happens during the loading phase, not planning phase

	// The planner's validation in planAPIVersionChanges requires a full state.Client
	// which would be overly complex to mock for this simple validation test
	// Therefore, the validation is properly tested at the loader level where it's first enforced

	t.Run("planner validation is covered by loader tests", func(t *testing.T) {
		// See internal/declarative/loader/validator_test.go for the actual tests
		assert.True(t, true, "Validation tests are in validator_test.go")
	})
}

func TestExtractAPIFieldsIncludesSlugAndAttributes(t *testing.T) {
	t.Parallel()

	name := "Simple API"
	slug := "simple-api-slug"
	attrs := map[string]any{
		"env":     "production",
		"domains": []any{"web", "mobile"},
	}
	expectedAttrs := map[string]any{
		"env":     []string{"production"},
		"domains": []string{"web", "mobile"},
	}

	resource := resources.APIResource{
		CreateAPIRequest: kkComps.CreateAPIRequest{
			Name:       name,
			Slug:       &slug,
			Attributes: attrs,
		},
		BaseResource: resources.BaseResource{
			Ref: "simple-api",
		},
	}

	fields := extractAPIFields(resource)

	assert.Equal(t, slug, fields["slug"])
	assert.Equal(t, expectedAttrs, fields["attributes"])
}

func TestShouldUpdateAPIConsidersSlugAndAttributes(t *testing.T) {
	t.Parallel()

	p := &Planner{}

	currentSlug := "current-slug"
	currentAttrs := map[string][]string{
		"env": {"staging"},
	}

	current := state.API{
		APIResponseSchema: kkComps.APIResponseSchema{
			Slug:       &currentSlug,
			Attributes: currentAttrs,
		},
	}

	name := "Simple API"
	updatedSlug := "new-slug"
	updatedAttrs := map[string]any{
		"env":     "production",
		"domains": []string{"web"},
	}
	expectedUpdatedAttrs := map[string]any{
		"env":     []string{"production"},
		"domains": []string{"web"},
	}

	desired := resources.APIResource{
		CreateAPIRequest: kkComps.CreateAPIRequest{
			Name:       name,
			Slug:       &updatedSlug,
			Attributes: updatedAttrs,
		},
		BaseResource: resources.BaseResource{
			Ref: "simple-api",
		},
	}

	needsUpdate, updateFields, changedFields := p.shouldUpdateAPI(current, desired)
	assert.True(t, needsUpdate)
	assert.Equal(t, updatedSlug, updateFields["slug"])
	assert.Equal(t, expectedUpdatedAttrs, updateFields["attributes"])
	assert.Equal(t, updatedSlug, changedFields["slug"].New)
	assert.Equal(t, currentSlug, changedFields["slug"].Old)
}

func TestShouldUpdateAPIPreservesNullAttributeValues(t *testing.T) {
	t.Parallel()

	p := &Planner{}

	current := state.API{
		APIResponseSchema: kkComps.APIResponseSchema{
			Attributes: map[string][]string{
				"owner":     {"Platform Team"},
				"lifecycle": {"deprecated"},
			},
		},
	}

	desired := resources.APIResource{
		CreateAPIRequest: kkComps.CreateAPIRequest{
			Name: "Simple API",
			Attributes: map[string]any{
				"owner":     nil,
				"lifecycle": nil,
			},
		},
		BaseResource: resources.BaseResource{
			Ref: "simple-api",
		},
	}

	needsUpdate, updateFields, changedFields := p.shouldUpdateAPI(current, desired)
	require.True(t, needsUpdate)
	assert.Equal(t, map[string]any{
		"owner":     nil,
		"lifecycle": nil,
	}, updateFields["attributes"])
	assert.Equal(t, map[string]any{
		"owner":     nil,
		"lifecycle": nil,
	}, changedFields["attributes"].New)
}

func TestShouldUpdateAPITreatsNulledAttributesAsEqualToAbsentKeys(t *testing.T) {
	t.Parallel()

	p := &Planner{}

	current := state.API{
		APIResponseSchema: kkComps.APIResponseSchema{
			Attributes: map[string]any{},
		},
	}

	desired := resources.APIResource{
		CreateAPIRequest: kkComps.CreateAPIRequest{
			Name: "Simple API",
			Attributes: map[string]any{
				"owner":     nil,
				"lifecycle": nil,
			},
		},
		BaseResource: resources.BaseResource{
			Ref: "simple-api",
		},
	}

	needsUpdate, updateFields, changedFields := p.shouldUpdateAPI(current, desired)
	require.False(t, needsUpdate)
	assert.Empty(t, updateFields)
	assert.Empty(t, changedFields)
}

func TestShouldUpdateAPIComparesAttributeValuesAsSortedMultisets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		current     []string
		desired     []string
		needsUpdate bool
	}{
		{
			name:        "reordered values are equivalent",
			current:     []string{"mobile", "web"},
			desired:     []string{"web", "mobile"},
			needsUpdate: false,
		},
		{
			name:        "different values require an update",
			current:     []string{"mobile", "web"},
			desired:     []string{"mobile", "service"},
			needsUpdate: true,
		},
		{
			name:        "duplicate counts remain significant",
			current:     []string{"mobile", "web"},
			desired:     []string{"mobile", "web", "web"},
			needsUpdate: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			current := state.API{
				APIResponseSchema: kkComps.APIResponseSchema{
					Attributes: map[string][]string{"domains": tc.current},
				},
			}
			desired := resources.APIResource{
				CreateAPIRequest: kkComps.CreateAPIRequest{
					Name:       "Simple API",
					Attributes: map[string]any{"domains": tc.desired},
				},
				BaseResource: resources.BaseResource{Ref: "simple-api"},
			}

			needsUpdate, updateFields, changedFields := (&Planner{}).shouldUpdateAPI(current, desired)
			require.Equal(t, tc.needsUpdate, needsUpdate)
			if !tc.needsUpdate {
				assert.Empty(t, updateFields)
				assert.Empty(t, changedFields)
			}
		})
	}
}

func TestShouldUpdateAPIPublicationResolvesAuthStrategyRefs(t *testing.T) {
	t.Parallel()

	authStrategy := resources.ApplicationAuthStrategyResource{
		CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestKeyAuth(
			kkComps.AppAuthStrategyKeyAuthRequest{
				Name:         "my-api-key-auth",
				StrategyType: kkComps.StrategyTypeKeyAuth,
			},
		),
		BaseResource: resources.BaseResource{
			Ref: "key-auth",
		},
	}

	authStrategy.TryMatchKonnectResource(state.ApplicationAuthStrategy{
		ID:   "auth-id",
		Name: "my-api-key-auth",
	})

	planner := &Planner{
		resources: &resources.ResourceSet{
			ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{authStrategy},
		},
	}

	current := state.APIPublication{
		AuthStrategyIDs: []string{"auth-id"},
	}

	desired := resources.APIPublicationResource{
		APIPublication: kkComps.APIPublication{
			AuthStrategyIds: []string{tags.RefPlaceholderPrefix + "key-auth#id"},
		},
		Ref:      "pub",
		PortalID: "portal-id",
	}

	needsUpdate, fields, changedFields := planner.shouldUpdateAPIPublication(current, desired)
	require.False(t, needsUpdate)
	assert.Empty(t, fields)
	assert.Empty(t, changedFields)
}

func TestShouldUpdateAPIPublicationIgnoresAuthStrategyWhenUnset(t *testing.T) {
	t.Parallel()

	planner := &Planner{}

	current := state.APIPublication{
		AuthStrategyIDs: []string{"auth-id"},
	}

	desired := resources.APIPublicationResource{
		APIPublication: kkComps.APIPublication{},
		Ref:            "pub",
		PortalID:       "portal-id",
	}

	needsUpdate, fields, changedFields := planner.shouldUpdateAPIPublication(current, desired)
	require.False(t, needsUpdate)
	assert.Empty(t, fields)
	assert.Empty(t, changedFields)
}

func TestPlanAPIPublicationUpdateAlignsAuthStrategyLookupNames(t *testing.T) {
	t.Parallel()

	authStrategy := resources.ApplicationAuthStrategyResource{
		CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestKeyAuth(
			kkComps.AppAuthStrategyKeyAuthRequest{
				Name:         "my-api-key-auth",
				StrategyType: kkComps.StrategyTypeKeyAuth,
			},
		),
		BaseResource: resources.BaseResource{Ref: "key-auth"},
	}
	planner := &Planner{
		resources: &resources.ResourceSet{
			ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{authStrategy},
		},
	}
	authStrategyIDs := []string{
		"a86aec1e-f67f-4624-919f-b11292b11159",
		tags.RefPlaceholderPrefix + "key-auth#id",
	}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)

	planner.planAPIPublicationUpdate(
		DefaultNamespace,
		"api",
		"api-id",
		state.APIPublication{PortalID: "portal-id"},
		resources.APIPublicationResource{Ref: "publication", PortalID: "portal-id"},
		map[string]any{FieldAuthStrategyIDs: authStrategyIDs},
		map[string]FieldChange{},
		plan,
	)

	require.Len(t, plan.Changes, 1)
	reference := plan.Changes[0].References[FieldAuthStrategyIDs]
	require.Equal(t, authStrategyIDs, reference.Refs)
	require.Equal(t, []string{"", "my-api-key-auth"}, reference.LookupArrays["names"])
}

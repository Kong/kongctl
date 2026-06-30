package planner

import (
	"context"
	"log/slog"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type stubPortalCustomDomainAPI struct {
	getFn func(ctx context.Context, portalID string, opts ...kkOps.Option) (*kkOps.GetPortalCustomDomainResponse, error)
}

type stubPortalPageAPI struct {
	listData []kkComps.PortalPageInfo
	getData  map[string]kkComps.PortalPageResponse
}

func (s *stubPortalPageAPI) CreatePortalPage(
	_ context.Context,
	_ string,
	_ kkComps.CreatePortalPageRequest,
	_ ...kkOps.Option,
) (*kkOps.CreatePortalPageResponse, error) {
	return nil, nil
}

func (s *stubPortalPageAPI) UpdatePortalPage(
	_ context.Context,
	_ kkOps.UpdatePortalPageRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdatePortalPageResponse, error) {
	return nil, nil
}

func (s *stubPortalPageAPI) DeletePortalPage(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeletePortalPageResponse, error) {
	return nil, nil
}

func (s *stubPortalPageAPI) ListPortalPages(
	_ context.Context,
	_ kkOps.ListPortalPagesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListPortalPagesResponse, error) {
	data := s.listData
	if data == nil {
		data = []kkComps.PortalPageInfo{}
	}
	return &kkOps.ListPortalPagesResponse{
		StatusCode: 200,
		ListPortalPagesResponse: &kkComps.ListPortalPagesResponse{
			Data: data,
		},
	}, nil
}

func (s *stubPortalPageAPI) GetPortalPage(
	_ context.Context,
	_ string,
	pageID string,
	_ ...kkOps.Option,
) (*kkOps.GetPortalPageResponse, error) {
	page, ok := s.getData[pageID]
	if !ok {
		return &kkOps.GetPortalPageResponse{StatusCode: 404}, nil
	}
	return &kkOps.GetPortalPageResponse{
		StatusCode:         200,
		PortalPageResponse: &page,
	}, nil
}

func (s *stubPortalCustomDomainAPI) CreatePortalCustomDomain(
	_ context.Context,
	_ string,
	_ kkComps.CreatePortalCustomDomainRequest,
	_ ...kkOps.Option,
) (*kkOps.CreatePortalCustomDomainResponse, error) {
	return nil, nil
}

func (s *stubPortalCustomDomainAPI) UpdatePortalCustomDomain(
	_ context.Context,
	_ string,
	_ kkComps.UpdatePortalCustomDomainRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdatePortalCustomDomainResponse, error) {
	return nil, nil
}

func (s *stubPortalCustomDomainAPI) DeletePortalCustomDomain(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeletePortalCustomDomainResponse, error) {
	return nil, nil
}

func (s *stubPortalCustomDomainAPI) GetPortalCustomDomain(
	ctx context.Context,
	portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalCustomDomainResponse, error) {
	if s.getFn != nil {
		return s.getFn(ctx, portalID, opts...)
	}
	return nil, nil
}

func TestGeneratePlan_PortalCustomDomain(t *testing.T) {
	ctx := context.Background()
	mockPortalAPI := new(MockPortalAPI)
	mockAPIAPI := new(MockAPIAPI)
	mockAppAuthAPI := new(MockAppAuthStrategiesAPI)
	client := state.NewClient(state.ClientConfig{
		PortalAPI:  mockPortalAPI,
		APIAPI:     mockAPIAPI,
		AppAuthAPI: mockAppAuthAPI,
	})
	planner := NewPlanner(client, slog.Default())

	// Mock empty responses for existing resources
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: 0},
			},
		},
	}, nil)

	mockAppAuthAPI.On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
				Meta: kkComps.PaginatedMeta{
					Page: kkComps.PageMeta{Total: 0},
				},
			},
		}, nil)

	// Mock empty APIs list (needed for sync mode)
	mockAPIAPI.On("ListApis", mock.Anything, mock.Anything).Return(&kkOps.ListApisResponse{
		StatusCode: 200,
		ListAPIResponse: &kkComps.ListAPIResponse{
			Data: []kkComps.APIResponseSchema{},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: 0},
			},
		},
	}, nil)

	// Create test resources with portal custom domain
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{
					Name: "dev-portal",
				},
				BaseResource: resources.BaseResource{
					Ref: "dev-portal",
				},
			},
		},
		PortalCustomDomains: []resources.PortalCustomDomainResource{
			{
				Ref:    "portal-custom-domain",
				Portal: "dev-portal",
				CreatePortalCustomDomainRequest: kkComps.CreatePortalCustomDomainRequest{
					Hostname: "developer.example.com",
					Enabled:  true,
					Ssl:      kkComps.CreateCreatePortalCustomDomainSSLHTTP(kkComps.HTTP{}),
				},
			},
		},
	}

	opts := Options{Mode: PlanModeApply}
	plan, err := planner.GeneratePlan(ctx, rs, opts)
	assert.NoError(t, err)
	assert.NotNil(t, plan)

	// Find the portal custom domain change
	var customDomainChange *PlannedChange
	for i := range plan.Changes {
		if plan.Changes[i].ResourceType == "portal_custom_domain" {
			customDomainChange = &plan.Changes[i]
			break
		}
	}

	assert.NotNil(t, customDomainChange, "Should have a portal custom domain change")
	assert.Equal(t, ActionCreate, customDomainChange.Action)
	assert.Equal(t, "portal-custom-domain", customDomainChange.ResourceRef)

	// Verify fields
	assert.Equal(t, "developer.example.com", customDomainChange.Fields["hostname"])
	assert.Equal(t, true, customDomainChange.Fields["enabled"])

	// Verify SSL configuration
	ssl, ok := customDomainChange.Fields["ssl"].(map[string]any)
	assert.True(t, ok, "SSL should be a map")
	assert.Equal(t, "http", ssl["domain_verification_method"])

	// Verify dependencies
	assert.Contains(t, customDomainChange.DependsOn, "1:c:portal:dev-portal")
}

func TestFindPortalTeamGroupMappingDependenciesIncludesAuthSettings(t *testing.T) {
	plan := &Plan{Changes: []PlannedChange{
		{ID: "portal", ResourceType: ResourceTypePortal, ResourceRef: "portal-1"},
		{ID: "team", ResourceType: ResourceTypePortalTeam, ResourceRef: "team-1"},
		{
			ID:           "provider",
			ResourceType: ResourceTypePortalIdentityProvider,
			ResourceRef:  "oidc",
			Fields: map[string]any{
				FieldEnabled: true,
				FieldType:    string(kkComps.IdentityProviderTypeOidc),
			},
			References: map[string]ReferenceInfo{
				FieldPortalID: {Ref: "portal-1"},
			},
		},
		{
			ID:           "auth-settings",
			ResourceType: ResourceTypePortalAuthSettings,
			ResourceRef:  "auth",
			References: map[string]ReferenceInfo{
				FieldPortalID: {Ref: "portal-1"},
			},
		},
	}}

	dependencies := findPortalTeamGroupMappingDependencies(plan, "portal-1", "team-1")

	assert.ElementsMatch(t, []string{"portal", "team", "provider", "auth-settings"}, dependencies)
}

func TestFindEnabledPortalIdentityProviderDependencies(t *testing.T) {
	plan := &Plan{Changes: []PlannedChange{
		{
			ID:           "enabled-oidc",
			ResourceType: ResourceTypePortalIdentityProvider,
			Fields: map[string]any{
				FieldEnabled: true,
				FieldType:    string(kkComps.IdentityProviderTypeOidc),
			},
			References: map[string]ReferenceInfo{
				FieldPortalID: {Ref: "portal-1"},
			},
		},
		{
			ID:           "disabled-saml",
			ResourceType: ResourceTypePortalIdentityProvider,
			Fields: map[string]any{
				FieldEnabled: false,
				FieldType:    string(kkComps.IdentityProviderTypeSaml),
			},
			References: map[string]ReferenceInfo{
				FieldPortalID: {Ref: "portal-1"},
			},
		},
		{
			ID:           "enabled-other-portal",
			ResourceType: ResourceTypePortalIdentityProvider,
			Fields: map[string]any{
				FieldEnabled: true,
				FieldType:    string(kkComps.IdentityProviderTypeOidc),
			},
			References: map[string]ReferenceInfo{
				FieldPortalID: {Ref: "portal-2"},
			},
		},
	}}

	dependencies := findEnabledPortalIdentityProviderDependencies(plan, "portal-1")

	assert.Equal(t, []string{"enabled-oidc"}, dependencies)
}

func TestPlanPortalTeamGroupMappingUpdatePreservesEmptyGroups(t *testing.T) {
	planner := NewPlanner(nil, slog.Default())
	plan := &Plan{}
	mapping := resources.PortalTeamGroupMappingResource{
		Ref:    "developers-groups",
		Portal: "portal-1",
		Team:   "developers",
		Groups: []string{},
	}

	planner.planPortalTeamGroupMappingUpdate(
		context.Background(),
		"default",
		"",
		"portal-1",
		"team-id",
		"Developers",
		mapping,
		map[string]FieldChange{FieldGroups: {Old: []string{"old"}, New: []string{}}},
		plan,
	)

	require.Len(t, plan.Changes, 1)
	require.IsType(t, []string{}, plan.Changes[0].Fields[FieldGroups])
	assert.Empty(t, plan.Changes[0].Fields[FieldGroups])
	assert.NotNil(t, plan.Changes[0].Fields[FieldGroups])
	assert.Equal(t, []string{}, plan.Changes[0].ChangedFields[FieldGroups].New)
}

func TestPlanPortalPagesChanges_PlansContentOnlyUpdate(t *testing.T) {
	t.Parallel()

	title := "Getting Started"
	description := "A quick-start guide for new users"
	currentContent := "# Getting Started\n\nFollow this guide to get up and running quickly with our platform."
	desiredContent := `---
title: "Getting Started"
description: "A quick-start guide for new users"
---

# Getting Started

This body changed without changing page metadata.
Issue 1210 content-only update.
`
	desiredVisibility := kkComps.PageVisibilityStatusPublic
	desiredStatus := kkComps.PublishedStatusPublished

	pageAPI := &stubPortalPageAPI{
		listData: []kkComps.PortalPageInfo{
			{
				ID:          "page-getting-started",
				Slug:        "getting-started",
				Title:       title,
				Description: &description,
				Visibility:  kkComps.VisibilityStatusPublic,
				Status:      kkComps.PublishedStatusPublished,
			},
		},
		getData: map[string]kkComps.PortalPageResponse{
			"page-getting-started": {
				ID:          "page-getting-started",
				Slug:        "getting-started",
				Title:       title,
				Content:     currentContent,
				Description: &description,
				Visibility:  kkComps.VisibilityStatusPublic,
				Status:      kkComps.PublishedStatusPublished,
			},
		},
	}

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{PortalPageAPI: pageAPI}),
		logger: slog.Default(),
		desiredPortals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{Name: "pages-portal"},
				BaseResource: resources.BaseResource{
					Ref: "pages-portal",
				},
			},
		},
	}
	plan := NewPlan("1.0", "test", PlanModeApply)
	desired := []resources.PortalPageResource{
		{
			Ref:    "getting-started-page",
			Portal: "pages-portal",
			CreatePortalPageRequest: kkComps.CreatePortalPageRequest{
				Slug:        "getting-started",
				Title:       &title,
				Description: &description,
				Content:     desiredContent,
				Visibility:  &desiredVisibility,
				Status:      &desiredStatus,
			},
		},
	}

	err := planner.planPortalPagesChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"pages-portal",
		desired,
		plan,
	)
	require.NoError(t, err)

	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	assert.Equal(t, ActionUpdate, change.Action)
	assert.Equal(t, ResourceTypePortalPage, change.ResourceType)
	assert.Equal(t, "getting-started-page", change.ResourceRef)
	assert.Equal(t, "page-getting-started", change.ResourceID)
	assert.Equal(t, "getting-started", change.Fields[FieldSlug])
	assert.Equal(t, desiredContent, change.Fields[FieldContent])
	assert.NotContains(t, change.Fields, FieldTitle)
	assert.NotContains(t, change.Fields, FieldVisibility)
	assert.NotContains(t, change.Fields, FieldStatus)
	assert.Equal(t, currentContent, change.ChangedFields[FieldContent].Old)
	assert.Equal(t, desiredContent, change.ChangedFields[FieldContent].New)
}

func TestShouldUpdatePortalCustomizationDetectsSpecRendererAndRobots(t *testing.T) {
	planner := NewPlanner(nil, slog.Default())
	boolPtr := func(v bool) *bool { return &v }
	stringPtr := func(v string) *string { return &v }

	current := &kkComps.PortalCustomizationV3{
		SpecRenderer: &kkComps.SpecRenderer{
			TryItUI:               boolPtr(true),
			TryItInsomnia:         boolPtr(true),
			InfiniteScroll:        boolPtr(true),
			ShowSchemas:           boolPtr(true),
			HideInternal:          boolPtr(false),
			HideDeprecated:        boolPtr(false),
			AllowCustomServerUrls: boolPtr(true),
		},
		Robots: stringPtr("User-agent: *"),
	}
	desired := resources.PortalCustomizationResource{
		PortalCustomizationV3: kkComps.PortalCustomizationV3{
			SpecRenderer: &kkComps.SpecRenderer{
				TryItUI:               boolPtr(false),
				TryItInsomnia:         boolPtr(false),
				InfiniteScroll:        boolPtr(false),
				ShowSchemas:           boolPtr(false),
				HideInternal:          boolPtr(true),
				HideDeprecated:        boolPtr(true),
				AllowCustomServerUrls: boolPtr(false),
			},
			Robots: stringPtr("User-agent: *\nDisallow: /internal"),
		},
	}

	needsUpdate, updates, changedFields := planner.shouldUpdatePortalCustomization(current, desired)

	require.True(t, needsUpdate)
	require.Contains(t, updates, FieldSpecRenderer)
	specRenderer, ok := updates[FieldSpecRenderer].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, specRenderer[FieldTryItUI])
	assert.Equal(t, false, specRenderer[FieldTryItInsomnia])
	assert.Equal(t, false, specRenderer[FieldInfiniteScroll])
	assert.Equal(t, false, specRenderer[FieldShowSchemas])
	assert.Equal(t, true, specRenderer[FieldHideInternal])
	assert.Equal(t, true, specRenderer[FieldHideDeprecated])
	assert.Equal(t, false, specRenderer[FieldAllowCustomServerURLs])
	assert.Equal(t, "User-agent: *\nDisallow: /internal", updates[FieldRobots])
	assert.Contains(t, changedFields, FieldSpecRenderer)
	assert.Contains(t, changedFields, FieldRobots)
}

func TestShouldUpdatePortalCustomizationIgnoresMatchingSpecRendererAndRobots(t *testing.T) {
	planner := NewPlanner(nil, slog.Default())
	boolPtr := func(v bool) *bool { return &v }
	stringPtr := func(v string) *string { return &v }

	current := &kkComps.PortalCustomizationV3{
		SpecRenderer: &kkComps.SpecRenderer{
			TryItUI:               boolPtr(true),
			TryItInsomnia:         boolPtr(false),
			InfiniteScroll:        boolPtr(true),
			ShowSchemas:           boolPtr(false),
			HideInternal:          boolPtr(true),
			HideDeprecated:        boolPtr(false),
			AllowCustomServerUrls: boolPtr(true),
		},
		Robots: stringPtr("User-agent: *"),
	}
	desired := resources.PortalCustomizationResource{
		PortalCustomizationV3: kkComps.PortalCustomizationV3{
			SpecRenderer: &kkComps.SpecRenderer{
				TryItUI:               boolPtr(true),
				TryItInsomnia:         boolPtr(false),
				InfiniteScroll:        boolPtr(true),
				ShowSchemas:           boolPtr(false),
				HideInternal:          boolPtr(true),
				HideDeprecated:        boolPtr(false),
				AllowCustomServerUrls: boolPtr(true),
			},
			Robots: stringPtr("User-agent: *"),
		},
	}

	needsUpdate, updates, changedFields := planner.shouldUpdatePortalCustomization(current, desired)

	require.False(t, needsUpdate)
	assert.Empty(t, updates)
	assert.Empty(t, changedFields)
}

func TestBuildAllCustomizationFieldsIncludesSpecRendererAndRobots(t *testing.T) {
	planner := NewPlanner(nil, slog.Default())
	boolPtr := func(v bool) *bool { return &v }
	stringPtr := func(v string) *string { return &v }

	fields := planner.buildAllCustomizationFields(resources.PortalCustomizationResource{
		PortalCustomizationV3: kkComps.PortalCustomizationV3{
			SpecRenderer: &kkComps.SpecRenderer{
				TryItUI:               boolPtr(false),
				TryItInsomnia:         boolPtr(true),
				InfiniteScroll:        boolPtr(false),
				ShowSchemas:           boolPtr(true),
				HideInternal:          boolPtr(false),
				HideDeprecated:        boolPtr(true),
				AllowCustomServerUrls: boolPtr(false),
			},
			Robots: stringPtr("User-agent: *"),
		},
	})

	specRenderer, ok := fields[FieldSpecRenderer].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, specRenderer[FieldTryItUI])
	assert.Equal(t, true, specRenderer[FieldTryItInsomnia])
	assert.Equal(t, false, specRenderer[FieldInfiniteScroll])
	assert.Equal(t, true, specRenderer[FieldShowSchemas])
	assert.Equal(t, false, specRenderer[FieldHideInternal])
	assert.Equal(t, true, specRenderer[FieldHideDeprecated])
	assert.Equal(t, false, specRenderer[FieldAllowCustomServerURLs])
	assert.Equal(t, "User-agent: *", fields[FieldRobots])
}

func TestPlanPortalTeamGroupMappingsSkipsUnconfiguredPortalAuthSettingsAPI(t *testing.T) {
	client := state.NewClient(state.ClientConfig{})
	planner := NewPlanner(client, slog.Default())
	planner.resourceCache.portalTeamsByPortalID["portal-id"] = []state.PortalTeam{
		{ID: "team-id", Name: "Developers"},
	}
	plan := &Plan{}

	err := planner.planPortalTeamGroupMappingsChanges(
		context.Background(),
		"default",
		"portal-id",
		"portal-1",
		[]resources.PortalTeamGroupMappingResource{{
			Ref:    "developers-groups",
			Portal: "portal-1",
			Team:   "Developers",
			Groups: []string{"Developers"},
		}},
		plan,
	)

	require.NoError(t, err)
	assert.Empty(t, plan.Changes)
}

func buildPortalCustomDomain(
	host string,
	enabled bool,
) *kkComps.PortalCustomDomain {
	now := time.Now()
	return &kkComps.PortalCustomDomain{
		Hostname: host,
		Enabled:  enabled,
		Ssl: kkComps.PortalCustomDomainSSL{
			DomainVerificationMethod: kkComps.PortalCustomDomainVerificationMethodHTTP,
			VerificationStatus:       kkComps.PortalCustomDomainVerificationStatusPending,
		},
		CnameStatus: kkComps.PortalCustomDomainCnameStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestPlanPortalCustomDomain_NoChangeWhenStateMatches(t *testing.T) {
	t.Parallel()

	stub := &stubPortalCustomDomainAPI{
		getFn: func(
			_ context.Context,
			_ string,
			_ ...kkOps.Option,
		) (*kkOps.GetPortalCustomDomainResponse, error) {
			return &kkOps.GetPortalCustomDomainResponse{
				StatusCode: 200,
				PortalCustomDomain: buildPortalCustomDomain(
					"developer.example.com",
					true,
				),
			}, nil
		},
	}

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			PortalCustomDomainAPI: stub,
		}),
		logger: slog.Default(),
		desiredPortals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{Name: "portal"},
				BaseResource: resources.BaseResource{
					Ref: "portal-1",
				},
			},
		},
	}

	plan := NewPlan("1.0", "test", PlanModeApply)
	desired := []resources.PortalCustomDomainResource{
		{
			Ref:    "domain-1",
			Portal: "portal-1",
			CreatePortalCustomDomainRequest: kkComps.CreatePortalCustomDomainRequest{
				Hostname: "developer.example.com",
				Enabled:  true,
				Ssl:      kkComps.CreateCreatePortalCustomDomainSSLHTTP(kkComps.HTTP{}),
			},
		},
	}

	err := planner.planPortalCustomDomainsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-1",
		desired,
		plan,
	)
	assert.NoError(t, err)
	assert.Empty(t, plan.Changes)
}

func TestPlanPortalCustomDomain_UpdateEnabled(t *testing.T) {
	t.Parallel()

	stub := &stubPortalCustomDomainAPI{
		getFn: func(
			_ context.Context,
			_ string,
			_ ...kkOps.Option,
		) (*kkOps.GetPortalCustomDomainResponse, error) {
			return &kkOps.GetPortalCustomDomainResponse{
				StatusCode: 200,
				PortalCustomDomain: buildPortalCustomDomain(
					"developer.example.com",
					false,
				),
			}, nil
		},
	}

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			PortalCustomDomainAPI: stub,
		}),
		logger: slog.Default(),
		desiredPortals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{Name: "portal"},
				BaseResource: resources.BaseResource{
					Ref: "portal-1",
				},
			},
		},
	}

	plan := NewPlan("1.0", "test", PlanModeApply)
	desired := []resources.PortalCustomDomainResource{
		{
			Ref:    "domain-1",
			Portal: "portal-1",
			CreatePortalCustomDomainRequest: kkComps.CreatePortalCustomDomainRequest{
				Hostname: "developer.example.com",
				Enabled:  true,
				Ssl:      kkComps.CreateCreatePortalCustomDomainSSLHTTP(kkComps.HTTP{}),
			},
		},
	}

	err := planner.planPortalCustomDomainsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-1",
		desired,
		plan,
	)
	assert.NoError(t, err)
	assert.Len(t, plan.Changes, 1)

	change := plan.Changes[0]
	assert.Equal(t, ActionUpdate, change.Action)
	assert.Equal(t, "domain-1", change.ResourceRef)
	assert.Equal(t, "portal-id", change.ResourceID)
	assert.Equal(t, true, change.Fields["enabled"])
	if assert.NotNil(t, change.Parent) {
		assert.Equal(t, "portal-1", change.Parent.Ref)
		assert.Equal(t, "portal-id", change.Parent.ID)
	}
	if ref, ok := change.References["portal_id"]; assert.True(t, ok) {
		assert.Equal(t, "portal-id", ref.ID)
		assert.Equal(t, "portal-1", ref.Ref)
	}
}

func TestPlanPortalCustomDomain_ReplaceOnHostnameChange(t *testing.T) {
	t.Parallel()

	stub := &stubPortalCustomDomainAPI{
		getFn: func(
			_ context.Context,
			_ string,
			_ ...kkOps.Option,
		) (*kkOps.GetPortalCustomDomainResponse, error) {
			return &kkOps.GetPortalCustomDomainResponse{
				StatusCode: 200,
				PortalCustomDomain: buildPortalCustomDomain(
					"old.example.com",
					true,
				),
			}, nil
		},
	}

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			PortalCustomDomainAPI: stub,
		}),
		logger: slog.Default(),
		desiredPortals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{Name: "portal"},
				BaseResource: resources.BaseResource{
					Ref: "portal-1",
				},
			},
		},
	}

	plan := NewPlan("1.0", "test", PlanModeApply)
	desired := []resources.PortalCustomDomainResource{
		{
			Ref:    "domain-1",
			Portal: "portal-1",
			CreatePortalCustomDomainRequest: kkComps.CreatePortalCustomDomainRequest{
				Hostname: "developer.example.com",
				Enabled:  true,
				Ssl:      kkComps.CreateCreatePortalCustomDomainSSLHTTP(kkComps.HTTP{}),
			},
		},
	}

	err := planner.planPortalCustomDomainsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-1",
		desired,
		plan,
	)
	assert.NoError(t, err)
	assert.Len(t, plan.Changes, 2)

	deleteChange := plan.Changes[0]
	createChange := plan.Changes[1]

	assert.Equal(t, ActionDelete, deleteChange.Action)
	assert.Equal(t, "portal-id", deleteChange.ResourceID)
	assert.Equal(t, "old.example.com", deleteChange.Fields["hostname"])

	assert.Equal(t, ActionCreate, createChange.Action)
	assert.Equal(t, "developer.example.com", createChange.Fields["hostname"])
	assert.Contains(t, createChange.DependsOn, deleteChange.ID)
}

func TestPlanPortalCustomDomain_SyncDeleteWhenOmitted(t *testing.T) {
	t.Parallel()

	stub := &stubPortalCustomDomainAPI{
		getFn: func(
			_ context.Context,
			_ string,
			_ ...kkOps.Option,
		) (*kkOps.GetPortalCustomDomainResponse, error) {
			return &kkOps.GetPortalCustomDomainResponse{
				StatusCode: 200,
				PortalCustomDomain: buildPortalCustomDomain(
					"developer.example.com",
					true,
				),
			}, nil
		},
	}

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			PortalCustomDomainAPI: stub,
		}),
		logger: slog.Default(),
	}

	plan := NewPlan("1.0", "test", PlanModeSync)

	err := planner.planPortalCustomDomainsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-1",
		nil,
		plan,
	)
	assert.NoError(t, err)
	assert.Len(t, plan.Changes, 1)
	assert.Equal(t, ActionDelete, plan.Changes[0].Action)
	assert.Equal(t, "portal-id", plan.Changes[0].ResourceID)
}

func TestPlanPortalCustomDomain_CreateWhenAbsent(t *testing.T) {
	t.Parallel()

	stub := &stubPortalCustomDomainAPI{
		getFn: func(
			_ context.Context,
			_ string,
			_ ...kkOps.Option,
		) (*kkOps.GetPortalCustomDomainResponse, error) {
			return nil, &kkErrors.NotFoundError{}
		},
	}

	planner := &Planner{
		client: state.NewClient(state.ClientConfig{
			PortalCustomDomainAPI: stub,
		}),
		logger: slog.Default(),
		desiredPortals: []resources.PortalResource{
			{
				CreatePortal: kkComps.CreatePortal{Name: "portal"},
				BaseResource: resources.BaseResource{
					Ref: "portal-1",
				},
			},
		},
	}

	plan := NewPlan("1.0", "test", PlanModeApply)
	desired := []resources.PortalCustomDomainResource{
		{
			Ref:    "domain-1",
			Portal: "portal-1",
			CreatePortalCustomDomainRequest: kkComps.CreatePortalCustomDomainRequest{
				Hostname: "developer.example.com",
				Enabled:  true,
				Ssl:      kkComps.CreateCreatePortalCustomDomainSSLHTTP(kkComps.HTTP{}),
			},
		},
	}

	err := planner.planPortalCustomDomainsChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-1",
		desired,
		plan,
	)
	assert.NoError(t, err)
	assert.Len(t, plan.Changes, 1)
	assert.Equal(t, ActionCreate, plan.Changes[0].Action)
	assert.Equal(t, "developer.example.com", plan.Changes[0].Fields["hostname"])
}

type stubPortalIdentityProviderAPI struct {
	listFn func(
		ctx context.Context,
		request kkOps.GetPortalIdentityProvidersRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetPortalIdentityProvidersResponse, error)
}

func (s *stubPortalIdentityProviderAPI) ListPortalIdentityProviders(
	ctx context.Context,
	request kkOps.GetPortalIdentityProvidersRequest,
	opts ...kkOps.Option,
) (*kkOps.GetPortalIdentityProvidersResponse, error) {
	if s.listFn != nil {
		return s.listFn(ctx, request, opts...)
	}
	return &kkOps.GetPortalIdentityProvidersResponse{PortalIdentityProviders: []kkComps.PortalIdentityProvider{}}, nil
}

func (s *stubPortalIdentityProviderAPI) GetPortalIdentityProvider(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.GetPortalIdentityProviderResponse, error) {
	return nil, nil
}

func (s *stubPortalIdentityProviderAPI) CreatePortalIdentityProvider(
	_ context.Context,
	_ string,
	_ kkComps.CreateIdentityProvider,
	_ ...kkOps.Option,
) (*kkOps.CreatePortalIdentityProviderResponse, error) {
	return nil, nil
}

func (s *stubPortalIdentityProviderAPI) UpdatePortalIdentityProvider(
	_ context.Context,
	_ kkOps.UpdatePortalIdentityProviderRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdatePortalIdentityProviderResponse, error) {
	return nil, nil
}

func (s *stubPortalIdentityProviderAPI) DeletePortalIdentityProvider(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeletePortalIdentityProviderResponse, error) {
	return nil, nil
}

func TestPlanPortalIdentityProviders_CreateWhenAbsent(t *testing.T) {
	t.Parallel()

	stub := &stubPortalIdentityProviderAPI{}
	planner := &Planner{
		client: state.NewClient(state.ClientConfig{PortalIdentityProviderAPI: stub}),
		logger: slog.Default(),
		desiredPortals: []resources.PortalResource{{
			CreatePortal: kkComps.CreatePortal{Name: "portal"},
			BaseResource: resources.BaseResource{Ref: "portal-1"},
		}},
	}

	config := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(kkComps.OIDCIdentityProviderConfig{
		IssuerURL: "https://accounts.google.com",
		ClientID:  "client-id-1",
		Scopes:    []string{"openid"},
	})
	desired := []resources.PortalIdentityProviderResource{{
		Ref:    "portal-oidc",
		Portal: "portal-1",
		CreateIdentityProvider: kkComps.CreateIdentityProvider{
			Type:      kkComps.IdentityProviderTypeOidc.ToPointer(),
			LoginPath: new("oidc-login"),
			Config:    &config,
		},
	}}

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := planner.planPortalIdentityProvidersChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-1",
		desired,
		plan,
	)
	assert.NoError(t, err)
	if assert.Len(t, plan.Changes, 1) {
		change := plan.Changes[0]
		assert.Equal(t, ActionCreate, change.Action)
		assert.Equal(t, ResourceTypePortalIdentityProvider, change.ResourceType)
		assert.Equal(t, "portal-oidc", change.ResourceRef)
		assert.Equal(t, "oidc", change.Fields["type"])
		assert.Equal(t, "oidc-login", change.Fields["login_path"])
		if assert.NotNil(t, change.Parent) {
			assert.Equal(t, "portal-1", change.Parent.Ref)
			assert.Equal(t, "portal-id", change.Parent.ID)
		}
	}
}

func TestPlanPortalIdentityProviders_UpdateWhenStateDiffers(t *testing.T) {
	t.Parallel()

	stub := &stubPortalIdentityProviderAPI{
		listFn: func(
			_ context.Context,
			_ kkOps.GetPortalIdentityProvidersRequest,
			_ ...kkOps.Option,
		) (*kkOps.GetPortalIdentityProvidersResponse, error) {
			currentConfig := kkComps.CreatePortalIdentityProviderConfigOIDCIdentityProviderConfigOutput(
				kkComps.OIDCIdentityProviderConfigOutput{
					IssuerURL: "https://accounts.google.com",
					ClientID:  "client-id-old",
					Scopes:    []string{"openid"},
				},
			)
			return &kkOps.GetPortalIdentityProvidersResponse{
				PortalIdentityProviders: []kkComps.PortalIdentityProvider{{
					ID:      new("provider-id"),
					Type:    kkComps.IdentityProviderTypeOidc.ToPointer(),
					Enabled: new(false),
					Config:  &currentConfig,
				}},
			}, nil
		},
	}
	planner := &Planner{
		client: state.NewClient(state.ClientConfig{PortalIdentityProviderAPI: stub}),
		logger: slog.Default(),
		desiredPortals: []resources.PortalResource{{
			CreatePortal: kkComps.CreatePortal{Name: "portal"},
			BaseResource: resources.BaseResource{Ref: "portal-1"},
		}},
	}

	desiredConfig := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(
		kkComps.OIDCIdentityProviderConfig{
			IssuerURL: "https://accounts.google.com",
			ClientID:  "client-id-new",
			Scopes:    []string{"openid", "profile"},
		},
	)
	desired := []resources.PortalIdentityProviderResource{{
		Ref:    "portal-oidc",
		Portal: "portal-1",
		CreateIdentityProvider: kkComps.CreateIdentityProvider{
			Type:      kkComps.IdentityProviderTypeOidc.ToPointer(),
			Enabled:   new(true),
			LoginPath: new("oidc-login-updated"),
			Config:    &desiredConfig,
		},
	}}

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := planner.planPortalIdentityProvidersChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-1",
		desired,
		plan,
	)
	assert.NoError(t, err)
	if assert.Len(t, plan.Changes, 1) {
		change := plan.Changes[0]
		assert.Equal(t, ActionUpdate, change.Action)
		assert.Equal(t, ResourceTypePortalIdentityProvider, change.ResourceType)
		assert.Equal(t, "provider-id", change.ResourceID)
		assert.Equal(t, true, change.Fields["enabled"])
		assert.Equal(t, "oidc-login-updated", change.Fields["login_path"])
		assert.Contains(t, change.ChangedFields, "config")
	}
}

func TestPlanPortalIdentityProviders_IgnoresWriteOnlyClientSecret(t *testing.T) {
	t.Parallel()

	stub := &stubPortalIdentityProviderAPI{
		listFn: func(
			_ context.Context,
			_ kkOps.GetPortalIdentityProvidersRequest,
			_ ...kkOps.Option,
		) (*kkOps.GetPortalIdentityProvidersResponse, error) {
			currentConfig := kkComps.CreatePortalIdentityProviderConfigOIDCIdentityProviderConfigOutput(
				kkComps.OIDCIdentityProviderConfigOutput{
					IssuerURL: "https://accounts.google.com",
					ClientID:  "client-id-1",
					Scopes:    []string{"openid", "profile"},
					ClaimMappings: &kkComps.OIDCIdentityProviderClaimMappings{
						Name:   new("name"),
						Email:  new("email"),
						Groups: new("groups"),
					},
				},
			)
			return &kkOps.GetPortalIdentityProvidersResponse{
				PortalIdentityProviders: []kkComps.PortalIdentityProvider{{
					ID:      new("provider-id"),
					Type:    kkComps.IdentityProviderTypeOidc.ToPointer(),
					Enabled: new(true),
					Config:  &currentConfig,
				}},
			}, nil
		},
	}
	planner := &Planner{
		client: state.NewClient(state.ClientConfig{PortalIdentityProviderAPI: stub}),
		logger: slog.Default(),
		desiredPortals: []resources.PortalResource{{
			CreatePortal: kkComps.CreatePortal{Name: "portal"},
			BaseResource: resources.BaseResource{Ref: "portal-1"},
		}},
	}

	desiredConfig := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(
		kkComps.OIDCIdentityProviderConfig{
			IssuerURL:    "https://accounts.google.com",
			ClientID:     "client-id-1",
			ClientSecret: new("placeholder"),
			Scopes:       []string{"openid", "profile"},
			ClaimMappings: &kkComps.OIDCIdentityProviderClaimMappings{
				Name:   new("name"),
				Email:  new("email"),
				Groups: new("groups"),
			},
		},
	)
	desired := []resources.PortalIdentityProviderResource{{
		Ref:    "portal-oidc",
		Portal: "portal-1",
		CreateIdentityProvider: kkComps.CreateIdentityProvider{
			Type:    kkComps.IdentityProviderTypeOidc.ToPointer(),
			Enabled: new(true),
			Config:  &desiredConfig,
		},
	}}

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := planner.planPortalIdentityProvidersChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-1",
		desired,
		plan,
	)
	assert.NoError(t, err)
	assert.Empty(t, plan.Changes)
}

func TestPlanPortalIdentityProviders_IgnoresOIDCScopeOrderChanges(t *testing.T) {
	t.Parallel()

	stub := &stubPortalIdentityProviderAPI{
		listFn: func(
			_ context.Context,
			_ kkOps.GetPortalIdentityProvidersRequest,
			_ ...kkOps.Option,
		) (*kkOps.GetPortalIdentityProvidersResponse, error) {
			currentConfig := kkComps.CreatePortalIdentityProviderConfigOIDCIdentityProviderConfigOutput(
				kkComps.OIDCIdentityProviderConfigOutput{
					IssuerURL: "https://accounts.google.com",
					ClientID:  "client-id-1",
					Scopes:    []string{"profile", "openid"},
				},
			)
			return &kkOps.GetPortalIdentityProvidersResponse{
				PortalIdentityProviders: []kkComps.PortalIdentityProvider{{
					ID:      new("provider-id"),
					Type:    kkComps.IdentityProviderTypeOidc.ToPointer(),
					Enabled: new(true),
					Config:  &currentConfig,
				}},
			}, nil
		},
	}
	planner := &Planner{
		client: state.NewClient(state.ClientConfig{PortalIdentityProviderAPI: stub}),
		logger: slog.Default(),
		desiredPortals: []resources.PortalResource{{
			CreatePortal: kkComps.CreatePortal{Name: "portal"},
			BaseResource: resources.BaseResource{Ref: "portal-1"},
		}},
	}

	desiredConfig := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(
		kkComps.OIDCIdentityProviderConfig{
			IssuerURL: "https://accounts.google.com",
			ClientID:  "client-id-1",
			Scopes:    []string{"openid", "profile"},
		},
	)
	desired := []resources.PortalIdentityProviderResource{{
		Ref:    "portal-oidc",
		Portal: "portal-1",
		CreateIdentityProvider: kkComps.CreateIdentityProvider{
			Type:    kkComps.IdentityProviderTypeOidc.ToPointer(),
			Enabled: new(true),
			Config:  &desiredConfig,
		},
	}}

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := planner.planPortalIdentityProvidersChanges(
		context.Background(),
		DefaultNamespace,
		"portal-id",
		"portal-1",
		desired,
		plan,
	)
	assert.NoError(t, err)
	assert.Empty(t, plan.Changes)
}

func TestPortalIdentityProviderConfigDiffValueFromCreate_OmitsAbsentClientSecret(t *testing.T) {
	t.Parallel()

	config := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(
		kkComps.OIDCIdentityProviderConfig{
			IssuerURL: "https://accounts.google.com",
			ClientID:  "client-id-1",
			Scopes:    []string{"openid", "profile"},
		},
	)

	diffValue, ok := portalIdentityProviderConfigDiffValueFromCreate(&config).(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, diffValue, "client_secret")
}

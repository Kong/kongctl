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
)

type stubPortalCustomDomainAPI struct {
	getFn func(ctx context.Context, portalID string, opts ...kkOps.Option) (*kkOps.GetPortalCustomDomainResponse, error)
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

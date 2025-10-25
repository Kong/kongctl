package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
					Name: ptrString("dev-portal"),
				},
				Ref: "dev-portal",
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

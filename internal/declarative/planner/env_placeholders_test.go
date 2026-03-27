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
	"github.com/stretchr/testify/require"
)

func TestGeneratePlan_PreservesDeferredEnvPlaceholders(t *testing.T) {
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
	mockEmptyAPIsList(ctx, mockAPIAPI)

	description := "resolved secret"
	authRef := "basic-auth"

	rs := &resources.ResourceSet{
		ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{
			{
				BaseResource: resources.BaseResource{Ref: "basic-auth"},
				CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestKeyAuth(
					kkComps.AppAuthStrategyKeyAuthRequest{
						Name:         "basic-auth",
						DisplayName:  "Basic Auth",
						StrategyType: kkComps.StrategyTypeKeyAuth,
						Configs: kkComps.AppAuthStrategyKeyAuthRequestConfigs{
							KeyAuth: kkComps.AppAuthStrategyConfigKeyAuth{},
						},
					},
				),
			},
		},
		Portals: []resources.PortalResource{
			{
				BaseResource: resources.BaseResource{Ref: "env-portal"},
				CreatePortal: kkComps.CreatePortal{
					Name:                             "env-portal",
					Description:                      &description,
					DefaultApplicationAuthStrategyID: &authRef,
					Labels:                           map[string]*string{},
				},
			},
		},
		EnvSources: map[string]map[string]string{
			"env-portal": {
				"/description":                          "__ENV__:PORTAL_DESCRIPTION",
				"/default_application_auth_strategy_id": "__ENV__:PORTAL_AUTH_STRATEGY",
			},
		},
	}

	plan, err := planner.GeneratePlan(ctx, rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)

	var portalChange *PlannedChange
	for i := range plan.Changes {
		if plan.Changes[i].ResourceType == "portal" {
			portalChange = &plan.Changes[i]
			break
		}
	}
	require.NotNil(t, portalChange)

	assert.Equal(t, "__ENV__:PORTAL_DESCRIPTION", portalChange.Fields["description"])
	require.NotNil(t, portalChange.References)
	assert.Equal(
		t,
		"__ENV__:PORTAL_AUTH_STRATEGY",
		portalChange.References["default_application_auth_strategy_id"].Ref,
	)
	assert.Empty(t, portalChange.References["default_application_auth_strategy_id"].ID)
	assert.Equal(t, []string{
		"1:c:application_auth_strategy:basic-auth",
		"2:c:portal:env-portal",
	}, plan.ExecutionOrder)

	require.Len(t, plan.Warnings, 1)
	assert.Equal(t, portalChange.ID, plan.Warnings[0].ChangeID)
	assert.Contains(t, plan.Warnings[0].Message, "Contains deferred !env values")

	mockPortalAPI.AssertExpectations(t)
	mockAppAuthAPI.AssertExpectations(t)
	mockAPIAPI.AssertExpectations(t)
}

func TestApplyDeferredEnvPlaceholders_UnionFields(t *testing.T) {
	planner := &Planner{}
	plan := NewPlan("1.0", "test", PlanModeApply)
	plan.Changes = []PlannedChange{
		{
			ID:           "1:c:portal_custom_domain:env-domain",
			ResourceType: "portal_custom_domain",
			ResourceRef:  "env-domain",
			Action:       ActionCreate,
			Fields: map[string]any{
				"hostname": "env.example.com",
				"ssl": map[string]any{
					"domain_verification_method": "custom_certificate",
					"custom_certificate":         "resolved-cert",
					"custom_private_key":         "resolved-key",
					"skip_ca_check":              true,
				},
			},
		},
	}

	rs := &resources.ResourceSet{
		EnvSources: map[string]map[string]string{
			"env-domain": {
				"/ssl/custom_certificate": "__ENV__:CUSTOM_CERT",
				"/ssl/custom_private_key": "__ENV__:CUSTOM_KEY",
			},
		},
	}

	planner.applyDeferredEnvPlaceholders(plan, rs)

	require.Len(t, plan.Changes, 1)
	sslFields, ok := plan.Changes[0].Fields["ssl"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "__ENV__:CUSTOM_CERT", sslFields["custom_certificate"])
	assert.Equal(t, "__ENV__:CUSTOM_KEY", sslFields["custom_private_key"])
	require.Len(t, plan.Warnings, 1)
	assert.Contains(t, plan.Warnings[0].Message, "Contains deferred !env values")
}

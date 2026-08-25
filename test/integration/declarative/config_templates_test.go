//go:build integration

package declarative_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/declarative"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConfigTemplatesExpandAcrossFilesAndResourceLevels(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	templatesPath := filepath.Join(dir, "templates.yaml")
	require.NoError(t, os.WriteFile(templatesPath, []byte(`
_templates:
  standard-portal:
    authentication_enabled: true
    labels:
      managed-by: kongctl
  oidc-config:
    issuer: https://auth.example.com
    auth_methods: [session]
  oidc-policy:
    type: openid-connect
    enabled: true
    global: false
    config:
      _extends: oidc-config
`), 0o600))

	resourcesPath := filepath.Join(dir, "resources.yaml")
	require.NoError(t, os.WriteFile(resourcesPath, []byte(`
portals:
  - _extends: standard-portal
    ref: payments-portal
    name: Payments Developer Portal
    labels:
      business-unit: payments
ai_gateways:
  - ref: shared-gateway
    name: shared-gateway
    display_name: Shared Gateway
    policies:
      - _extends: oidc-policy
        ref: payments-oidc
        name: payments-oidc
        display_name: Payments OIDC
        config:
          auth_methods: [bearer]
          groups_required: [payments-api-users]
`), 0o600))

	rs, err := loader.NewWithBaseDir(dir).LoadFromSources([]loader.Source{
		{Path: templatesPath, Type: loader.SourceTypeFile},
		{Path: resourcesPath, Type: loader.SourceTypeFile},
	}, false)
	require.NoError(t, err)

	require.Len(t, rs.Portals, 1)
	require.NotNil(t, rs.Portals[0].AuthenticationEnabled)
	assert.True(t, *rs.Portals[0].AuthenticationEnabled)
	assert.Equal(t, "kongctl", *rs.Portals[0].Labels["managed-by"])
	assert.Equal(t, "payments", *rs.Portals[0].Labels["business-unit"])

	require.Len(t, rs.AIGatewayPolicies, 1)
	policy := rs.AIGatewayPolicies[0]
	assert.Equal(t, "openid-connect", policy.Type)
	assert.Equal(t, "https://auth.example.com", policy.Config["issuer"])
	assert.Equal(t, []any{"bearer"}, policy.Config["auth_methods"])
	assert.Equal(t, []any{"payments-api-users"}, policy.Config["groups_required"])
}

func TestConfigTemplateChangePlansUpdatesForEveryConsumer(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "portals.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
_templates:
  shared-portal:
    description: Updated shared description
portals:
  - _extends: shared-portal
    ref: payments-portal
    name: Payments Portal
  - _extends: shared-portal
    ref: reporting-portal
    name: Reporting Portal
`), 0o600))

	ctx := SetupTestContext(t)
	sdkFactory := ctx.Value(helpers.SDKAPIFactoryKey).(helpers.SDKAPIFactory)
	konnectSDK, err := sdkFactory(GetTestConfig(), nil)
	require.NoError(t, err)
	mockSDK := konnectSDK.(*helpers.MockKonnectSDK)
	mockPortalAPI := mockSDK.GetPortalAPI().(*MockPortalAPI)
	oldDescription := "Old shared description"
	mockPortalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{
				{
					ID: "payments-id", Name: "Payments Portal", Description: &oldDescription,
					Labels: map[string]string{labels.NamespaceKey: "default"},
				},
				{
					ID: "reporting-id", Name: "Reporting Portal", Description: &oldDescription,
					Labels: map[string]string{labels.NamespaceKey: "default"},
				},
			},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 2}},
		},
	}, nil)
	mockSDK.GetAppAuthStrategiesAPI().(*MockAppAuthStrategiesAPI).
		On("ListAppAuthStrategies", mock.Anything, mock.Anything).
		Return(&kkOps.ListAppAuthStrategiesResponse{
			StatusCode: 200,
			ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
				Data: []kkComps.AppAuthStrategy{},
			},
		}, nil).
		Maybe()
	mockSDK.GetAPIAPI().(*MockAPIAPI).
		On("ListApis", mock.Anything, mock.Anything).
		Return(&kkOps.ListApisResponse{
			StatusCode: 200,
			ListAPIResponse: &kkComps.ListAPIResponse{
				Data: []kkComps.APIResponseSchema{},
				Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 0}},
			},
		}, nil).
		Maybe()

	planCmd, err := declarative.NewDeclarativeCmd("plan")
	require.NoError(t, err)
	planCmd.SetContext(ctx)
	var output bytes.Buffer
	planCmd.SetOut(&output)
	planCmd.SetErr(&output)
	planPath := filepath.Join(dir, "plan.json")
	planCmd.SetArgs([]string{"-f", configPath, "--output-file", planPath})
	require.NoError(t, planCmd.Execute(), output.String())

	planData, err := os.ReadFile(planPath)
	require.NoError(t, err)
	var generatedPlan planner.Plan
	require.NoError(t, json.Unmarshal(planData, &generatedPlan))
	require.Len(t, generatedPlan.Changes, 2)
	for _, change := range generatedPlan.Changes {
		assert.Equal(t, planner.ActionUpdate, change.Action)
		assert.Equal(t, "Updated shared description", change.Fields["description"])
	}
	assert.ElementsMatch(t, []string{"payments-portal", "reporting-portal"}, []string{
		generatedPlan.Changes[0].ResourceRef,
		generatedPlan.Changes[1].ResourceRef,
	})
}

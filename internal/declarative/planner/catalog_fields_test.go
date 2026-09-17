package planner

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestNewCatalogFieldWritePlans(t *testing.T) {
	expression := envSecretExpression("UNAVAILABLE_CATALOG_SECRET")
	placeholder, err := tags.BuildSecretPlaceholder(expression)
	require.NoError(t, err)
	for _, kind := range []resources.ResourceType{
		resources.ResourceTypePortalCustomDomain, resources.ResourceTypeEventGatewayBackendCluster,
	} {
		for _, action := range []ActionType{ActionCreate, ActionUpdate} {
			for _, selected := range []bool{false, true} {
				rs := &resources.ResourceSet{}
				var resource resources.Resource
				var path string
				if kind == resources.ResourceTypePortalCustomDomain {
					domain := resources.PortalCustomDomainResource{
						Ref: "target", Portal: "parent",
						CreatePortalCustomDomainRequest: kkComps.CreatePortalCustomDomainRequest{
							Hostname: "developer.example.com", Enabled: true,
							Ssl: kkComps.CreateCreatePortalCustomDomainSSLCustomCertificate(kkComps.CustomCertificate{
								CustomCertificate: "public-certificate", CustomPrivateKey: placeholder,
							}),
						},
					}
					require.True(t, domain.TryMatchKonnectResource(struct{ Hostname, ID string }{
						"developer.example.com", "parent-id",
					}))
					rs.PortalCustomDomains = []resources.PortalCustomDomainResource{domain}
					resource, path = &rs.PortalCustomDomains[0], "/ssl/custom_private_key"
				} else {
					backend := resources.EventGatewayBackendClusterResource{
						Ref: "target", EventGateway: "parent",
						CreateBackendClusterRequest: kkComps.CreateBackendClusterRequest{
							Name: "backend",
							Authentication: kkComps.CreateBackendClusterAuthenticationSchemeSaslPlain(
								kkComps.BackendClusterAuthenticationSaslPlain{Username: "user", Password: placeholder},
							),
						},
					}
					require.True(t, backend.TryMatchKonnectResource(struct{ Name, ID string }{"backend", "backend-id"}))
					rs.EventGatewayBackendClusters = []resources.EventGatewayBackendClusterResource{backend}
					resource, path = &rs.EventGatewayBackendClusters[0], "/authentication/password"
				}
				rs.AddSecretSource("target", path, expression, false)
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
				if action == ActionCreate {
					fields, err := secretResourceFields(resource, action)
					require.NoError(t, err)
					plan.AddChange(PlannedChange{
						ID: "create", ResourceRef: "target", ResourceType: string(kind),
						Action: action, Fields: fields,
					})
				}
				err := (&Planner{}).applySecretWriteIntents(t.Context(), plan, rs, Options{WriteSecrets: selected})
				require.NoError(t, err)
				if action == ActionUpdate && !selected {
					require.Empty(t, plan.Changes)
					continue
				}
				require.Len(t, plan.Changes, 1)
				require.Equal(t, action, plan.Changes[0].Action)
				require.Len(t, plan.Changes[0].SecretWrites, 1)
				require.Equal(t, path, plan.Changes[0].SecretWrites[0].Field)
				data, err := json.Marshal(plan)
				require.NoError(t, err)
				require.NotContains(t, string(data), placeholder)
			}
		}
	}
}

func TestBackendDeferredPasswordDoesNotCauseDrift(t *testing.T) {
	placeholder, err := tags.BuildSecretPlaceholder(envSecretExpression("PASSWORD"))
	require.NoError(t, err)
	current := kkComps.CreateBackendClusterAuthenticationSensitiveDataAwareSchemeSaslPlain(
		kkComps.BackendClusterAuthenticationSaslPlainSensitiveDataAware{Username: "user"},
	)
	desired := kkComps.CreateBackendClusterAuthenticationSchemeSaslPlain(
		kkComps.BackendClusterAuthenticationSaslPlain{Username: "user", Password: placeholder},
	)
	require.True(t, compareAuthenticationSchemes(current, desired))
	desired.BackendClusterAuthenticationSaslPlain.Username = "another-user"
	require.False(t, compareAuthenticationSchemes(current, desired))
	desired.BackendClusterAuthenticationSaslPlain.Username = "user"
	desired.BackendClusterAuthenticationSaslPlain.Password = "${vault.env['PASSWORD']}"
	require.False(t, compareAuthenticationSchemes(current, desired))
}

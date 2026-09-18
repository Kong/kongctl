package planner

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestBackendUpdateLogsExcludeAuthenticationValues(t *testing.T) {
	for _, kind := range []string{"sasl_plain", "sasl_scram"} {
		for _, jsonLog := range []bool{false, true} {
			var logs bytes.Buffer
			options := &slog.HandlerOptions{Level: slog.LevelDebug}
			var handler slog.Handler = slog.NewTextHandler(&logs, options)
			if jsonLog {
				handler = slog.NewJSONHandler(&logs, options)
			}
			current := kkComps.BackendCluster{ID: "backend-id", Name: "backend"}
			desired := resources.EventGatewayBackendClusterResource{
				Ref: "backend-ref", EventGateway: "gateway-ref",
				CreateBackendClusterRequest: kkComps.CreateBackendClusterRequest{Name: "backend"},
			}
			for _, value := range []struct {
				password    string
				destination any
			}{
				{"old-password-sentinel", &current.Authentication},
				{"new-password-sentinel", &desired.Authentication},
			} {
				data, err := json.Marshal(map[string]any{
					FieldType: kind, "algorithm": "sha256", "username": "private-username-sentinel",
					"password": value.password,
				})
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(data, value.destination))
			}
			p := &Planner{
				logger: slog.New(handler),
				client: state.NewClient(state.ClientConfig{
					EventGatewayBackendClusterAPI: &stubExternalEventGatewayBackendClusterAPI{
						clusters: []kkComps.BackendCluster{current},
					},
				}),
			}
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
			err := p.planBackendClusterChangesForExistingGateway(t.Context(), DefaultNamespace,
				"gateway-id", "gateway-ref", "gateway", []resources.EventGatewayBackendClusterResource{desired}, plan)
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			require.Contains(t, logs.String(), "Planning backend cluster UPDATE")
			require.Contains(t, logs.String(), "Enqueuing backend cluster UPDATE")
			require.NotContains(t, logs.String(), "old-password-sentinel")
			require.NotContains(t, logs.String(), "new-password-sentinel")
			require.NotContains(t, logs.String(), "private-username-sentinel")
		}
	}
}

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

func TestBackendOrdinaryUpdateStripsDeferredPasswords(t *testing.T) {
	expression := envSecretExpression("UNAVAILABLE_BACKEND_PASSWORD")
	secret, err := tags.BuildSecretPlaceholder(expression)
	require.NoError(t, err)
	for _, kind := range []string{"sasl_plain", "sasl_scram"} {
		for _, placeholder := range []string{secret, "__ENV__:UNAVAILABLE_BACKEND_PASSWORD"} {
			for _, selected := range []bool{false, true} {
				for _, renameUser := range []bool{false, true} {
					current := state.EventGatewayBackendCluster{BackendCluster: kkComps.BackendCluster{Name: "backend"}}
					description := "ordinary update"
					desired := resources.EventGatewayBackendClusterResource{
						Ref: "backend", EventGateway: "gateway",
						CreateBackendClusterRequest: kkComps.CreateBackendClusterRequest{
							Name: "backend", Description: &description,
						},
					}
					username := "user"
					if renameUser {
						username = "new-user"
					}
					if kind == "sasl_plain" {
						current.Authentication = kkComps.CreateBackendClusterAuthenticationSensitiveDataAwareSchemeSaslPlain(
							kkComps.BackendClusterAuthenticationSaslPlainSensitiveDataAware{Username: "user"},
						)
						desired.Authentication = kkComps.CreateBackendClusterAuthenticationSchemeSaslPlain(
							kkComps.BackendClusterAuthenticationSaslPlain{Username: username, Password: placeholder},
						)
					} else {
						current.Authentication = kkComps.CreateBackendClusterAuthenticationSensitiveDataAwareSchemeSaslScram(
							kkComps.BackendClusterAuthenticationSaslScramSensitiveDataAware{
								Username: "user", Algorithm: kkComps.AlgorithmSha256,
							},
						)
						desired.Authentication = kkComps.CreateBackendClusterAuthenticationSchemeSaslScram(
							kkComps.BackendClusterAuthenticationSaslScram{
								Username: username, Password: placeholder,
								Algorithm: kkComps.BackendClusterAuthenticationSaslScramAlgorithmSha256,
							},
						)
					}
					p := &Planner{}
					needed, fields, changed := p.shouldUpdateBackendCluster(current, desired)
					require.True(t, needed)
					rs := &resources.ResourceSet{EventGatewayBackendClusters: []resources.EventGatewayBackendClusterResource{desired}}
					rs.AddSecretSource(desired.Ref, "/authentication/password", expression, tags.IsEnvPlaceholder(placeholder))
					plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
					plan.AddChange(PlannedChange{
						ID: "update", Action: ActionUpdate, ResourceRef: desired.Ref,
						ResourceType: ResourceTypeEventGatewayBackendCluster, Fields: fields, ChangedFields: changed,
					})
					require.NoError(t, p.applySecretWriteIntents(t.Context(), plan, rs, Options{WriteSecrets: selected}))
					change := plan.Changes[0]
					if selected {
						require.Len(t, change.SecretWrites, 1)
					} else {
						require.Empty(t, change.SecretWrites)
					}
					data, err := json.Marshal(plan)
					require.NoError(t, err)
					require.NotContains(t, string(data), placeholder)
					auth, ok := change.Fields[FieldAuthentication].(map[string]any)
					require.True(t, ok)
					require.NotContains(t, auth, "password")
					require.Equal(t, username, auth["username"])
					if renameUser {
						changedAuth, ok := change.ChangedFields[FieldAuthentication].New.(map[string]any)
						require.True(t, ok)
						require.NotContains(t, changedAuth, "password")
					}
				}
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

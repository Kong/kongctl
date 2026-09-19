package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

const certificateIdentityOtherID = "11111111-1111-4111-8111-111111111111"

var certificateIdentityKinds = []string{
	ResourceTypeAIGatewayDataPlaneCertificate,
	ResourceTypeAIGatewayCertificate,
	ResourceTypeAIGatewayCACertificate,
	ResourceTypeAIGatewaySNI,
}

type certificateIdentityResource interface {
	resources.Resource
	SetKonnectID(string)
}

func certificateIdentityResources(kind, ref, name string) (certificateIdentityResource, func(string, string) any) {
	base := resources.BaseResource{Ref: ref}
	switch kind {
	case ResourceTypeAIGatewayDataPlaneCertificate:
		return &resources.AIGatewayDataPlaneCertificateResource{
				BaseResource: base, AIGateway: "gateway",
				CreateAIGatewayDataPlaneCertificateRequest: kkComps.CreateAIGatewayDataPlaneCertificateRequest{
					Title: name, Cert: "new-cert",
				},
			}, func(id, name string) any {
				return kkComps.AIGatewayDataPlaneClientCertificate{ID: id, Title: name, Cert: "old-cert"}
			}
	case ResourceTypeAIGatewayCertificate:
		return &resources.AIGatewayCertificateResource{
				BaseResource: base, AIGateway: "gateway", Name: name, Cert: "new-cert",
			}, func(id, name string) any {
				return kkComps.AIGatewayCertificate{ID: id, Name: name, Cert: "old-cert"}
			}
	case ResourceTypeAIGatewayCACertificate:
		return &resources.AIGatewayCACertificateResource{
				BaseResource: base, AIGateway: "gateway", Name: name, Cert: "new-cert",
			}, func(id, name string) any {
				return kkComps.AIGatewayCACertificate{ID: id, Name: name, Cert: "old-cert"}
			}
	default:
		return &resources.AIGatewaySNIResource{
				BaseResource: base, AIGateway: "gateway", Name: name,
				Hostname: "new.example.test", Certificate: "unchanged-cert",
			}, func(id, name string) any {
				return kkComps.AIGatewaySNI{
					ID: id, Name: name, Hostname: "old.example.test", Certificate: "unchanged-cert",
				}
			}
	}
}

func TestAIGatewayCertificateIdentityMatchers(t *testing.T) {
	for _, kind := range certificateIdentityKinds {
		t.Run(kind, func(t *testing.T) {
			for _, tc := range []struct {
				name, ref, cachedID, desiredName, remoteID, remoteName string
				match                                                  bool
			}{
				{
					"UUID ref cannot select another name", certificateIdentityOtherID, "",
					"wanted", certificateIdentityOtherID, "other", false,
				},
				{
					"cached ID cannot select another name", "local-ref", certificateIdentityOtherID,
					"wanted", certificateIdentityOtherID, "other", false,
				},
				{
					"name wins conflicting IDs", certificateIdentityOtherID, certificateIdentityOtherID,
					"wanted", "name-id", "wanted", true,
				},
				{
					"name wins missing IDs", "22222222-2222-4222-8222-222222222222", "missing-id",
					"wanted", "name-id", "wanted", true,
				},
				{
					"empty identity cannot match", certificateIdentityOtherID, certificateIdentityOtherID,
					"", certificateIdentityOtherID, "", false,
				},
				{"missing remote ID cannot match", "local-ref", "cached-id", "wanted", "", "wanted", false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					desired, remote := certificateIdentityResources(kind, tc.ref, tc.desiredName)
					desired.SetKonnectID(tc.cachedID)
					require.Equal(t, tc.match, desired.TryMatchKonnectResource(remote(tc.remoteID, tc.remoteName)))
					wantID := tc.cachedID
					if tc.match {
						wantID = tc.remoteID
					}
					require.Equal(t, wantID, desired.GetKonnectID())
				})
			}
		})
	}
}

func TestAIGatewayCertificateIdentityPlans(t *testing.T) {
	for _, kind := range certificateIdentityKinds {
		t.Run(kind, func(t *testing.T) {
			for _, tc := range []struct{ name, ref, cachedID, desiredName string }{
				{"UUID ref conflict", certificateIdentityOtherID, "", "wanted"},
				{"cached ID conflict", "local-ref", certificateIdentityOtherID, "wanted"},
				{"missing IDs", "22222222-2222-4222-8222-222222222222", "missing-id", "wanted"},
				{"new name with UUID ref", certificateIdentityOtherID, "", "new-name"},
				{"new name with cached ID", "local-ref", certificateIdentityOtherID, "new-name"},
			} {
				for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
					t.Run(tc.name+"/"+string(mode), func(t *testing.T) {
						desired, remote := certificateIdentityResources(kind, tc.ref, tc.desiredName)
						desired.SetKonnectID(tc.cachedID)
						api := &certificateIdentityAPI{}
						p := NewPlanner(api.client(), slog.Default())
						p.resources = &resources.ResourceSet{}
						plan := NewPlan(CurrentPlanVersion, "test", mode)
						current := []any{remote(certificateIdentityOtherID, "other"), remote("name-id", "wanted")}
						err := planCertificateIdentity(t.Context(), p, api, desired, current, plan)
						require.NoError(t, err)
						require.Equal(t, []string{"gateway-id"}, api.gatewayIDs)

						matched := tc.desiredName == "wanted"
						offset := 0
						wantAction := ActionCreate
						wantID := ""
						if matched && kind == ResourceTypeAIGatewayDataPlaneCertificate {
							require.GreaterOrEqual(t, len(plan.Changes), 2)
							require.Equal(t, ActionDelete, plan.Changes[0].Action)
							require.Equal(t, "name-id", plan.Changes[0].ResourceID)
							require.Equal(t, []string{plan.Changes[0].ID}, plan.Changes[1].DependsOn)
							offset = 1
						} else if matched {
							wantAction, wantID = ActionUpdate, "name-id"
						}
						pruned := []string{}
						if mode == PlanModeSync {
							pruned = append(pruned, certificateIdentityOtherID)
							if !matched {
								pruned = append(pruned, "name-id")
							}
						}
						require.Len(t, plan.Changes, offset+1+len(pruned))
						upsert := plan.Changes[offset]
						require.Equal(t, wantAction, upsert.Action)
						require.Equal(t, wantID, upsert.ResourceID)
						require.Equal(t, tc.ref, upsert.ResourceRef)
						identityField := FieldName
						if kind == ResourceTypeAIGatewayDataPlaneCertificate {
							identityField = FieldTitle
						}
						require.Equal(t, tc.desiredName, upsert.Fields[identityField])
						for i, id := range pruned {
							change := plan.Changes[offset+1+i]
							require.Equal(t, ActionDelete, change.Action)
							require.Equal(t, id, change.ResourceID)
						}
						for _, change := range plan.Changes {
							require.Equal(t, kind, change.ResourceType)
							require.Equal(t, &ParentInfo{Ref: "gateway", ID: "gateway-id"}, change.Parent)
						}
					})
				}
			}
		})
	}
}

func planCertificateIdentity(
	ctx context.Context, p *Planner, api *certificateIdentityAPI,
	desired resources.Resource, current []any, plan *Plan,
) error {
	switch desired := desired.(type) {
	case *resources.AIGatewayDataPlaneCertificateResource:
		for _, item := range current {
			api.dataPlane = append(api.dataPlane, item.(kkComps.AIGatewayDataPlaneClientCertificate))
		}
		return p.planAIGatewayDataPlaneCertificateChanges(ctx, DefaultNamespace, "gateway", "gateway", "gateway-id", "",
			[]resources.AIGatewayDataPlaneCertificateResource{*desired}, plan)
	case *resources.AIGatewayCertificateResource:
		for _, item := range current {
			api.certificates = append(api.certificates, item.(kkComps.AIGatewayCertificate))
		}
		pending, err := p.planAIGatewayCertificateUpserts(ctx, DefaultNamespace, "gateway", "gateway-id", "",
			[]resources.AIGatewayCertificateResource{*desired}, plan)
		if err != nil {
			return err
		}
		return p.planAIGatewayCertificateDeletes(pending, nil, plan)
	case *resources.AIGatewayCACertificateResource:
		for _, item := range current {
			api.cas = append(api.cas, item.(kkComps.AIGatewayCACertificate))
		}
		return p.planAIGatewayCACertificateChanges(ctx, DefaultNamespace, "gateway", "gateway-id", "",
			[]resources.AIGatewayCACertificateResource{*desired}, plan)
	default:
		for _, item := range current {
			api.snis = append(api.snis, item.(kkComps.AIGatewaySNI))
		}
		_, err := p.planAIGatewaySNIChanges(ctx, DefaultNamespace, "gateway", "gateway-id", "",
			[]resources.AIGatewaySNIResource{*desired.(*resources.AIGatewaySNIResource)}, plan)
		return err
	}
}

func TestAIGatewayCertificateIdentitySNIDependencies(t *testing.T) {
	for _, tc := range []struct {
		name                                   string
		renameSNI, keepReference, omitSNIScope bool
	}{
		{name: "reassign existing SNI"},
		{name: "replace SNI", renameSNI: true},
		{name: "unchanged SNI blocks deletion", keepReference: true},
		{name: "out of scope SNI blocks deletion", omitSNIScope: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &certificateIdentityAPI{
				certificates: []kkComps.AIGatewayCertificate{
					{ID: "old-cert-id", Name: "old-cert", Cert: "old-public-cert"},
				},
				snis: []kkComps.AIGatewaySNI{
					{ID: "sni-id", Name: "sni", Hostname: "api.example.test", Certificate: "old-cert"},
				},
			}
			rs := &resources.ResourceSet{
				SyncScope: resources.NewSyncScope(),
				AIGatewayCertificates: []resources.AIGatewayCertificateResource{{
					BaseResource: resources.BaseResource{Ref: "cert-ref"}, AIGateway: "gateway",
					Name: "new-cert", Cert: "new-public-cert",
				}},
				AIGatewaySNIs: []resources.AIGatewaySNIResource{{
					BaseResource: resources.BaseResource{Ref: "sni-ref"}, AIGateway: "gateway",
					Name: "sni", Hostname: "api.example.test", Certificate: "__REF__:cert-ref#name",
				}},
			}
			rs.AIGatewayCertificates[0].SetKonnectID("old-cert-id")
			rs.AIGatewaySNIs[0].SetKonnectID("sni-id")
			rs.SyncScope.AddChild(
				resources.ResourceTypeAIGateway,
				"gateway",
				resources.ResourceTypeAIGatewayCertificate,
			)
			if !tc.omitSNIScope {
				rs.SyncScope.AddChild(resources.ResourceTypeAIGateway, "gateway", resources.ResourceTypeAIGatewaySNI)
			}
			if tc.renameSNI {
				rs.AIGatewaySNIs[0].Name = "new-sni"
			}
			if tc.keepReference {
				rs.AIGatewaySNIs[0].Certificate = "old-cert"
			}
			p := NewPlanner(api.client(), slog.Default())
			p.resources = rs
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			err := p.planAIGatewayTLSChanges(t.Context(), DefaultNamespace, "gateway", "gateway-id", "", plan)
			require.Equal(t, []string{"gateway-id", "gateway-id"}, api.gatewayIDs)
			if tc.keepReference || tc.omitSNIScope {
				require.ErrorContains(
					t,
					err,
					`cannot delete AI Gateway certificate "old-cert" while SNI "sni" still references it`,
				)
				require.Len(t, plan.Changes, 1)
				require.Equal(t, ActionCreate, plan.Changes[0].Action)
				return
			}
			require.NoError(t, err)
			wantCount := 3
			if tc.renameSNI {
				wantCount = 4
			}
			require.Len(t, plan.Changes, wantCount)
			certificateCreate, sniUpsert := plan.Changes[0], plan.Changes[1]
			require.Equal(t, ResourceTypeAIGatewayCertificate, certificateCreate.ResourceType)
			require.Equal(t, ActionCreate, certificateCreate.Action)
			require.Empty(t, certificateCreate.ResourceID)
			require.Equal(t, "new-cert", certificateCreate.Fields[FieldName])
			require.Equal(t, ResourceTypeAIGatewaySNI, sniUpsert.ResourceType)
			require.Equal(t, "new-cert", sniUpsert.Fields[FieldCertificate])
			require.Equal(t, []string{certificateCreate.ID}, sniUpsert.DependsOn)
			certificateDelete := plan.Changes[wantCount-1]
			require.Equal(t, ResourceTypeAIGatewayCertificate, certificateDelete.ResourceType)
			require.Equal(t, ActionDelete, certificateDelete.Action)
			require.Equal(t, "old-cert-id", certificateDelete.ResourceID)
			if tc.renameSNI {
				require.Equal(t, ActionCreate, sniUpsert.Action)
				require.Empty(t, sniUpsert.ResourceID)
				sniDelete := plan.Changes[2]
				require.Equal(t, ResourceTypeAIGatewaySNI, sniDelete.ResourceType)
				require.Equal(t, ActionDelete, sniDelete.Action)
				require.Equal(t, "sni-id", sniDelete.ResourceID)
				require.Equal(t, []string{sniDelete.ID}, certificateDelete.DependsOn)
			} else {
				require.Equal(t, ActionUpdate, sniUpsert.Action)
				require.Equal(t, "sni-id", sniUpsert.ResourceID)
				require.Equal(t, []string{sniUpsert.ID}, certificateDelete.DependsOn)
			}
		})
	}
}

// Embedding the interfaces makes any unexpected API operation fail the test.
type certificateIdentityAPI struct {
	helpers.AIGatewayDataPlaneCertificatesAPI
	helpers.AIGatewayCertificatesAPI
	helpers.AIGatewayCACertificatesAPI
	helpers.AIGatewaySNIsAPI
	dataPlane    []kkComps.AIGatewayDataPlaneClientCertificate
	certificates []kkComps.AIGatewayCertificate
	cas          []kkComps.AIGatewayCACertificate
	snis         []kkComps.AIGatewaySNI
	gatewayIDs   []string
}

func (a *certificateIdentityAPI) client() *state.Client {
	return state.NewClient(state.ClientConfig{
		AIGatewayDataPlaneCertificatesAPI: a, AIGatewayCertificatesAPI: a,
		AIGatewayCACertificatesAPI: a, AIGatewaySNIsAPI: a,
	})
}

func (a *certificateIdentityAPI) ListAiGatewayDataPlaneCertificates(
	_ context.Context, req kkOps.ListAiGatewayDataPlaneCertificatesRequest, _ ...kkOps.Option,
) (*kkOps.ListAiGatewayDataPlaneCertificatesResponse, error) {
	a.gatewayIDs = append(a.gatewayIDs, req.GatewayID)
	return &kkOps.ListAiGatewayDataPlaneCertificatesResponse{
		ListAIGatewayDataPlaneCertificatesResponse: &kkComps.ListAIGatewayDataPlaneCertificatesResponse{
			Data: a.dataPlane,
		},
	}, nil
}

func (a *certificateIdentityAPI) ListAiGatewayCertificates(
	_ context.Context, req kkOps.ListAiGatewayCertificatesRequest, _ ...kkOps.Option,
) (*kkOps.ListAiGatewayCertificatesResponse, error) {
	a.gatewayIDs = append(a.gatewayIDs, req.GatewayID)
	return &kkOps.ListAiGatewayCertificatesResponse{
		ListAIGatewayCertificatesResponse: &kkComps.ListAIGatewayCertificatesResponse{Data: a.certificates},
	}, nil
}

func (a *certificateIdentityAPI) ListAiGatewayCaCertificates(
	_ context.Context, req kkOps.ListAiGatewayCaCertificatesRequest, _ ...kkOps.Option,
) (*kkOps.ListAiGatewayCaCertificatesResponse, error) {
	a.gatewayIDs = append(a.gatewayIDs, req.GatewayID)
	return &kkOps.ListAiGatewayCaCertificatesResponse{
		ListAIGatewayCACertificatesResponse: &kkComps.ListAIGatewayCACertificatesResponse{Data: a.cas},
	}, nil
}

func (a *certificateIdentityAPI) ListAiGatewaySnis(
	_ context.Context, req kkOps.ListAiGatewaySnisRequest, _ ...kkOps.Option,
) (*kkOps.ListAiGatewaySnisResponse, error) {
	a.gatewayIDs = append(a.gatewayIDs, req.GatewayID)
	return &kkOps.ListAiGatewaySnisResponse{
		ListAIGatewaySNIsResponse: &kkComps.ListAIGatewaySNIsResponse{Data: a.snis},
	}, nil
}

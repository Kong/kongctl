package planner

import (
	"fmt"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayCertificateUpdateOmitsPrivateKeysFromPlanFields(t *testing.T) {
	desired := resources.AIGatewayCertificateResource{
		BaseResource: resources.BaseResource{Ref: "runtime-cert"},
		AIGateway:    "gateway", Name: "runtime-cert", Cert: "new-public-cert", Key: "private-key-placeholder",
	}
	current := state.AIGatewayCertificate{AIGatewayCertificate: kkComps.AIGatewayCertificate{
		ID: "certificate-id", Name: "runtime-cert", Cert: "old-public-cert",
	}}

	fields, changed := diffAIGatewayCertificate(current, desired)
	require.Equal(t, "new-public-cert", fields[FieldCert])
	require.NotContains(t, fields, FieldKey)
	require.NotContains(t, fields, FieldKeyAlt)
	require.Contains(t, changed, FieldCert)
}

func TestAIGatewayCertificateAlternativeCertificateConverges(t *testing.T) {
	certAlt := "alternate-public-cert"
	desired := resources.AIGatewayCertificateResource{
		BaseResource: resources.BaseResource{Ref: "runtime-cert"},
		AIGateway:    "gateway", Name: "runtime-cert", Cert: "public-cert", CertAlt: &certAlt,
		ManagedBy: resources.ManagedByWithProvenance(nil, "older"),
	}
	current := state.AIGatewayCertificate{AIGatewayCertificate: kkComps.AIGatewayCertificate{
		ID: "certificate-id", Name: "runtime-cert", Cert: "public-cert", CertAlt: &certAlt,
		ManagedBy: resources.ManagedByWithProvenance(nil, "older"),
	}}

	_, changed := diffAIGatewayCertificate(current, desired)
	require.Empty(t, changed)
}

func TestAIGatewayTLSOptionalFieldRemovalPlansUpdate(t *testing.T) {
	certAlt := "alternate-public-cert"
	current := state.AIGatewayCertificate{AIGatewayCertificate: kkComps.AIGatewayCertificate{
		ID: "certificate-id", Name: "runtime-cert", Cert: "public-cert", CertAlt: &certAlt,
		Labels: map[string]string{"revision": "initial"}, ManagedBy: map[string]string{"other-tool": "test"},
	}}
	desired := resources.AIGatewayCertificateResource{
		BaseResource: resources.BaseResource{Ref: "runtime-cert"},
		AIGateway:    "gateway", Name: "runtime-cert", Cert: "public-cert",
	}

	fields, changed := diffAIGatewayCertificate(current, desired)
	require.NotContains(t, fields, FieldCertAlt)
	require.NotContains(t, fields, FieldLabels)
	require.Equal(t, resources.ManagedByWithProvenance(nil, ""), fields[FieldManagedBy])
	require.Equal(t, FieldChange{Old: certAlt, New: nil}, changed[FieldCertAlt])
	require.Equal(t, FieldChange{Old: current.Labels, New: nil}, changed[FieldLabels])
	require.Equal(t, FieldChange{Old: current.ManagedBy, New: fields[FieldManagedBy]}, changed[FieldManagedBy])
}

func TestAIGatewayTLSEmptyMetadataPlansProvenanceWrite(t *testing.T) {
	current := state.AIGatewayCertificate{AIGatewayCertificate: kkComps.AIGatewayCertificate{
		ID: "certificate-id", Name: "runtime-cert", Cert: "public-cert",
		Labels: map[string]string{}, ManagedBy: map[string]string{},
	}}
	desired := resources.AIGatewayCertificateResource{
		BaseResource: resources.BaseResource{Ref: "runtime-cert"},
		AIGateway:    "gateway", Name: "runtime-cert", Cert: "public-cert",
	}

	fields, changed := diffAIGatewayCertificate(current, desired)
	require.Len(t, changed, 1)
	require.Equal(t, resources.ManagedByWithProvenance(nil, ""), fields[FieldManagedBy])
}

func TestAIGatewayTLSPlannerCreatesCertificateBeforeSNI(t *testing.T) {
	placeholder, err := tags.BuildSecretPlaceholder(envSecretExpression("RUNTIME_PRIVATE_KEY"))
	require.NoError(t, err)
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{Ref: "gateway", Kongctl: &resources.KongctlMeta{
				Namespace: new(DefaultNamespace),
			}},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{Name: "gateway", DisplayName: "Gateway"},
		}},
		AIGatewayCertificates: []resources.AIGatewayCertificateResource{{
			BaseResource: resources.BaseResource{Ref: "runtime-cert"}, AIGateway: "gateway",
			Name: "runtime-cert", Cert: "public-cert", Key: placeholder,
		}},
		AIGatewayCACertificates: []resources.AIGatewayCACertificateResource{{
			BaseResource: resources.BaseResource{Ref: "root-ca"}, AIGateway: "gateway",
			Name: "root-ca", Cert: "public-ca",
		}},
		AIGatewaySNIs: []resources.AIGatewaySNIResource{{
			BaseResource: resources.BaseResource{Ref: "runtime-sni"}, AIGateway: "gateway",
			Name: "runtime-sni", DisplayName: "Runtime SNI", Hostname: "api.example.test",
			Certificate: tags.RefPlaceholderPrefix + "runtime-cert#name",
		}},
	}
	rs.AddSecretSource("runtime-cert", "/key", envSecretExpression("RUNTIME_PRIVATE_KEY"), false)
	client := state.NewClient(state.ClientConfig{AIGatewayAPI: &testAIGatewayAPI{}})

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 4)
	certificate := findPlannedResourceChange(plan, ResourceTypeAIGatewayCertificate, "runtime-cert")
	sni := findPlannedResourceChange(plan, ResourceTypeAIGatewaySNI, "runtime-sni")
	require.NotNil(t, certificate)
	require.NotNil(t, sni)
	require.Contains(t, sni.DependsOn, certificate.ID)
	require.Equal(t, "runtime-cert", sni.Fields[FieldCertificate])
	require.NotContains(t, certificate.Fields, FieldKey)
	require.Len(t, certificate.SecretWrites, 1)
}

func TestAIGatewaySNIReferenceUsesCertificateName(t *testing.T) {
	rs := &resources.ResourceSet{AIGatewayCertificates: []resources.AIGatewayCertificateResource{{
		BaseResource: resources.BaseResource{Ref: "runtime-cert"}, Name: "tls-production",
	}}}

	actual, err := normalizeAIGatewaySNICertificateReference("__REF__:runtime-cert#name", rs)
	require.NoError(t, err)
	require.Equal(t, "tls-production", actual)
}

func TestAIGatewaySNIReferenceRejectsCertificateID(t *testing.T) {
	_, err := normalizeAIGatewaySNICertificateReference("__REF__:runtime-cert#id", nil)
	require.ErrorContains(t, err, "certificate reference must use #name, got #id")
}

func TestAIGatewaySNICreateRejectsInvalidCertificateReference(t *testing.T) {
	for _, existingGateway := range []bool{false, true} {
		for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
			t.Run(fmt.Sprintf("existing=%t/%s", existingGateway, mode), func(t *testing.T) {
				rs := federatedResourceSet()
				if !existingGateway {
					rs.AIGateways = []resources.AIGatewayResource{{
						BaseResource: resources.BaseResource{
							Ref: "shared", Kongctl: &resources.KongctlMeta{Namespace: new(DefaultNamespace)},
						},
						CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{Name: "shared", DisplayName: "Shared"},
					}}
				}
				placeholder, err := tags.BuildSecretPlaceholder(envSecretExpression("RUNTIME_PRIVATE_KEY"))
				require.NoError(t, err)
				rs.AddSecretSource("cert", "/key", envSecretExpression("RUNTIME_PRIVATE_KEY"), false)
				rs.AIGatewayCertificates = []resources.AIGatewayCertificateResource{{
					BaseResource: resources.BaseResource{Ref: "cert"}, AIGateway: "shared", Name: "cert", Cert: "public-cert",
					Key: placeholder,
				}}
				rs.AIGatewaySNIs = []resources.AIGatewaySNIResource{{
					BaseResource: resources.BaseResource{Ref: "invalid-sni"}, AIGateway: "shared",
					Name: "invalid-sni", Hostname: "api.example.test", Certificate: tags.RefPlaceholderPrefix + "cert#id",
				}}
				rs.SyncScope.AddChild(resources.ResourceTypeAIGateway, "shared", resources.ResourceTypeAIGatewaySNI)
				cfg := emptyAIGatewayCollectionsConfig()
				cfg.AIGatewayAPI = &testAIGatewayAPI{}
				p := NewPlanner(state.NewClient(cfg), slog.Default())
				plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: mode})
				require.ErrorContains(t, err, `failed to build AI Gateway SNI "invalid-sni" create payload`)
				require.ErrorContains(t, err, "certificate reference must use #name, got #id")
				require.Nil(t, plan, "invalid declarations must not return a usable partial plan")
			})
		}
	}
}

func TestAIGatewaySNILiteralCertificateNameIsNotTreatedAsResourceRef(t *testing.T) {
	rs := &resources.ResourceSet{AIGatewayCertificates: []resources.AIGatewayCertificateResource{{
		BaseResource: resources.BaseResource{Ref: "runtime-cert"}, Name: "tls-production",
	}}}

	actual, err := normalizeAIGatewaySNICertificateReference("runtime-cert", rs)
	require.NoError(t, err)
	require.Equal(t, "runtime-cert", actual)
}

func TestAIGatewayCertificateUpdateRequiresSelectedPrivateKey(t *testing.T) {
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
	plan.AddChange(PlannedChange{
		ResourceType: ResourceTypeAIGatewayCertificate, ResourceRef: "runtime-cert",
		Action: ActionUpdate, Fields: map[string]any{FieldName: "runtime-cert", FieldCert: "public-cert"},
	})

	require.ErrorContains(t, validateAIGatewayCertificateSecretWrites(plan), "private key /key")
	plan.Changes[0].SecretWrites = []SecretWriteIntent{{Field: "/key"}}
	require.NoError(t, validateAIGatewayCertificateSecretWrites(plan))
}

func TestAIGatewayCertificateFinalDeleteDependencies(t *testing.T) {
	for _, reference := range []string{"runtime-cert", "certificate-id"} {
		for _, tc := range []struct {
			name                                                    string
			action                                                  ActionType
			certificate                                             string
			observed, certificateField, retainsReference, wantError bool
		}{
			{name: "retained SNI blocks deletion", observed: true, wantError: true},
			{name: "SNI deletion precedes certificate deletion", action: ActionDelete, observed: true},
			{
				name: "SNI detach precedes certificate deletion", action: ActionUpdate, observed: true,
				certificateField: true, certificate: "replacement-cert",
			},
			{
				name: "SNI update retains reference", action: ActionUpdate, observed: true, certificateField: true,
				retainsReference: true, wantError: true,
			},
			{name: "SNI update omits certificate", action: ActionUpdate, observed: true, wantError: true},
			{
				name: "SNI update clears certificate", action: ActionUpdate, observed: true, certificateField: true,
				retainsReference: false, wantError: true,
			},
			{
				name: "new SNI cannot reference deleted certificate", action: ActionCreate, certificateField: true,
				retainsReference: true, wantError: true,
			},
		} {
			t.Run(reference+"/"+tc.name, func(t *testing.T) {
				api := &certificateIdentityAPI{}
				if tc.observed {
					api.snis = []kkComps.AIGatewaySNI{{ID: "sni-id", Name: "runtime-sni", Certificate: reference}}
				}
				p := NewPlanner(api.client(), slog.Default())
				plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
				plan.AddChange(newAIGatewayTLSDelete(p, ResourceTypeAIGatewayCertificate, DefaultNamespace,
					"gateway", "gateway-id", "certificate-id", "runtime-cert"))
				if tc.action != "" {
					change := PlannedChange{
						ID: "change-sni", ResourceType: ResourceTypeAIGatewaySNI, ResourceRef: "runtime-sni",
						Action: tc.action, Namespace: "other-namespace", Parent: &ParentInfo{Ref: "gateway-alias", ID: "gateway-id"},
						Fields: map[string]any{FieldDisplayName: "Updated SNI"},
					}
					if tc.observed {
						change.ResourceID = "sni-id"
					}
					if tc.certificateField {
						value := tc.certificate
						if tc.retainsReference {
							value = reference
						}
						change.Fields[FieldCertificate] = value
					}
					plan.AddChange(change)
				}
				err := p.resolveFinalAIGatewayDependencies(t.Context(), plan)
				if tc.wantError {
					require.ErrorContains(t, err, "still references it")
					return
				}
				require.NoError(t, err)
				require.Equal(t, []string{"change-sni"}, plan.Changes[0].DependsOn)
			})
		}
	}
}

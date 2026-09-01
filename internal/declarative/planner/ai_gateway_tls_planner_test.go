package planner

import (
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
			Certificate: tags.RefPlaceholderPrefix + "runtime-cert#id",
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

	require.Equal(t, "tls-production", normalizeAIGatewaySNICertificateReference("__REF__:runtime-cert#id", rs))
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

func TestAIGatewayCertificateDeleteDependsOnReferencingSNIDelete(t *testing.T) {
	planner := &Planner{}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	sniDelete := PlannedChange{
		ID: "delete-sni", ResourceType: ResourceTypeAIGatewaySNI,
		ResourceID: "sni-id", ResourceRef: "runtime-sni", Action: ActionDelete,
	}
	plan.AddChange(sniDelete)
	certificate := state.AIGatewayCertificate{AIGatewayCertificate: kkComps.AIGatewayCertificate{
		ID: "certificate-id", Name: "runtime-cert", Cert: "public-cert",
	}}
	pending := pendingAIGatewayCertificateDelete{
		certificate: certificate,
		change: PlannedChange{
			ID: "delete-certificate", ResourceType: ResourceTypeAIGatewayCertificate,
			ResourceID: certificate.ID, ResourceRef: certificate.Name, Action: ActionDelete,
		},
	}
	snis := []state.AIGatewaySNI{{AIGatewaySNI: kkComps.AIGatewaySNI{
		ID: "sni-id", Name: "runtime-sni", Certificate: certificate.Name,
	}}}

	require.NoError(t, planner.planAIGatewayCertificateDeletes(
		[]pendingAIGatewayCertificateDelete{pending}, snis, plan,
	))
	require.Len(t, plan.Changes, 2)
	certificateDelete := plan.Changes[1]
	require.Equal(t, ResourceTypeAIGatewayCertificate, certificateDelete.ResourceType)
	require.Equal(t, []string{sniDelete.ID}, certificateDelete.DependsOn)
}

func TestAIGatewayCertificateDeleteRejectsRemainingSNIReference(t *testing.T) {
	planner := &Planner{}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	certificate := state.AIGatewayCertificate{AIGatewayCertificate: kkComps.AIGatewayCertificate{
		ID: "certificate-id", Name: "runtime-cert", Cert: "public-cert",
	}}
	pending := pendingAIGatewayCertificateDelete{
		certificate: certificate,
		change: PlannedChange{
			ID: "delete-certificate", ResourceType: ResourceTypeAIGatewayCertificate,
			ResourceID: certificate.ID, ResourceRef: certificate.Name, Action: ActionDelete,
		},
	}
	snis := []state.AIGatewaySNI{{AIGatewaySNI: kkComps.AIGatewaySNI{
		ID: "sni-id", Name: "runtime-sni", Certificate: certificate.Name,
	}}}

	err := planner.planAIGatewayCertificateDeletes(
		[]pendingAIGatewayCertificateDelete{pending}, snis, plan,
	)
	require.ErrorContains(t, err, "still references it")
}

package planner

import (
	"context"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayDataPlaneCertificatePlannerCreatesChildForExistingGateway(t *testing.T) {
	cert := testAIGatewayDataPlaneCertificateResource()
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayDataPlaneCertificatesAPI: &testAIGatewayDataPlaneCertificateAPI{},
	})
	rs := testAIGatewayDataPlaneCertificateResourceSet(cert, nil)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionCreate, change.Action)
	require.Equal(t, ResourceTypeAIGatewayDataPlaneCertificate, change.ResourceType)
	require.Equal(t, "support-data-plane-cert", change.ResourceRef)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
	require.Equal(t, "support-gateway", change.Parent.Ref)
	require.Equal(t, "support-data-plane-cert", change.Fields[FieldTitle])
	require.Equal(t, "first-cert", change.Fields[FieldCert])
}

func TestAIGatewayDataPlaneCertificatePlannerNoopsMatchingTitleCertAndDescription(t *testing.T) {
	cert := testAIGatewayDataPlaneCertificateResource()
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayDataPlaneCertificatesAPI: &testAIGatewayDataPlaneCertificateAPI{
			certs: []kkComps.AIGatewayDataPlaneClientCertificate{
				testAIGatewayDataPlaneCertificate("cert-id", "support-data-plane-cert", "first-cert", "Support cert"),
			},
		},
	})
	rs := testAIGatewayDataPlaneCertificateResourceSet(cert, nil)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestAIGatewayDataPlaneCertificatePlannerReplacesChangedCertificate(t *testing.T) {
	cert := testAIGatewayDataPlaneCertificateResource()
	cert.Cert = "second-cert"
	rotatedDescription := "Support cert rotated"
	cert.Description = &rotatedDescription
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayDataPlaneCertificatesAPI: &testAIGatewayDataPlaneCertificateAPI{
			certs: []kkComps.AIGatewayDataPlaneClientCertificate{
				testAIGatewayDataPlaneCertificate("cert-id", "support-data-plane-cert", "first-cert", "Support cert"),
			},
		},
	})
	rs := testAIGatewayDataPlaneCertificateResourceSet(cert, nil)

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)
	deleteChange := plan.Changes[0]
	createChange := plan.Changes[1]
	require.Equal(t, ActionDelete, deleteChange.Action)
	require.Equal(t, ResourceTypeAIGatewayDataPlaneCertificate, deleteChange.ResourceType)
	require.Equal(t, "cert-id", deleteChange.ResourceID)
	require.Contains(t, deleteChange.ChangedFields, FieldCert)
	require.Contains(t, deleteChange.ChangedFields, FieldDescription)
	require.Equal(t, ActionCreate, createChange.Action)
	require.Equal(t, []string{deleteChange.ID}, createChange.DependsOn)
	require.Equal(t, "second-cert", createChange.Fields[FieldCert])
	require.Equal(t, rotatedDescription, createChange.Fields[FieldDescription])
}

func TestAIGatewayDataPlaneCertificatePlannerSyncDeletesScopedCertificates(t *testing.T) {
	scope := resources.NewSyncScope()
	scope.AddRoot(resources.ResourceTypeAIGateway)
	scope.AddChild(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayDataPlaneCertificate,
	)
	client := state.NewClient(state.ClientConfig{
		AIGatewayAPI: &testAIGatewayAPI{
			gateways: []kkComps.AIGateway{testAIGateway()},
		},
		AIGatewayDataPlaneCertificatesAPI: &testAIGatewayDataPlaneCertificateAPI{
			certs: []kkComps.AIGatewayDataPlaneClientCertificate{
				testAIGatewayDataPlaneCertificate("cert-id", "support-data-plane-cert", "first-cert", "Support cert"),
			},
		},
	})
	rs := testAIGatewayDataPlaneCertificateResourceSet(
		resources.AIGatewayDataPlaneCertificateResource{},
		scope,
	)
	rs.AIGatewayDataPlaneCertificates = nil

	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ActionDelete, change.Action)
	require.Equal(t, ResourceTypeAIGatewayDataPlaneCertificate, change.ResourceType)
	require.Equal(t, "cert-id", change.ResourceID)
	require.NotNil(t, change.Parent)
	require.Equal(t, "gateway-id", change.Parent.ID)
}

func testAIGatewayDataPlaneCertificateResourceSet(
	cert resources.AIGatewayDataPlaneCertificateResource,
	scope *resources.SyncScope,
) *resources.ResourceSet {
	namespace := "default"
	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{{
			BaseResource: resources.BaseResource{
				Ref:     "support-gateway",
				Kongctl: &resources.KongctlMeta{Namespace: &namespace},
			},
			CreateAIGatewayRequest: kkComps.CreateAIGatewayRequest{
				Name:        "support-gateway",
				DisplayName: "Support Gateway",
			},
		}},
		SyncScope: scope,
	}
	if cert.Ref != "" {
		rs.AIGatewayDataPlaneCertificates = []resources.AIGatewayDataPlaneCertificateResource{cert}
	}
	return rs
}

func testAIGatewayDataPlaneCertificateResource() resources.AIGatewayDataPlaneCertificateResource {
	description := "Support cert"
	return resources.AIGatewayDataPlaneCertificateResource{
		BaseResource: resources.BaseResource{Ref: "support-data-plane-cert"},
		AIGateway:    "support-gateway",
		CreateAIGatewayDataPlaneCertificateRequest: kkComps.CreateAIGatewayDataPlaneCertificateRequest{
			Cert:        "first-cert",
			Title:       "support-data-plane-cert",
			Description: &description,
		},
	}
}

func testAIGatewayDataPlaneCertificate(
	id string,
	title string,
	cert string,
	description string,
) kkComps.AIGatewayDataPlaneClientCertificate {
	return kkComps.AIGatewayDataPlaneClientCertificate{
		ID:          id,
		Title:       title,
		Cert:        cert,
		Description: &description,
	}
}

type testAIGatewayDataPlaneCertificateAPI struct {
	certs []kkComps.AIGatewayDataPlaneClientCertificate
}

func (t *testAIGatewayDataPlaneCertificateAPI) ListAiGatewayDataPlaneCertificates(
	context.Context,
	kkOps.ListAiGatewayDataPlaneCertificatesRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayDataPlaneCertificatesResponse, error) {
	return &kkOps.ListAiGatewayDataPlaneCertificatesResponse{
		ListAIGatewayDataPlaneCertificatesResponse: &kkComps.ListAIGatewayDataPlaneCertificatesResponse{
			Data: t.certs,
		},
	}, nil
}

func (t *testAIGatewayDataPlaneCertificateAPI) CreateAiGatewayDataPlaneCertificate(
	context.Context,
	string,
	kkComps.CreateAIGatewayDataPlaneCertificateRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayDataPlaneCertificateResponse, error) {
	return nil, nil
}

func (t *testAIGatewayDataPlaneCertificateAPI) GetAiGatewayDataPlaneCertificate(
	_ context.Context,
	_ string,
	certificateID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayDataPlaneCertificateResponse, error) {
	for _, cert := range t.certs {
		if resources.AIGatewayDataPlaneCertificateID(cert) == certificateID ||
			resources.AIGatewayDataPlaneCertificateTitle(cert) == certificateID {
			return &kkOps.GetAiGatewayDataPlaneCertificateResponse{
				AIGatewayDataPlaneClientCertificate: &cert,
			}, nil
		}
	}
	return &kkOps.GetAiGatewayDataPlaneCertificateResponse{}, nil
}

func (t *testAIGatewayDataPlaneCertificateAPI) DeleteAiGatewayDataPlaneCertificate(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayDataPlaneCertificateResponse, error) {
	return nil, nil
}

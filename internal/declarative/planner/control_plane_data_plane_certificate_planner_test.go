package planner

import (
	"context"
	"io"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockDataPlaneCertificateAPI struct {
	list func(context.Context, string) (*kkOps.ListDpClientCertificatesResponse, error)
}

func (m *mockDataPlaneCertificateAPI) ListDpClientCertificates(
	ctx context.Context,
	controlPlaneID string,
	_ ...kkOps.Option,
) (*kkOps.ListDpClientCertificatesResponse, error) {
	if m.list == nil {
		return dataPlaneCertificateListResponse(nil), nil
	}
	return m.list(ctx, controlPlaneID)
}

func (m *mockDataPlaneCertificateAPI) CreateDataplaneCertificate(
	_ context.Context,
	_ string,
	_ *kkComps.DataPlaneClientCertificateRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateDataplaneCertificateResponse, error) {
	return nil, nil
}

func (m *mockDataPlaneCertificateAPI) GetDataplaneCertificate(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.GetDataplaneCertificateResponse, error) {
	return nil, nil
}

func (m *mockDataPlaneCertificateAPI) DeleteDataplaneCertificate(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeleteDataplaneCertificateResponse, error) {
	return nil, nil
}

func TestControlPlaneDataPlaneCertificatePlanner_CreateForNewControlPlane(t *testing.T) {
	mockAPI := helpers.NewMockControlPlaneAPI(t)
	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.Anything).
		Return(newListControlPlaneResponse(nil, 0), nil).
		Once()

	planner := newControlPlaneDataPlaneCertificateTestPlanner(
		state.NewClient(state.ClientConfig{ControlPlaneAPI: mockAPI}),
		&resources.ResourceSet{
			ControlPlanes: []resources.ControlPlaneResource{
				{
					CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: "cp"},
					BaseResource:              resources.BaseResource{Ref: "cp"},
					DataPlaneCertificates: []resources.ControlPlaneDataPlaneCertificateResource{
						{
							Ref:  "dp-cert",
							Cert: "-----BEGIN CERTIFICATE-----\nCERT\n-----END CERTIFICATE-----",
						},
					},
				},
			},
		},
	)

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := NewControlPlanePlanner(NewBasePlanner(planner)).PlanChanges(t.Context(), NewConfig("default"), plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)

	controlPlaneChange := plan.Changes[0]
	certChange := plan.Changes[1]
	assert.Equal(t, ResourceTypeControlPlane, controlPlaneChange.ResourceType)
	assert.Equal(t, ResourceTypeControlPlaneDataPlaneCertificate, certChange.ResourceType)
	assert.Equal(t, ActionCreate, certChange.Action)
	assert.Equal(t, "dp-cert", certChange.ResourceRef)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nCERT\n-----END CERTIFICATE-----", certChange.Fields[FieldCert])
	assert.Contains(t, certChange.DependsOn, controlPlaneChange.ID)
	require.Contains(t, certChange.References, FieldControlPlaneID)
	assert.Equal(t, "cp", certChange.References[FieldControlPlaneID].Ref)
}

func TestControlPlaneDataPlaneCertificatePlanner_NoopsWhenCertificateExists(t *testing.T) {
	certValue := "-----BEGIN CERTIFICATE-----\nCERT\n-----END CERTIFICATE-----"
	client := newControlPlaneDataPlaneCertificateStateClient(t, []kkComps.DataPlaneClientCertificate{
		{
			ID:   stringPtr("cert-id"),
			Cert: &certValue,
		},
	})
	planner := newControlPlaneDataPlaneCertificateTestPlanner(
		client,
		resourceSetWithRootDataPlaneCertificate("cp", "dp-cert", certValue),
	)

	plan := NewPlan("1.0", "test", PlanModeApply)
	err := NewControlPlanePlanner(NewBasePlanner(planner)).PlanChanges(t.Context(), NewConfig("default"), plan)
	require.NoError(t, err)
	assert.Empty(t, plan.Changes)
}

func TestControlPlaneDataPlaneCertificatePlanner_SyncCreatesBeforeDeletes(t *testing.T) {
	oldCert := "-----BEGIN CERTIFICATE-----\nOLD\n-----END CERTIFICATE-----"
	newCert := "-----BEGIN CERTIFICATE-----\nNEW\n-----END CERTIFICATE-----"
	client := newControlPlaneDataPlaneCertificateStateClient(t, []kkComps.DataPlaneClientCertificate{
		{
			ID:   stringPtr("old-cert-id"),
			Cert: &oldCert,
		},
	})
	planner := newControlPlaneDataPlaneCertificateTestPlanner(
		client,
		resourceSetWithRootDataPlaneCertificate("cp", "new-dp-cert", newCert),
	)

	plan := NewPlan("1.0", "test", PlanModeSync)
	err := NewControlPlanePlanner(NewBasePlanner(planner)).PlanChanges(t.Context(), NewConfig("default"), plan)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)

	var createChange, deleteChange *PlannedChange
	for i := range plan.Changes {
		change := &plan.Changes[i]
		require.Equal(t, ResourceTypeControlPlaneDataPlaneCertificate, change.ResourceType)
		switch change.Action {
		case ActionCreate:
			createChange = change
		case ActionDelete:
			deleteChange = change
		case ActionUpdate, ActionExternalTool:
			t.Fatalf("unexpected action %s", change.Action)
		}
	}

	require.NotNil(t, createChange)
	require.NotNil(t, deleteChange)
	assert.Equal(t, newCert, createChange.Fields[FieldCert])
	assert.Equal(t, "old-cert-id", deleteChange.ResourceID)
	assert.Contains(t, deleteChange.DependsOn, createChange.ID)
}

func newControlPlaneDataPlaneCertificateTestPlanner(
	client *state.Client,
	rs *resources.ResourceSet,
) *Planner {
	planner := &Planner{
		client:    client,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		resources: rs,
	}
	planner.genericPlanner = NewGenericPlanner(planner)
	return planner
}

func newControlPlaneDataPlaneCertificateStateClient(
	t *testing.T,
	currentCerts []kkComps.DataPlaneClientCertificate,
) *state.Client {
	t.Helper()

	current := kkComps.ControlPlane{
		ID:   "cp-id",
		Name: "cp",
		Labels: map[string]string{
			labels.NamespaceKey: "default",
		},
		Config: kkComps.ControlPlaneConfig{
			ClusterType: kkComps.ControlPlaneClusterTypeClusterTypeControlPlane,
		},
	}
	mockAPI := helpers.NewMockControlPlaneAPI(t)
	mockAPI.EXPECT().
		ListControlPlanes(mock.Anything, mock.Anything).
		Return(newListControlPlaneResponse([]kkComps.ControlPlane{current}, 1), nil).
		Once()

	certAPI := &mockDataPlaneCertificateAPI{
		list: func(_ context.Context, controlPlaneID string) (*kkOps.ListDpClientCertificatesResponse, error) {
			assert.Equal(t, "cp-id", controlPlaneID)
			return dataPlaneCertificateListResponse(currentCerts), nil
		},
	}

	return state.NewClient(state.ClientConfig{
		ControlPlaneAPI:         mockAPI,
		DataPlaneCertificateAPI: certAPI,
	})
}

func resourceSetWithRootDataPlaneCertificate(
	controlPlaneRef string,
	certRef string,
	cert string,
) *resources.ResourceSet {
	return &resources.ResourceSet{
		ControlPlanes: []resources.ControlPlaneResource{
			{
				CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{Name: "cp"},
				BaseResource:              resources.BaseResource{Ref: controlPlaneRef},
			},
		},
		ControlPlaneDataPlaneCertificates: []resources.ControlPlaneDataPlaneCertificateResource{
			{
				Ref:          certRef,
				ControlPlane: controlPlaneRef,
				Cert:         cert,
			},
		},
	}
}

func dataPlaneCertificateListResponse(
	items []kkComps.DataPlaneClientCertificate,
) *kkOps.ListDpClientCertificatesResponse {
	return &kkOps.ListDpClientCertificatesResponse{
		ListDataPlaneCertificatesResponse: &kkComps.ListDataPlaneCertificatesResponse{
			Items: items,
		},
	}
}

func stringPtr(value string) *string {
	return &value
}

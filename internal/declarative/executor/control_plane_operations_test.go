package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneAdapter_MapCreateFields(t *testing.T) {
	adapter := NewControlPlaneAdapter(nil)
	execCtx := NewExecutionContext(&planner.PlannedChange{
		Namespace:  "team-a",
		Protection: true,
	})

	fields := map[string]any{
		"name":          "cp-create",
		"description":   "desc",
		"cluster_type":  string(kkComps.CreateControlPlaneRequestClusterTypeClusterTypeControlPlane),
		"auth_type":     string(kkComps.AuthTypePinnedClientCerts),
		"cloud_gateway": true,
		"proxy_urls": []any{
			map[string]any{"host": "example.com", "port": float64(443), "protocol": "https"},
		},
		"labels": map[string]any{"env": "prod"},
	}

	var req kkComps.CreateControlPlaneRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), execCtx, fields, &req))
	assert.Equal(t, "cp-create", req.Name)
	require.NotNil(t, req.ClusterType)
	assert.Equal(t, kkComps.CreateControlPlaneRequestClusterTypeClusterTypeControlPlane, *req.ClusterType)
	require.NotNil(t, req.AuthType)
	assert.Equal(t, kkComps.AuthTypePinnedClientCerts, *req.AuthType)
	require.NotNil(t, req.CloudGateway)
	assert.True(t, *req.CloudGateway)
	require.Len(t, req.ProxyUrls, 1)
	assert.Equal(t, int64(443), req.ProxyUrls[0].Port)
	assert.Equal(t, "team-a", req.Labels[labels.NamespaceKey])
	assert.Equal(t, labels.TrueValue, req.Labels[labels.ProtectedKey])
	assert.Equal(t, "prod", req.Labels["env"])
}

func TestControlPlaneAdapter_MapUpdateFields(t *testing.T) {
	adapter := NewControlPlaneAdapter(nil)
	execCtx := NewExecutionContext(&planner.PlannedChange{
		Namespace:  "team-a",
		Protection: false,
	})

	fields := map[string]any{
		"description": "updated",
		"auth_type":   string(kkComps.UpdateControlPlaneRequestAuthTypePkiClientCerts),
		"proxy_urls":  []kkComps.ProxyURL{{Host: "example.com", Port: 8443, Protocol: "https"}},
		"labels":      map[string]any{"env": "staging"},
		planner.FieldCurrentLabels: map[string]any{
			"env":               "prod",
			labels.NamespaceKey: "team-a",
		},
	}

	currentLabels := map[string]string{"env": "prod"}
	var update kkComps.UpdateControlPlaneRequest
	require.NoError(t, adapter.MapUpdateFields(context.Background(), execCtx, fields, &update, currentLabels))
	require.NotNil(t, update.Description)
	assert.Equal(t, "updated", *update.Description)
	require.NotNil(t, update.AuthType)
	assert.Equal(t, kkComps.UpdateControlPlaneRequestAuthTypePkiClientCerts, *update.AuthType)
	require.Len(t, update.ProxyUrls, 1)
	assert.Equal(t, int64(8443), update.ProxyUrls[0].Port)
	require.NotNil(t, update.Labels)
	assert.Equal(t, "team-a", update.Labels[labels.NamespaceKey])
	assert.Equal(t, "staging", update.Labels["env"])
}

func TestControlPlaneAdapter_CreateUpdateDelete(t *testing.T) {
	mockAPI := helpers.NewMockControlPlaneAPI(t)
	createReq := kkComps.CreateControlPlaneRequest{Name: "cp"}
	updateReq := kkComps.UpdateControlPlaneRequest{}

	mockAPI.EXPECT().
		CreateControlPlane(mock.Anything, createReq).
		Return(&kkOps.CreateControlPlaneResponse{ControlPlane: &kkComps.ControlPlane{ID: "cp-1"}}, nil).
		Once()

	mockAPI.EXPECT().
		UpdateControlPlane(mock.Anything, "cp-1", updateReq).
		Return(&kkOps.UpdateControlPlaneResponse{ControlPlane: &kkComps.ControlPlane{ID: "cp-1"}}, nil).
		Once()

	mockAPI.EXPECT().
		DeleteControlPlane(mock.Anything, "cp-1").
		Return(&kkOps.DeleteControlPlaneResponse{}, nil).
		Once()

	client := state.NewClient(state.ClientConfig{ControlPlaneAPI: mockAPI})
	adapter := NewControlPlaneAdapter(client)

	id, err := adapter.Create(testContextWithLogger(), createReq, "team-a", nil)
	require.NoError(t, err)
	assert.Equal(t, "cp-1", id)

	id, err = adapter.Update(testContextWithLogger(), "cp-1", updateReq, "team-a", nil)
	require.NoError(t, err)
	assert.Equal(t, "cp-1", id)

	require.NoError(t, adapter.Delete(testContextWithLogger(), "cp-1", nil))
}

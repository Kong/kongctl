package adopt

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	helpers "github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
)

type egwControlPlaneAPIStub struct {
	t             *testing.T
	fetchResponse *kkComps.EventGatewayInfo
	listResponse  []kkComps.EventGatewayInfo
	lastUpdate    kkComps.UpdateGatewayRequest
	updateCalls   int
}

func (e *egwControlPlaneAPIStub) ListEGWControlPlanes(
	context.Context,
	kkOps.ListEventGatewaysRequest,
	...kkOps.Option,
) (*kkOps.ListEventGatewaysResponse, error) {
	resp := &kkOps.ListEventGatewaysResponse{
		ListEventGatewaysResponse: &kkComps.ListEventGatewaysResponse{
			Data: e.listResponse,
			Meta: kkComps.CursorMeta{},
		},
	}
	return resp, nil
}

func (e *egwControlPlaneAPIStub) FetchEGWControlPlane(
	_ context.Context,
	id string,
	_ ...kkOps.Option,
) (*kkOps.GetEventGatewayResponse, error) {
	if id != e.fetchResponse.ID {
		e.t.Fatalf("unexpected Event Gateway Control Plane id: %s", id)
	}
	return &kkOps.GetEventGatewayResponse{EventGatewayInfo: e.fetchResponse}, nil
}

func (e *egwControlPlaneAPIStub) CreateEGWControlPlane(
	context.Context,
	kkComps.CreateGatewayRequest,
	...kkOps.Option,
) (*kkOps.CreateEventGatewayResponse, error) {
	e.t.Fatalf("unexpected CreateEGWControlPlane call")
	return nil, nil
}

func (e *egwControlPlaneAPIStub) UpdateEGWControlPlane(
	_ context.Context,
	id string,
	req kkComps.UpdateGatewayRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateEventGatewayResponse, error) {
	if id != e.fetchResponse.ID {
		e.t.Fatalf("unexpected Event Gateway Control Plane id: %s", id)
	}
	e.updateCalls++
	e.lastUpdate = req

	labels := make(map[string]string)
	if req.Labels != nil {
		for k, v := range req.Labels {
			labels[k] = v
		}
	}

	resp := &kkOps.UpdateEventGatewayResponse{
		EventGatewayInfo: &kkComps.EventGatewayInfo{
			ID:     e.fetchResponse.ID,
			Name:   e.fetchResponse.Name,
			Labels: labels,
		},
	}
	return resp, nil
}

func (e *egwControlPlaneAPIStub) DeleteEGWControlPlane(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.DeleteEventGatewayResponse, error) {
	e.t.Fatalf("unexpected DeleteEGWControlPlane call")
	return nil, nil
}

func TestAdoptEventGatewayControlPlaneByName(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	egw := &egwControlPlaneAPIStub{
		t: t,
		fetchResponse: &kkComps.EventGatewayInfo{
			ID:     "f3b8c0d1-9a2e-4f12-8d3c-1e4a5b6c7d8e",
			Name:   "production-egw",
			Labels: map[string]string{"env": "prod"},
		},
		listResponse: []kkComps.EventGatewayInfo{
			{
				ID:     "f3b8c0d1-9a2e-4f12-8d3c-1e4a5b6c7d8e",
				Name:   "production-egw",
				Labels: map[string]string{"env": "prod"},
			},
		},
	}

	cfg := stubConfig{pageSize: 50}

	result, err := adoptEventGatewayControlPlane(helper, egw, cfg, "team-events", "production-egw")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "event_gateway", result.ResourceType)
	assert.Equal(t, "f3b8c0d1-9a2e-4f12-8d3c-1e4a5b6c7d8e", result.ID)
	assert.Equal(t, "production-egw", result.Name)
	assert.Equal(t, "team-events", result.Namespace)

	assert.Equal(t, 1, egw.updateCalls)
	assert.Equal(t, "prod", egw.lastUpdate.Labels["env"])
	assert.Equal(t, "team-events", egw.lastUpdate.Labels[labels.NamespaceKey])

	helper.AssertExpectations(t)
}

func TestAdoptEventGatewayControlPlaneById(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	egw := &egwControlPlaneAPIStub{
		t: t,
		fetchResponse: &kkComps.EventGatewayInfo{
			ID:     "f3b8c0d1-9a2e-4f12-8d3c-1e4a5b6c7d8e",
			Name:   "production-egw",
			Labels: map[string]string{"env": "prod"},
		},
	}

	cfg := stubConfig{pageSize: 50}

	result, err := adoptEventGatewayControlPlane(
		helper,
		egw,
		cfg,
		"team-events",
		"f3b8c0d1-9a2e-4f12-8d3c-1e4a5b6c7d8e",
	)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "event_gateway", result.ResourceType)
	assert.Equal(t, "f3b8c0d1-9a2e-4f12-8d3c-1e4a5b6c7d8e", result.ID)
	assert.Equal(t, "production-egw", result.Name)
	assert.Equal(t, "team-events", result.Namespace)

	assert.Equal(t, 1, egw.updateCalls)
	assert.Equal(t, "prod", egw.lastUpdate.Labels["env"])
	assert.Equal(t, "team-events", egw.lastUpdate.Labels[labels.NamespaceKey])

	helper.AssertExpectations(t)
}

func TestAdoptEventGatewayControlPlaneRejectsExistingNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	egw := &egwControlPlaneAPIStub{
		t: t,
		listResponse: []kkComps.EventGatewayInfo{
			{
				ID:     "f3b8c0d1-9a2e-4f12-8d3c-1e4a5b6c7d8e",
				Name:   "production-egw",
				Labels: map[string]string{labels.NamespaceKey: "existing-team"},
			},
		},
	}

	cfg := stubConfig{pageSize: 50}

	_, err := adoptEventGatewayControlPlane(helper, egw, cfg, "team-events", "production-egw")
	assert.Error(t, err)
	var cfgErr *cmd.ConfigurationError
	assert.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, 0, egw.updateCalls)

	helper.AssertExpectations(t)
}

var (
	_ helpers.EGWControlPlaneAPI = (*egwControlPlaneAPIStub)(nil)
	_ config.Hook                = stubConfig{}
)

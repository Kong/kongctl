package adopt

import (
	"context"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/assert"
)

type portalAPIStub struct {
	t             *testing.T
	fetchResponse *kkComps.PortalResponse
	lastUpdate    kkComps.UpdatePortal
	updateCalls   int
}

func (p *portalAPIStub) ListPortals(context.Context, kkOps.ListPortalsRequest) (*kkOps.ListPortalsResponse, error) {
	return nil, nil
}

func (p *portalAPIStub) GetPortal(_ context.Context, id string) (*kkOps.GetPortalResponse, error) {
	if id != p.fetchResponse.ID {
		p.t.Fatalf("unexpected portal id: %s", id)
	}
	resp := &kkOps.GetPortalResponse{
		PortalResponse: p.fetchResponse,
	}
	return resp, nil
}

func (p *portalAPIStub) CreatePortal(context.Context, kkComps.CreatePortal) (*kkOps.CreatePortalResponse, error) {
	p.t.Fatalf("unexpected CreatePortal call")
	return nil, nil
}

func (p *portalAPIStub) UpdatePortal(
	_ context.Context,
	id string,
	portal kkComps.UpdatePortal,
) (*kkOps.UpdatePortalResponse, error) {
	if id != p.fetchResponse.ID {
		p.t.Fatalf("unexpected portal id: %s", id)
	}
	p.updateCalls++
	p.lastUpdate = portal

	labels := make(map[string]string)
	for k, v := range portal.Labels {
		if v != nil {
			labels[k] = *v
		}
	}

	resp := &kkOps.UpdatePortalResponse{
		PortalResponse: &kkComps.PortalResponse{
			ID:        p.fetchResponse.ID,
			Name:      p.fetchResponse.Name,
			Labels:    labels,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	return resp, nil
}

func (p *portalAPIStub) DeletePortal(context.Context, string, bool) (*kkOps.DeletePortalResponse, error) {
	p.t.Fatalf("unexpected DeletePortal call")
	return nil, nil
}

func TestAdoptPortalAssignsNamespaceLabel(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	existingLabels := map[string]string{"team": "platform"}
	portal := &portalAPIStub{
		t: t,
		fetchResponse: &kkComps.PortalResponse{
			ID:        "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
			Name:      "dev-portal",
			Labels:    existingLabels,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	result, err := adoptPortal(helper, portal, stubConfig{pageSize: 50}, "team-alpha", portal.fetchResponse.ID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "portal", result.ResourceType)
	assert.Equal(t, portal.fetchResponse.ID, result.ID)
	assert.Equal(t, "dev-portal", result.Name)
	assert.Equal(t, "team-alpha", result.Namespace)

	assert.Equal(t, 1, portal.updateCalls)
	if assert.NotNil(t, portal.lastUpdate.Labels) {
		assert.Equal(t, "platform", derefString(portal.lastUpdate.Labels["team"]))
		assert.Equal(t, "team-alpha", derefString(portal.lastUpdate.Labels[labels.NamespaceKey]))
	}

	helper.AssertExpectations(t)
}

func TestAdoptPortalRejectsExistingNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	portal := &portalAPIStub{
		t: t,
		fetchResponse: &kkComps.PortalResponse{
			ID:     "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
			Name:   "dev-portal",
			Labels: map[string]string{labels.NamespaceKey: "existing"},
		},
	}

	_, err := adoptPortal(helper, portal, stubConfig{pageSize: 50}, "team-alpha", portal.fetchResponse.ID)
	assert.Error(t, err)
	var cfgErr *cmd.ConfigurationError
	assert.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, 0, portal.updateCalls)

	helper.AssertExpectations(t)
}

func TestAdoptPortalDefaultsPageSize(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	portal := &portalAPIStub{
		t: t,
		fetchResponse: &kkComps.PortalResponse{
			ID:        "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
			Name:      "dev-portal",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	result, err := adoptPortal(helper, portal, stubConfig{pageSize: 0}, "default", portal.fetchResponse.ID)
	assert.NoError(t, err)
	assert.Equal(t, "default", result.Namespace)
	assert.Equal(t, 1, portal.updateCalls)

	helper.AssertExpectations(t)
}

func derefString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

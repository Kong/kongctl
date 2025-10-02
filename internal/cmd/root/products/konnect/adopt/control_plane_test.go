package adopt

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/assert"
)

type controlPlaneAPIStub struct {
	t          *testing.T
	control    *kkComps.ControlPlane
	lastUpdate kkComps.UpdateControlPlaneRequest
	updateCall int
}

func (c *controlPlaneAPIStub) ListControlPlanes(
	context.Context,
	kkOps.ListControlPlanesRequest,
	...kkOps.Option,
) (*kkOps.ListControlPlanesResponse, error) {
	return nil, nil
}

func (c *controlPlaneAPIStub) CreateControlPlane(
	context.Context,
	kkComps.CreateControlPlaneRequest,
	...kkOps.Option,
) (*kkOps.CreateControlPlaneResponse, error) {
	c.t.Fatalf("unexpected CreateControlPlane call")
	return nil, nil
}

func (c *controlPlaneAPIStub) GetControlPlane(
	_ context.Context,
	id string,
	_ ...kkOps.Option,
) (*kkOps.GetControlPlaneResponse, error) {
	if id != c.control.ID {
		c.t.Fatalf("unexpected control plane id: %s", id)
	}
	return &kkOps.GetControlPlaneResponse{ControlPlane: c.control}, nil
}

func (c *controlPlaneAPIStub) UpdateControlPlane(
	_ context.Context,
	id string,
	req kkComps.UpdateControlPlaneRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateControlPlaneResponse, error) {
	if id != c.control.ID {
		c.t.Fatalf("unexpected control plane id: %s", id)
	}
	c.updateCall++
	c.lastUpdate = req

	resp := &kkOps.UpdateControlPlaneResponse{
		ControlPlane: &kkComps.ControlPlane{
			ID:     c.control.ID,
			Name:   c.control.Name,
			Labels: req.Labels,
		},
	}
	return resp, nil
}

func (c *controlPlaneAPIStub) DeleteControlPlane(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.DeleteControlPlaneResponse, error) {
	c.t.Fatalf("unexpected DeleteControlPlane call")
	return nil, nil
}

func TestAdoptControlPlaneAssignsNamespaceLabel(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	stub := &controlPlaneAPIStub{
		t: t,
		control: &kkComps.ControlPlane{
			ID:     "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
			Name:   "runtime",
			Labels: map[string]string{"env": "prod"},
		},
	}

	result, err := adoptControlPlane(helper, stub, "team-alpha", stub.control.ID)
	assert.NoError(t, err)
	assert.Equal(t, "control_plane", result.ResourceType)
	assert.Equal(t, stub.control.ID, result.ID)
	assert.Equal(t, "runtime", result.Name)
	assert.Equal(t, "team-alpha", result.Namespace)

	assert.Equal(t, 1, stub.updateCall)
	assert.Equal(t, "prod", stub.lastUpdate.Labels["env"])
	assert.Equal(t, "team-alpha", stub.lastUpdate.Labels[labels.NamespaceKey])

	helper.AssertExpectations(t)
}

func TestAdoptControlPlaneRejectsExistingNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	stub := &controlPlaneAPIStub{
		t: t,
		control: &kkComps.ControlPlane{
			ID:     "22cd8a0b-72e7-4212-9099-0764f8e9c5ac",
			Name:   "runtime",
			Labels: map[string]string{labels.NamespaceKey: "existing"},
		},
	}

	_, err := adoptControlPlane(helper, stub, "team-alpha", stub.control.ID)
	assert.Error(t, err)
	var cfgErr *cmd.ConfigurationError
	assert.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, 0, stub.updateCall)

	helper.AssertExpectations(t)
}

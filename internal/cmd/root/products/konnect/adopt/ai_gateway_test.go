package adopt

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/declarative/labels"
	helpers "github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type adoptAIGatewayAPIStub struct {
	t          *testing.T
	gateways   []kkComps.AIGateway
	lastUpdate kkComps.UpdateAIGatewayRequest
	updateID   string
}

func (s *adoptAIGatewayAPIStub) ListAiGateways(
	_ context.Context,
	_ *int64,
	_ *int64,
	_ ...kkOps.Option,
) (*kkOps.ListAiGatewaysResponse, error) {
	return &kkOps.ListAiGatewaysResponse{
		ListAIGatewaysResponse: &kkComps.ListAIGatewaysResponse{
			Data: s.gateways,
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: float64(len(s.gateways))},
			},
		},
	}, nil
}

func (s *adoptAIGatewayAPIStub) CreateAiGateway(
	context.Context,
	kkComps.CreateAIGatewayRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayResponse, error) {
	s.t.Fatalf("unexpected CreateAiGateway call")
	return nil, nil
}

func (s *adoptAIGatewayAPIStub) GetAiGateway(
	_ context.Context,
	gatewayID string,
	_ ...kkOps.Option,
) (*kkOps.GetAiGatewayResponse, error) {
	for _, gateway := range s.gateways {
		if gateway.ID == gatewayID {
			gatewayCopy := gateway
			return &kkOps.GetAiGatewayResponse{AIGateway: &gatewayCopy}, nil
		}
	}
	return &kkOps.GetAiGatewayResponse{}, nil
}

func (s *adoptAIGatewayAPIStub) UpdateAiGateway(
	_ context.Context,
	gatewayID string,
	req kkComps.UpdateAIGatewayRequest,
	_ ...kkOps.Option,
) (*kkOps.UpdateAiGatewayResponse, error) {
	s.updateID = gatewayID
	s.lastUpdate = req
	updated := kkComps.AIGateway{
		ID:          gatewayID,
		DisplayName: req.DisplayName,
		Name:        req.Name,
		Description: req.Description,
		ProxyUrls:   req.ProxyUrls,
		Labels:      req.Labels,
	}
	return &kkOps.UpdateAiGatewayResponse{AIGateway: &updated}, nil
}

func (s *adoptAIGatewayAPIStub) DeleteAiGateway(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayResponse, error) {
	s.t.Fatalf("unexpected DeleteAiGateway call")
	return nil, nil
}

func TestAdoptAIGatewayAssignsNamespacePreservingAPIName(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background()).Times(2)

	id := "22cd8a0b-72e7-4212-9099-0764f8e9c5ac"
	description := "AI Gateway for e2e adoption"
	stub := &adoptAIGatewayAPIStub{
		t: t,
		gateways: []kkComps.AIGateway{
			{
				ID:          id,
				DisplayName: "AI Gateway Adopt E2E 1",
				Name:        "ai-gateway-adopt-e2e-1",
				Description: &description,
				Labels: map[string]string{
					"team": "platform",
				},
			},
		},
	}

	result, err := adoptAIGateway(
		helper,
		stub,
		stubConfig{pageSize: 50},
		"team-alpha",
		false,
		"AI Gateway Adopt E2E 1",
	)
	require.NoError(t, err)
	assert.Equal(t, "ai_gateway", result.ResourceType)
	assert.Equal(t, id, result.ID)
	assert.Equal(t, "AI Gateway Adopt E2E 1", result.Name)
	assert.Equal(t, "team-alpha", result.Namespace)
	assert.Equal(t, id, stub.updateID)
	assert.Equal(t, "AI Gateway Adopt E2E 1", stub.lastUpdate.DisplayName)
	assert.Equal(t, "ai-gateway-adopt-e2e-1", stub.lastUpdate.Name)
	assert.Equal(t, "platform", stub.lastUpdate.Labels["team"])
	assert.Equal(t, "team-alpha", stub.lastUpdate.Labels[labels.NamespaceKey])

	helper.AssertExpectations(t)
}

var _ helpers.AIGatewayAPI = (*adoptAIGatewayAPIStub)(nil)

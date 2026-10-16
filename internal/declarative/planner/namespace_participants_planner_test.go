package planner

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestExternalOnlyDefaultDoesNotExpandSyncNamespaces(t *testing.T) {
	file := filepath.Join(t.TempDir(), "external.yaml")
	require.NoError(t, os.WriteFile(file, []byte(`
_defaults:
  kongctl:
    namespace: platform
portals:
  - ref: shared-portal
    _external:
      selector:
        matchFields:
          name: shared-portal
`), 0o600))
	rs, err := loader.New().LoadFromSources([]loader.Source{{Type: loader.SourceTypeFile, Path: file}}, false)
	require.NoError(t, err)
	require.NoError(t, validator.NewNamespaceValidator().ValidateNamespaceRequirement(rs, validator.NamespaceRequirement{
		Mode: validator.NamespaceRequirementSpecific, AllowedNamespaces: []string{"platform"},
	}))

	portalAPI := new(MockPortalAPI)
	portalAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		StatusCode: 200,
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.ListPortalsResponsePortal{
				newListPortal("external-id", "shared-portal", nil),
				newListPortal("managed-id", "managed-portal", map[string]string{labels.NamespaceKey: "platform"}),
			},
			Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 2}},
		},
	}, nil)
	client := state.NewClient(state.ClientConfig{PortalAPI: portalAPI})
	p := NewPlanner(client, slog.New(slog.NewTextHandler(io.Discard, nil)))
	plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	assert.Empty(t, plan.Changes, "an external-only default must not prune managed portals in that namespace")
	portalAPI.AssertExpectations(t)
}

func TestGetResourceNamespacesExternalAIGatewayMapsToExternal(t *testing.T) {
	planner := &Planner{}
	gateway := resources.AIGatewayResource{External: &resources.ExternalBlock{ID: "ext-gateway"}}
	gateway.Ref = "gateway-1"

	rs := &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{gateway}}

	assert.Equal(t, []string{resources.NamespaceExternal}, planner.getResourceNamespaces(rs))
}

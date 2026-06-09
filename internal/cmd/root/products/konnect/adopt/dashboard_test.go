package adopt

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	helpers "github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnalyticsCmdAddsDashboardChild(t *testing.T) {
	cmd, err := NewAnalyticsCmd(verbs.Adopt, nil, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, cmd.Aliases, "analytic")

	dashboardCmd, _, err := cmd.Find([]string{"dashboard"})
	require.NoError(t, err)
	require.NotNil(t, dashboardCmd)
	assert.Equal(t, "dashboard <dashboard-id|dashboard-name>", dashboardCmd.Use)
}

type adoptDashboardAPIStub struct {
	t          *testing.T
	dashboards []kkComps.DashboardResponse
	lastUpdate kkComps.DashboardUpdateRequest
	updateID   string
}

func (s *adoptDashboardAPIStub) DashboardsList(
	_ context.Context,
	_ kkOps.DashboardsListRequest,
	_ ...kkOps.Option,
) (*kkOps.DashboardsListResponse, error) {
	return &kkOps.DashboardsListResponse{
		Object: &kkOps.DashboardsListResponseBody{
			Meta: &kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: float64(len(s.dashboards))},
			},
			Data: s.dashboards,
		},
	}, nil
}

func (s *adoptDashboardAPIStub) DashboardsCreate(
	context.Context,
	kkComps.DashboardUpdateRequest,
	...kkOps.Option,
) (*kkOps.DashboardsCreateResponse, error) {
	s.t.Fatalf("unexpected DashboardsCreate call")
	return nil, nil
}

func (s *adoptDashboardAPIStub) DashboardsGet(
	_ context.Context,
	dashboardID string,
	_ ...kkOps.Option,
) (*kkOps.DashboardsGetResponse, error) {
	for _, dashboard := range s.dashboards {
		if dashboard.ID != nil && *dashboard.ID == dashboardID {
			dashboardCopy := dashboard
			return &kkOps.DashboardsGetResponse{DashboardResponse: &dashboardCopy}, nil
		}
	}
	return &kkOps.DashboardsGetResponse{}, nil
}

func (s *adoptDashboardAPIStub) DashboardsUpdate(
	_ context.Context,
	dashboardID string,
	req kkComps.DashboardUpdateRequest,
	_ ...kkOps.Option,
) (*kkOps.DashboardsUpdateResponse, error) {
	s.updateID = dashboardID
	s.lastUpdate = req
	updated := kkComps.DashboardResponse{
		ID:         &dashboardID,
		Name:       req.Name,
		Definition: req.Definition,
		Labels:     req.Labels,
	}
	return &kkOps.DashboardsUpdateResponse{DashboardResponse: &updated}, nil
}

func (s *adoptDashboardAPIStub) DashboardsDelete(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.DashboardsDeleteResponse, error) {
	s.t.Fatalf("unexpected DashboardsDelete call")
	return nil, nil
}

func TestAdoptDashboardAssignsNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background()).Times(2)

	id := "22cd8a0b-72e7-4212-9099-0764f8e9c5ac"
	stub := &adoptDashboardAPIStub{
		t: t,
		dashboards: []kkComps.DashboardResponse{
			{
				ID:         &id,
				Name:       "API Summary",
				Definition: kkComps.Dashboard{Tiles: []kkComps.Tile{}},
				Labels: map[string]string{
					"team": "platform",
				},
			},
		},
	}

	result, err := adoptDashboard(helper, stub, stubConfig{pageSize: 50}, "team-alpha", false, "API Summary")
	require.NoError(t, err)
	assert.Equal(t, "dashboard", result.ResourceType)
	assert.Equal(t, id, result.ID)
	assert.Equal(t, "API Summary", result.Name)
	assert.Equal(t, "team-alpha", result.Namespace)
	assert.Equal(t, id, stub.updateID)
	assert.Equal(t, "API Summary", stub.lastUpdate.Name)
	assert.Empty(t, stub.lastUpdate.Definition.Tiles)
	assert.Equal(t, "platform", stub.lastUpdate.Labels["team"])
	assert.Equal(t, "team-alpha", stub.lastUpdate.Labels[labels.NamespaceKey])

	helper.AssertExpectations(t)
}

func TestAdoptDashboardRejectsExistingNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	id := "22cd8a0b-72e7-4212-9099-0764f8e9c5ac"
	stub := &adoptDashboardAPIStub{
		t: t,
		dashboards: []kkComps.DashboardResponse{
			{
				ID:   &id,
				Name: "API Summary",
				Labels: map[string]string{
					labels.NamespaceKey: "existing",
				},
			},
		},
	}

	_, err := adoptDashboard(helper, stub, stubConfig{pageSize: 50}, "team-alpha", false, "API Summary")
	assert.Error(t, err)
	var cfgErr *cmd.ConfigurationError
	assert.ErrorAs(t, err, &cfgErr)
	assert.Empty(t, stub.updateID)

	helper.AssertExpectations(t)
}

func TestAdoptDashboardOverwritesExistingNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background()).Times(2)

	id := "22cd8a0b-72e7-4212-9099-0764f8e9c5ac"
	stub := &adoptDashboardAPIStub{
		t: t,
		dashboards: []kkComps.DashboardResponse{
			{
				ID:         &id,
				Name:       "API Summary",
				Definition: kkComps.Dashboard{Tiles: []kkComps.Tile{}},
				Labels: map[string]string{
					"team":              "platform",
					labels.NamespaceKey: "existing",
				},
			},
		},
	}

	result, err := adoptDashboard(helper, stub, stubConfig{pageSize: 50}, "team-alpha", true, "API Summary")
	require.NoError(t, err)
	assert.Equal(t, "team-alpha", result.Namespace)
	assert.Equal(t, id, stub.updateID)
	assert.Equal(t, "platform", stub.lastUpdate.Labels["team"])
	assert.Equal(t, "team-alpha", stub.lastUpdate.Labels[labels.NamespaceKey])

	helper.AssertExpectations(t)
}

func TestResolveDashboardRejectsDuplicateNames(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	firstID := "22cd8a0b-72e7-4212-9099-0764f8e9c5ac"
	secondID := "6d306b92-8fb9-4b18-b9f4-69a6296344b7"
	stub := &adoptDashboardAPIStub{
		t: t,
		dashboards: []kkComps.DashboardResponse{
			{ID: &firstID, Name: "Duplicate"},
			{ID: &secondID, Name: "Duplicate"},
		},
	}

	_, err := resolveDashboard(helper, stub, stubConfig{pageSize: 50}, "Duplicate")
	assert.Error(t, err)
	var cfgErr *cmd.ConfigurationError
	assert.ErrorAs(t, err, &cfgErr)
	assert.Contains(t, err.Error(), "use the dashboard ID")

	helper.AssertExpectations(t)
}

var (
	_ helpers.DashboardsAPI = (*adoptDashboardAPIStub)(nil)
	_ config.Hook           = stubConfig{}
)

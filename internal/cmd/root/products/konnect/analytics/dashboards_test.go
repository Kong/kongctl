package analytics

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type dashboardAPIStub struct {
	pages [][]kkComps.DashboardResponse
}

func (s *dashboardAPIStub) DashboardsList(
	_ context.Context,
	request kkOps.DashboardsListRequest,
	_ ...kkOps.Option,
) (*kkOps.DashboardsListResponse, error) {
	pageNumber := int64(1)
	if request.PageNumber != nil {
		pageNumber = *request.PageNumber
	}
	index := int(pageNumber - 1)
	if index < 0 || index >= len(s.pages) {
		return &kkOps.DashboardsListResponse{
			Object: &kkOps.DashboardsListResponseBody{
				Meta: &kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(s.total())}},
			},
		}, nil
	}

	return &kkOps.DashboardsListResponse{
		Object: &kkOps.DashboardsListResponseBody{
			Meta: &kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(s.total())}},
			Data: s.pages[index],
		},
	}, nil
}

func (s *dashboardAPIStub) DashboardsCreate(
	context.Context,
	kkComps.DashboardUpdateRequest,
	...kkOps.Option,
) (*kkOps.DashboardsCreateResponse, error) {
	return nil, nil
}

func (s *dashboardAPIStub) DashboardsGet(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.DashboardsGetResponse, error) {
	return nil, nil
}

func (s *dashboardAPIStub) DashboardsUpdate(
	context.Context,
	string,
	kkComps.DashboardUpdateRequest,
	...kkOps.Option,
) (*kkOps.DashboardsUpdateResponse, error) {
	return nil, nil
}

func (s *dashboardAPIStub) DashboardsDelete(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.DashboardsDeleteResponse, error) {
	return nil, nil
}

func (s *dashboardAPIStub) total() int {
	total := 0
	for _, page := range s.pages {
		total += len(page)
	}
	return total
}

type dashboardTestHelper struct {
	cfg config.Hook
	cmd *cobra.Command
}

func (h dashboardTestHelper) GetCmd() *cobra.Command { return h.cmd }
func (h dashboardTestHelper) GetArgs() []string      { return nil }
func (h dashboardTestHelper) GetVerb() (verbs.VerbValue, error) {
	return verbs.Get, nil
}

func (h dashboardTestHelper) GetProduct() (products.ProductValue, error) {
	return products.ProductKonnect, nil
}

func (h dashboardTestHelper) GetStreams() *iostreams.IOStreams {
	return iostreams.NewTestIOStreamsOnly()
}

func (h dashboardTestHelper) GetConfig() (config.Hook, error) {
	return h.cfg, nil
}

func (h dashboardTestHelper) GetOutputFormat() (cmdCommon.OutputFormat, error) {
	return cmdCommon.TEXT, nil
}

func (h dashboardTestHelper) GetLogger() (*slog.Logger, error) {
	return slog.Default(), nil
}

func (h dashboardTestHelper) GetBuildInfo() (*build.Info, error) {
	return nil, nil
}

func (h dashboardTestHelper) GetContext() context.Context {
	return context.Background()
}

func (h dashboardTestHelper) GetKonnectSDK(config.Hook, *slog.Logger) (helpers.SDKAPI, error) {
	return nil, nil
}

func newDashboardTestHelper() dashboardTestHelper {
	cfg := config.BuildProfiledConfig("default", "", viper.New())
	cfg.Set(common.RequestPageSizeConfigPath, 1)
	return dashboardTestHelper{
		cfg: cfg,
		cmd: &cobra.Command{Use: "dashboard"},
	}
}

func TestNewAnalyticsCmdAddsDashboardAliases(t *testing.T) {
	analyticsCmd, err := NewAnalyticsCmd(verbs.Get, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !slicesContains(analyticsCmd.Aliases, "analytic") {
		t.Fatalf("expected analytic alias in %v", analyticsCmd.Aliases)
	}

	dashboardCmd, _, err := analyticsCmd.Find([]string{"dashboards"})
	if err != nil {
		t.Fatalf("expected dashboard alias to resolve, got %v", err)
	}
	if dashboardCmd == nil || dashboardCmd.Use != "dashboard [name]" {
		t.Fatalf("expected dashboard command, got %#v", dashboardCmd)
	}
}

func TestRunDashboardListPaginates(t *testing.T) {
	api := &dashboardAPIStub{
		pages: [][]kkComps.DashboardResponse{
			{{Name: "Dashboard One"}},
			{{Name: "Dashboard Two"}},
		},
	}

	dashboards, err := runDashboardList(api, newDashboardTestHelper(), newDashboardTestHelper().cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(dashboards) != 2 {
		t.Fatalf("expected 2 dashboards, got %d", len(dashboards))
	}
}

func TestRunDashboardGetByName(t *testing.T) {
	api := &dashboardAPIStub{
		pages: [][]kkComps.DashboardResponse{
			{
				{Name: "API Summary"},
				{Name: "Other"},
			},
		},
	}

	dashboard, err := runDashboardGetByName("API Summary", api, newDashboardTestHelper(), newDashboardTestHelper().cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dashboard.Name != "API Summary" {
		t.Fatalf("expected API Summary, got %q", dashboard.Name)
	}
}

func TestRunDashboardGetByNameRejectsDuplicates(t *testing.T) {
	api := &dashboardAPIStub{
		pages: [][]kkComps.DashboardResponse{
			{
				{Name: "API Summary"},
				{Name: "API Summary"},
			},
		},
	}

	_, err := runDashboardGetByName("API Summary", api, newDashboardTestHelper(), newDashboardTestHelper().cfg)
	if err == nil {
		t.Fatal("expected duplicate name error")
	}
	if !strings.Contains(err.Error(), "matches 2 dashboards") {
		t.Fatalf("expected duplicate count error, got %v", err)
	}
}

func TestDashboardDetailView(t *testing.T) {
	id := "d67a4203-b1e8-4631-a626-5fe7c55efe88"
	dashboard := kkComps.DashboardResponse{
		ID:   &id,
		Name: "API Summary",
		Definition: kkComps.Dashboard{
			Tiles:         []kkComps.Tile{{}},
			PresetFilters: []kkComps.AllFilterItems{{}},
		},
		Labels: map[string]string{"team": "platform"},
	}

	detail := dashboardDetailView(dashboard)
	for _, expected := range []string{
		"id: d67a4203-b1e8-4631-a626-5fe7c55efe88",
		"name: API Summary",
		"tiles: 1",
		"preset_filters: 1",
		"labels: team=platform",
	} {
		if !strings.Contains(detail, expected) {
			t.Fatalf("expected detail to contain %q, got:\n%s", expected, detail)
		}
	}
}

func slicesContains(values []string, value string) bool {
	return slices.Contains(values, value)
}

var (
	_ cmd.Helper            = (*dashboardTestHelper)(nil)
	_ helpers.DashboardsAPI = (*dashboardAPIStub)(nil)
)

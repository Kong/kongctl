package authstrategy

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	configtest "github.com/kong/kongctl/test/config"
	"github.com/stretchr/testify/require"
)

type authStrategiesListStub struct {
	t     *testing.T
	pages [][]kkComps.AppAuthStrategy
	total int
}

func (s *authStrategiesListStub) ListAppAuthStrategies(
	_ context.Context,
	req kkOps.ListAppAuthStrategiesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAppAuthStrategiesResponse, error) {
	pageNumber := int(*req.PageNumber)
	if pageNumber < 1 || pageNumber > len(s.pages) {
		s.t.Fatalf("unexpected page number requested: %d", pageNumber)
	}
	return &kkOps.ListAppAuthStrategiesResponse{
		ListAppAuthStrategiesResponse: &kkComps.ListAppAuthStrategiesResponse{
			Data: s.pages[pageNumber-1],
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: float64(s.total)},
			},
		},
	}, nil
}

func (s *authStrategiesListStub) GetAppAuthStrategy(
	context.Context, string,
) (*kkOps.GetAppAuthStrategyResponse, error) {
	s.t.Fatalf("unexpected GetAppAuthStrategy call")
	return nil, nil
}

func (s *authStrategiesListStub) CreateAppAuthStrategy(
	context.Context, kkComps.CreateAppAuthStrategyRequest,
) (*kkOps.CreateAppAuthStrategyResponse, error) {
	s.t.Fatalf("unexpected CreateAppAuthStrategy call")
	return nil, nil
}

func (s *authStrategiesListStub) UpdateAppAuthStrategy(
	context.Context, string, kkComps.UpdateAppAuthStrategyRequest,
) (*kkOps.UpdateAppAuthStrategyResponse, error) {
	s.t.Fatalf("unexpected UpdateAppAuthStrategy call")
	return nil, nil
}

func (s *authStrategiesListStub) DeleteAppAuthStrategy(
	context.Context, string,
) (*kkOps.DeleteAppAuthStrategyResponse, error) {
	s.t.Fatalf("unexpected DeleteAppAuthStrategy call")
	return nil, nil
}

func keyAuthStrategyNamed(id, name string) kkComps.AppAuthStrategy {
	return kkComps.CreateAppAuthStrategyKeyAuth(
		kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse{
			ID:           id,
			Name:         name,
			DisplayName:  name,
			StrategyType: kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyStrategyType("key_auth"),
			Configs: kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyConfigs{
				KeyAuth: kkComps.AppAuthStrategyConfigKeyAuth{},
			},
		},
	)
}

func newTestHelper(t *testing.T) *cmd.MockHelper {
	t.Helper()
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(t.Context())
	return helper
}

func newTestCfg(pageSize int) *configtest.MockConfigHook {
	return &configtest.MockConfigHook{
		GetIntMock: func(key string) int {
			if key == common.RequestPageSizeConfigPath {
				return pageSize
			}
			return 0
		},
	}
}

func TestRunListByNameFindsMatchAcrossPages(t *testing.T) {
	stub := &authStrategiesListStub{
		t: t,
		pages: [][]kkComps.AppAuthStrategy{
			{keyAuthStrategyNamed("id-1", "one")},
			{keyAuthStrategyNamed("id-2", "two")},
		},
		total: 2,
	}

	result, err := runListByName("two", "", stub, newTestHelper(t), newTestCfg(1))
	require.NoError(t, err)
	require.Equal(t, "id-2", extractAuthStrategyVariant(*result).id)
}

func TestRunListByNameReturnsNotFoundWhenNoMatch(t *testing.T) {
	stub := &authStrategiesListStub{
		t: t,
		pages: [][]kkComps.AppAuthStrategy{
			{keyAuthStrategyNamed("id-1", "one")},
			{keyAuthStrategyNamed("id-2", "two")},
		},
		total: 2,
	}

	result, err := runListByName("missing", "", stub, newTestHelper(t), newTestCfg(1))
	require.Error(t, err)
	require.Nil(t, result)
}

func TestRunListReturnsAllPages(t *testing.T) {
	stub := &authStrategiesListStub{
		t: t,
		pages: [][]kkComps.AppAuthStrategy{
			{keyAuthStrategyNamed("id-1", "one")},
			{keyAuthStrategyNamed("id-2", "two")},
		},
		total: 2,
	}

	result, err := runList("", stub, newTestHelper(t), newTestCfg(1))
	require.NoError(t, err)
	require.Len(t, result, 2)
}

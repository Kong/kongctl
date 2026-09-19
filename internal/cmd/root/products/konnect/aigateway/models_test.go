package aigateway

import (
	"context"
	"errors"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

type paginatedModelAPI struct {
	helpers.AIGatewayModelAPI
	list func(kkOps.ListAiGatewayModelsRequest) (*kkOps.ListAiGatewayModelsResponse, error)
}

type modelPagingConfig struct {
	aiGatewayTestConfig
}

func (modelPagingConfig) GetInt(string) int { return 0 }

func TestAIGatewayModelsDefaultPageSizeAndEmptyCollection(t *testing.T) {
	command := &cobra.Command{Use: "view"}
	command.SetContext(context.WithValue(t.Context(), config.ConfigKey, modelPagingConfig{}))
	calls := 0
	api := paginatedModelAPI{list: func(request kkOps.ListAiGatewayModelsRequest) (
		*kkOps.ListAiGatewayModelsResponse, error,
	) {
		calls++
		require.Equal(t, 1, calls)
		require.Equal(t, int64(common.DefaultRequestPageSize), *request.PageSize)
		require.Nil(t, request.PageAfter)
		return &kkOps.ListAiGatewayModelsResponse{ListAIGatewayModelsResponse: &kkComps.ListAIGatewayModelsResponse{}}, nil
	}}
	models, err := listAIGatewayModels(cmd.BuildHelper(command, nil), api, "gateway")
	require.NoError(t, err)
	require.Empty(t, models)
	require.Equal(t, 1, calls)
}

func TestAIGatewayModelsRejectIncompletePagination(t *testing.T) {
	for _, tc := range []struct {
		name    string
		next    []string
		failure string
		want    string
	}{
		{name: "nil response", next: []string{"first"}, failure: "nil", want: "empty response"},
		{name: "missing body", next: []string{"first"}, failure: "body", want: "empty response"},
		{name: "later page error", next: []string{"first"}, failure: "error", want: "page unavailable"},
		{name: "repeated cursor", next: []string{"first", "first"}, want: "previously seen cursor"},
		{name: "cursor cycle", next: []string{"first", "second", "first"}, want: "previously seen cursor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := &cobra.Command{Use: "view"}
			command.SetContext(context.WithValue(t.Context(), config.ConfigKey, aiGatewayTestConfig{}))
			calls := 0
			api := paginatedModelAPI{list: func(_ kkOps.ListAiGatewayModelsRequest) (*kkOps.ListAiGatewayModelsResponse, error) {
				calls++
				require.LessOrEqual(t, calls, len(tc.next)+1)
				if calls > len(tc.next) {
					switch tc.failure {
					case "nil":
						return nil, nil
					case "body":
						return &kkOps.ListAiGatewayModelsResponse{}, nil
					default:
						return nil, errors.New("page unavailable")
					}
				}
				return &kkOps.ListAiGatewayModelsResponse{ListAIGatewayModelsResponse: &kkComps.ListAIGatewayModelsResponse{
					Data: []kkComps.AIGatewayModel{kkComps.CreateAIGatewayModelModel(
						kkComps.AIGatewayModelAIGatewayModelModel{ID: "model"},
					)},
					Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{Next: &tc.next[calls-1]}},
				}}, nil
			}}
			models, err := listAIGatewayModels(cmd.BuildHelper(command, nil), api, "gateway")
			require.ErrorContains(t, err, tc.want)
			require.Nil(t, models, "a failed later page must not appear to be a complete collection")
		})
	}
}

func (s paginatedModelAPI) ListAiGatewayModels(
	_ context.Context, request kkOps.ListAiGatewayModelsRequest, _ ...kkOps.Option,
) (*kkOps.ListAiGatewayModelsResponse, error) {
	return s.list(request)
}

func TestAIGatewayModelViewIncludesEveryPage(t *testing.T) {
	for _, next := range []string{
		"opaque+cursor/value==",
		"/v3/ai-gateways/gateway/models?page%5Bafter%5D=opaque%2Bcursor%2Fvalue%3D%3D",
	} {
		t.Run(next, func(t *testing.T) {
			command := &cobra.Command{Use: "view"}
			command.SetContext(context.WithValue(t.Context(), config.ConfigKey, aiGatewayTestConfig{}))
			helper := cmd.BuildHelper(command, nil)
			calls := 0
			api := paginatedModelAPI{list: func(request kkOps.ListAiGatewayModelsRequest) (
				*kkOps.ListAiGatewayModelsResponse, error,
			) {
				calls++
				require.LessOrEqual(t, calls, 2)
				require.Equal(t, "gateway", request.GatewayID)
				require.Equal(t, int64(50), *request.PageSize)
				body := &kkComps.ListAIGatewayModelsResponse{}
				count := 3
				if calls == 1 {
					require.Nil(t, request.PageAfter)
					body.Meta.Page.Next = &next
					count = 50
				} else {
					require.Equal(t, "opaque+cursor/value==", *request.PageAfter)
				}
				for i := range count {
					name := fmt.Sprintf("model-%d-%d", calls, i)
					body.Data = append(body.Data, kkComps.CreateAIGatewayModelModel(
						kkComps.AIGatewayModelAIGatewayModelModel{ID: name, Name: name},
					))
				}
				return &kkOps.ListAiGatewayModelsResponse{ListAIGatewayModelsResponse: body}, nil
			}}

			models, err := listAIGatewayModels(helper, api, "gateway")
			require.NoError(t, err)
			require.Equal(t, 2, calls)
			view := buildAIGatewayModelChildView(models)
			require.Len(t, view.Rows, 53)
			require.Equal(t, "model-2-2", view.Rows[52][0])
			require.Contains(t, view.DetailRenderer(52), "name: model-2-2")
			require.Equal(t, &models[52], view.DetailContext(52))
		})
	}
}

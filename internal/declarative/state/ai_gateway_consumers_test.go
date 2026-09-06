package state

import (
	"context"
	"errors"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type paginatedAIGatewayConsumersAPI struct {
	helpers.AIGatewayConsumersAPI
	list            func(kkOps.ListAiGatewayConsumersRequest) (*kkOps.ListAiGatewayConsumersResponse, error)
	listCredentials func(
		kkOps.ListAiGatewayConsumerCredentialsRequest,
	) (*kkOps.ListAiGatewayConsumerCredentialsResponse, error)
}

func (a paginatedAIGatewayConsumersAPI) ListAiGatewayConsumerCredentials(
	_ context.Context, req kkOps.ListAiGatewayConsumerCredentialsRequest, _ ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerCredentialsResponse, error) {
	return a.listCredentials(req)
}

func (a paginatedAIGatewayConsumersAPI) ListAiGatewayConsumers(
	_ context.Context, req kkOps.ListAiGatewayConsumersRequest, _ ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumersResponse, error) {
	return a.list(req)
}

func TestListAIGatewayConsumersFollowsBareCursor(t *testing.T) {
	for _, failSecondPage := range []bool{false, true} {
		t.Run(fmt.Sprintf("second-page-error=%t", failSecondPage), func(t *testing.T) {
			const cursor = "abc+def/ghi=="
			pageError := errors.New("second page unavailable")
			calls := 0
			client := NewClient(ClientConfig{AIGatewayConsumersAPI: &paginatedAIGatewayConsumersAPI{
				list: func(req kkOps.ListAiGatewayConsumersRequest) (*kkOps.ListAiGatewayConsumersResponse, error) {
					calls++
					require.LessOrEqual(t, calls, 2, "pagination must terminate")
					require.Equal(t, "gateway-id", req.GatewayID)
					require.EqualValues(t, 100, *req.PageSize)
					start, count := 0, 100
					var next *string
					if calls == 1 {
						require.Nil(t, req.PageAfter)
						next = new(cursor)
					} else {
						require.Equal(t, cursor, *req.PageAfter)
						if failSecondPage {
							return nil, pageError
						}
						start, count = 100, 4
					}
					data := make([]kkComps.AIGatewayConsumer, count)
					for i := range count {
						data[i] = kkComps.AIGatewayConsumer{
							ID: fmt.Sprintf("id-%d", start+i), Name: fmt.Sprintf("consumer-%d", start+i),
							DisplayName: "Shared display name",
						}
					}
					return &kkOps.ListAiGatewayConsumersResponse{
						ListAIGatewayConsumersResponse: &kkComps.ListAIGatewayConsumersResponse{
							Data: data, Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{Next: next}},
						},
					}, nil
				},
			}})
			consumers, err := client.ListAIGatewayConsumers(t.Context(), "gateway-id")
			require.Equal(t, 2, calls)
			if failSecondPage {
				require.ErrorIs(t, err, pageError)
				require.Nil(t, consumers, "a failed page must not return incomplete live state")
				return
			}
			require.NoError(t, err)
			require.Len(t, consumers, 104)
			for i, consumer := range consumers {
				require.Equal(t, fmt.Sprintf("consumer-%d", i), consumer.Name)
			}
		})
	}
}

func TestListAIGatewayConsumerCredentialsFollowsBareCursor(t *testing.T) {
	calls := 0
	client := NewClient(ClientConfig{AIGatewayConsumersAPI: &paginatedAIGatewayConsumersAPI{
		listCredentials: func(
			req kkOps.ListAiGatewayConsumerCredentialsRequest,
		) (*kkOps.ListAiGatewayConsumerCredentialsResponse, error) {
			calls++
			require.LessOrEqual(t, calls, 2)
			require.Equal(t, "gateway-id", req.GatewayID)
			require.Equal(t, "consumer-id", req.ConsumerID)
			var next *string
			if calls == 1 {
				require.Nil(t, req.PageAfter)
				next = new("next-credential")
			} else {
				require.Equal(t, "next-credential", *req.PageAfter)
			}
			return &kkOps.ListAiGatewayConsumerCredentialsResponse{
				ListAIGatewayConsumerCredentialsResponse: &kkComps.ListAIGatewayConsumerCredentialsResponse{
					Data: []kkComps.AIGatewayConsumerCredential{{ID: fmt.Sprintf("credential-%d", calls)}},
					Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{Next: next}},
				},
			}, nil
		},
	}})
	credentials, err := client.ListAIGatewayConsumerCredentials(t.Context(), "gateway-id", "consumer-id")
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Len(t, credentials, 2)
}

func TestListAIGatewayConsumersRejectsCursorCycles(t *testing.T) {
	for _, sequence := range [][]string{{"first", "first"}, {"first", "second", "first"}} {
		t.Run(sequence[1], func(t *testing.T) {
			calls := 0
			client := NewClient(ClientConfig{AIGatewayConsumersAPI: &paginatedAIGatewayConsumersAPI{
				list: func(req kkOps.ListAiGatewayConsumersRequest) (*kkOps.ListAiGatewayConsumersResponse, error) {
					require.Less(t, calls, len(sequence), "pagination must terminate")
					if calls > 0 {
						require.Equal(t, sequence[calls-1], *req.PageAfter)
					}
					next := sequence[calls]
					calls++
					return &kkOps.ListAiGatewayConsumersResponse{
						ListAIGatewayConsumersResponse: &kkComps.ListAIGatewayConsumersResponse{
							Data: []kkComps.AIGatewayConsumer{{ID: "consumer-id"}},
							Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{Next: &next}},
						},
					}, nil
				},
			}})
			consumers, err := client.ListAIGatewayConsumers(t.Context(), "gateway-id")
			require.ErrorContains(t, err, "previously seen cursor")
			require.Nil(t, consumers, "never return incomplete live state")
			require.Equal(t, len(sequence), calls)
		})
	}
}

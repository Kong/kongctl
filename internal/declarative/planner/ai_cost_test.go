package planner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestAICostChannelDeletePlanningReusesRuleObservation(t *testing.T) {
	const budgetID = "4078f559-063b-4f10-9098-9e6553b21d9b"
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.URL.Path != "/v1/ai-cost-management/budgets/"+budgetID+"/notification-rule" {
			t.Errorf("unexpected path %q", request.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"budget":{"id":%q},"channels":[{"id":"referenced"}],"thresholds":[],"managed_by":{}}`, budgetID)
	}))
	defer server.Close()
	api := state.NewAICostAPI(&helpers.KonnectSDK{
		SDK: kkSDK.New(kkSDK.WithServerURL(server.URL), kkSDK.WithClient(server.Client())),
	})
	p := &Planner{
		client:        state.NewClient(state.ClientConfig{AICostAPI: api}),
		resourceCache: newPlanningResourceCache(),
	}
	p.resourceCache.aiCost[resources.ResourceTypeAICostBudget] = []state.AICostObject{
		{FieldID: budgetID, FieldAICostEffect: "limit"},
	}
	plan := &Plan{Changes: []PlannedChange{
		{ID: "rule", Action: ActionDelete, ResourceType: string(resources.ResourceTypeAICostRule), ResourceID: budgetID},
		{
			ID: "channel1", Action: ActionDelete, ResourceType: string(resources.ResourceTypeAICostChannel),
			ResourceID: "referenced",
		},
		{ID: "channel2", Action: ActionDelete, ResourceType: string(resources.ResourceTypeAICostChannel)},
	}}
	require.NoError(t, p.resolveAICostDeletes(t.Context(), plan))
	require.EqualValues(t, 1, calls.Load())
	require.Equal(t, []string{"rule"}, plan.Changes[1].DependsOn)
	require.Empty(t, plan.Changes[2].DependsOn)
}

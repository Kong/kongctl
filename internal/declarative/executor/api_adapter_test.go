package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAPIAdapter_UpdateOperationRemovalAuthorization(t *testing.T) {
	for _, test := range []struct {
		name    string
		execCtx *ExecutionContext
		force   bool
	}{
		{name: "missing context"},
		{name: "missing plan", execCtx: &ExecutionContext{}},
		{name: "not authorized", execCtx: NewExecutionContext(&planner.PlannedChange{})},
		{name: "authorized", force: true, execCtx: NewExecutionContext(&planner.PlannedChange{
			ForceDelete: true,
		})},
	} {
		t.Run(test.name, func(t *testing.T) {
			api := new(MockAPIAPI)
			api.Test(t)
			request := kkComps.UpdateAPIRequest{Version: new("beta.2")}
			api.On("UpdateAPI", mock.Anything, "api-id", request, test.force, mock.Anything).
				Return(&kkOps.UpdateAPIResponse{APIResponseSchema: &kkComps.APIResponseSchema{ID: "api-id"}}, nil).
				Once()
			adapter := NewAPIAdapter(state.NewClient(state.ClientConfig{APIAPI: api}))
			id, err := adapter.Update(t.Context(), "api-id", request, "default", test.execCtx)
			require.NoError(t, err)
			require.Equal(t, "api-id", id)
			api.AssertExpectations(t)
		})
	}
}

func TestAPIAdapter_MapUpdateFieldsPreservesNullAttributeValues(t *testing.T) {
	t.Parallel()

	adapter := NewAPIAdapter(nil)
	execCtx := NewExecutionContext(&planner.PlannedChange{
		Namespace: "default",
	})

	fields := map[string]any{
		"attributes": map[string]any{
			"owner":     nil,
			"lifecycle": nil,
		},
	}

	var update kkComps.UpdateAPIRequest
	require.NoError(t, adapter.MapUpdateFields(context.Background(), execCtx, fields, &update, nil))
	assert.Equal(t, map[string]any{
		"owner":     nil,
		"lifecycle": nil,
	}, update.Attributes)
}

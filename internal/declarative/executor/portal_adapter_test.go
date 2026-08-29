package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortalAdapterMapsSIPREnabled(t *testing.T) {
	tests := []struct {
		name  string
		value bool
	}{
		{name: "enabled", value: true},
		{name: "disabled", value: false},
	}

	adapter := NewPortalAdapter(nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := map[string]any{
				planner.FieldName:        "customer-portal",
				planner.FieldSIPREnabled: tt.value,
			}

			create := kkComps.CreatePortal{}
			err := adapter.MapCreateFields(
				t.Context(),
				&ExecutionContext{},
				fields,
				&create,
			)
			require.NoError(t, err)
			require.NotNil(t, create.SiprEnabled)
			assert.Equal(t, tt.value, *create.SiprEnabled)

			update := kkComps.UpdatePortal{}
			err = adapter.MapUpdateFields(
				t.Context(),
				&ExecutionContext{},
				fields,
				&update,
				nil,
			)
			require.NoError(t, err)
			require.NotNil(t, update.SiprEnabled)
			assert.Equal(t, tt.value, *update.SiprEnabled)
		})
	}
}

func TestPortalAdapterMapUpdateFieldsKeepsPatchPayloadSparse(t *testing.T) {
	adapter := NewPortalAdapter(nil)
	update := kkComps.UpdatePortal{}

	err := adapter.MapUpdateFields(
		t.Context(),
		&ExecutionContext{},
		map[string]any{planner.FieldDisplayName: "Updated portal"},
		&update,
		nil,
	)
	require.NoError(t, err)

	body, err := json.Marshal(update)
	require.NoError(t, err)
	assert.JSONEq(t, `{"display_name":"Updated portal"}`, string(body))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.NotContains(t, payload, planner.FieldRBACEnabled)
	assert.NotContains(t, payload, planner.FieldAutoApproveDevelopers)
}

func TestErrUnresolvedRef(t *testing.T) {
	err := errUnresolvedRef(planner.FieldDefaultApplicationStrategyID, "__REF__:default-strategy#id")

	assert.EqualError(
		t,
		err,
		"unresolved reference for default_application_auth_strategy_id: __REF__:default-strategy#id",
	)
}

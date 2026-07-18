package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortalAuditLogWebhookAdapterMapUpdateFieldsKeepsPatchPayloadSparse(t *testing.T) {
	adapter := NewPortalAuditLogWebhookAdapter(nil)
	update := kkComps.UpdatePortalAuditLogWebhook{}
	destinationID := "11111111-1111-1111-1111-111111111111"
	execCtx := NewExecutionContext(&planner.PlannedChange{
		References: map[string]planner.ReferenceInfo{
			planner.FieldAuditLogDestinationID: {ID: destinationID, Ref: "audit-log-destination"},
		},
	})

	err := adapter.MapUpdateFields(
		t.Context(),
		execCtx,
		map[string]any{planner.FieldAuditLogDestinationID: destinationID},
		&update,
		nil,
	)
	require.NoError(t, err)

	body, err := json.Marshal(update)
	require.NoError(t, err)
	assert.JSONEq(t, `{"audit_log_destination_id":"11111111-1111-1111-1111-111111111111"}`, string(body))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.NotContains(t, payload, planner.FieldEnabled)
}

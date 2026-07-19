package executor

import (
	"encoding/json"
	"testing"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortalEmailTemplateAdapterMapUpdateFieldsKeepsPatchPayloadSparse(t *testing.T) {
	adapter := NewPortalEmailTemplateAdapter(nil)
	update := kkOps.UpdatePortalCustomEmailTemplateRequest{}

	err := adapter.MapUpdateFields(
		t.Context(),
		nil,
		map[string]any{
			planner.FieldName: "app-registration-approved",
			planner.FieldContent: map[string]any{
				"subject": "Updated subject",
			},
		},
		&update,
		nil,
	)
	require.NoError(t, err)

	body, err := json.Marshal(update.PatchCustomPortalEmailTemplatePayload)
	require.NoError(t, err)
	assert.JSONEq(t, `{"content":{"subject":"Updated subject"}}`, string(body))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.NotContains(t, payload, planner.FieldEnabled)
}

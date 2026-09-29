package executor

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyOperationLoggingRedactsOpaqueConfig(t *testing.T) {
	for _, field := range []string{"password", "sentinel_password", "clientSecret", "api-key", "accessToken"} {
		t.Run(field, func(t *testing.T) {
			fields := map[string]any{planner.FieldConfig: map[string]any{
				"nested": []any{map[string]any{field: "literal-test-value", "token_count": 17}},
			}}
			before, err := json.Marshal(fields)
			require.NoError(t, err)
			redacted := redactResourceOperationFields(planner.ResourceTypeAIGatewayPolicy, fields)
			output, err := json.Marshal(redacted)
			require.NoError(t, err)
			assert.NotContains(t, string(output), "literal-test-value")
			assert.Contains(t, string(output), "[REDACTED]")
			assert.Contains(t, string(output), `"token_count":17`)
			after, err := json.Marshal(fields)
			require.NoError(t, err)
			assert.Equal(t, before, after)
		})
	}
}

func TestRedactResourceOperationFieldsScopesConfigStoreSecretValue(t *testing.T) {
	fields := map[string]any{planner.FieldValue: "secret-value"}

	redacted := redactResourceOperationFields(planner.ResourceTypeAIGatewayConfigStoreSecret, fields).(map[string]any)
	assert.Equal(t, "[REDACTED]", redacted[planner.FieldValue])

	unrelated := redactResourceOperationFields(planner.ResourceTypeAIGatewayConfigStore, fields).(map[string]any)
	assert.Equal(t, "secret-value", unrelated[planner.FieldValue])
}

func TestRedactResourceOperationFieldsRedactsCertificateKeys(t *testing.T) {
	fields := map[string]any{planner.FieldKey: "private-key", planner.FieldKeyAlt: "alternative-private-key"}

	redacted := redactResourceOperationFields(planner.ResourceTypeAIGatewayCertificate, fields).(map[string]any)
	assert.Equal(t, "[REDACTED]", redacted[planner.FieldKey])
	assert.Equal(t, "[REDACTED]", redacted[planner.FieldKeyAlt])
}

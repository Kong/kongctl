package executor

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
)

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

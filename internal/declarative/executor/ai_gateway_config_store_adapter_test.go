package executor

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayConfigStoreReferenceHydratesNestedVaultField(t *testing.T) {
	fields := map[string]any{
		planner.FieldConfig: map[string]any{
			planner.FieldConfigStoreID: "__REF__:support-store#id",
		},
	}

	require.True(
		t,
		setResolvedFieldValue(
			fields,
			planner.FieldConfig+"."+planner.FieldConfigStoreID,
			"resolved-config-store-id",
		),
	)
	config := fields[planner.FieldConfig].(map[string]any)
	require.Equal(t, "resolved-config-store-id", config[planner.FieldConfigStoreID])
}

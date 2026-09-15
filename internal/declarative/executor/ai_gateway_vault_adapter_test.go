package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	testdecl "github.com/kong/kongctl/test/declarative"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayVaultAdapterPreservesAPIConfigurations(t *testing.T) {
	for _, tc := range testdecl.VaultCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			fields := map[string]any{
				planner.FieldType: tc.Type, planner.FieldName: tc.Name, planner.FieldConfig: tc.Config,
			}
			adapter := NewAIGatewayVaultAdapter(nil)
			var create kkComps.CreateAIGatewayVaultRequest
			require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, &create))
			var update kkComps.UpdateAIGatewayVaultRequest
			require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, fields, &update, nil))
			for _, request := range []any{create, update} {
				data, err := json.Marshal(request)
				require.NoError(t, err)
				var payload map[string]any
				require.NoError(t, json.Unmarshal(data, &payload))
				require.Equal(t, tc.Config, payload[planner.FieldConfig])
			}
		})
	}
}

func TestAIGatewayVaultAdapterMapsDeferredCertificateKey(t *testing.T) {
	t.Setenv("VAULT_CERT_KEY", "private-key-value")
	plan := secretExecutionPlan(secretExecutionIntent("/config/key", "VAULT_CERT_KEY"))
	plan.Changes[0].ResourceType = planner.ResourceTypeAIGatewayVault
	plan.Changes[0].Fields = map[string]any{
		planner.FieldType: "hcv", planner.FieldName: "vault",
		planner.FieldConfig: map[string]any{
			"auth_method": "cert", "host": "vault.example.net", "port": 8200, "cert": "public-cert",
		},
	}
	executor := &Executor{}
	require.NoError(t, executor.preflightSecretWrites(plan))
	change, err := cloneChangeForExecution(&plan.Changes[0])
	require.NoError(t, err)
	require.NoError(t, executor.injectResolvedSecretWrites(change))
	var create kkComps.CreateAIGatewayVaultRequest
	adapter := NewAIGatewayVaultAdapter(nil)
	require.NoError(t, adapter.MapCreateFields(t.Context(), nil, change.Fields, &create))
	require.Equal(t, "private-key-value", *create.HashiCorpVault.Config.HashiCorpVaultCertConfig.Key)
	var update kkComps.UpdateAIGatewayVaultRequest
	require.NoError(t, adapter.MapUpdateFields(t.Context(), nil, change.Fields, &update, nil))
	require.Equal(t, "private-key-value", *update.HashiCorpVault.Config.HashiCorpVaultCertConfig.Key)
	data, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NotContains(t, string(data), "private-key-value")
	require.NotContains(t, plan.Changes[0].Fields[planner.FieldConfig], "key")
}

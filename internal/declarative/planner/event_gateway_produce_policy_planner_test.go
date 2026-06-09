package planner

import (
	"log/slog"
	"os"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPlanner() *Planner {
	return &Planner{logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}
}

func modifyHeadersResource(
	name, desc string,
	enabled bool,
	actions []kkComps.EventGatewayModifyHeaderAction,
) resources.EventGatewayProducePolicyResource {
	return resources.EventGatewayProducePolicyResource{
		EventGatewayProducePolicyCreate: kkComps.EventGatewayProducePolicyCreate{
			EventGatewayModifyHeadersPolicyCreate: &kkComps.EventGatewayModifyHeadersPolicyCreate{
				Name:        new(name),
				Description: new(desc),
				Enabled:     new(enabled),
				Config: kkComps.EventGatewayModifyHeadersPolicyCreateConfig{
					Actions: actions,
				},
			},
		},
		Ref: name + "-ref",
	}
}

func modifyHeadersCurrentPolicy(
	id, name, desc string,
	enabled bool,
	rawConfig map[string]any,
) state.EventGatewayVirtualClusterProducePolicyInfo {
	return state.EventGatewayVirtualClusterProducePolicyInfo{
		EventGatewayPolicy: kkComps.EventGatewayPolicy{
			ID:          id,
			Name:        new(name),
			Description: new(desc),
			Enabled:     new(enabled),
			Type:        "modify_headers",
		},
		RawConfig: rawConfig,
	}
}

func setAction(key, value string) kkComps.EventGatewayModifyHeaderAction {
	return kkComps.CreateEventGatewayModifyHeaderActionSet(
		kkComps.EventGatewayModifyHeaderSetAction{Key: key, Value: value},
	)
}

// rawConfigFromSetAction produces the RawConfig map[string]any that matches a single set action.
func rawConfigFromSetAction(key, value string) map[string]any {
	return map[string]any{
		"actions": []any{
			map[string]any{"op": "set", "key": key, "value": value},
		},
	}
}

// ---------------------------------------------------------------------------
// shouldUpdateProducePolicy
// ---------------------------------------------------------------------------

func TestShouldUpdateProducePolicy_NoChanges(t *testing.T) {
	current := modifyHeadersCurrentPolicy(
		"policy-1", "my-policy", "a description", true,
		rawConfigFromSetAction("X-Foo", "bar"),
	)
	desired := modifyHeadersResource(
		"my-policy", "a description", true,
		[]kkComps.EventGatewayModifyHeaderAction{setAction("X-Foo", "bar")},
	)

	p := newTestPlanner()
	needsUpdate, updateFields, changedFields := p.shouldUpdateProducePolicy(current, desired)

	assert.False(t, needsUpdate, "no update expected when all fields match")
	assert.Nil(t, updateFields)
	assert.Empty(t, changedFields)
}

func TestShouldUpdateProducePolicy_NameChanged(t *testing.T) {
	current := modifyHeadersCurrentPolicy("p1", "old-name", "desc", true, nil)
	desired := modifyHeadersResource("new-name", "desc", true, nil)

	p := newTestPlanner()
	needsUpdate, updateFields, changedFields := p.shouldUpdateProducePolicy(current, desired)

	require.True(t, needsUpdate)
	require.Contains(t, changedFields, "name")
	assert.Equal(t, "old-name", changedFields["name"].Old)
	assert.Equal(t, "new-name", changedFields["name"].New)
	assert.NotNil(t, updateFields)
	assert.NotContains(t, changedFields, "description")
	assert.NotContains(t, changedFields, "enabled")
}

func TestShouldUpdateProducePolicy_ConfigChanged(t *testing.T) {
	current := modifyHeadersCurrentPolicy(
		"p1", "my-policy", "desc", true,
		rawConfigFromSetAction("X-Old", "old-value"),
	)
	desired := modifyHeadersResource(
		"my-policy", "desc", true,
		[]kkComps.EventGatewayModifyHeaderAction{setAction("X-New", "new-value")},
	)

	p := newTestPlanner()
	needsUpdate, updateFields, changedFields := p.shouldUpdateProducePolicy(current, desired)

	require.True(t, needsUpdate)
	require.Contains(t, changedFields, "config")
	assert.NotNil(t, updateFields)

	// Inspect new config: derived from the desired variant's Config struct
	newConfig, ok := changedFields["config"].New.(map[string]any)
	require.True(t, ok, "new config should be map[string]any")
	newActions, ok := newConfig["actions"].([]any)
	require.True(t, ok, "new config should have an 'actions' array")
	require.Len(t, newActions, 1)
	newAction, ok := newActions[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "set", newAction["op"])
	assert.Equal(t, "X-New", newAction["key"])
	assert.Equal(t, "new-value", newAction["value"])

	// Other fields should not be flagged
	assert.NotContains(t, changedFields, "name")
	assert.NotContains(t, changedFields, "description")
	assert.NotContains(t, changedFields, "enabled")
}

func TestShouldUpdateProducePolicy_SchemaValidationConfigChanged(t *testing.T) {
	keyActionMark := kkComps.ProduceKeyValidationActionMark

	// Current policy is modify_headers; desired switches the type to schema_validation.
	current := state.EventGatewayVirtualClusterProducePolicyInfo{
		EventGatewayPolicy: kkComps.EventGatewayPolicy{
			ID:          "sv-1",
			Name:        new("sv-policy"),
			Description: new("schema val desc"),
			Enabled:     new(true),
			Type:        "modify_headers",
		},
		RawConfig: rawConfigFromSetAction("X-Foo", "bar"),
	}

	desired := resources.EventGatewayProducePolicyResource{
		EventGatewayProducePolicyCreate: kkComps.EventGatewayProducePolicyCreate{
			EventGatewayProduceSchemaValidationPolicy: &kkComps.EventGatewayProduceSchemaValidationPolicy{
				Name:        new("sv-policy"),
				Description: new("schema val desc"),
				Enabled:     new(true),
				Config: kkComps.CreateEventGatewayProduceSchemaValidationPolicyConfigJSON(
					kkComps.EventGatewayProduceSchemaValidationPolicyJSONConfig{
						KeyValidationAction: &keyActionMark,
					},
				),
			},
		},
		Ref: "sv-policy-ref",
	}

	p := newTestPlanner()
	needsUpdate, updateFields, changedFields := p.shouldUpdateProducePolicy(current, desired)

	require.True(t, needsUpdate)
	require.Contains(t, changedFields, "config")
	assert.NotNil(t, updateFields)

	// New config derived from the desired variant's Config struct
	newConfig, ok := changedFields["config"].New.(map[string]any)
	require.True(t, ok, "new config should be map[string]any")
	assert.Equal(t, "json", newConfig["type"])
	assert.Equal(t, "mark", newConfig["key_validation_action"])

	// Other fields should not be flagged
	assert.NotContains(t, changedFields, "name")
	assert.NotContains(t, changedFields, "description")
	assert.NotContains(t, changedFields, "enabled")
}

package planner

import (
	"encoding/json"
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

func producePolicyResourceFromJSON(t *testing.T, data string) resources.EventGatewayProducePolicyResource {
	t.Helper()

	var policy resources.EventGatewayProducePolicyResource
	require.NoError(t, json.Unmarshal([]byte(data), &policy))
	return policy
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

func TestShouldUpdateProducePolicy_IgnoresUnresolvedParentPolicyPlaceholder(t *testing.T) {
	desired := producePolicyResourceFromJSON(t, `{
		"ref": "encrypt-fields",
		"type": "encrypt_fields",
		"name": "encrypt-fields",
		"parent_policy_id": "__REF__:schema-validation#id",
		"config": {
			"failure_mode": "reject",
			"encrypt_fields": []
		}
	}`)
	p := newTestPlanner()
	current := state.EventGatewayVirtualClusterProducePolicyInfo{
		EventGatewayPolicy: kkComps.EventGatewayPolicy{
			ID:             "encrypt-fields-id",
			Name:           new("encrypt-fields"),
			Enabled:        new(true),
			Type:           "encrypt_fields",
			ParentPolicyID: new("schema-validation-id"),
		},
		RawConfig: p.extractProducePolicyConfig(desired),
	}

	needsUpdate, updateFields, changedFields := p.shouldUpdateProducePolicy(current, desired)

	assert.False(t, needsUpdate)
	assert.Nil(t, updateFields)
	assert.Empty(t, changedFields)
}

func TestProducePolicyToFieldsEncryptFields(t *testing.T) {
	policy := producePolicyResourceFromJSON(t, `{
		"ref": "encrypt-fields",
		"type": "encrypt_fields",
		"name": "encrypt-fields",
		"parent_policy_id": "__REF__:schema-validation#id",
		"config": {
			"failure_mode": "reject",
			"encrypt_fields": [
				{
					"paths": "record.value.content.customer.ssn",
					"encryption_key": {
						"type": "static",
						"key": {
							"id": "__REF__:static-key#id"
						}
					}
				}
			]
		}
	}`)

	p := newTestPlanner()
	fields := p.producePolicyToFields(policy)

	require.Equal(t, "encrypt_fields", fields[FieldType])
	require.Equal(t, "__REF__:schema-validation#id", fields[FieldParentPolicyID])
	config, ok := fields[FieldConfig].(map[string]any)
	require.True(t, ok)
	encryptFields, ok := config["encrypt_fields"].([]any)
	require.True(t, ok)
	require.Len(t, encryptFields, 1)
}

func TestPlanProducePolicyCreateEncryptFieldsReferences(t *testing.T) {
	policy := producePolicyResourceFromJSON(t, `{
		"ref": "encrypt-fields",
		"type": "encrypt_fields",
		"name": "encrypt-fields",
		"parent_policy_id": "__REF__:schema-validation#id",
		"config": {
			"failure_mode": "reject",
			"encrypt_fields": [
				{
					"paths": "record.value.content.customer.ssn",
					"encryption_key": {
						"type": "static",
						"key": {
							"id": "__REF__:static-key#id"
						}
					}
				}
			]
		}
	}`)

	p := newTestPlanner()
	plan := NewPlan("1.0", "test", PlanModeApply)
	p.planProducePolicyCreate(
		"default",
		"gateway-id",
		"gateway-ref",
		"virtual-cluster-id",
		"virtual-cluster-ref",
		"virtual-cluster",
		policy,
		nil,
		plan,
	)

	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	parentRef, ok := change.References[FieldParentPolicyID]
	require.True(t, ok)
	require.Equal(t, "__REF__:schema-validation#id", parentRef.Ref)

	staticKeyRef, ok := change.References["config.encrypt_fields.0.encryption_key.key.id"]
	require.True(t, ok)
	require.Equal(t, "__REF__:static-key#id", staticKeyRef.Ref)
}

func TestPrepareProducePolicyParentRefsResolvesExistingSchemaValidationParent(t *testing.T) {
	parent := producePolicyResourceFromJSON(t, `{
		"ref": "schema-validation",
		"type": "schema_validation",
		"name": "schema-validation",
		"config": {
			"type": "json"
		}
	}`)
	child := producePolicyResourceFromJSON(t, `{
		"ref": "encrypt-fields",
		"type": "encrypt_fields",
		"name": "encrypt-fields",
		"parent_policy_id": "__REF__:schema-validation#id",
		"config": {
			"failure_mode": "reject",
			"encrypt_fields": [
				{
					"paths": "record.value.content.customer.ssn",
					"encryption_key": {
						"type": "static",
						"key": {
							"id": "__REF__:static-key#id"
						}
					}
				}
			]
		}
	}`)

	currentByName := map[string]state.EventGatewayVirtualClusterProducePolicyInfo{
		"schema-validation": {
			EventGatewayPolicy: kkComps.EventGatewayPolicy{
				ID:   "schema-validation-id",
				Name: new("schema-validation"),
				Type: "schema_validation",
			},
		},
	}

	p := newTestPlanner()
	prepared, err := p.prepareProducePolicyParentRefs(
		[]resources.EventGatewayProducePolicyResource{parent, child},
		currentByName,
	)

	require.NoError(t, err)
	require.Len(t, prepared, 2)
	require.Equal(
		t,
		"schema-validation-id",
		prepared[1].EventGatewayParsedRecordEncryptFieldsPolicyCreate.ParentPolicyID,
	)
}

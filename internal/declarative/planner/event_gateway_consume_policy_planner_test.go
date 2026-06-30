package planner

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func consumePolicyResourceFromJSON(t *testing.T, data string) resources.EventGatewayConsumePolicyResource {
	t.Helper()

	var policy resources.EventGatewayConsumePolicyResource
	require.NoError(t, json.Unmarshal([]byte(data), &policy))
	return policy
}

func TestShouldUpdateConsumePolicy_NoChanges(t *testing.T) {
	name := "test-consume-policy"
	desc := "test description"
	enabled := true
	keyAction := kkComps.ConsumeKeyValidationActionMark
	valueAction := kkComps.ConsumeValueValidationActionSkip

	current := state.EventGatewayConsumePolicyInfo{
		EventGatewayPolicy: kkComps.EventGatewayPolicy{
			ID:          "policy-123",
			Name:        &name,
			Description: &desc,
			Enabled:     &enabled,
			Type:        "schema_validation",
		},
		NormalizedLabels: map[string]string{
			"env":  "prod",
			"team": "platform",
		},
		RawConfig: map[string]any{
			"type":                    "json",
			"key_validation_action":   "mark",
			"value_validation_action": "skip",
		},
	}

	desired := resources.EventGatewayConsumePolicyResource{
		EventGatewayConsumePolicyCreate: kkComps.CreateEventGatewayConsumePolicyCreateSchemaValidation(
			kkComps.EventGatewayConsumeSchemaValidationPolicy{
				Name:        &name,
				Description: &desc,
				Enabled:     &enabled,
				Labels: map[string]string{
					"env":  "prod",
					"team": "platform",
				},
				Config: kkComps.CreateEventGatewayConsumeSchemaValidationPolicyConfigJSON(
					kkComps.EventGatewayConsumeSchemaValidationPolicyJSONConfig{
						KeyValidationAction:   &keyAction,
						ValueValidationAction: &valueAction,
					},
				),
			},
		),
		Ref: "test-consume-policy-ref",
	}

	p := &Planner{}
	needsUpdate, updateFields, changedFields := p.shouldUpdateConsumePolicy(current, desired)

	assert.False(t, needsUpdate, "no update should be needed when all fields match")
	assert.Nil(t, updateFields, "updateFields should be nil when no update needed")
	assert.Empty(t, changedFields, "changedFields should be empty when no update needed")
}

func TestShouldUpdateConsumePolicy_DescriptionChanged(t *testing.T) {
	name := "test-consume-policy"
	oldDesc := "old description"
	newDesc := "new description"
	enabled := true

	current := state.EventGatewayConsumePolicyInfo{
		EventGatewayPolicy: kkComps.EventGatewayPolicy{
			ID:          "policy-123",
			Name:        &name,
			Description: &oldDesc,
			Enabled:     &enabled,
			Type:        "schema_validation",
		},
		NormalizedLabels: map[string]string{},
		RawConfig: map[string]any{
			"type": "json",
		},
	}

	desired := resources.EventGatewayConsumePolicyResource{
		EventGatewayConsumePolicyCreate: kkComps.CreateEventGatewayConsumePolicyCreateSchemaValidation(
			kkComps.EventGatewayConsumeSchemaValidationPolicy{
				Name:        &name,
				Description: &newDesc,
				Enabled:     &enabled,
				Config: kkComps.CreateEventGatewayConsumeSchemaValidationPolicyConfigJSON(
					kkComps.EventGatewayConsumeSchemaValidationPolicyJSONConfig{},
				),
			},
		),
		Ref: "test-consume-policy-ref",
	}

	p := &Planner{}
	needsUpdate, updateFields, changedFields := p.shouldUpdateConsumePolicy(current, desired)

	require.True(t, needsUpdate, "update should be needed when description changes")
	require.Contains(t, changedFields, "description")
	assert.Equal(t, oldDesc, changedFields["description"].Old)
	assert.Equal(t, newDesc, changedFields["description"].New)
	assert.NotNil(t, updateFields)
	assert.NotContains(t, changedFields, "name")
}

func TestShouldUpdateConsumePolicy_ConfigChangedNestedField(t *testing.T) {
	name := "test-consume-policy"
	desc := "test description"
	enabled := true
	oldKeyAction := kkComps.ConsumeKeyValidationActionMark
	newKeyAction := kkComps.ConsumeKeyValidationActionSkip

	current := state.EventGatewayConsumePolicyInfo{
		EventGatewayPolicy: kkComps.EventGatewayPolicy{
			ID:          "policy-123",
			Name:        &name,
			Description: &desc,
			Enabled:     &enabled,
			Type:        "schema_validation",
		},
		NormalizedLabels: map[string]string{},
		RawConfig: map[string]any{
			"type":                  "json",
			"key_validation_action": string(oldKeyAction),
		},
	}

	desired := resources.EventGatewayConsumePolicyResource{
		EventGatewayConsumePolicyCreate: kkComps.CreateEventGatewayConsumePolicyCreateSchemaValidation(
			kkComps.EventGatewayConsumeSchemaValidationPolicy{
				Name:        &name,
				Description: &desc,
				Enabled:     &enabled,
				Config: kkComps.CreateEventGatewayConsumeSchemaValidationPolicyConfigJSON(
					kkComps.EventGatewayConsumeSchemaValidationPolicyJSONConfig{
						KeyValidationAction: &newKeyAction,
					},
				),
			},
		),
		Ref: "test-consume-policy-ref",
	}

	p := &Planner{}
	needsUpdate, updateFields, changedFields := p.shouldUpdateConsumePolicy(current, desired)

	require.True(t, needsUpdate, "update should be needed when config changes")
	assert.NotNil(t, updateFields, "updateFields should contain the new config")
	require.Contains(t, changedFields, "config", "config should be in changed fields")
}

func TestShouldUpdateConsumePolicy_TypeChanged(t *testing.T) {
	name := "test-consume-policy"
	desc := "test description"
	enabled := true

	current := state.EventGatewayConsumePolicyInfo{
		EventGatewayPolicy: kkComps.EventGatewayPolicy{
			ID:          "policy-123",
			Name:        &name,
			Description: &desc,
			Enabled:     &enabled,
			Type:        "decrypt",
		},
		NormalizedLabels: map[string]string{},
		RawConfig:        map[string]any{},
	}

	desired := resources.EventGatewayConsumePolicyResource{
		EventGatewayConsumePolicyCreate: kkComps.CreateEventGatewayConsumePolicyCreateSchemaValidation(
			kkComps.EventGatewayConsumeSchemaValidationPolicy{
				Name:        &name,
				Description: &desc,
				Enabled:     &enabled,
				Config: kkComps.CreateEventGatewayConsumeSchemaValidationPolicyConfigJSON(
					kkComps.EventGatewayConsumeSchemaValidationPolicyJSONConfig{},
				),
			},
		),
		Ref: "test-consume-policy-ref",
	}

	p := &Planner{}
	needsUpdate, _, changedFields := p.shouldUpdateConsumePolicy(current, desired)

	require.True(t, needsUpdate)
	require.Contains(t, changedFields, FieldType)
	assert.Equal(t, "decrypt", changedFields[FieldType].Old)
	assert.Equal(t, "schema_validation", changedFields[FieldType].New)
}

func TestConsumePolicyToFieldsDecryptFields(t *testing.T) {
	policy := consumePolicyResourceFromJSON(t, `{
		"ref": "decrypt-fields",
		"type": "decrypt_fields",
		"name": "decrypt-fields",
		"parent_policy_id": "__REF__:schema-validation#id",
		"config": {
			"failure_mode": "error",
			"key_sources": [
				{"type": "static"}
			],
			"decrypt_fields": {
				"paths": "record.value.content.customer.ssn"
			}
		}
	}`)

	p := &Planner{}
	fields := p.consumePolicyToFields(policy)

	require.Equal(t, "decrypt_fields", fields[FieldType])
	require.Equal(t, "__REF__:schema-validation#id", fields[FieldParentPolicyID])
	config, ok := fields[FieldConfig].(map[string]any)
	require.True(t, ok)
	_, ok = config["decrypt_fields"].(map[string]any)
	require.True(t, ok)
}

func TestPlanConsumePolicyCreateDecryptFieldsReference(t *testing.T) {
	policy := consumePolicyResourceFromJSON(t, `{
		"ref": "decrypt-fields",
		"type": "decrypt_fields",
		"name": "decrypt-fields",
		"parent_policy_id": "__REF__:schema-validation#id",
		"config": {
			"failure_mode": "error",
			"key_sources": [
				{"type": "static"}
			],
			"decrypt_fields": {
				"paths": "record.value.content.customer.ssn"
			}
		}
	}`)

	p := newTestPlanner()
	plan := NewPlan("1.0", "test", PlanModeApply)
	p.planConsumePolicyCreate(
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
	parentRef, ok := plan.Changes[0].References[FieldParentPolicyID]
	require.True(t, ok)
	require.Equal(t, "__REF__:schema-validation#id", parentRef.Ref)
}

func TestPrepareConsumePolicyParentRefsResolvesExistingSchemaValidationParent(t *testing.T) {
	parent := consumePolicyResourceFromJSON(t, `{
		"ref": "schema-validation",
		"type": "schema_validation",
		"name": "schema-validation",
		"config": {
			"type": "json"
		}
	}`)
	child := consumePolicyResourceFromJSON(t, `{
		"ref": "decrypt-fields",
		"type": "decrypt_fields",
		"name": "decrypt-fields",
		"parent_policy_id": "__REF__:schema-validation#id",
		"config": {
			"failure_mode": "error",
			"key_sources": [
				{"type": "static"}
			],
			"decrypt_fields": {
				"paths": "record.value.content.customer.ssn"
			}
		}
	}`)

	parentName := "schema-validation"
	currentByName := map[string]state.EventGatewayConsumePolicyInfo{
		parentName: {
			EventGatewayPolicy: kkComps.EventGatewayPolicy{
				ID:   "schema-validation-id",
				Name: &parentName,
				Type: "schema_validation",
			},
		},
	}

	p := &Planner{}
	prepared, err := p.prepareConsumePolicyParentRefs(
		[]resources.EventGatewayConsumePolicyResource{parent, child},
		currentByName,
	)

	require.NoError(t, err)
	require.Len(t, prepared, 2)
	require.Equal(
		t,
		"schema-validation-id",
		prepared[1].EventGatewayParsedRecordDecryptFieldsPolicyCreate.ParentPolicyID,
	)
}

package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				Config: kkComps.EventGatewayConsumeSchemaValidationPolicyConfig{
					Type:                  kkComps.SchemaValidationTypeJSON,
					KeyValidationAction:   &keyAction,
					ValueValidationAction: &valueAction,
				},
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
				Config: kkComps.EventGatewayConsumeSchemaValidationPolicyConfig{
					Type: kkComps.SchemaValidationTypeJSON,
				},
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
				Config: kkComps.EventGatewayConsumeSchemaValidationPolicyConfig{
					Type:                kkComps.SchemaValidationTypeJSON,
					KeyValidationAction: &newKeyAction,
				},
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

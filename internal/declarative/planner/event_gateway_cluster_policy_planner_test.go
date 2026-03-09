package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldUpdateClusterPolicy_NoChanges(t *testing.T) {
	name := "test-policy"
	desc := "test description"
	enabled := true

	current := state.EventGatewayClusterPolicyInfo{
		EventGatewayPolicy: kkComps.EventGatewayPolicy{
			ID:          "policy-123",
			Name:        &name,
			Description: &desc,
			Enabled:     &enabled,
			Type:        "acls",
		},
		NormalizedLabels: map[string]string{
			"env":  "prod",
			"team": "platform",
		},
	}

	desired := resources.EventGatewayClusterPolicyResource{
		EventGatewayClusterPolicyModify: kkComps.EventGatewayClusterPolicyModify{
			EventGatewayACLsPolicy: &kkComps.EventGatewayACLsPolicy{
				Name:        &name,
				Description: &desc,
				Enabled:     &enabled,
				Labels: map[string]string{
					"env":  "prod",
					"team": "platform",
				},
				Config: kkComps.EventGatewayACLPolicyConfig{
					Rules: []kkComps.EventGatewayACLRule{},
				},
			},
			Type: kkComps.EventGatewayClusterPolicyModifyTypeAcls,
		},
		Ref: "test-policy-ref",
	}

	p := &Planner{}
	needsUpdate, updateFields, changedFields := p.shouldUpdateClusterPolicy(current, desired)

	assert.False(t, needsUpdate, "no update should be needed when all fields match")
	assert.Nil(t, updateFields, "updateFields should be nil when no update needed")
	assert.Empty(t, changedFields, "changedFields should be empty when no update needed")
}

func TestShouldUpdateClusterPolicy_DescriptionChanged(t *testing.T) {
	name := "test-policy"
	oldDesc := "old description"
	newDesc := "new description"
	enabled := true

	current := state.EventGatewayClusterPolicyInfo{
		EventGatewayPolicy: kkComps.EventGatewayPolicy{
			ID:          "policy-123",
			Name:        &name,
			Description: &oldDesc,
			Enabled:     &enabled,
			Type:        "acls",
		},
		NormalizedLabels: map[string]string{},
	}

	desired := resources.EventGatewayClusterPolicyResource{
		EventGatewayClusterPolicyModify: kkComps.EventGatewayClusterPolicyModify{
			EventGatewayACLsPolicy: &kkComps.EventGatewayACLsPolicy{
				Name:        &name,
				Description: &newDesc,
				Enabled:     &enabled,
				Config: kkComps.EventGatewayACLPolicyConfig{
					Rules: []kkComps.EventGatewayACLRule{},
				},
			},
			Type: kkComps.EventGatewayClusterPolicyModifyTypeAcls,
		},
		Ref: "test-policy-ref",
	}

	p := &Planner{}
	needsUpdate, updateFields, changedFields := p.shouldUpdateClusterPolicy(current, desired)

	require.True(t, needsUpdate, "update should be needed when description changes")
	require.Contains(t, changedFields, "description")
	assert.Equal(t, oldDesc, changedFields["description"].Old)
	assert.Equal(t, newDesc, changedFields["description"].New)
	assert.NotNil(t, updateFields)
	// Verify only description is in changed fields (not name, enabled, labels)
	assert.NotContains(t, changedFields, "name")
	assert.NotContains(t, changedFields, "enabled")
	assert.NotContains(t, changedFields, "labels")
}

func TestShouldUpdateClusterPolicy_ConfigChangedNestedField(t *testing.T) {
	// NOTE: This test demonstrates that config changes are NOT currently detected
	// by shouldUpdateClusterPolicy. The config field is not compared.
	// This test documents the current behavior - if config comparison is needed,
	// the shouldUpdateClusterPolicy function needs to be updated.
	name := "test-policy"
	desc := "test description"
	enabled := true

	current := state.EventGatewayClusterPolicyInfo{
		EventGatewayPolicy: kkComps.EventGatewayPolicy{
			ID:          "policy-123",
			Name:        &name,
			Description: &desc,
			Enabled:     &enabled,
			Type:        "acls",
		},
		NormalizedLabels: map[string]string{},
		RawConfig: map[string]any{
			"rules": []any{
				map[string]any{
					"action":        "deny",
					"resource_type": "topic",
					"operations": []any{
						map[string]any{"name": "read"},
					},
					"resource_names": []any{
						map[string]any{"match": "old-topic-*"},
					},
				},
			},
		},
	}

	// Desired has different config - nested rule changed
	desired := resources.EventGatewayClusterPolicyResource{
		EventGatewayClusterPolicyModify: kkComps.EventGatewayClusterPolicyModify{
			EventGatewayACLsPolicy: &kkComps.EventGatewayACLsPolicy{
				Name:        &name,
				Description: &desc,
				Enabled:     &enabled,
				Config: kkComps.EventGatewayACLPolicyConfig{
					Rules: []kkComps.EventGatewayACLRule{
						{
							Action:       kkComps.ActionAllow, // Changed from deny to allow
							ResourceType: kkComps.ResourceTypeTopic,
							Operations: []kkComps.EventGatewayACLOperation{
								{Name: kkComps.NameWrite}, // Changed from read to write
							},
							ResourceNames: kkComps.CreateResourceNamesArrayOfEventGatewayACLResourceName(
								[]kkComps.EventGatewayACLResourceName{
									{Match: "new-topic-*"}, // Changed pattern
								},
							),
						},
					},
				},
			},
			Type: kkComps.EventGatewayClusterPolicyModifyTypeAcls,
		},
		Ref: "test-policy-ref",
	}

	p := &Planner{}
	needsUpdate, _, _ := p.shouldUpdateClusterPolicy(current, desired)

	// TODO: Config changes should trigger an update, but currently they don't.
	// This test documents current behavior. When config comparison is implemented,
	// this assertion should change to require.True(t, needsUpdate).
	assert.True(t, needsUpdate)
	//assert.Nil(t, updateFields)
	//assert.Empty(t, changedFields)
}

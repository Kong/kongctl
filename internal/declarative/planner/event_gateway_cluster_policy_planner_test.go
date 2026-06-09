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
		RawConfig: map[string]any{
			"rules": []any{
				map[string]any{
					"action":        "deny",
					"resource_type": "topic",
					"operations":    []any{},
					"resource_names": []any{
						map[string]any{"match": "*"},
					},
				},
			},
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
					Rules: []kkComps.EventGatewayACLRule{
						{
							Action:       kkComps.ActionDeny,
							ResourceType: kkComps.ResourceTypeTopic,
							Operations:   []kkComps.EventGatewayACLOperation{},
							ResourceNames: kkComps.CreateResourceNamesArrayOfEventGatewayACLResourceName(
								[]kkComps.EventGatewayACLResourceName{
									{Match: "*"},
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
		RawConfig: map[string]any{
			"rules": []any{
				map[string]any{
					"action":         "deny",
					"resource_type":  "topic",
					"operations":     []any{},
					"resource_names": []any{},
				},
			},
		},
	}

	desired := resources.EventGatewayClusterPolicyResource{
		EventGatewayClusterPolicyModify: kkComps.EventGatewayClusterPolicyModify{
			EventGatewayACLsPolicy: &kkComps.EventGatewayACLsPolicy{
				Name:        &name,
				Description: &newDesc,
				Enabled:     &enabled,
				Config: kkComps.EventGatewayACLPolicyConfig{
					Rules: []kkComps.EventGatewayACLRule{
						{
							Action:       kkComps.ActionDeny,
							ResourceType: kkComps.ResourceTypeTopic,
							Operations:   []kkComps.EventGatewayACLOperation{},
							ResourceNames: kkComps.CreateResourceNamesArrayOfEventGatewayACLResourceName(
								[]kkComps.EventGatewayACLResourceName{},
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
	needsUpdate, updateFields, changedFields := p.shouldUpdateClusterPolicy(current, desired)

	require.True(t, needsUpdate, "update should be needed when config changes")
	assert.NotNil(t, updateFields, "updateFields should contain the new config")
	require.Contains(t, changedFields, "config", "config should be in changed fields")
}

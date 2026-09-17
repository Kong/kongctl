package declarative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNestedDiffChanges(t *testing.T) {
	oldValue := map[string]any{
		"redis":            map[string]int{"connect_timeout": 1000, "port": 6379},
		"scopes_claim":     []string{"old_claim"},
		"removed":          nil,
		"type":             "scalar",
		"empty":            map[string]any{},
		"null":             nil,
		"unchanged_parent": map[string]any{"value": true},
	}
	newValue := map[string]any{
		"redis":            map[string]int{"connect_timeout": 2000, "port": 6379},
		"scopes_claim":     []string{"new_claim"},
		"added":            nil,
		"type":             map[string]any{},
		"empty":            []string{},
		"null":             map[string]any{},
		"unchanged_parent": map[string]any{"value": true},
	}
	for i := range 100 {
		key := fmt.Sprintf("unchanged_default_%03d", i)
		oldValue[key], newValue[key] = i, i
	}
	var out bytes.Buffer
	displayFieldChange(&out, planner.FieldConfig, oldValue, newValue, "  ", false)
	require.Equal(t, `  config:
    + added: null
    ~ empty: {} → [] (type)
    ~ null: null → {} (type)
    redis:
      ~ connect_timeout: 1000 → 2000
    - removed: null
    scopes_claim:
      ~ [0]: "old_claim" → "new_claim"
    ~ type: "scalar" → {} (type)
`, out.String())
}

func TestNestedDiffCollections(t *testing.T) {
	for _, tc := range []struct {
		name     string
		old, new any
		want     string
	}{
		{"array order", []string{"a", "b"}, []any{"b", "a"}, `  config:
    ~ [0]: "a" → "b"
    ~ [1]: "b" → "a"` + "\n"},
		{"typed equality", map[string]int{"count": 1}, map[string]any{"count": float64(1)}, ""},
		{"nil slice", []string(nil), []string{}, "  ~ config: null → [] (type)\n"},
		{"nil map", map[string]string(nil), map[string]string{}, "  ~ config: null → {} (type)\n"},
		{"empty parent", map[string]any{}, map[string]any{"child": map[string]any{}}, "  config:\n    + child: {}\n"},
		{"removed child", map[string]any{"child": []string{}}, map[string]any{}, "  config:\n    - child: []\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			displayFieldChange(&out, planner.FieldConfig, tc.old, tc.new, "  ", false)
			assert.Equal(t, tc.want, out.String())
		})
	}
}

func TestDiffRecursiveRedaction(t *testing.T) {
	for _, full := range []bool{false, true} {
		var out bytes.Buffer
		output := &diffOutput{Writer: &out, resourceType: resources.ResourceTypeAIGatewayProvider}
		oldValue := map[string]any{"auth": map[string]any{"headers": []any{
			map[string]any{"name": "Authorization", "value": "old-secret"},
		}}}
		newValue := map[string]any{"auth": map[string]any{"headers": []any{
			map[string]any{"name": "Authorization", "value": "new-secret"},
		}}}
		displayFieldChange(output, planner.FieldConfig, oldValue, newValue, "  ", full)
		assert.Contains(t, out.String(), "~ value: (sensitive value changed)")
		displayField(output, planner.FieldConfig, newValue, "  ", full)
		displayFieldChange(output, planner.FieldConfig, newValue, "replacement", "  ", full)
		displayFieldChange(output, planner.FieldConfig, oldValue, map[string]any{}, "  ", full)
		displayFieldChange(output, planner.FieldConfig, map[string]any{}, newValue, "  ", full)
		assert.NotContains(t, out.String(), "old-secret")
		assert.NotContains(t, out.String(), "new-secret")
		assert.Contains(t, out.String(), "[REDACTED]")
		assert.Contains(t, out.String(), "Authorization")
	}
}

func TestDiffArrayLeavesAndTypes(t *testing.T) {
	oldValue := []any{
		map[string]any{"path": "/v1", "methods": []string{"GET"}, "unchanged": true},
		map[string]any{"port": 80},
		nil,
	}
	newValue := []any{
		map[string]any{"path": "/v2", "methods": []string{"GET", "POST"}, "unchanged": true},
		map[string]any{"port": "80"},
	}
	var out bytes.Buffer
	displayFieldChange(&out, planner.FieldConfig, oldValue, newValue, "  ", false)
	assert.Equal(t, `  config:
    [0]:
      methods:
        + [1]: "POST"
      ~ path: "/v1" → "/v2"
    [1]:
      ~ port: 80 → "80" (type)
    - [2]: null
`, out.String())
}

func TestDiffSensitiveNullTransitions(t *testing.T) {
	var out bytes.Buffer
	output := &diffOutput{Writer: &out, resourceType: resources.ResourceTypeAIGatewayAuthStrategy}
	output = output.child(planner.FieldConfig)
	displayFieldChange(output, "client_secret", nil, "hidden", "  ", true)
	displayFieldChange(output, "client_secret", "hidden", nil, "  ", true)
	assert.Equal(t, "  ~ client_secret: null → [REDACTED]\n  ~ client_secret: [REDACTED] → null\n", out.String())
}

func TestDiffMultilineArray(t *testing.T) {
	var out bytes.Buffer
	displayFieldChange(&out, planner.FieldConfig, []string{},
		[]string{strings.Repeat("a", 40), strings.Repeat("b", 40)}, "  ", false)
	assert.Equal(t, `  config:
    + [0]: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    + [1]: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
`, out.String())
}

func TestDiffCatalogClassification(t *testing.T) {
	output := &diffOutput{resourceType: resources.ResourceTypeAIGatewayAuthStrategy}
	for _, key := range []string{"client_secret", "http_proxy_authorization", "https_proxy_authorization"} {
		assert.True(t, output.child(planner.FieldConfig).child(key).sensitive("literal"), key)
		assert.False(t, output.child("unrelated").child(key).sensitive("literal"), key)
	}
	for _, key := range []string{"cache_tokens_salt", "password", "token_count", "credential_claim"} {
		assert.False(t, output.child(planner.FieldConfig).child(key).sensitive("literal"), key)
	}
	var out bytes.Buffer
	output.Writer = &out
	displayFieldChange(output, planner.FieldConfig, map[string]any{"cache_tokens_salt": "old"},
		map[string]any{"cache_tokens_salt": "new", "password": "ordinary"}, "", true)
	assert.Contains(t, out.String(), `cache_tokens_salt: "old" → "new"`)
	assert.Contains(t, out.String(), `password: "ordinary"`)
}

func TestDiffPlanRoundTripAndLegacyFields(t *testing.T) {
	fc := planner.FieldChange{
		Old: map[string]any{"scopes_claim": []string{"old"}, "redis": map[string]int{"port": 6379}},
		New: map[string]any{"scopes_claim": []string{"new"}, "redis": map[string]int{"port": 6379}},
	}
	var expected string
	for _, legacy := range []bool{false, true} {
		plan := planner.NewPlan("1.0", "test", planner.PlanModeApply)
		change := planner.PlannedChange{
			ID: "update-auth", Action: planner.ActionUpdate,
			ResourceType: planner.ResourceTypeAIGatewayAuthStrategy, ResourceRef: "auth", Namespace: "default",
		}
		if legacy {
			change.Fields = map[string]any{planner.FieldConfig: fc}
		} else {
			change.ChangedFields = map[string]planner.FieldChange{planner.FieldConfig: fc}
		}
		plan.AddChange(change)
		plan.SetExecutionOrder([]string{change.ID})
		data, err := json.Marshal(plan)
		require.NoError(t, err)
		var reloaded planner.Plan
		require.NoError(t, json.Unmarshal(data, &reloaded))
		for _, candidate := range []*planner.Plan{plan, &reloaded} {
			var out bytes.Buffer
			command := &cobra.Command{}
			command.SetOut(&out)
			require.NoError(t, displayTextDiff(command, candidate, false))
			if expected == "" {
				expected = out.String()
			}
			assert.Equal(t, expected, out.String())
			assert.Contains(t, out.String(), `~ [0]: "old" → "new"`)
			assert.NotContains(t, out.String(), "redis:")
			assert.Contains(t, out.String(), "will be updated")
		}
		after, err := json.Marshal(plan)
		require.NoError(t, err)
		assert.Equal(t, string(data), string(after), "rendering must not mutate the plan")
	}
}

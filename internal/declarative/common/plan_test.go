package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPlan(t *testing.T) {
	// Create temp file with valid plan
	tmpDir := t.TempDir()
	planFile := filepath.Join(tmpDir, "test-plan.json")
	planJSON := `{
		"metadata": {
			"generated_at": "2024-01-01T00:00:00Z",
			"version": "1.0",
			"mode": "apply",
			"config_hash": "abc123"
		},
		"changes": [
			{
				"id": "1-c-portal",
				"resource_type": "portal",
				"resource_ref": "test-portal",
				"action": "CREATE",
				"fields": {
					"name": "Test Portal"
				}
			}
		],
		"summary": {
			"total_changes": 1,
			"by_action": {
				"CREATE": 1
			}
		},
		"execution_order": ["1-c-portal"]
	}`
	err := os.WriteFile(planFile, []byte(planJSON), 0o600)
	require.NoError(t, err)

	t.Run("load from file", func(t *testing.T) {
		plan, err := LoadPlan(planFile, nil)
		require.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, "1.0", plan.Metadata.Version)
		assert.Equal(t, planner.PlanModeApply, plan.Metadata.Mode)
		assert.Len(t, plan.Changes, 1)
		assert.Equal(t, "1-c-portal", plan.Changes[0].ID)
	})

	t.Run("load from stdin", func(t *testing.T) {
		stdin := strings.NewReader(planJSON)
		plan, err := LoadPlan("-", stdin)
		require.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, "1.0", plan.Metadata.Version)
		assert.Equal(t, planner.PlanModeApply, plan.Metadata.Mode)
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadPlan("/non/existent/file.json", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read plan file")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		invalidFile := filepath.Join(tmpDir, "invalid.json")
		err := os.WriteFile(invalidFile, []byte("not valid json"), 0o600)
		require.NoError(t, err)

		_, err = LoadPlan(invalidFile, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse plan")
	})

	t.Run("missing version", func(t *testing.T) {
		invalidPlan := `{
			"metadata": {
				"mode": "apply"
			}
		}`
		invalidFile := filepath.Join(tmpDir, "no-version.json")
		err := os.WriteFile(invalidFile, []byte(invalidPlan), 0o600)
		require.NoError(t, err)

		_, err = LoadPlan(invalidFile, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid plan: missing version")
	})

	t.Run("missing mode", func(t *testing.T) {
		invalidPlan := `{
			"metadata": {
				"version": "1.0"
			}
		}`
		invalidFile := filepath.Join(tmpDir, "no-mode.json")
		err := os.WriteFile(invalidFile, []byte(invalidPlan), 0o600)
		require.NoError(t, err)

		_, err = LoadPlan(invalidFile, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid plan: missing mode")
	})

	t.Run("normalizes protection change from JSON map", func(t *testing.T) {
		protectedPlan := `{
			"metadata": {
				"version": "1.0",
				"mode": "apply"
			},
			"changes": [
				{
					"id": "1-u-api-service",
					"resource_type": "api",
					"resource_ref": "service",
					"action": "UPDATE",
					"fields": { "name": "service" },
					"protection": { "old": true, "new": false }
				}
			],
			"summary": {
				"total_changes": 1,
				"by_action": { "UPDATE": 1 },
				"by_resource": { "api": 1 },
				"protection_changes": { "protecting": 0, "unprotecting": 1 }
			},
			"execution_order": ["1-u-api-service"]
		}`

		file := filepath.Join(tmpDir, "protected-plan.json")
		err := os.WriteFile(file, []byte(protectedPlan), 0o600)
		require.NoError(t, err)

		plan, err := LoadPlan(file, nil)
		require.NoError(t, err)
		require.Len(t, plan.Changes, 1)

		pc, ok := plan.Changes[0].Protection.(planner.ProtectionChange)
		if !ok {
			t.Fatalf("expected ProtectionChange, got %T", plan.Changes[0].Protection)
		}
		assert.True(t, pc.Old)
		assert.False(t, pc.New)
	})
}

package executor

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDeferredEnvPlaceholders(t *testing.T) {
	t.Setenv("EXEC_DESCRIPTION", "resolved-description")
	t.Setenv("EXEC_PORTAL_ID", "12345678-1234-5678-1234-567812345678")
	t.Setenv("EXEC_AUTH_ID", "87654321-4321-8765-4321-876543218765")

	exec := New(nil, nil, false)
	change := planner.PlannedChange{
		Fields: map[string]any{
			"description": "__ENV__:EXEC_DESCRIPTION",
			"metadata": map[string]any{
				"note": "__ENV__:EXEC_DESCRIPTION",
			},
		},
		References: map[string]planner.ReferenceInfo{
			"portal_id": {
				Ref: "__ENV__:EXEC_PORTAL_ID",
			},
			"auth_strategy_ids": {
				IsArray: true,
				Refs:    []string{"__ENV__:EXEC_AUTH_ID"},
			},
		},
	}

	err := exec.resolveDeferredEnvPlaceholders(&change)
	require.NoError(t, err)

	assert.Equal(t, "resolved-description", change.Fields["description"])
	metadata := change.Fields["metadata"].(map[string]any)
	assert.Equal(t, "resolved-description", metadata["note"])

	portalRef := change.References["portal_id"]
	assert.Equal(t, "12345678-1234-5678-1234-567812345678", portalRef.Ref)
	assert.Equal(t, "12345678-1234-5678-1234-567812345678", portalRef.ID)

	authRefs := change.References["auth_strategy_ids"]
	assert.Equal(t, []string{"87654321-4321-8765-4321-876543218765"}, authRefs.Refs)
	assert.Equal(t, []string{"87654321-4321-8765-4321-876543218765"}, authRefs.ResolvedIDs)
}

func TestResolveDeferredEnvPlaceholders_MissingVariable(t *testing.T) {
	exec := New(nil, nil, false)
	change := planner.PlannedChange{
		Fields: map[string]any{
			"description": "__ENV__:MISSING_EXEC_ENV",
		},
	}

	err := exec.resolveDeferredEnvPlaceholders(&change)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment variable not set: MISSING_EXEC_ENV")
}

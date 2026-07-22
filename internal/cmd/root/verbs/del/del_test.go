package del

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteHelpUsesDeleteForDeleteModePlans(t *testing.T) {
	cmd, err := NewDeleteCmd()
	require.NoError(t, err)

	assert.Contains(t, cmd.Long, "kongctl plan --mode delete -f <files> | kongctl delete --plan -")
	assert.NotContains(t, cmd.Long, "kongctl sync --plan -")
}

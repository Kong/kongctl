package install

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInstallCmd(t *testing.T) {
	cmd, err := NewInstallCmd()
	require.NoError(t, err)
	require.NotNil(t, cmd)

	assert.Equal(t, "install", cmd.Use)

	var skillsFound bool
	for _, subcmd := range cmd.Commands() {
		if subcmd.Name() == "skills" {
			skillsFound = true
			break
		}
	}

	assert.True(t, skillsFound, "install command should include skills subcommand")
}

func TestInstallVerb(t *testing.T) {
	assert.Equal(t, verbs.Install, Verb)
	assert.Equal(t, "install", Verb.String())
}

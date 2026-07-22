package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyNamespaceDefaultsResolvesNamespaceOriginAndProtected(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
_defaults:
  kongctl:
    namespace: team-a
    protected: true
apis:
  - ref: api-1
    name: API One
organization:
  users:
    - ref: user-1
      email: user@example.com
`), 0o600))

	rs, err := New().LoadFile(configPath)
	require.NoError(t, err)

	require.Len(t, rs.APIs, 1)
	require.NotNil(t, rs.APIs[0].Kongctl)
	assert.Equal(t, "team-a", *rs.APIs[0].Kongctl.Namespace)
	assert.Equal(t, resources.NamespaceOriginFileDefault, rs.APIs[0].Kongctl.NamespaceOrigin)
	require.NotNil(t, rs.APIs[0].Kongctl.Protected)
	assert.True(t, *rs.APIs[0].Kongctl.Protected)

	require.NotNil(t, rs.Organization)
	require.Len(t, rs.Organization.Users, 1)
	require.NotNil(t, rs.Organization.Users[0].Kongctl)
	assert.Equal(t, "team-a", *rs.Organization.Users[0].Kongctl.Namespace)
	assert.Equal(t, resources.NamespaceOriginFileDefault, rs.Organization.Users[0].Kongctl.NamespaceOrigin)
	// Organization users receive a namespace but no protected defaulting.
	assert.Nil(t, rs.Organization.Users[0].Kongctl.Protected)
}

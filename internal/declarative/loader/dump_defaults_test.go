package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestSkipDefaultsOutputLoadsWithAPIDefaultsRestored(t *testing.T) {
	input := []byte(`
portals:
  - ref: developer-portal
    name: Developer Portal
    authentication_enabled: true
    rbac_enabled: true
`)

	filtered, err := resources.OmitAPIDefaults(input)
	require.NoError(t, err)
	require.NotContains(t, string(filtered), "authentication_enabled")

	loader := New()
	resourceSet, err := loader.parseYAML(strings.NewReader(string(filtered)), "inline", "")
	require.NoError(t, err)
	require.NoError(t, loader.validateResourceSet(resourceSet))
	require.Len(t, resourceSet.Portals, 1)
	require.NotNil(t, resourceSet.Portals[0].AuthenticationEnabled)
	require.True(t, *resourceSet.Portals[0].AuthenticationEnabled)
	require.NotNil(t, resourceSet.Portals[0].RbacEnabled)
	require.True(t, *resourceSet.Portals[0].RbacEnabled)
}

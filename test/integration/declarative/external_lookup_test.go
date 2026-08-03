//go:build integration

package declarative_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestExternalLookupTagsAcrossRelationshipKinds(t *testing.T) {
	t.Parallel()

	configFile := filepath.Join(t.TempDir(), "external-lookups.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(`
apis:
  - ref: products
    name: Products
    publications:
      - ref: products-publication
        portal_id: !external name:Shared Portal

gateway_services:
  - ref: billing-service
    control_plane: !lookup {name: Shared Control Plane}
    _external:
      id: service-id
`), 0o600))

	rs, err := loader.New().LoadFile(configFile)
	require.NoError(t, err)
	require.Len(t, rs.APIPublications, 1)
	require.Len(t, rs.GatewayServices, 1)

	publicationLookup, ok := tags.ParseExternalPlaceholder(rs.APIPublications[0].PortalID)
	require.True(t, ok)
	parentLookup, ok := tags.ParseExternalPlaceholder(rs.GatewayServices[0].ControlPlane)
	require.True(t, ok)
	require.Equal(t, map[string]string{"name": "Shared Portal"}, publicationLookup.MatchFields)
	require.Equal(t, map[string]string{"name": "Shared Control Plane"}, parentLookup.MatchFields)
}

func TestNestedEnvExternalLookupTags(t *testing.T) {
	t.Setenv("BLOCK_PORTAL_NAME", "Block Portal")
	t.Setenv("FLOW_CONTROL_PLANE_NAME", "Flow Control Plane")

	configFile := filepath.Join(t.TempDir(), "nested-external-lookups.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(`
apis:
  - ref: products
    name: Products
    publications:
      - ref: products-publication
        portal_id: !lookup
          name: !env BLOCK_PORTAL_NAME

gateway_services:
  - ref: billing-service
    control_plane: !external {name: !env FLOW_CONTROL_PLANE_NAME}
    _external:
      id: service-id
`), 0o600))

	rs, err := loader.New().LoadFile(configFile)
	require.NoError(t, err)
	require.False(t, rs.HasEnvSources())

	publicationLookup, ok := tags.ParseExternalPlaceholder(rs.APIPublications[0].PortalID)
	require.True(t, ok)
	require.Equal(t, map[string]string{"name": "Block Portal"}, publicationLookup.MatchFields)
	require.Equal(t, []string{"name"}, publicationLookup.SensitiveFields)

	parentLookup, ok := tags.ParseExternalPlaceholder(rs.GatewayServices[0].ControlPlane)
	require.True(t, ok)
	require.Equal(t, map[string]string{"name": "Flow Control Plane"}, parentLookup.MatchFields)
	require.Equal(t, []string{"name"}, parentLookup.SensitiveFields)
}

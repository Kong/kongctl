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

package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveDeckConfigPaths(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "gateway-service.yaml")
	require.NoError(t, os.WriteFile(statePath, []byte("_format_version: \"3.0\""), 0o600))

	config := `
control_planes:
  - ref: cp
    name: "cp"
    cluster_type: "CLUSTER_TYPE_SERVERLESS"
    _deck:
      files:
        - "gateway-service.yaml"
    gateway_services:
      - ref: svc
        _external:
          selector:
            matchFields:
              name: "svc"
`
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	loader := New()
	rs, err := loader.LoadFile(configPath)
	require.NoError(t, err)
	require.Len(t, rs.ControlPlanes, 1)
	require.NotNil(t, rs.ControlPlanes[0].Deck)
	require.Equal(t, []string{"gateway-service.yaml"}, rs.ControlPlanes[0].Deck.Files)
	require.Equal(t, dir, rs.ControlPlanes[0].DeckBaseDir())
}

func TestResolveDeckConfigPathsBaseDirBoundary(t *testing.T) {
	rootDir := t.TempDir()
	configDir := filepath.Join(rootDir, "configs")
	require.NoError(t, os.MkdirAll(configDir, 0o700))

	config := `
control_planes:
  - ref: cp
    name: "cp"
    cluster_type: "CLUSTER_TYPE_SERVERLESS"
    _deck:
      files:
        - "../../../../outside.yaml"
    gateway_services:
      - ref: svc
        _external:
          selector:
            matchFields:
              name: "svc"
`
	configPath := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	loader := NewWithBaseDir(rootDir)
	_, err := loader.LoadFile(configPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside base dir")
}

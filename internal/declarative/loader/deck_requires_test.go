package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveDeckRequiresPaths(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "gateway-service.yaml")
	require.NoError(t, os.WriteFile(statePath, []byte("_format_version: \"3.0\""), 0o600))

	config := `
control_planes:
  - ref: cp
    name: "cp"
    cluster_type: "CLUSTER_TYPE_SERVERLESS"
    gateway_services:
      - ref: svc
        _external:
          selector:
            matchFields:
              name: "svc"
          requires:
            deck:
              - args: ["gateway", "{{kongctl.mode}}", "gateway-service.yaml"]
`
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	loader := New()
	rs, err := loader.LoadFile(configPath)
	require.NoError(t, err)
	require.Len(t, rs.GatewayServices, 1)

	steps := rs.GatewayServices[0].External.Requires.Deck
	require.Len(t, steps, 1)
	require.Equal(t, []string{"gateway", "{{kongctl.mode}}", "gateway-service.yaml"}, steps[0].Args)
	require.Equal(t, dir, rs.GatewayServices[0].DeckBaseDir())
}

func TestResolveDeckRequiresPathsBaseDirBoundary(t *testing.T) {
	rootDir := t.TempDir()
	configDir := filepath.Join(rootDir, "configs")
	require.NoError(t, os.MkdirAll(configDir, 0o700))

	config := `
control_planes:
  - ref: cp
    name: "cp"
    cluster_type: "CLUSTER_TYPE_SERVERLESS"
    gateway_services:
      - ref: svc
        _external:
          selector:
            matchFields:
              name: "svc"
          requires:
            deck:
              - args: ["gateway", "{{kongctl.mode}}", "../../../../outside.yaml"]
`
	configPath := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	loader := NewWithBaseDir(rootDir)
	_, err := loader.LoadFile(configPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside base dir")
}

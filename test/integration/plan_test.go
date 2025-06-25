//go:build integration
// +build integration

package integration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanCommand_Integration(t *testing.T) {
	// Create test configuration directory
	tmpDir, err := os.MkdirTemp("", "plan-integration-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test YAML file with various resources
	testYAML := `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: "Integration test portal"

application_auth_strategies:
  - ref: oauth-strategy
    name: "OAuth Strategy"
    display_name: "OAuth Auth"
    strategy_type: key_auth
    configs:
      key_auth:
        key_names: ["api-key"]

control_planes:
  - ref: test-cp
    name: "Test Control Plane"
    description: "Integration test control plane"

apis:
  - ref: test-api
    name: "Test API"
    description: "Integration test API"
    versions:
      - ref: test-api-v1
        name: "v1"
        publish_status: "published"
    publications:
      - ref: test-api-pub
        portal_id: test-portal
        publish_status: "published"
    implementations:
      - ref: test-api-impl
        type: "proxy"
        service:
          id: "12345678-1234-1234-1234-123456789012"
          control_plane_id: test-cp
`
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(yamlFile, []byte(testYAML), 0600)
	require.NoError(t, err)

	// Create plan command
	cmd, err := plan.NewPlanCmd()
	require.NoError(t, err)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Set arguments
	cmd.SetArgs([]string{"--dir", tmpDir})

	// Execute command
	err = cmd.Execute()
	assert.NoError(t, err)

	// Verify output
	outputStr := output.String()
	assert.Contains(t, outputStr, "Configuration loaded successfully:")
	assert.Contains(t, outputStr, "1 portal(s) found: \"test-portal\"")
	assert.Contains(t, outputStr, "1 auth strategy(ies) found: \"oauth-strategy\"")
	assert.Contains(t, outputStr, "1 control plane(s) found: \"test-cp\"")
	assert.Contains(t, outputStr, "1 API(s) found: \"test-api\"")
	assert.Contains(t, outputStr, "Plan generation not yet implemented")
}

func TestPlanCommand_MultiFile(t *testing.T) {
	// Create test configuration directory
	tmpDir, err := os.MkdirTemp("", "plan-multifile-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create multiple YAML files
	portalsYAML := `
portals:
  - ref: portal1
    name: "Portal One"
  - ref: portal2
    name: "Portal Two"
`
	err = os.WriteFile(filepath.Join(tmpDir, "portals.yaml"), []byte(portalsYAML), 0600)
	require.NoError(t, err)

	apisYAML := `
apis:
  - ref: api1
    name: "API One"
  - ref: api2
    name: "API Two"
`
	err = os.WriteFile(filepath.Join(tmpDir, "apis.yaml"), []byte(apisYAML), 0600)
	require.NoError(t, err)

	// Create plan command
	cmd, err := plan.NewPlanCmd()
	require.NoError(t, err)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Set arguments
	cmd.SetArgs([]string{"--dir", tmpDir})

	// Execute command
	err = cmd.Execute()
	assert.NoError(t, err)

	// Verify output shows resources from both files
	outputStr := output.String()
	assert.Contains(t, outputStr, "2 portal(s) found:")
	assert.Contains(t, outputStr, "\"portal1\"")
	assert.Contains(t, outputStr, "\"portal2\"")
	assert.Contains(t, outputStr, "2 API(s) found:")
	assert.Contains(t, outputStr, "\"api1\"")
	assert.Contains(t, outputStr, "\"api2\"")
}

func TestPlanCommand_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		expectError string
	}{
		{
			name: "missing ref",
			yaml: `
portals:
  - name: "Portal without ref"
`,
			expectError: "portal ref is required",
		},
		{
			name: "duplicate refs",
			yaml: `
portals:
  - ref: dup-portal
    name: "Portal One"
  - ref: dup-portal
    name: "Portal Two"
`,
			expectError: "duplicate portal ref",
		},
		{
			name: "invalid reference",
			yaml: `
application_auth_strategies:
  - ref: auth1
    name: "Auth Strategy"
    display_name: "Test Auth"
    strategy_type: key_auth

apis:
  - ref: api1
    name: "API One"
    publications:
      - ref: pub1
        portal_id: non-existent-portal
`,
			expectError: "references unknown portal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			tmpDir, err := os.MkdirTemp("", "plan-error-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Write test YAML
			err = os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(tt.yaml), 0600)
			require.NoError(t, err)

			// Create plan command
			cmd, err := plan.NewPlanCmd()
			require.NoError(t, err)

			// Capture output
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			// Set arguments
			cmd.SetArgs([]string{"--dir", tmpDir})

			// Execute command - should fail
			err = cmd.Execute()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestPlanCommand_NonExistentDirectory(t *testing.T) {
	// Create plan command
	cmd, err := plan.NewPlanCmd()
	require.NoError(t, err)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Set arguments with non-existent directory
	cmd.SetArgs([]string{"--dir", "/non/existent/directory/path"})

	// Execute command - should fail
	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load configuration")
	assert.Contains(t, err.Error(), "failed to stat path")
}

func TestPlanCommand_SingleFile(t *testing.T) {
	// Create test file
	tmpDir, err := os.MkdirTemp("", "plan-single-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testYAML := `
portals:
  - ref: single-portal
    name: "Single Portal"
`
	yamlFile := filepath.Join(tmpDir, "single.yaml")
	err = os.WriteFile(yamlFile, []byte(testYAML), 0600)
	require.NoError(t, err)

	// Create plan command
	cmd, err := plan.NewPlanCmd()
	require.NoError(t, err)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Set arguments to point to single file
	cmd.SetArgs([]string{"--dir", yamlFile})

	// Execute command
	err = cmd.Execute()
	assert.NoError(t, err)

	// Verify output
	outputStr := output.String()
	assert.Contains(t, outputStr, "Configuration loaded successfully:")
	assert.Contains(t, outputStr, "1 portal(s) found: \"single-portal\"")
}

func TestPlanCommand_EmptyConfig(t *testing.T) {
	// Create empty test directory
	tmpDir, err := os.MkdirTemp("", "plan-empty-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create empty YAML file
	err = os.WriteFile(filepath.Join(tmpDir, "empty.yaml"), []byte(""), 0600)
	require.NoError(t, err)

	// Create plan command
	cmd, err := plan.NewPlanCmd()
	require.NoError(t, err)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Set arguments
	cmd.SetArgs([]string{"--dir", tmpDir})

	// Execute command
	err = cmd.Execute()
	assert.NoError(t, err)

	// Verify output shows empty configuration
	outputStr := output.String()
	assert.Contains(t, outputStr, "Configuration loaded successfully:")
	// Should not show any resources
	assert.NotContains(t, outputStr, "portal(s) found:")
	assert.NotContains(t, outputStr, "auth strategy(ies) found:")
	assert.Contains(t, outputStr, "Plan generation not yet implemented")
}
//go:build integration
// +build integration

package integration

import (
	"bytes"
	"fmt"
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
	cmd.SetArgs([]string{"-f", tmpDir})

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
	cmd.SetArgs([]string{"-f", tmpDir})

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
			cmd.SetArgs([]string{"-f", tmpDir})

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
	cmd.SetArgs([]string{"-f", "/non/existent/directory/path"})

	// Execute command - should fail
	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse sources")
	assert.Contains(t, err.Error(), "does not exist")
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
	cmd.SetArgs([]string{"-f", yamlFile})

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
	cmd.SetArgs([]string{"-f", tmpDir})

	// Execute command - should fail
	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no resources found in configuration files")
}

func TestPlanCommand_MultipleFileFlags(t *testing.T) {
	// Create test files
	tmpDir, err := os.MkdirTemp("", "plan-multi-flags-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create portal file
	portalYAML := `
portals:
  - ref: test-portal
    name: "Test Portal"
`
	portalFile := filepath.Join(tmpDir, "portal.yaml")
	err = os.WriteFile(portalFile, []byte(portalYAML), 0600)
	require.NoError(t, err)

	// Create auth strategy file
	authYAML := `
application_auth_strategies:
  - ref: test-auth
    name: "Test Auth"
    display_name: "Test Auth Strategy"
    strategy_type: key_auth
    configs:
      key_auth:
        key_names: ["api-key"]
`
	authFile := filepath.Join(tmpDir, "auth.yaml")
	err = os.WriteFile(authFile, []byte(authYAML), 0600)
	require.NoError(t, err)

	// Create plan command
	cmd, err := plan.NewPlanCmd()
	require.NoError(t, err)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Set arguments with multiple -f flags
	cmd.SetArgs([]string{"-f", portalFile, "-f", authFile})

	// Execute command
	err = cmd.Execute()
	assert.NoError(t, err)

	// Verify output
	outputStr := output.String()
	assert.Contains(t, outputStr, "Configuration loaded successfully:")
	assert.Contains(t, outputStr, "1 portal(s) found: \"test-portal\"")
	assert.Contains(t, outputStr, "1 auth strategy(ies) found: \"test-auth\"")
}

func TestPlanCommand_CommaSeparatedFiles(t *testing.T) {
	// Create test files
	tmpDir, err := os.MkdirTemp("", "plan-comma-sep-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create API file
	apiYAML := `
apis:
  - ref: test-api
    name: "Test API"
`
	apiFile := filepath.Join(tmpDir, "api.yaml")
	err = os.WriteFile(apiFile, []byte(apiYAML), 0600)
	require.NoError(t, err)

	// Create control plane file
	cpYAML := `
control_planes:
  - ref: test-cp
    name: "Test CP"
`
	cpFile := filepath.Join(tmpDir, "cp.yaml")
	err = os.WriteFile(cpFile, []byte(cpYAML), 0600)
	require.NoError(t, err)

	// Create plan command
	cmd, err := plan.NewPlanCmd()
	require.NoError(t, err)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Set arguments with comma-separated files
	cmd.SetArgs([]string{"-f", fmt.Sprintf("%s,%s", apiFile, cpFile)})

	// Execute command
	err = cmd.Execute()
	assert.NoError(t, err)

	// Verify output
	outputStr := output.String()
	assert.Contains(t, outputStr, "Configuration loaded successfully:")
	assert.Contains(t, outputStr, "1 API(s) found: \"test-api\"")
	assert.Contains(t, outputStr, "1 control plane(s) found: \"test-cp\"")
}

func TestPlanCommand_RecursiveFlag(t *testing.T) {
	// Create nested directory structure
	tmpDir, err := os.MkdirTemp("", "plan-recursive-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	// Create file in subdirectory
	subYAML := `
portals:
  - ref: sub-portal
    name: "Sub Portal"
`
	err = os.WriteFile(filepath.Join(subDir, "sub.yaml"), []byte(subYAML), 0600)
	require.NoError(t, err)

	// Test without recursive flag - should fail
	cmd, err := plan.NewPlanCmd()
	require.NoError(t, err)

	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	cmd.SetArgs([]string{"-f", tmpDir})
	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no YAML files found")
	assert.Contains(t, err.Error(), "Use -R to search subdirectories")

	// Test with recursive flag - should succeed
	cmd, err = plan.NewPlanCmd()
	require.NoError(t, err)

	output.Reset()
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	cmd.SetArgs([]string{"-f", tmpDir, "-R"})
	err = cmd.Execute()
	assert.NoError(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Configuration loaded successfully:")
	assert.Contains(t, outputStr, "1 portal(s) found: \"sub-portal\"")
}

func TestPlanCommand_NoArgs(t *testing.T) {
	// Create a temp directory and cd into it to avoid reading actual project files
	tmpDir, err := os.MkdirTemp("", "plan-no-args-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Save current directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)

	// Change to temp directory
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer os.Chdir(originalDir) // Restore original directory

	// Create plan command
	cmd, err := plan.NewPlanCmd()
	require.NoError(t, err)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Execute command with no args - should fail
	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no configuration files found in current directory")
	assert.Contains(t, err.Error(), "Use -f to specify files or directories")
}

func TestPlanCommand_ValidYAMLNoResources(t *testing.T) {
	// Create test file with valid YAML but no resources
	tmpDir, err := os.MkdirTemp("", "plan-no-resources-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a YAML file with valid content but no resources
	yamlContent := `# This is a valid YAML file
# But it contains no resources
some_key: some_value
another_key:
  nested: value
`
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	err = os.WriteFile(yamlFile, []byte(yamlContent), 0600)
	require.NoError(t, err)

	// Create plan command
	cmd, err := plan.NewPlanCmd()
	require.NoError(t, err)

	// Capture output
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	// Set arguments
	cmd.SetArgs([]string{"-f", yamlFile})

	// Execute command - should fail
	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no resources found in configuration files")
}
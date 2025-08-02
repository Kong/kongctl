package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSyncCmd(t *testing.T) {
	cmd, err := NewSyncCmd()
	if err != nil {
		t.Fatalf("NewSyncCmd should not return an error: %v", err)
	}
	if cmd == nil {
		t.Fatal("NewSyncCmd should return a command")
	}

	// Test basic command properties
	assert.Equal(t, "sync", cmd.Use, "Command use should be 'sync'")
	assert.Contains(t, cmd.Short, "Full state synchronization",
		"Short description should mention synchronization")
	assert.Contains(t, cmd.Long, "Synchronize configuration with Kong Konnect", 
		"Long description should mention synchronize")
	assert.Contains(t, cmd.Example, meta.CLIName, "Examples should include CLI name")

	// Test that konnect subcommand is added
	subcommands := cmd.Commands()
	if len(subcommands) != 1 {
		t.Fatalf("Should have exactly one subcommand, got %d", len(subcommands))
	}
	assert.Equal(t, "konnect", subcommands[0].Name(), "Subcommand should be 'konnect'")
}

func TestSyncCmdVerb(t *testing.T) {
	assert.Equal(t, verbs.Sync, Verb, "Verb constant should be verbs.Sync")
	assert.Equal(t, "sync", Verb.String(), "Verb string should be 'sync'")
}

func TestSyncCmdHelpText(t *testing.T) {
	cmd, err := NewSyncCmd()
	if err != nil {
		t.Fatalf("NewSyncCmd should not return an error: %v", err)
	}

	// Test that help text contains expected content
	assert.Contains(t, cmd.Short, "Full state synchronization", "Short should mention synchronization")
	assert.Contains(t, cmd.Long, "Synchronize configuration", "Long should mention configuration")
	assert.Contains(t, cmd.Example, "-f", "Examples should show -f flag usage")
	assert.Contains(t, cmd.Example, "--dry-run", "Examples should show dry-run option")
	assert.Contains(t, cmd.Example, "help sync", "Examples should mention extended help")
}

func TestSyncCmd_Flags(t *testing.T) {
	cmd, err := NewSyncCmd()
	require.NoError(t, err)

	// Find konnect subcommand
	var konnectCmd *cobra.Command
	for _, subcmd := range cmd.Commands() {
		if subcmd.Name() == "konnect" {
			konnectCmd = subcmd
			break
		}
	}
	require.NotNil(t, konnectCmd, "Should have konnect subcommand")

	// Test flags on konnect subcommand
	fileFlag := konnectCmd.Flags().Lookup("filename")
	assert.NotNil(t, fileFlag, "Should have --filename flag")
	assert.Equal(t, "f", fileFlag.Shorthand, "Should have -f shorthand")
	assert.Contains(t, fileFlag.Usage, "Filename", "Usage should mention filename")

	autoApproveFlag := konnectCmd.Flags().Lookup("auto-approve")
	assert.NotNil(t, autoApproveFlag, "Should have --auto-approve flag")
	assert.Contains(t, autoApproveFlag.Usage, "Skip confirmation", "Usage should mention skipping confirmation")
	assert.Equal(t, "false", autoApproveFlag.DefValue)

	dryRunFlag := konnectCmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag, "Should have --dry-run flag")
	assert.Contains(t, dryRunFlag.Usage, "Preview", "Usage should mention preview")
	assert.Equal(t, "false", dryRunFlag.DefValue)
}

func TestSyncCmd_ConfigFileValidation(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectError   bool
		errorContains string
	}{
		{
			name: "valid portal config",
			configContent: `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: "Test description"
`,
			expectError: false,
		},
		{
			name: "valid API config",
			configContent: `
apis:
  - ref: test-api
    name: "Test API"
    version: "1.0.0"
`,
			expectError: false,
		},
		{
			name: "invalid YAML",
			configContent: `
portals:
  - ref: test-portal
    name: "Test Portal
    description: "Missing quote
`,
			expectError:   true,
			errorContains: "yaml",
		},
		{
			name:          "empty file",
			configContent: "",
			expectError:   false, // Empty file is valid YAML
		},
		{
			name: "multiple resource types",
			configContent: `
portals:
  - ref: portal-1
    name: "Portal 1"

apis:
  - ref: api-1
    name: "API 1"
    version: "1.0.0"
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp config file
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "config.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(tt.configContent), 0600))

			// Create sync command
			cmd, err := NewSyncCmd()
			require.NoError(t, err)

			// Note: Actual execution will fail without proper SDK setup,
			// but this tests the command structure and flag handling
			
			// Verify the command accepts the file flag
			var konnectCmd *cobra.Command
			for _, subcmd := range cmd.Commands() {
				if subcmd.Name() == "konnect" {
					konnectCmd = subcmd
					break
				}
			}
			require.NotNil(t, konnectCmd)

			// Test that we can set the filename flag
			err = konnectCmd.Flags().Set("filename", configFile)
			assert.NoError(t, err)

			// Verify the flag was set
			fileFlag := konnectCmd.Flags().Lookup("filename")
			// The flag value is a string slice, so it will be formatted with brackets
			assert.Equal(t, "["+configFile+"]", fileFlag.Value.String())
		})
	}
}

func TestSyncCmd_DirectorySupport(t *testing.T) {
	// Create a directory with multiple config files
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "configs")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	// Create multiple config files
	portalConfig := `
portals:
  - ref: portal-1
    name: "Portal 1"
`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "portals.yaml"), []byte(portalConfig), 0600))

	apiConfig := `
apis:
  - ref: api-1
    name: "API 1"
    version: "1.0.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "apis.yaml"), []byte(apiConfig), 0600))

	// Create sync command
	cmd, err := NewSyncCmd()
	require.NoError(t, err)

	// Find konnect subcommand
	var konnectCmd *cobra.Command
	for _, subcmd := range cmd.Commands() {
		if subcmd.Name() == "konnect" {
			konnectCmd = subcmd
			break
		}
	}
	require.NotNil(t, konnectCmd)

	// Test that we can set a directory as the filename flag
	err = konnectCmd.Flags().Set("filename", configDir)
	assert.NoError(t, err)

	// Verify the flag was set
	fileFlag := konnectCmd.Flags().Lookup("filename")
	// The flag value is a string slice, so it will be formatted with brackets
	assert.Equal(t, "["+configDir+"]", fileFlag.Value.String())
}

func TestSyncCmd_MultipleFileSupport(t *testing.T) {
	// Create multiple config files
	tempDir := t.TempDir()
	
	portalFile := filepath.Join(tempDir, "portals.yaml")
	portalConfig := `
portals:
  - ref: portal-1
    name: "Portal 1"
`
	require.NoError(t, os.WriteFile(portalFile, []byte(portalConfig), 0600))

	apiFile := filepath.Join(tempDir, "apis.yaml")
	apiConfig := `
apis:
  - ref: api-1
    name: "API 1"
    version: "1.0.0"
`
	require.NoError(t, os.WriteFile(apiFile, []byte(apiConfig), 0600))

	// Create sync command
	cmd, err := NewSyncCmd()
	require.NoError(t, err)

	// Find konnect subcommand
	var konnectCmd *cobra.Command
	for _, subcmd := range cmd.Commands() {
		if subcmd.Name() == "konnect" {
			konnectCmd = subcmd
			break
		}
	}
	require.NotNil(t, konnectCmd)

	// Test that the filename flag is a string slice
	fileFlag := konnectCmd.Flags().Lookup("filename")
	assert.NotNil(t, fileFlag)
	
	// The flag should accept multiple values
	assert.Equal(t, "stringSlice", fileFlag.Value.Type())
}
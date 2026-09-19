package root

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/cmd/common"
	extensioncore "github.com/kong/kongctl/internal/extensions"
)

func TestConfigFileEnvironment(t *testing.T) {
	for _, name := range []string{"environment", "flag", "empty", "expansion", "extension", "extension flag"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			envPath := filepath.Join(dir, "workspace config.yaml")
			flagPath := filepath.Join(dir, "flag.yaml")
			contents := []byte("workspace:\n  output: json\n  color-theme: auto\n")
			requireNoError(t, os.WriteFile(envPath, contents, 0o600))
			requireNoError(t, os.WriteFile(flagPath, contents, 0o600))
			t.Setenv("KONGCTL_CONFIG_FILE", envPath)
			t.Setenv(extensioncore.ContextEnvName, "")
			args := []string{"get", "profiles", "--profile", "workspace"}
			wantPath := envPath
			switch name {
			case "flag", "extension flag":
				args = append(args, "--config-file", flagPath)
				wantPath = flagPath
			case "empty":
				t.Setenv("KONGCTL_CONFIG_FILE", "")
			case "expansion":
				t.Setenv("KONGCTL_TEST_CONFIG_DIR", dir)
				t.Setenv("KONGCTL_CONFIG_FILE", "$KONGCTL_TEST_CONFIG_DIR/workspace config.yaml")
			}
			if strings.HasPrefix(name, "extension") {
				contextPath := filepath.Join(dir, "extension.json")
				data, err := json.Marshal(extensioncore.RuntimeContext{
					SchemaVersion: 1,
					Resolved: extensioncore.ResolvedContext{
						ConfigFile: flagPath,
					},
				})
				requireNoError(t, err)
				requireNoError(t, os.WriteFile(contextPath, data, 0o600))
				t.Setenv(extensioncore.ContextEnvName, contextPath)
				wantPath = flagPath
				if name == "extension flag" {
					args = append(args[:len(args)-2], "--config-file", envPath)
					wantPath = envPath
				}
			}

			result := executeRootForTest(t, args...)
			if result.exitCode != 0 {
				t.Fatalf("command failed: %s", result.stderr)
			}
			if name == "empty" {
				wantPath = defaultConfigFilePath
			}
			if currConfig == nil || currConfig.GetPath() != wantPath {
				t.Fatalf("expected selected config path %q, got %v", wantPath, currConfig)
			}
			if name != "empty" && currConfig.GetString(common.OutputConfigPath) != "json" {
				t.Fatal("expected output setting from selected file")
			}
		})
	}
}

func TestConfigFileEnvironmentInvalid(t *testing.T) {
	if os.Getenv("KONGCTL_TEST_INVALID_CONFIG_SUBPROCESS") == "1" {
		result := executeRootForTest(t, "get", "profiles")
		os.Exit(result.exitCode)
	}
	for _, name := range []string{"missing", "malformed"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if name == "malformed" {
				requireNoError(t, os.WriteFile(path, []byte("default: [invalid\n"), 0o600))
			}
			t.Setenv("KONGCTL_CONFIG_FILE", path)
			t.Setenv(extensioncore.ContextEnvName, "")
			t.Setenv("KONGCTL_TEST_INVALID_CONFIG_SUBPROCESS", "1")
			executable, err := os.Executable()
			requireNoError(t, err)
			cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestConfigFileEnvironmentInvalid$")
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("invalid custom config unexpectedly succeeded")
			}
			want := "provided config file path does not exist"
			if name == "malformed" {
				want = "While parsing config"
			}
			if !strings.Contains(string(output), want) {
				t.Fatalf("expected %q in error, got %s", want, output)
			}
		})
	}
}

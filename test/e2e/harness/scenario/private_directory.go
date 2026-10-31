//go:build e2e

package scenario

import (
	"fmt"
	"os"
	"regexp"
	"testing"
)

var privateDirectoryVariable = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Private outputs stay outside the public artifact tree and are removed even
// when a scenario stops before its last command. Only the path enters vars.
func registerPrivateTempDirectory(t *testing.T, name string, vars map[string]any) error {
	t.Helper()
	if name == "" {
		return nil
	}
	if !privateDirectoryVariable.MatchString(name) {
		return fmt.Errorf("invalid privateTempDirVar %q", name)
	}
	if _, exists := vars[name]; exists {
		return fmt.Errorf("privateTempDirVar %q is already defined", name)
	}
	path, err := os.MkdirTemp(t.TempDir(), "private-")
	if err != nil {
		return fmt.Errorf("create private scenario directory: %w", err)
	}
	vars[name] = path
	return nil
}

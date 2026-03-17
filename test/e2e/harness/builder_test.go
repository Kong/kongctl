//go:build e2e

package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFilePreservesExecutablePermissions(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	if err := os.WriteFile(src, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "dst")
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copy file: %v", err)
	}

	fi, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat destination file: %v", err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Fatalf("destination perms = %o, want 755", fi.Mode().Perm())
	}
}

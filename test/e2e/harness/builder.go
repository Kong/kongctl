//go:build e2e

package harness

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

// BinPath returns the path to the e2e-built kongctl binary. It builds once per test session.
func BinPath() (string, error) {
	buildOnce.Do(func() {
		// Ensure run dir exists and logging is configured.
		rd, err := ensureRunDir()
		if err != nil {
			buildErr = err
			return
		}
		// Allow overriding the binary path for faster iteration.
		if override := os.Getenv("KONGCTL_E2E_BIN"); override != "" {
			if fi, err := os.Stat(override); err == nil && !fi.IsDir() {
				Debugf("Using overridden binary: %s", override)
				// Copy into run/bin to co-locate artifacts
				dest := filepath.Join(rd, "bin", exeName("kongctl"))
				_ = os.MkdirAll(filepath.Dir(dest), 0o755)
				if err := copyFile(override, dest); err == nil {
					binPath = dest
					Infof("Copied override binary to: %s", dest)
				} else {
					// Fallback to direct use
					Debugf("Copy failed, using override path directly: %v", err)
					binPath = override
				}
				return
			}
			buildErr = errors.New("KONGCTL_E2E_BIN set but not a file")
			return
		}

		modRoot, err := moduleRoot()
		if err != nil {
			buildErr = err
			return
		}
		Debugf("Module root located at: %s", modRoot)
		out := filepath.Join(rd, "bin", exeName("kongctl"))
		_ = os.MkdirAll(filepath.Dir(out), 0o755)
		cmd := exec.Command("go", "build", "-trimpath", "-o", out)
		cmd.Dir = modRoot
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		// Optional ldflags via env for reproducibility if desired.
		if ld := os.Getenv("KONGCTL_E2E_LDFLAGS"); ld != "" {
			Debugf("Building with ldflags: %s", ld)
			cmd = exec.Command("go", "build", "-trimpath", "-ldflags", ld, "-o", out)
			cmd.Dir = modRoot
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		}
		Debugf("Executing: %s (dir=%s)", strings.Join(cmd.Args, " "), cmd.Dir)
		if err := cmd.Run(); err != nil {
			buildErr = err
			return
		}
		Infof("Built kongctl binary: %s", out)
		binPath = out
	})
	return binPath, buildErr
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// moduleRoot finds the repository root by walking up to the nearest go.mod.
func moduleRoot() (string, error) {
	// Use the caller file to start from the repository tree.
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	for i := 0; i < 10; i++ { // avoid infinite loops
		cand := filepath.Join(dir, "go.mod")
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("could not locate module root (go.mod)")
}

// ensureRunDir creates or returns the per-run artifacts dir and configures logging.
func ensureRunDir() (string, error) {
	if runDirPath != "" {
		return runDirPath, nil
	}
	// User-provided root folder
	if base := os.Getenv("KONGCTL_E2E_ARTIFACTS_DIR"); base != "" {
		if err := os.MkdirAll(base, 0o755); err != nil {
			return "", err
		}
		initRunLogging(base)
		Infof("Using provided artifacts dir: %s", base)
		// persist path for Makefile discovery
		if mr, err := moduleRoot(); err == nil {
			_ = os.WriteFile(filepath.Join(mr, ".e2e_artifacts_dir"), []byte(base+"\n"), 0o644)
		}
		return base, nil
	}
	// Create a temp run directory
	d, err := os.MkdirTemp("", "kongctl-e2e-run-")
	if err != nil {
		return "", err
	}
	initRunLogging(d)
	Infof("Created artifacts dir: %s", d)
	// persist path for Makefile discovery
	if mr, err := moduleRoot(); err == nil {
		_ = os.WriteFile(filepath.Join(mr, ".e2e_artifacts_dir"), []byte(d+"\n"), 0o644)
	}
	return d, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

package meta

import (
	"strings"
	"sync"
)

const (
	CLIName           = "kongctl"
	DefaultCLIVersion = "dev"
)

var (
	mu         sync.RWMutex
	cliVersion = DefaultCLIVersion
)

// SetCLIVersion updates the process-wide CLI version used for metadata headers.
func SetCLIVersion(version string) {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		trimmed = DefaultCLIVersion
	}

	mu.Lock()
	cliVersion = trimmed
	mu.Unlock()
}

// CLIVersion returns the process-wide CLI version used for metadata headers.
func CLIVersion() string {
	mu.RLock()
	defer mu.RUnlock()
	return cliVersion
}

// UserAgent returns the canonical User-Agent value for kongctl requests.
func UserAgent() string {
	version := CLIVersion()
	if version != DefaultCLIVersion && !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	return CLIName + "/" + version
}

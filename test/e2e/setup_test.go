//go:build e2e

package e2e

import (
	"testing"

	"github.com/kong/kongctl/test/e2e/harness"
)

// TestMain ensures the binary is prepared once before running e2e tests.
func TestMain(m *testing.M) {
	// Initialize artifacts dir and build/resolve binary once so early failures are clear.
	if _, err := harness.BinPath(); err != nil {
		panic(err)
	}
	// Continue with tests.
	m.Run()
}

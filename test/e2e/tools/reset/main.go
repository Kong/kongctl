//go:build e2e

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kong/kongctl/test/e2e/harness"
)

// Usage:
//
//	go run -tags e2e ./test/e2e/tools/reset
//
// Respects KONGCTL_E2E_{KONNECT_PAT,KONNECT_BASE_URL,RESET,ARTIFACTS_DIR} like the harness.
func main() {
	stage := flag.String("stage", "manual-reset", "label used in log output")
	flag.Parse()

	if err := harness.ResetOrg(*stage); err != nil {
		fmt.Fprintf(os.Stderr, "reset failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Konnect organization reset complete")
}

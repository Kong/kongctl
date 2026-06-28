//go:build e2e

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kong/kongctl/test/e2e/harness"
)

func main() {
	stage := flag.String("stage", "adhoc", "label recorded for the reset event (e.g. adhoc)")
	capture := flag.Bool("capture", false, "record a synthetic reset command in e2e artifacts")
	flag.Parse()

	var (
		summary harness.ResetSummary
		err     error
	)
	if *capture {
		summary, err = harness.ResetOrgWithCaptureSummary(*stage)
	} else {
		summary, err = harness.ResetOrgSummary(*stage)
	}
	if printErr := harness.WriteResetDeletionSummary(os.Stdout, summary); printErr != nil && err == nil {
		err = printErr
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "reset failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "reset complete")
}

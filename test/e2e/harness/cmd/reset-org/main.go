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

	var err error
	if *capture {
		err = harness.ResetOrgWithCapture(*stage)
	} else {
		err = harness.ResetOrg(*stage)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "reset failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "reset complete")
}

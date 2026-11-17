// Command scenariorunner wraps `go test -json -run '^Test_Scenarios$' ...` to
// give `make test-e2e-scenarios` a compact live progress view, permanent
// failure/skip call-outs, and a concise final summary, while still writing
// the full raw test output (and every scenario's duration) to run.log.
//
// Usage: scenariorunner -artifacts-dir <dir> -- <go test flags and package>
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out *os.File) int {
	artifactsDir, testArgs, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	cfg := runConfig{
		artifactsDir: artifactsDir,
		testArgs:     testArgs,
		interactive:  isInteractiveWriter(out),
		now:          time.Now,
		heartbeat:    defaultHeartbeat,
		tickInterval: defaultTickInterval,
	}

	code, err := runScenarios(cfg, out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	return code
}

func parseArgs(args []string) (artifactsDir string, testArgs []string, err error) {
	sep := -1
	for i, a := range args {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep < 0 {
		return "", nil, errors.New("usage: scenariorunner -artifacts-dir <dir> -- <go test args...>")
	}

	fs := flag.NewFlagSet("scenariorunner", flag.ContinueOnError)
	dir := fs.String("artifacts-dir", "", "directory to write run.log into")
	if err := fs.Parse(args[:sep]); err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(*dir) == "" {
		return "", nil, errors.New("-artifacts-dir is required")
	}
	return *dir, args[sep+1:], nil
}

// isInteractiveWriter reports whether f is a real terminal.
func isInteractiveWriter(f interface{ Fd() uintptr }) bool {
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

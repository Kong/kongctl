//go:build e2e

package scenario

import (
	"fmt"
	"strings"
)

type retryPlan struct {
	args         []string
	outputFormat string
}

// commandRetryPlan limits recovery to read-only planning for an explicit saved
// apply plan. The original plan and its assertions remain the first attempt.
func commandRetryPlan(cmd Command, args []string, tmplCtx map[string]any) (*retryPlan, error) {
	if len(cmd.ReplanOnRetry) == 0 {
		return nil, nil
	}
	if len(args) == 0 || args[0] != "apply" || cmd.ExpectFail != nil {
		return nil, fmt.Errorf("replanOnRetry requires apply --plan without expectFailure")
	}
	path := ""
	for i, arg := range args {
		if arg == "--plan" && i+1 < len(args) {
			path = args[i+1]
		} else if value, ok := strings.CutPrefix(arg, "--plan="); ok {
			path = value
		}
	}
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("replanOnRetry requires an explicit --plan file")
	}
	planArgs := []string{"plan", "--mode", "apply", "--output-file", path}
	for _, input := range cmd.ReplanOnRetry {
		input = renderString(input, tmplCtx)
		if strings.TrimSpace(input) == "" {
			return nil, fmt.Errorf("replanOnRetry input must not be empty")
		}
		planArgs = append(planArgs, "-f", input)
	}
	return &retryPlan{args: planArgs, outputFormat: cmd.OutputFormat}, nil
}

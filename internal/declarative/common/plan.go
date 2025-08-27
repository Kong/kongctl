package common

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/kong/kongctl/internal/declarative/planner"
)

// LoadPlan loads a plan from the given source.
// If source is "-", reads from stdin.
// Otherwise, reads from the specified file path.
func LoadPlan(source string, stdin io.Reader) (*planner.Plan, error) {
	var planData []byte
	var err error

	if source == "-" {
		// Read from stdin
		planData, err = io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read plan from stdin: %w", err)
		}
	} else {
		// Read from file
		planData, err = os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("failed to read plan file: %w", err)
		}
	}

	// Parse plan
	plan := &planner.Plan{}
	if err := json.Unmarshal(planData, plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	// Basic validation
	if plan.Metadata.Version == "" {
		return nil, fmt.Errorf("invalid plan: missing version")
	}
	if plan.Metadata.Mode == "" {
		return nil, fmt.Errorf("invalid plan: missing mode")
	}

	return plan, nil
}

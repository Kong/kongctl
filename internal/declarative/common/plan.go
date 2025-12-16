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

	normalizePlanProtection(plan)

	// Basic validation
	if plan.Metadata.Version == "" {
		return nil, fmt.Errorf("invalid plan: missing version")
	}
	if plan.Metadata.Mode == "" {
		return nil, fmt.Errorf("invalid plan: missing mode")
	}

	return plan, nil
}

// normalizePlanProtection converts loosely-typed protection values (e.g. maps from JSON)
// into planner.ProtectionChange so downstream logic can consistently detect protection updates.
func normalizePlanProtection(plan *planner.Plan) {
	if plan == nil {
		return
	}

	for i := range plan.Changes {
		plan.Changes[i].Protection = normalizeProtectionValue(plan.Changes[i].Protection)
	}
}

func normalizeProtectionValue(val any) any {
	switch p := val.(type) {
	case map[string]any:
		// JSON plans deserialize struct fields into map[string]any
		oldVal, hasOld := p["old"].(bool)
		newVal, hasNew := p["new"].(bool)
		if hasOld && hasNew {
			return planner.ProtectionChange{Old: oldVal, New: newVal}
		}
		// Fallback to legacy shape
		if prot, ok := p["protected"].(bool); ok {
			return prot
		}
	}
	return val
}

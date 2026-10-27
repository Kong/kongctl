package resources

import (
	"fmt"
	"regexp"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

var aiGatewayMCPTargetName = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

func validateAIGatewayMCPRateLimiting(rate kkComps.AIGatewayMCPServerRateLimiting) error {
	if len(rate.Limits) == 0 {
		return fmt.Errorf("rate_limiting.limits must contain at least one limit")
	}
	seen := map[string]bool{}
	for _, limit := range rate.Limits {
		if (limit.Method == nil) == (limit.Tool == nil) {
			return fmt.Errorf("each rate limit must target exactly one of method or tool")
		}
		if limit.Limit < 1 || limit.Limit > 2147483646 || !limit.WindowSize.IsExact() {
			return fmt.Errorf("invalid rate limit or window_size")
		}
		var name, kind string
		if limit.Method != nil {
			name, kind = *limit.Method, "method"
		} else {
			name, kind = *limit.Tool, "tool"
		}
		if !aiGatewayMCPTargetName.MatchString(name) || (kind == "tool" && len(name) > 128) {
			return fmt.Errorf("invalid rate limit target")
		}
		key := fmt.Sprintf("%s:%s:%d", kind, name, limit.WindowSize)
		if seen[key] {
			return fmt.Errorf("duplicate rate limit target and window_size")
		}
		seen[key] = true
	}
	return nil
}

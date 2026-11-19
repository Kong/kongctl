package resources

import "fmt"

// ValidateAIGatewayUpstreamTargetsConfig enforces the AI Gateway 2.4 upstream
// target rules that the API documents but does not enforce in its schema: an
// Agent or MCP Server config must set exactly one of url or targets, and
// balancer, protocol, and path are only meaningful alongside targets.
func ValidateAIGatewayUpstreamTargetsConfig(
	label, ref string,
	hasURL, hasTargets, hasBalancer, hasProtocol, hasPath bool,
) error {
	if hasURL == hasTargets {
		return fmt.Errorf("%s %s must define exactly one of config.url or config.targets", label, ref)
	}
	if hasTargets {
		return nil
	}
	switch {
	case hasBalancer:
		return fmt.Errorf("%s %s config.balancer is only valid when config.targets is set", label, ref)
	case hasProtocol:
		return fmt.Errorf("%s %s config.protocol is only valid when config.targets is set", label, ref)
	case hasPath:
		return fmt.Errorf("%s %s config.path is only valid when config.targets is set", label, ref)
	}
	return nil
}

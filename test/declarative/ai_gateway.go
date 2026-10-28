package declarative

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// AIGatewayChildCase describes one independently owned child family.
type AIGatewayChildCase struct {
	ResourceType string         `json:"resource_type"`
	Collection   string         `json:"collection"`
	Path         string         `json:"path"`
	Body         map[string]any `json:"body"`
}

//go:embed ai_gateway_owned_children.json
var aiGatewayOwnedChildFixtures []byte

// AIGatewayOwnedChildren returns independent fixtures for the enrolled families.
func AIGatewayOwnedChildren(t *testing.T) []AIGatewayChildCase {
	t.Helper()
	var cases []AIGatewayChildCase
	require.NoError(t, json.Unmarshal(aiGatewayOwnedChildFixtures, &cases))
	return cases
}

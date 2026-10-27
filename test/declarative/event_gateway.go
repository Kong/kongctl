package declarative

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// EventGatewayPolicyCase describes a declarative policy and its Kafka phase.
type EventGatewayPolicyCase struct {
	Phase string         `json:"phase"`
	Body  map[string]any `json:"body"`
}

// Fixtures follow the Event Gateway 1.3 policy API, including both schema sources.
// https://developer.konghq.com/event-gateway/changelog/#1-3-0
//
//go:embed event_gateway_v13.json
var eventGatewayPolicyFixtures []byte

// EventGatewayV13Policies returns independent fixtures for each policy variant.
func EventGatewayV13Policies(t *testing.T) []EventGatewayPolicyCase {
	t.Helper()
	var cases []EventGatewayPolicyCase
	require.NoError(t, json.Unmarshal(eventGatewayPolicyFixtures, &cases))
	return cases
}

package declarative

import (
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// VaultCase covers every config field in one public API vault variant.
type VaultCase struct {
	Name   string         `json:"name"`
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
}

// Fixtures follow the public AI Gateway specification as of 2026-09-15.
// https://developer.konghq.com/api/konnect/ai-gateway/v1/#/operations/create-ai-gateway-vault
// Write-only fields use public vault references; deferred secrets are tested separately.
//
//go:embed vaults.json
var vaultFixtures []byte

// VaultCases returns independent fixtures for all seven vault types and all
// ten HashiCorp authentication methods.
func VaultCases(t *testing.T) []VaultCase {
	t.Helper()
	var cases []VaultCase
	require.NoError(t, json.Unmarshal(vaultFixtures, &cases))
	return cases
}

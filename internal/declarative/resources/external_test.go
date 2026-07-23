package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExternalSelectorMatchUsesSerializedFieldNames(t *testing.T) {
	t.Parallel()

	type EmbeddedCandidate struct {
		Name        string  `json:"name"`
		DisplayName *string `json:"display_name,omitempty"`
	}
	type candidate struct {
		EmbeddedCandidate
	}

	displayName := "Shared Gateway"
	value := candidate{EmbeddedCandidate: EmbeddedCandidate{
		Name:        "shared-gateway",
		DisplayName: &displayName,
	}}

	require.True(t, (&ExternalSelector{MatchFields: map[string]string{
		"name":         "shared-gateway",
		"display_name": "Shared Gateway",
	}}).Match(value))
	require.False(t, (&ExternalSelector{MatchFields: map[string]string{
		"display_name": "Other Gateway",
	}}).Match(value))
}

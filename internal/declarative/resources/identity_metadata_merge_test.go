package resources

import (
	"encoding/json"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestIdentityMetadataMergePreservesUnmentionedValuesAndIntegerPrecision(t *testing.T) {
	current := map[string]kkComps.KongIdentityMetadata{
		"keep":  kkComps.CreateKongIdentityMetadataBoolean(true),
		"large": kkComps.CreateKongIdentityMetadataInteger(9007199254740994),
		"roles": kkComps.CreateKongIdentityMetadataArrayOfStr([]string{"reader", "writer"}),
	}
	updates := map[string]any{"roles": []string{"operator"}, "team": "fulfillment"}
	merged, err := MergeIdentityMetadata(current, updates)
	require.NoError(t, err)
	require.Equal(t, true, merged["keep"])
	require.Equal(t, json.Number("9007199254740994"), merged["large"])
	require.Equal(t, []string{"operator"}, merged["roles"], "arrays replace a key's value; they do not concatenate")
	require.Equal(t, "fulfillment", merged["team"])
	require.Len(t, current, 3)
	require.Len(t, updates, 2)
	merged, err = MergeIdentityMetadata(nil, map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, merged)
	require.Empty(t, merged)
}

func TestIdentityMetadataMergeValidatesTheCompleteReplacement(t *testing.T) {
	current := make(map[string]any)
	for i := range 50 {
		current[fmt.Sprint(i)] = "retained"
	}
	_, err := MergeIdentityMetadata(current, map[string]any{"new": true})
	require.ErrorContains(t, err, "50 entries")
	_, err = MergeIdentityMetadata(map[string]any{}, map[string]any{"bad": nil})
	require.Error(t, err)
	_, err = MergeIdentityMetadata(map[string]any{}, map[string]any{"rounded": json.Number("9007199254740993")})
	require.ErrorContains(t, err, "Q31")
}

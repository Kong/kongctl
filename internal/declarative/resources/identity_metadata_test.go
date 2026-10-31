package resources

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentityMetadataRejectsObservedAPIRounding(t *testing.T) {
	for _, number := range []int64{
		9007199254740993, -9007199254740993, 1152921504606846976,
		math.MaxInt64, math.MinInt64,
	} {
		for _, value := range []any{
			number, json.Number(strconv.FormatInt(number, 10)),
			[]int64{1, number},
			[]any{json.Number("1"), json.Number(strconv.FormatInt(number, 10))},
		} {
			err := ValidateIdentityMetadata(map[string]any{"identifier": value})
			require.ErrorContains(t, err, "Q31")
			require.NotContains(t, err.Error(), strconv.FormatInt(number, 10))
		}
	}
}

func TestIdentityMetadataAllowsStableObservedValuesAndStrings(t *testing.T) {
	for _, value := range []any{
		int64(9007199254740991), int64(9007199254740992), int64(9007199254740994),
		int64(1152921504606847000), "9007199254740993",
		[]string{"9007199254740993"},
		[]int64{9007199254740992, 9007199254740994},
		json.Number("9007199254740994"),
		float64(9007199254740994), true,
	} {
		require.NoError(t, ValidateIdentityMetadata(map[string]any{"value": value}))
	}
	for _, value := range []any{
		1.5, nil,
		map[string]any{"nested": true},
		[]any{1, "mixed"},
		[]int{1, 1},
		make([]int, 256),
	} {
		require.Error(t, ValidateIdentityMetadata(map[string]any{"value": value}))
	}
}

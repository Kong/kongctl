package tags

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactEnvOldValue(t *testing.T) {
	const source = "__ENV__:TEST_VALUE"
	for _, tc := range []struct {
		name     string
		old, new any
		want     any
	}{
		{"scalar", "remote-value", source, DeferredEnvRedactedDisplay},
		{"null", nil, source, nil},
		{"empty string", "", source, DeferredEnvRedactedDisplay},
		{"literal", "old", "new", "old"},
		{"stored env value", "old", "resolved stored value", "old"},
		{"container to env", map[string]any{"value": "remote"}, source, DeferredEnvRedactedDisplay},
		{"scalar to container", "remote", map[string]any{"value": source}, DeferredEnvRedactedDisplay},
		{
			"typed maps",
			map[string]string{"a/b~c": "remote", "visible": "old"},
			map[string]string{"a/b~c": source, "visible": "new"},
			map[string]any{"a/b~c": DeferredEnvRedactedDisplay, "visible": "old"},
		},
		{
			"typed arrays",
			[2]string{"remote", "visible"},
			[2]string{source, "changed"},
			[]any{DeferredEnvRedactedDisplay, "visible"},
		},
		{"added item", []string{"visible"}, []string{"changed", source}, []any{"visible"}},
		{
			"missing field",
			map[string]string{"visible": "old"},
			map[string]string{"added": source},
			map[string]any{"visible": "old"},
		},
		{
			"nil field",
			map[string]any{"value": nil},
			map[string]any{"value": source},
			map[string]any{"value": nil},
		},
		{
			"preserve precision",
			map[string]any{"value": "remote", "number": uint64(9007199254740993)},
			map[string]any{"value": source},
			map[string]any{"value": DeferredEnvRedactedDisplay, "number": json.Number("9007199254740993")},
		},
		{"pointer", new("remote"), new(source), DeferredEnvRedactedDisplay},
	} {
		t.Run(tc.name, func(t *testing.T) {
			beforeOld, err := json.Marshal(tc.old)
			require.NoError(t, err)
			beforeNew, err := json.Marshal(tc.new)
			require.NoError(t, err)
			assert.Equal(t, tc.want, RedactEnvOldValue(tc.old, tc.new))
			afterOld, err := json.Marshal(tc.old)
			require.NoError(t, err)
			afterNew, err := json.Marshal(tc.new)
			require.NoError(t, err)
			assert.Equal(t, beforeOld, afterOld)
			assert.Equal(t, beforeNew, afterNew)
		})
	}
}

func TestRedactEnvOldValueUnsupportedValues(t *testing.T) {
	assert.Equal(t, DeferredEnvRedactedDisplay, RedactEnvOldValue(make(chan string), "__ENV__:TEST_VALUE"))
	assert.Equal(t, DeferredEnvRedactedDisplay, RedactEnvOldValue("remote", make(chan string)))
}

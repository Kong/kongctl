package tags

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // custom tag tests
)

func TestStoredEnvTypes(t *testing.T) {
	for _, tc := range []struct {
		kind, raw string
		want      any
	}{
		{"string", "true", "true"},
		{"string", "", ""},
		{"string", "0012", "0012"},
		{"boolean", "true", true},
		{"boolean", "false", false},
		{"integer", "9007199254740991", int64(9007199254740991)},
		{"integer", "-9007199254740991", int64(-9007199254740991)},
		{"number", "0.5", 0.5},
		{"number", "42", int64(42)},
		{"null", "null", nil},
		{"array", `[true, 42, null, "42"]`, []any{true, int64(42), nil, "42"}},
		{"object", "redis: {ssl: true}\nsync_rate: 0.5", map[string]any{
			"redis": map[string]any{"ssl": true}, "sync_rate": 0.5,
		}},
	} {
		t.Run(tc.kind+tc.raw, func(t *testing.T) {
			for _, extract := range []bool{false, true} {
				raw, path := tc.raw, ""
				if extract {
					// Encode the expected type to exercise extraction for every value kind.
					encoded, err := yaml.Marshal(map[string]any{"value": tc.want})
					require.NoError(t, err)
					raw, path = string(encoded), "value"
				}
				t.Setenv("STORED_VALUE", raw)
				for _, spelling := range []string{"!env {var: STORED_VALUE, store: true,", "!env_store {var: STORED_VALUE,"} {
					var doc yaml.Node
					require.NoError(
						t,
						yaml.Unmarshal(fmt.Appendf(nil, "%s type: %q, extract: %q}", spelling, tc.kind, path), &doc),
					)
					opts, err := ParseEnvOptions(doc.Content[0])
					require.NoError(t, err)
					actual, err := ResolveStoredEnv(opts, opts.Type)
					require.NoError(t, err)
					require.Equal(t, tc.want, actual)
				}
			}
		})
	}
}

func TestStoredEnvInvalidValues(t *testing.T) {
	for _, tc := range []struct{ kind, raw, message string }{
		{"boolean", "yes", "expected boolean"},
		{"boolean", "!!bool nonsense", "invalid stored boolean"},
		{"null", "!!null nonsense", "invalid stored null"},
		{"array", "!!seq nonsense", "value kind"},
		{"number", "4503599627370496.1", "loses its fractional part"},
		{"boolean", "1", "expected boolean"},
		{"integer", "1.5", "expected integer"},
		{"integer", "!!int 1.5", "invalid stored integer"},
		{"number", "9007199254740993", "2^53"},
		{"number", "9007199254740993.0", "2^53"},
		{"number", "-9007199254740992", "2^53"},
		{"number", "1e-999", "binary64"},
		{"number", ".nan", "finite JSON"},
		{"number", ".inf", "finite JSON"},
		{"number", "0x10", "decimal syntax"},
		{"boolean", "", "empty"},
		{"null", " ", "empty"},
		{"object", "{1: foo}", "keys must be strings"},
		{"object", "{a: 1, a: 2}", "duplicate"},
		{"object", "a: &x [1]\nb: *x", "anchors"},
		{"object", "a: !env OTHER", "unsupported stored YAML tag"},
		{"object", "date: 2026-01-01", "unsupported stored YAML tag"},
		{"object", "a: 1\n---\nb: 2", "one YAML/JSON document"},
		{"string", "__ENV__:OTHER", "reserved placeholder"},
		{"array", "[__SECRET__:other]", "reserved placeholder"}, // pragma: allowlist secret
		{"object", "{", "invalid YAML/JSON"},
	} {
		t.Run(tc.kind+tc.raw, func(t *testing.T) {
			t.Setenv("STORED_BAD", tc.raw)
			_, err := ResolveStoredEnv(EnvOptions{Var: "STORED_BAD"}, tc.kind)
			require.ErrorContains(t, err, tc.message)
		})
	}
	_, err := ResolveStoredEnv(EnvOptions{Var: "KONGCTL_TEST_STORED_ABSENT"}, "string")
	require.ErrorContains(t, err, "environment variable not set")
	t.Setenv("STORED_BAD", "a: 1")
	_, err = ResolveStoredEnv(EnvOptions{Var: "STORED_BAD", Extract: "missing"}, "number")
	require.ErrorContains(t, err, "extraction")
}

func TestStoredEnvOptionsAndNesting(t *testing.T) {
	for _, input := range []string{
		"!env_store", "!env_store {var: FOO, store: false}",
		"!env {var: FOO, store: yes}", "!env {var: FOO, store: 'true'}",
		"!env {var: FOO, type: string}", "!env_store {var: FOO, type: auto}",
		"!env_store {var: FOO, type: null}", "!env_store {var: FOO, eager: true}",
		"!env_store {var: FOO, var: BAR}", "!env_store FOO#",
	} {
		t.Run(input, func(t *testing.T) {
			var doc yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(input), &doc))
			_, err := ParseEnvOptions(doc.Content[0])
			require.Error(t, err)
		})
	}
	for _, outer := range []string{TagSecret, TagLookup, TagExternal} {
		for _, inner := range []string{"!env_store FOO", "!env {var: FOO, store: true}"} {
			var doc yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(outer+" {source: "+inner+"}"), &doc))
			require.ErrorContains(t, validateNestedTags(doc.Content[0]), "stored environment values are not supported")
		}
	}
}

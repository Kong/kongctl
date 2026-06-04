package loader

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 required for custom tag inspection
)

func TestValidateEnvNodeAllowsDynamicMapEnvTags(t *testing.T) {
	type dynamicConfig struct {
		Values map[string]any `yaml:"values"`
	}

	content := []byte(`
values:
  direct: !env DIRECT_VALUE
  nested:
    leaf: !env NESTED_VALUE
  list:
    - !env LIST_VALUE
    - child: !env LIST_CHILD_VALUE
`)

	var node yaml.Node
	require.NoError(t, yaml.Unmarshal(content, &node))

	err := validateEnvNode(&node, reflect.TypeFor[dynamicConfig](), nil)
	require.NoError(t, err)
}

func TestValidateEnvNodeRejectsBoolEnvTag(t *testing.T) {
	type staticConfig struct {
		Enabled bool `yaml:"enabled"`
	}

	content := []byte(`enabled: !env ENABLED`)

	var node yaml.Node
	require.NoError(t, yaml.Unmarshal(content, &node))

	err := validateEnvNode(&node, reflect.TypeFor[staticConfig](), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "!env currently supports string-typed fields only")
	assert.Contains(t, err.Error(), "enabled")
}

package resources

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

const dumpDefaultInventorySchemaVersion = 1

func TestOmitAPIDefaults(t *testing.T) {
	input := []byte(`
portals:
  - ref: developer-portal
    name: Developer Portal
    authentication_enabled: true
    rbac_enabled: true
    sipr_enabled: false
    teams:
      - ref: developers
        name: Developers
        can_own_applications: false
apis:
  - ref: orders
    name: Orders
    publications:
      - ref: orders-publication
        portal_id: developer-portal
        visibility: private
application_auth_strategies:
  - ref: key-auth
    name: key-auth
    display_name: API Key Authentication
    strategy_type: key_auth
    configs:
      key-auth:
        key_names:
          - X-API-Key
`)

	actual, err := OmitAPIDefaults(input)
	require.NoError(t, err)
	require.Contains(t, string(actual), "portals:\n- ref: developer-portal")
	require.NotContains(t, string(actual), "portals:\n    -")

	var document map[string]any
	require.NoError(t, yaml.Unmarshal(actual, &document))
	portals := document["portals"].([]any)
	portal := portals[0].(map[string]any)
	require.NotContains(t, portal, "authentication_enabled")
	require.Equal(t, true, portal["rbac_enabled"])
	require.NotContains(t, portal, "sipr_enabled")
	teams := portal["teams"].([]any)
	require.NotContains(t, teams[0].(map[string]any), "can_own_applications")

	apis := document["apis"].([]any)
	publications := apis[0].(map[string]any)["publications"].([]any)
	require.NotContains(t, publications[0].(map[string]any), "visibility")
}

func TestOmitAPIDefaultsPreservesNullAndUnknownFields(t *testing.T) {
	input := []byte(`
portals:
  - ref: developer-portal
    name: Developer Portal
    authentication_enabled: null
    future_api_field: false
`)

	actual, err := OmitAPIDefaults(input)
	require.NoError(t, err)
	require.Contains(t, string(actual), "authentication_enabled: null")
	require.Contains(t, string(actual), "future_api_field: false")
}

func TestDumpNodeEqualsDefaultScalarTypes(t *testing.T) {
	tests := []struct {
		name         string
		node         yaml.Node
		defaultValue dumpDefault
		want         bool
	}{
		{
			name:         "bool scalar",
			node:         yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"},
			defaultValue: dumpDefault{kind: reflect.Bool, value: false},
			want:         true,
		},
		{
			name:         "boolean-looking string is not boolean",
			node:         yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "false"},
			defaultValue: dumpDefault{kind: reflect.Bool, value: false},
		},
		{
			name:         "string enum",
			node:         yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "private"},
			defaultValue: dumpDefault{kind: reflect.String, value: "private"},
			want:         true,
		},
		{
			name:         "integer",
			node:         yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "-2"},
			defaultValue: dumpDefault{kind: reflect.Int64, value: int64(-2)},
			want:         true,
		},
		{
			name:         "integer-looking float is not integer",
			node:         yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: "2.0"},
			defaultValue: dumpDefault{kind: reflect.Int64, value: int64(2)},
		},
		{
			name:         "unsigned integer",
			node:         yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "42"},
			defaultValue: dumpDefault{kind: reflect.Uint32, value: uint64(42)},
			want:         true,
		},
		{
			name:         "floating point",
			node:         yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: "1.5"},
			defaultValue: dumpDefault{kind: reflect.Float64, value: 1.5},
			want:         true,
		},
		{
			name:         "integer representation of floating point",
			node:         yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "2"},
			defaultValue: dumpDefault{kind: reflect.Float64, value: 2.0},
			want:         true,
		},
		{
			name:         "explicit null",
			node:         yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"},
			defaultValue: dumpDefault{kind: reflect.Bool, value: false},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, dumpNodeEqualsDefault(&test.node, &test.defaultValue))
		})
	}
}

func TestOmitAPIDefaultsUnionHeavyFixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		validate func(*testing.T, map[string]any)
	}{
		{
			name:    "AI Gateway model",
			fixture: filepath.Join("ai-gateway", "model", "testdata", "config.yaml"),
			validate: func(t *testing.T, document map[string]any) {
				gateway := firstDumpFixtureResource(t, document, "ai_gateways")
				model := firstDumpFixtureResource(t, gateway, "models")
				require.NotContains(t, model, "enabled")
				require.Equal(t, "model", model["type"])
			},
		},
		{
			name:    "AI Gateway policy branches",
			fixture: filepath.Join("ai-gateway", "policy", "testdata", "config.yaml"),
			validate: func(t *testing.T, document map[string]any) {
				gateway := firstDumpFixtureResource(t, document, "ai_gateways")
				policies, ok := gateway["policies"].([]any)
				require.True(t, ok)
				require.Len(t, policies, 2)
				for _, value := range policies {
					policy, ok := value.(map[string]any)
					require.True(t, ok)
					require.NotContains(t, policy, "enabled")
					require.NotContains(t, policy, "global")
				}
			},
		},
		{
			name:    "Event Gateway nested policies",
			fixture: filepath.Join("event-gateway", "dump", "testdata", "config.yaml"),
			validate: func(t *testing.T, document map[string]any) {
				gateway := firstDumpFixtureResource(t, document, "event_gateways")
				listener := firstDumpFixtureResource(t, gateway, "listeners")
				listenerPolicy := firstDumpFixtureResource(t, listener, "policies")
				require.NotContains(t, listenerPolicy, "enabled")

				virtualCluster := firstDumpFixtureResource(t, gateway, "virtual_clusters")
				consumePolicy := firstDumpFixtureResource(t, virtualCluster, "consume_policies")
				require.NotContains(t, consumePolicy, "condition")
				require.NotContains(t, consumePolicy, "description")
				require.NotContains(t, consumePolicy, "enabled")

				schemaRegistry := firstDumpFixtureResource(t, gateway, "schema_registries")
				config, ok := schemaRegistry["config"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, 30, config["timeout_seconds"])
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixturePath := filepath.Join("..", "..", "..", "test", "e2e", "scenarios", test.fixture)
			input, err := os.ReadFile(fixturePath)
			require.NoError(t, err)

			actual, err := OmitAPIDefaults(input)
			require.NoError(t, err)
			var document map[string]any
			require.NoError(t, yaml.Unmarshal(actual, &document))
			test.validate(t, document)
		})
	}
}

func firstDumpFixtureResource(t *testing.T, parent map[string]any, field string) map[string]any {
	t.Helper()
	values, ok := parent[field].([]any)
	require.True(t, ok, "%s is not a resource sequence", field)
	require.NotEmpty(t, values, "%s is empty", field)
	resource, ok := values[0].(map[string]any)
	require.True(t, ok, "%s does not contain a resource object", field)
	return resource
}

func TestDumpDefaultRulesAreResourceScoped(t *testing.T) {
	builder := dumpSchemaBuilder{cache: make(map[reflect.Type]*dumpSchema)}
	schema, err := builder.build(reflect.TypeFor[PortalResource]())
	require.NoError(t, err)

	err = applyDumpDefaultRules(ResourceTypePortal, schema, map[string]dumpDefaultRule{
		"authentication_enabled": {
			kind: dumpDefaultRuleExclusion,
		},
		"rbac_enabled": {
			kind:  dumpDefaultRuleOverride,
			value: true,
		},
	})
	require.NoError(t, err)

	authentication := lookupDumpFields(schema, []string{"authentication_enabled"}, make(map[*dumpSchema]bool))[0]
	rbac := lookupDumpFields(schema, []string{"rbac_enabled"}, make(map[*dumpSchema]bool))[0]
	require.Nil(t, authentication.effectiveDefault(ResourceTypePortal))
	require.NotNil(t, authentication.effectiveDefault(ResourceTypeAPI))
	require.Equal(t, true, rbac.effectiveDefault(ResourceTypePortal).value)
	require.Equal(t, false, rbac.effectiveDefault(ResourceTypeAPI).value)
}

func TestDumpDefaultRulesAffectEmittedYAML(t *testing.T) {
	builder := dumpSchemaBuilder{cache: make(map[reflect.Type]*dumpSchema)}
	schema, err := builder.build(reflect.TypeFor[PortalResource]())
	require.NoError(t, err)
	schema.resourceType = ResourceTypePortal
	require.NoError(t, applyDumpDefaultRules(ResourceTypePortal, schema, map[string]dumpDefaultRule{
		"authentication_enabled": {kind: dumpDefaultRuleExclusion},
		"rbac_enabled":           {kind: dumpDefaultRuleOverride, value: true},
	}))

	var document yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(`
ref: portal
name: Portal
authentication_enabled: true
rbac_enabled: true
auto_approve_developers: false
`), &document))
	require.NoError(t, pruneDumpDefaults(document.Content[0], schema, "", nil))

	var output map[string]any
	require.NoError(t, document.Content[0].Decode(&output))
	require.Equal(t, true, output["authentication_enabled"], "exclusion must preserve the SDK default")
	require.NotContains(t, output, "rbac_enabled", "override must replace the SDK default used for omission")
	require.NotContains(t, output, "auto_approve_developers", "unmodified SDK defaults must still be omitted")
}

func TestDumpDefaultRulesRejectDuplicatePaths(t *testing.T) {
	ops := &resourceOps{}
	require.NoError(t, WithDumpDefaultOverride("enabled", true, "API documentation")(ops))
	require.Error(t, WithDumpDefaultExclusion("enabled", "unsafe to omit")(ops))
}

func TestDumpDefaultInventory(t *testing.T) {
	actual, err := renderDumpDefaultInventory(t)
	require.NoError(t, err)

	goldenPath := filepath.Join("testdata", "dump_defaults_inventory.yaml")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile(goldenPath, actual, 0o600))
	}
	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "run UPDATE_GOLDEN=1 go test ./internal/declarative/resources -run TestDumpDefaultInventory")
	require.Equal(t, string(expected), string(actual),
		"SDK defaults changed; review the diff, update rules if needed, then regenerate the golden inventory")
}

type dumpDefaultInventory struct {
	SDKVersion    string                         `yaml:"sdk_version"`
	SchemaVersion int                            `yaml:"schema_version"`
	Resources     []dumpDefaultInventoryResource `yaml:"resources"`
}

type dumpDefaultInventoryResource struct {
	ResourceType ResourceType                `yaml:"resource_type"`
	Defaults     []dumpDefaultInventoryEntry `yaml:"defaults"`
	Overrides    []dumpDefaultInventoryRule  `yaml:"overrides"`
	Exclusions   []dumpDefaultInventoryRule  `yaml:"exclusions"`
}

type dumpDefaultInventoryEntry struct {
	Path   string            `yaml:"path"`
	Type   string            `yaml:"type"`
	Value  any               `yaml:"value"`
	Source dumpDefaultSource `yaml:"source"`
}

type dumpDefaultInventoryRule struct {
	Path   string `yaml:"path"`
	Reason string `yaml:"reason"`
}

func renderDumpDefaultInventory(t *testing.T) ([]byte, error) {
	t.Helper()
	root, err := buildDumpDefaultSchema()
	if err != nil {
		return nil, err
	}

	inventory := dumpDefaultInventory{
		SDKVersion:    sdkVersionFromGoMod(t),
		SchemaVersion: dumpDefaultInventorySchemaVersion,
	}
	types := RegisteredTypes()
	slices.Sort(types)
	for _, resourceType := range types {
		schema := dumpSchemaForResource(root, resourceType, make(map[*dumpSchema]bool))
		resource := dumpDefaultInventoryResource{ResourceType: resourceType}
		if schema != nil {
			collectDumpDefaultInventory(schema, resourceType, nil, &resource.Defaults, make(map[*dumpSchema]bool))
		}
		slices.SortFunc(resource.Defaults, func(a, b dumpDefaultInventoryEntry) int {
			if pathCmp := cmp.Compare(a.Path, b.Path); pathCmp != 0 {
				return pathCmp
			}
			return cmp.Compare(fmt.Sprint(a.Value), fmt.Sprint(b.Value))
		})
		resource.Defaults = slices.CompactFunc(resource.Defaults, func(a, b dumpDefaultInventoryEntry) bool {
			return a.Path == b.Path && a.Type == b.Type && reflect.DeepEqual(a.Value, b.Value) && a.Source == b.Source
		})
		for path, rule := range registry[resourceType].dumpDefaultRules {
			entry := dumpDefaultInventoryRule{Path: path, Reason: rule.reason}
			if rule.kind == dumpDefaultRuleExclusion {
				resource.Exclusions = append(resource.Exclusions, entry)
			} else {
				resource.Overrides = append(resource.Overrides, entry)
			}
		}
		slices.SortFunc(resource.Overrides, func(a, b dumpDefaultInventoryRule) int { return cmp.Compare(a.Path, b.Path) })
		slices.SortFunc(resource.Exclusions, func(a, b dumpDefaultInventoryRule) int { return cmp.Compare(a.Path, b.Path) })
		inventory.Resources = append(inventory.Resources, resource)
	}

	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(inventory); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func sdkVersionFromGoMod(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "go.mod"))
	require.NoError(t, err)
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "github.com/Kong/sdk-konnect-go" {
			return fields[1]
		}
	}
	t.Fatal("github.com/Kong/sdk-konnect-go is not declared in go.mod")
	return ""
}

func dumpSchemaForResource(schema *dumpSchema, resourceType ResourceType, visiting map[*dumpSchema]bool) *dumpSchema {
	if schema == nil || visiting[schema] {
		return nil
	}
	if schema.resourceType == resourceType {
		return schema
	}
	visiting[schema] = true
	defer delete(visiting, schema)
	for _, field := range schema.fields {
		if found := dumpSchemaForResource(field.schema, resourceType, visiting); found != nil {
			return found
		}
	}
	for _, inline := range schema.inline {
		if found := dumpSchemaForResource(inline, resourceType, visiting); found != nil {
			return found
		}
	}
	for _, branch := range schema.union {
		if found := dumpSchemaForResource(branch.schema, resourceType, visiting); found != nil {
			return found
		}
	}
	if found := dumpSchemaForResource(schema.elem, resourceType, visiting); found != nil {
		return found
	}
	return dumpSchemaForResource(schema.additional, resourceType, visiting)
}

func collectDumpDefaultInventory(
	schema *dumpSchema,
	resourceType ResourceType,
	path []string,
	entries *[]dumpDefaultInventoryEntry,
	visiting map[*dumpSchema]bool,
) {
	if schema == nil || visiting[schema] {
		return
	}
	if schema.resourceType != "" {
		resourceType = schema.resourceType
	}
	visiting[schema] = true
	defer delete(visiting, schema)
	for name, field := range schema.fields {
		fieldPath := append(path, name)
		if defaultValue := field.effectiveDefault(resourceType); defaultValue != nil {
			*entries = append(*entries, dumpDefaultInventoryEntry{
				Path:   strings.Join(fieldPath, "."),
				Type:   defaultValue.typ,
				Value:  defaultValue.value,
				Source: defaultValue.source,
			})
		}
		collectDumpDefaultInventory(field.schema, resourceType, fieldPath, entries, visiting)
	}
	for _, inline := range schema.inline {
		collectDumpDefaultInventory(inline, resourceType, path, entries, visiting)
	}
	for _, branch := range schema.union {
		collectDumpDefaultInventory(branch.schema, resourceType, path, entries, visiting)
	}
	collectDumpDefaultInventory(schema.elem, resourceType, path, entries, visiting)
	collectDumpDefaultInventory(schema.additional, resourceType, path, entries, visiting)
}

package openapi2mcp

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func convertFile(t *testing.T, name string, opts Options) *Result {
	t.Helper()
	spec, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	result, err := Convert(spec, opts)
	require.NoError(t, err)
	return result
}

func TestConvertPetStoreMatchesGolden(t *testing.T) {
	result := convertFile(t, "petstore.yaml", Options{AIGateway: "my-ai-gateway"})

	out, err := yaml.Marshal(result.Config)
	require.NoError(t, err)
	golden := filepath.Join("testdata", "petstore.golden.yaml")
	if *update {
		require.NoError(t, os.WriteFile(golden, out, 0o600))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(out))

	assert.Equal(t, []string{
		"security requirements are not converted; configure access on the MCP Server or its tools",
		"recursive schema #/components/schemas/Pet is cut off at its second level",
		"HEAD /pets/{petId}: method not supported by AI Gateway MCP tools; skipped",
		`GET /pets/{petId}: no description or summary; the tool description is "GET /pets/{petId}"`,
		`GET /pets/{petId}: cookie parameter "session" is not supported by AI Gateway MCP tools; skipped`,
	}, result.Warnings)
}

// The output must load through the declarative loader next to the AI Gateway it references.
func TestConvertedOutputLoads(t *testing.T) {
	result := convertFile(t, "petstore.yaml", Options{AIGateway: "my-ai-gateway"})
	out, err := yaml.Marshal(result.Config)
	require.NoError(t, err)

	manifest := "ai_gateways:\n- ref: my-ai-gateway\n  name: my-ai-gateway\n  display_name: My AI Gateway\n" +
		string(out)
	path := filepath.Join(t.TempDir(), "kongctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))

	rs, err := loader.New().LoadFile(path)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayMCPServers, 1)
	server := rs.AIGatewayMCPServers[0]
	assert.Equal(t, "my-ai-gateway", server.AIGateway)
	assert.Equal(t, ServerType, server.MCPServerType())
	require.NotNil(t, server.AIGatewayMCPServerConversionListener)
	listener := server.AIGatewayMCPServerConversionListener
	assert.Equal(t, "https://api.pets.example.com/v1", listener.Config.URL)
	names := make([]string, 0, len(listener.Tools))
	for _, tool := range listener.Tools {
		names = append(names, tool.Name)
	}
	assert.Equal(t, []string{"list-pets", "create-pet", "fetch-pet"}, names)
}

func TestConvertOptionsOverrideDocumentDefaults(t *testing.T) {
	result := convertFile(t, "petstore.yaml", Options{
		AIGateway: "gw",
		Name:      "pets",
		ServerURL: "https://staging.pets.example.com/v2/",
		RoutePath: "/mcp/pets",
	})
	server := result.Config.MCPServers[0]
	assert.Equal(t, "pets", server.Ref)
	assert.Equal(t, "pets", server.Name)
	assert.Equal(t, "Pet Store", server.DisplayName)
	assert.Equal(t, "https://staging.pets.example.com/v2", server.Config.URL)
	assert.Equal(t, []string{"/mcp/pets"}, server.Config.Route.Paths)
}

func TestConvertMergesParametersByNameAndLocation(t *testing.T) {
	spec := `openapi: 3.1.0
info: {title: Items, version: "1"}
servers: [{url: https://items.example.com}]
paths:
  /items/{id}:
    parameters:
      - {name: id, in: path, required: true, description: path level, schema: {type: string}}
      - {name: id, in: header, description: tracing id, schema: {type: string}}
    get:
      summary: Get an item
      parameters:
        - {name: id, in: path, required: true, description: operation level, schema: {type: integer}}
`
	result, err := Convert([]byte(spec), Options{AIGateway: "gw"})
	require.NoError(t, err)
	params := result.Config.MCPServers[0].Tools[0].Parameters
	require.Len(t, params, 2)
	assert.Equal(t, Parameter{
		Name: "id", In: "path", Description: "operation level", Required: true,
		Schema: map[string]any{"type": "integer"},
	}, params[0])
	assert.Equal(t, Parameter{
		Name: "id", In: "header", Description: "tracing id",
		Schema: map[string]any{"type": "string"},
	}, params[1])
}

func TestConvertErrors(t *testing.T) {
	tests := []struct {
		name, spec, opt, want string
	}{
		{
			name: "duplicate tool names",
			spec: `openapi: 3.0.3
info: {title: Dup, version: "1"}
servers: [{url: https://dup.example.com}]
paths:
  /a: {get: {operationId: getThing, summary: a}}
  /b: {get: {operationId: get-thing, summary: b}}
`,
			want: `tool name "get-thing" is generated for both GET /a and GET /b; set x-kong-mcp-tool-name on one of them`,
		},
		{
			name: "no servers",
			spec: `openapi: 3.0.3
info: {title: None, version: "1"}
paths: {/a: {get: {summary: a}}}
`,
			want: "the OpenAPI document has no servers; set the upstream URL explicitly",
		},
		{
			name: "relative server",
			spec: `openapi: 3.0.3
info: {title: Rel, version: "1"}
servers: [{url: /v1}]
paths: {/a: {get: {summary: a}}}
`,
			want: `server URL "/v1" is not an absolute URL; set the upstream URL explicitly`,
		},
		{
			name: "swagger 2",
			spec: `swagger: "2.0"
info: {title: Old, version: "1"}
paths: {}
`,
			want: `unsupported OpenAPI version "2.0": only OpenAPI 3.x is supported`,
		},
		{
			name: "no gateway",
			spec: `openapi: 3.0.3
info: {title: Gw, version: "1"}
paths: {}
`,
			opt:  "no-gateway",
			want: "an AI Gateway ref is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{AIGateway: "gw"}
			if tt.opt == "no-gateway" {
				opts.AIGateway = ""
			}
			_, err := Convert([]byte(tt.spec), opts)
			require.EqualError(t, err, tt.want)
		})
	}
}

func TestKebab(t *testing.T) {
	for in, want := range map[string]string{
		"listPets":         "list-pets",
		"get-/pets/{id}":   "get-pets-id",
		"Pet Store":        "pet-store",
		"getHTTPResponse":  "get-httpresponse",
		"  __weird__name ": "weird-name",
	} {
		assert.Equal(t, want, kebab(in), in)
	}
}

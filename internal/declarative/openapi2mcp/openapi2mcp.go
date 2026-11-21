// Package openapi2mcp converts an OpenAPI 3 document into the kongctl declarative configuration for an
// AI Gateway MCP Server of type conversion-listener. The conversion is local: nothing is sent to Konnect.
//
// The tool rules follow `deck file openapi2mcp`: the tool name comes from x-kong-mcp-tool-name, then
// operationId, then the method and path; the description from x-kong-mcp-tool-description, then the
// operation description, then its summary; an operation with x-kong-mcp-exclude: true is skipped.
// Unlike decK, parameters are merged by (name, in), duplicate tool names are an error, and recursive
// schemas are cut off instead of followed.
package openapi2mcp

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	yaml "github.com/pb33f/go-yaml"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

const (
	extToolName        = "x-kong-mcp-tool-name"
	extToolDescription = "x-kong-mcp-tool-description"
	extExclude         = "x-kong-mcp-exclude"

	// ServerType is the MCP Server type this package produces.
	ServerType = "conversion-listener"
)

// methods are the HTTP methods an AI Gateway MCP conversion tool supports, in output order.
var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

// Options control the conversion.
type Options struct {
	// AIGateway is the ref of the AI Gateway the MCP Server belongs to. Required.
	AIGateway string
	// Name is the MCP Server ref and name. Defaults to the kebab-cased info.title.
	Name string
	// ServerURL is the upstream API URL. Defaults to the document's first server URL.
	ServerURL string
	// RoutePath is the AI Gateway route path for the MCP Server. Defaults to "/" + Name.
	RoutePath string
}

// Config is the declarative configuration document the conversion produces.
type Config struct {
	MCPServers []MCPServer `yaml:"ai_gateway_mcp_servers"`
}

// MCPServer is one ai_gateway_mcp_servers entry.
type MCPServer struct {
	Ref         string       `yaml:"ref"`
	AIGateway   string       `yaml:"ai_gateway"`
	Type        string       `yaml:"type"`
	Name        string       `yaml:"name"`
	DisplayName string       `yaml:"display_name"`
	Config      ServerConfig `yaml:"config"`
	Tools       []Tool       `yaml:"tools"`
}

// ServerConfig is the MCP Server config block.
type ServerConfig struct {
	URL   string `yaml:"url"`
	Route Route  `yaml:"route"`
}

// Route is the MCP Server route.
type Route struct {
	Paths []string `yaml:"paths"`
}

// Tool is one MCP conversion tool.
type Tool struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Method      string         `yaml:"method"`
	Path        string         `yaml:"path"`
	Parameters  []Parameter    `yaml:"parameters,omitempty"`
	RequestBody map[string]any `yaml:"request_body,omitempty"`
}

// Parameter is one tool parameter.
type Parameter struct {
	Name        string         `yaml:"name"`
	In          string         `yaml:"in"`
	Description string         `yaml:"description,omitempty"`
	Required    bool           `yaml:"required"`
	Schema      map[string]any `yaml:"schema,omitempty"`
}

// Result is a converted document plus the warnings for whatever was dropped or changed on the way.
type Result struct {
	Config   Config
	Warnings []string
}

// Convert converts an OpenAPI 3.x document (YAML or JSON) into an AI Gateway MCP Server configuration.
func Convert(spec []byte, opts Options) (*Result, error) {
	if opts.AIGateway == "" {
		return nil, errors.New("an AI Gateway ref is required")
	}

	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		return nil, fmt.Errorf("reading OpenAPI document: %w", err)
	}
	if !strings.HasPrefix(doc.GetVersion(), "3.") {
		return nil, fmt.Errorf("unsupported OpenAPI version %q: only OpenAPI 3.x is supported", doc.GetVersion())
	}
	model, err := doc.BuildV3Model()
	if err != nil && model == nil {
		return nil, fmt.Errorf("building OpenAPI model: %w", err)
	}
	api := model.Model

	c := &converter{}
	if err != nil {
		c.warn("the OpenAPI document has errors and was converted on a best-effort basis: %v", err)
	}

	server, err := c.server(&api, opts)
	if err != nil {
		return nil, err
	}

	if len(api.Security) > 0 {
		c.warn("security requirements are not converted; configure access on the MCP Server or its tools")
	}

	server.Tools, err = c.tools(&api)
	if err != nil {
		return nil, err
	}
	if len(server.Tools) == 0 {
		return nil, errors.New("the OpenAPI document has no operations to convert into tools")
	}

	return &Result{Config: Config{MCPServers: []MCPServer{*server}}, Warnings: c.warnings}, nil
}

type converter struct {
	warnings          []string
	operationSecurity bool
}

func (c *converter) warn(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}

func (c *converter) server(api *v3.Document, opts Options) (*MCPServer, error) {
	title := ""
	if api.Info != nil {
		title = strings.TrimSpace(api.Info.Title)
	}

	name := opts.Name
	if name == "" {
		name = kebab(title)
	}
	if name == "" {
		return nil, errors.New("the OpenAPI document has no info.title; set a name for the MCP Server")
	}
	displayName := title
	if displayName == "" {
		displayName = name
	}

	serverURL := opts.ServerURL
	if serverURL == "" {
		var err error
		if serverURL, err = c.documentServerURL(api.Servers); err != nil {
			return nil, err
		}
	}
	if u, err := url.Parse(serverURL); err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("server URL %q is not an absolute URL; set the upstream URL explicitly", serverURL)
	}

	routePath := opts.RoutePath
	if routePath == "" {
		routePath = "/" + name
	}

	return &MCPServer{
		Ref:         name,
		AIGateway:   opts.AIGateway,
		Type:        ServerType,
		Name:        name,
		DisplayName: displayName,
		Config: ServerConfig{
			URL:   strings.TrimSuffix(serverURL, "/"),
			Route: Route{Paths: []string{routePath}},
		},
	}, nil
}

var serverVariable = regexp.MustCompile(`\{([^{}]+)\}`)

// documentServerURL is the first document-level server URL with its variables set to their defaults.
func (c *converter) documentServerURL(servers []*v3.Server) (string, error) {
	if len(servers) == 0 || servers[0] == nil || servers[0].URL == "" {
		return "", errors.New("the OpenAPI document has no servers; set the upstream URL explicitly")
	}
	if len(servers) > 1 {
		c.warn("the OpenAPI document lists %d servers; using the first, %s", len(servers), servers[0].URL)
	}
	server := servers[0]
	var missing []string
	resolved := serverVariable.ReplaceAllStringFunc(server.URL, func(match string) string {
		name := match[1 : len(match)-1]
		if server.Variables != nil {
			if v, ok := server.Variables.Get(name); ok && v != nil {
				return v.Default
			}
		}
		missing = append(missing, name)
		return match
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("server URL %q has variables without a default: %s", server.URL,
			strings.Join(missing, ", "))
	}
	return resolved, nil
}

func (c *converter) tools(api *v3.Document) ([]Tool, error) {
	if api.Paths == nil || api.Paths.PathItems == nil {
		return nil, nil
	}

	var tools []Tool
	seen := map[string]string{}
	for path, item := range api.Paths.PathItems.FromOldest() {
		if item == nil {
			continue
		}
		if len(item.Servers) > 0 {
			c.warn("%s: path-level servers are not supported; the MCP Server URL is used", path)
		}
		ops := item.GetOperations()
		for method := range ops.KeysFromOldest() {
			upper := strings.ToUpper(method)
			if !slices.Contains(methods, upper) {
				c.warn("%s %s: method not supported by AI Gateway MCP tools; skipped", upper, path)
			}
		}
		for _, method := range methods {
			op, ok := ops.Get(strings.ToLower(method))
			if !ok || op == nil {
				continue
			}
			excluded, err := extensionBool(op.Extensions, extExclude)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			if excluded {
				continue
			}
			tool, err := c.tool(path, method, op, item.Parameters)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			where := method + " " + path
			if prev, dup := seen[tool.Name]; dup {
				return nil, fmt.Errorf("tool name %q is generated for both %s and %s; set %s on one of them",
					tool.Name, prev, where, extToolName)
			}
			seen[tool.Name] = where
			tools = append(tools, *tool)
		}
	}
	return tools, nil
}

func (c *converter) tool(path, method string, op *v3.Operation, pathParams []*v3.Parameter) (*Tool, error) {
	name, err := extensionString(op.Extensions, extToolName)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = kebab(op.OperationId)
	}
	if name == "" {
		name = kebab(strings.ToLower(method) + "-" + path)
	}

	description, err := extensionString(op.Extensions, extToolDescription)
	if err != nil {
		return nil, err
	}
	for _, d := range []string{op.Description, op.Summary} {
		if description == "" {
			description = strings.TrimSpace(d)
		}
	}
	if description == "" {
		description = method + " " + path
		c.warn("%s %s: no description or summary; the tool description is %q", method, path, description)
	}

	if len(op.Security) > 0 && !c.operationSecurity {
		c.operationSecurity = true
		c.warn("operation-level security requirements are not converted; configure access on the tools")
	}
	if len(op.Servers) > 0 {
		c.warn("%s %s: operation-level servers are not supported; the MCP Server URL is used", method, path)
	}

	tool := &Tool{Name: name, Description: description, Method: method, Path: path}
	tool.Parameters = c.parameters(method, path, op.Parameters, pathParams)
	if op.RequestBody != nil {
		tool.RequestBody = c.requestBody(op.RequestBody)
	}
	return tool, nil
}

// parameters merges path-level and operation-level parameters by (name, in); the operation's win.
func (c *converter) parameters(method, path string, opParams, pathParams []*v3.Parameter) []Parameter {
	type key struct{ name, in string }
	var merged []*v3.Parameter
	overridden := map[key]bool{}
	for _, p := range opParams {
		if p != nil {
			merged = append(merged, p)
			overridden[key{p.Name, p.In}] = true
		}
	}
	for _, p := range pathParams {
		if p != nil && !overridden[key{p.Name, p.In}] {
			merged = append(merged, p)
		}
	}

	var params []Parameter
	for _, p := range merged {
		if p.In == "cookie" {
			c.warn("%s %s: cookie parameter %q is not supported by AI Gateway MCP tools; skipped", method, path,
				p.Name)
			continue
		}
		params = append(params, Parameter{
			Name:        p.Name,
			In:          p.In,
			Description: strings.TrimSpace(p.Description),
			Required:    p.Required != nil && *p.Required,
			Schema:      c.schema(p.Schema, nil),
		})
	}
	return params
}

func (c *converter) requestBody(rb *v3.RequestBody) map[string]any {
	body := map[string]any{}
	if rb.Required != nil {
		body["required"] = *rb.Required
	}
	if rb.Content != nil {
		content := map[string]any{}
		for mediaType, media := range rb.Content.FromOldest() {
			entry := map[string]any{}
			if media != nil {
				if s := c.schema(media.Schema, nil); s != nil {
					entry["schema"] = s
				}
			}
			content[mediaType] = entry
		}
		body["content"] = content
	}
	return body
}

// schema keeps the type, object properties and required list, and array items of a schema, the same
// subset decK keeps. References are resolved; a reference already being expanded is cut off.
func (c *converter) schema(proxy *base.SchemaProxy, expanding []string) map[string]any {
	if proxy == nil {
		return nil
	}
	if ref := proxy.GetReference(); ref != "" {
		if slices.Contains(expanding, ref) {
			c.warn("recursive schema %s is cut off at its second level", ref)
			return map[string]any{}
		}
		expanding = append(slices.Clone(expanding), ref)
	}
	s := proxy.Schema()
	if s == nil {
		return nil
	}

	out := map[string]any{}
	if len(s.Type) > 0 {
		out["type"] = s.Type[0]
	}
	if out["type"] == "object" && s.Properties != nil {
		props := map[string]any{}
		for name, prop := range s.Properties.FromOldest() {
			props[name] = c.schema(prop, expanding)
		}
		out["properties"] = props
		if len(s.Required) > 0 {
			out["required"] = s.Required
		}
	}
	if out["type"] == "array" && s.Items != nil && s.Items.A != nil {
		out["items"] = c.schema(s.Items.A, expanding)
	}
	return out
}

func extensionString(ext *orderedmap.Map[string, *yaml.Node], key string) (string, error) {
	node := extension(ext, key)
	if node == nil {
		return "", nil
	}
	var v string
	if err := node.Decode(&v); err != nil {
		return "", fmt.Errorf("%s must be a string: %w", key, err)
	}
	return strings.TrimSpace(v), nil
}

func extensionBool(ext *orderedmap.Map[string, *yaml.Node], key string) (bool, error) {
	node := extension(ext, key)
	if node == nil {
		return false, nil
	}
	var v bool
	if err := node.Decode(&v); err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return v, nil
}

func extension(ext *orderedmap.Map[string, *yaml.Node], key string) *yaml.Node {
	if ext == nil {
		return nil
	}
	node, ok := ext.Get(key)
	if !ok {
		return nil
	}
	return node
}

var (
	camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	nonWord       = regexp.MustCompile(`[^a-z0-9]+`)
)

// kebab lower-cases s and joins its words with "-": "getUser by_ID" becomes "get-user-by-id".
func kebab(s string) string {
	s = camelBoundary.ReplaceAllString(s, "$1-$2")
	s = nonWord.ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(s, "-")
}

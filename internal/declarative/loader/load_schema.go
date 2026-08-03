package loader

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"sigs.k8s.io/yaml"
)

const declarativeLoadSchemaURL = "kongctl://declarative/load"

type declarativeLoadSchemaBundle struct {
	rendered *resources.JSONSchema
	compiled *jsonschema.Schema
}

type offlineSchemaLoader struct{}

func (offlineSchemaLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema loading is disabled: %s", url)
}

var (
	declarativeSchemaOnce   sync.Once
	declarativeSchemaBundle *declarativeLoadSchemaBundle
	declarativeSchemaErr    error
)

func compiledDeclarativeLoadSchema() (*declarativeLoadSchemaBundle, error) {
	declarativeSchemaOnce.Do(func() {
		rendered, err := resources.RenderLoadSchema(resources.LoadSchemaProfileShape)
		if err != nil {
			declarativeSchemaErr = err
			return
		}

		data, err := json.Marshal(rendered)
		if err != nil {
			declarativeSchemaErr = fmt.Errorf("marshal declarative load schema: %w", err)
			return
		}
		var document any
		if err := json.Unmarshal(data, &document); err != nil {
			declarativeSchemaErr = fmt.Errorf("decode declarative load schema: %w", err)
			return
		}

		compiler := jsonschema.NewCompiler()
		compiler.UseLoader(offlineSchemaLoader{})
		if err := compiler.AddResource(declarativeLoadSchemaURL, document); err != nil {
			declarativeSchemaErr = fmt.Errorf("register declarative load schema: %w", err)
			return
		}
		compiled, err := compiler.Compile(declarativeLoadSchemaURL)
		if err != nil {
			declarativeSchemaErr = fmt.Errorf("compile declarative load schema: %w", err)
			return
		}
		declarativeSchemaBundle = &declarativeLoadSchemaBundle{rendered: rendered, compiled: compiled}
	})
	return declarativeSchemaBundle, declarativeSchemaErr
}

func validateDeclarativeDocumentShape(content []byte, sourcePath string) error {
	var document any
	if err := yaml.Unmarshal(content, &document); err != nil {
		return fmt.Errorf("failed to parse YAML in %s: %w", sourcePath, err)
	}

	bundle, err := compiledDeclarativeLoadSchema()
	if err != nil {
		return fmt.Errorf("prepare declarative validation schema: %w", err)
	}
	if err := bundle.compiled.Validate(document); err != nil {
		return formatDeclarativeSchemaError(bundle.rendered, document, sourcePath, err)
	}
	return nil
}

type unknownDeclarativeField struct {
	field    string
	location []string
}

func formatDeclarativeSchemaError(
	schema *resources.JSONSchema,
	document any,
	sourcePath string,
	err error,
) error {
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return fmt.Errorf("declarative schema validation failed for %s: %w", sourcePath, err)
	}

	unknownFields := collectUnknownDeclarativeFields(validationErr)
	unknownFields = slices.DeleteFunc(unknownFields, func(unknown unknownDeclarativeField) bool {
		containerPath := unknown.location[:len(unknown.location)-1]
		candidates := declarativeSchemaPropertiesAt(schema, document, containerPath)
		return slices.Contains(candidates, unknown.field)
	})
	if len(unknownFields) > 0 {
		sort.Slice(unknownFields, func(i, j int) bool {
			if len(unknownFields[i].location) != len(unknownFields[j].location) {
				return len(unknownFields[i].location) < len(unknownFields[j].location)
			}
			return declarativePath(unknownFields[i].location) < declarativePath(unknownFields[j].location)
		})
		unknown := unknownFields[0]
		path := declarativePath(unknown.location)
		containerPath := unknown.location[:len(unknown.location)-1]
		if message := declarativeRejectedFieldMessageAt(
			schema,
			document,
			containerPath,
			unknown.field,
		); message != "" {
			return errors.New(message)
		}
		if strings.HasSuffix(path, ".config.auth.header_name") ||
			strings.HasSuffix(path, ".config.auth.header_value") {
			return errors.New(
				"config.auth.header_name and config.auth.header_value are not supported; " +
					"use config.auth.headers[].name and config.auth.headers[].value",
			)
		}
		if strings.HasPrefix(path, "ai_gateways[") && strings.HasSuffix(path, ".providers") {
			return errors.New("ai_gateways.providers is not supported; use ai_gateways.model_providers")
		}
		if strings.HasPrefix(path, "apis[") && strings.HasSuffix(path, ".spec_content") {
			return errors.New(
				"apis[].spec_content is not supported in declarative configuration; use versions[].spec instead",
			)
		}
		candidates := declarativeSchemaPropertiesAt(schema, document, containerPath)
		if suggestion := suggestDeclarativeField(unknown.field, candidates); suggestion != "" {
			return fmt.Errorf(
				"unknown field '%s' at %s in %s. Did you mean '%s'?",
				unknown.field,
				path,
				sourcePath,
				suggestion,
			)
		}
		return fmt.Errorf(
			"unknown field '%s' at %s in %s. Please check the field name against the schema",
			unknown.field,
			path,
			sourcePath,
		)
	}

	leaf := firstDeclarativeValidationLeaf(validationErr)
	path := declarativePath(leaf.InstanceLocation)
	if message := portalSingletonNullSchemaError(path); message != "" {
		return errors.New(message)
	}
	if message := dashboardDeclarativeSchemaError(sourcePath, path); message != "" {
		return errors.New(message)
	}
	if path == "" {
		path = "<document>"
	}
	return fmt.Errorf("invalid declarative value at %s in %s: %s", path, sourcePath, declarativeErrorMessage(leaf))
}

func portalSingletonNullSchemaError(path string) string {
	const portalPrefix = "portals["
	if !strings.HasPrefix(path, portalPrefix) {
		return ""
	}
	for key := range portalSingletonChildKeys {
		if strings.HasSuffix(path, "."+key) {
			return fmt.Sprintf(
				"portal child singleton %q cannot be null; omit the key to ignore it or provide an object to manage it",
				key,
			)
		}
	}
	if strings.HasSuffix(path, ".assets") {
		return "portal child singleton \"assets\" cannot be null; " +
			"omit the key to ignore assets or provide an object to manage them"
	}
	for _, key := range []string{"assets.logo", "assets.favicon"} {
		if strings.HasSuffix(path, "."+key) {
			return fmt.Sprintf(
				"portal child singleton %q cannot be null; omit the key to ignore it or provide a value",
				key,
			)
		}
	}
	return ""
}

func dashboardDeclarativeSchemaError(sourcePath, path string) string {
	switch {
	case strings.HasSuffix(path, ".definition.query.datasource"):
		return fmt.Sprintf(
			"failed to parse YAML in %s: invalid analytics dashboard tile query.datasource; "+
				"expected one of api_usage, llm_usage, agentic_usage, platform_usage",
			sourcePath,
		)
	case strings.HasSuffix(path, ".definition.chart.type"):
		return fmt.Sprintf(
			"failed to parse YAML in %s: invalid analytics dashboard tile chart.type; "+
				"expected one of donut, timeseries_line, timeseries_bar, horizontal_bar, vertical_bar, "+
				"single_value, choropleth_map, top_n",
			sourcePath,
		)
	case strings.HasSuffix(path, ".query.time_range.type"):
		return fmt.Sprintf(
			"failed to parse YAML in %s: invalid analytics dashboard tile query.time_range.type; "+
				"expected one of relative, absolute",
			sourcePath,
		)
	default:
		return ""
	}
}

func collectUnknownDeclarativeFields(validationErr *jsonschema.ValidationError) []unknownDeclarativeField {
	if validationErr == nil {
		return nil
	}
	var result []unknownDeclarativeField
	if additional, ok := validationErr.ErrorKind.(*kind.AdditionalProperties); ok {
		for _, property := range additional.Properties {
			location := append([]string(nil), validationErr.InstanceLocation...)
			location = append(location, property)
			result = append(result, unknownDeclarativeField{field: property, location: location})
		}
	}
	for _, cause := range validationErr.Causes {
		result = append(result, collectUnknownDeclarativeFields(cause)...)
	}
	return result
}

func firstDeclarativeValidationLeaf(validationErr *jsonschema.ValidationError) *jsonschema.ValidationError {
	if validationErr == nil || len(validationErr.Causes) == 0 {
		return validationErr
	}
	leaves := make([]*jsonschema.ValidationError, 0)
	var collect func(*jsonschema.ValidationError)
	collect = func(current *jsonschema.ValidationError) {
		if len(current.Causes) == 0 {
			leaves = append(leaves, current)
			return
		}
		for _, cause := range current.Causes {
			collect(cause)
		}
	}
	collect(validationErr)
	sort.Slice(leaves, func(i, j int) bool {
		return declarativePath(leaves[i].InstanceLocation) < declarativePath(leaves[j].InstanceLocation)
	})
	return leaves[0]
}

func declarativeErrorMessage(validationErr *jsonschema.ValidationError) string {
	if validationErr == nil {
		return "schema validation failed"
	}
	switch failure := validationErr.ErrorKind.(type) {
	case *kind.Type:
		return "expected " + strings.Join(failure.Want, " or ")
	case *kind.Required:
		return "missing required union selector " + strings.Join(failure.Missing, ", ")
	case *kind.Const:
		return fmt.Sprintf("expected union discriminator %v", failure.Want)
	case *kind.OneOf:
		return "does not match exactly one supported union shape"
	default:
		return validationErr.Error()
	}
}

func declarativePath(tokens []string) string {
	var result strings.Builder
	for _, token := range tokens {
		if _, err := strconv.Atoi(token); err == nil {
			fmt.Fprintf(&result, "[%s]", token)
			continue
		}
		if result.Len() > 0 {
			result.WriteByte('.')
		}
		result.WriteString(token)
	}
	return result.String()
}

func declarativeSchemaPropertiesAt(
	root *resources.JSONSchema,
	document any,
	path []string,
) []string {
	currentSchema := declarativeSchemaAt(root, document, path)
	if currentSchema == nil {
		return nil
	}
	properties := make(map[string]struct{})
	for name := range currentSchema.Properties {
		properties[name] = struct{}{}
	}
	for _, branch := range currentSchema.OneOf {
		branch = resolveDeclarativeSchemaRef(root, branch)
		for name := range branch.Properties {
			properties[name] = struct{}{}
		}
	}
	result := make([]string, 0, len(properties))
	for name := range properties {
		result = append(result, name)
	}
	slices.Sort(result)
	return result
}

func declarativeRejectedFieldMessageAt(
	root *resources.JSONSchema,
	document any,
	path []string,
	field string,
) string {
	schema := declarativeSchemaAt(root, document, path)
	if schema == nil {
		return ""
	}
	return schema.LoadRejectedFieldMessage(field)
}

func declarativeSchemaAt(
	root *resources.JSONSchema,
	document any,
	path []string,
) *resources.JSONSchema {
	currentSchema := root
	currentValue := document
	for _, token := range path {
		currentSchema = selectDeclarativeSchema(root, currentSchema, currentValue, token)
		if currentSchema == nil {
			return nil
		}
		if index, err := strconv.Atoi(token); err == nil {
			currentSchema = currentSchema.Items
			if values, ok := currentValue.([]any); ok && index >= 0 && index < len(values) {
				currentValue = values[index]
			} else {
				currentValue = nil
			}
			continue
		}
		currentSchema = currentSchema.Properties[token]
		if values, ok := currentValue.(map[string]any); ok {
			currentValue = values[token]
		} else {
			currentValue = nil
		}
	}

	currentSchema = selectDeclarativeSchema(root, currentSchema, currentValue, "")
	return currentSchema
}

func selectDeclarativeSchema(
	root *resources.JSONSchema,
	schema *resources.JSONSchema,
	value any,
	nextToken string,
) *resources.JSONSchema {
	schema = resolveDeclarativeSchemaRef(root, schema)
	if schema == nil || len(schema.OneOf) == 0 {
		return schema
	}
	var matches []*resources.JSONSchema
	for _, branch := range schema.OneOf {
		branch = resolveDeclarativeSchemaRef(root, branch)
		if declarativeSchemaBranchMatches(root, branch, value) {
			matches = append(matches, branch)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	for _, branch := range matches {
		if _, ok := branch.Properties[nextToken]; ok {
			return branch
		}
	}
	for _, branch := range schema.OneOf {
		branch = resolveDeclarativeSchemaRef(root, branch)
		if _, ok := branch.Properties[nextToken]; ok {
			return branch
		}
	}
	return schema
}

func declarativeSchemaBranchMatches(root *resources.JSONSchema, schema *resources.JSONSchema, value any) bool {
	values, ok := value.(map[string]any)
	if !ok {
		return false
	}
	for _, required := range schema.Required {
		if _, ok := values[required]; !ok {
			return false
		}
	}
	for name, property := range schema.Properties {
		property = resolveDeclarativeSchemaRef(root, property)
		if property == nil || property.Const == nil {
			continue
		}
		if !reflect.DeepEqual(values[name], property.Const) {
			return false
		}
	}
	return true
}

func resolveDeclarativeSchemaRef(root *resources.JSONSchema, schema *resources.JSONSchema) *resources.JSONSchema {
	for schema != nil && strings.HasPrefix(schema.Ref, "#/$defs/") {
		name := strings.TrimPrefix(schema.Ref, "#/$defs/")
		name = strings.ReplaceAll(name, "~1", "/")
		name = strings.ReplaceAll(name, "~0", "~")
		schema = root.Defs[name]
	}
	return schema
}

func suggestDeclarativeField(field string, candidates []string) string {
	field = strings.ToLower(field)
	legacy := map[string]string{
		"lables":            "labels",
		"label":             "labels",
		"strategytype":      "strategy_type",
		"integration":       "integrations",
		"service_reference": "service",
	}
	if candidate := legacy[field]; candidate != "" && slices.Contains(candidates, candidate) {
		return candidate
	}
	for _, candidate := range candidates {
		if levenshteinClose(field, strings.ToLower(candidate)) {
			return candidate
		}
	}
	return ""
}

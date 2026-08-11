package resources

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

const loadSchemaID = "kongctl://declarative/load"

// LoadSchemaProfile selects which parts of the declarative schema are enforced
// before typed unmarshalling.
type LoadSchemaProfile int

const (
	// LoadSchemaProfileShape validates collection and object shapes, rejects
	// unknown fields, and retains only the constraints needed to select unions.
	LoadSchemaProfileShape LoadSchemaProfile = iota
)

type declarativeLoadDocument struct {
	Defaults    *FileDefaults `yaml:"_defaults,omitempty" json:"_defaults,omitempty"`
	ResourceSet `yaml:",inline" json:",inline"`
}

// RenderLoadSchema renders the schema used to validate a complete declarative
// document before decoding it into ResourceSet and embedded SDK request types.
func RenderLoadSchema(profile LoadSchemaProfile) (*JSONSchema, error) {
	if profile != LoadSchemaProfileShape {
		return nil, fmt.Errorf("unsupported declarative load schema profile %d", profile)
	}

	definitions := make(map[string]*JSONSchema)
	resourceTypes := RegisteredTypes()
	slices.Sort(resourceTypes)
	for _, resourceType := range resourceTypes {
		doc, err := buildExplainDoc(resourceType)
		if err != nil {
			return nil, fmt.Errorf("build explain schema for %s: %w", resourceType, err)
		}

		definition := renderShapeSchema(doc.Schema)
		for _, relation := range doc.NestedRelations {
			childType := ResourceType(relation.ChildAlias)
			replaceLoadSchemaProperty(
				definition,
				relation.FieldName,
				loadSchemaResourceRef(childType, relation.FieldArray),
			)
		}
		definitions[string(resourceType)] = definition
	}

	root := reflectLoadSchema(reflect.TypeFor[declarativeLoadDocument](), nil)
	root.Schema = jsonSchemaDraft202012
	root.ID = loadSchemaID
	root.Title = "kongctl declarative load schema"
	root.Defs = definitions
	return root, nil
}

func loadSchemaResourceRef(resourceType ResourceType, array bool) *JSONSchema {
	ref := &JSONSchema{Ref: "#/$defs/" + escapeJSONPointerToken(string(resourceType))}
	if !array {
		return ref
	}
	return &JSONSchema{Type: explainKindArray, Items: ref}
}

func replaceLoadSchemaProperty(schema *JSONSchema, name string, replacement *JSONSchema) {
	if schema == nil {
		return
	}
	if _, ok := schema.Properties[name]; ok {
		schema.Properties[name] = replacement
	}
	for _, branch := range schema.OneOf {
		replaceLoadSchemaProperty(branch, name, replacement)
	}
}

func renderShapeSchema(node *ExplainNode) *JSONSchema {
	if node == nil || node.loadOpaque {
		return &JSONSchema{}
	}

	schema := &JSONSchema{loadRejected: maps.Clone(node.loadRejected)}

	if len(node.OneOf) > 0 {
		if node.Kind == explainKindObject || node.Kind == explainKindArray {
			schema.Type = schemaTypeValue(node.Kind, node.Nullable)
		}
		for _, branch := range node.OneOf {
			branchSchema := renderShapeSchema(branch)
			retainLoadSchemaBranchType(branchSchema, branch)
			retainLoadSchemaBranchDiscriminators(branchSchema, branch)
			branchSchema.Required = loadSchemaBranchSelectors(branch, node.OneOf)
			schema.OneOf = append(schema.OneOf, branchSchema)
		}
		return schema
	}

	switch node.Kind {
	case explainKindObject:
		schema.Type = schemaTypeValue(explainKindObject, node.Nullable)
		if node.truncated {
			schema.Additional = true
			return schema
		}
		schema.Additional = false
		if node.Additional != nil {
			schema.Additional = renderShapeSchema(node.Additional)
		}
		if len(node.Properties) > 0 {
			schema.Properties = make(map[string]*JSONSchema, len(node.Properties))
		}
		for _, field := range node.Properties {
			schema.Properties[field.Name] = renderShapeSchema(field.Node)
		}
	case explainKindArray:
		schema.Type = schemaTypeValue(explainKindArray, node.Nullable)
		schema.Items = renderShapeSchema(node.Items)
	}

	return schema
}

func retainLoadSchemaBranchType(schema *JSONSchema, node *ExplainNode) {
	if schema == nil || node == nil {
		return
	}
	switch node.Kind {
	case explainKindString, explainKindInteger, "number", "boolean", jsonNullLiteral:
		schema.Type = schemaTypeValue(node.Kind, node.Nullable)
	}
}

func retainLoadSchemaBranchDiscriminators(schema *JSONSchema, node *ExplainNode) {
	if schema == nil || node == nil {
		return
	}
	for _, field := range node.Properties {
		if field.Node == nil || field.Node.Const == nil {
			continue
		}
		if property := schema.Properties[field.Name]; property != nil {
			property.Const = field.Node.Const
		}
	}
}

func loadSchemaBranchSelectors(branch *ExplainNode, branches []*ExplainNode) []string {
	if branch == nil || branch.Kind != explainKindObject {
		return nil
	}

	selectors := make([]string, 0)
	for _, field := range branch.Properties {
		if field.Required && field.Node != nil && field.Node.Const != nil {
			selectors = append(selectors, field.Name)
		}
	}
	if len(selectors) == 0 {
		for _, field := range branch.Properties {
			if field.Required && !loadSchemaFieldExistsInEveryBranch(field.Name, branches) {
				selectors = append(selectors, field.Name)
			}
		}
	}
	if len(selectors) == 0 {
		for _, field := range branch.Properties {
			if field.Required {
				selectors = append(selectors, field.Name)
			}
		}
	}
	slices.Sort(selectors)
	return slices.Compact(selectors)
}

func loadSchemaFieldExistsInEveryBranch(name string, branches []*ExplainNode) bool {
	for _, branch := range branches {
		if branch == nil || !branch.propertyExists(name) {
			return false
		}
	}
	return true
}

func reflectLoadSchema(typ reflect.Type, stack []reflect.Type) *JSONSchema {
	nullable := typ.Kind() == reflect.Pointer
	typ = derefExplainType(typ)

	if typ.Kind() == reflect.Struct {
		if resourceType, ok := explainRegisteredResourceType(typ); ok {
			return loadSchemaResourceRef(resourceType, false)
		}
	}

	if slices.Contains(stack, typ) {
		return &JSONSchema{Type: schemaTypeValue(explainKindObject, nullable), Additional: true}
	}

	switch typ.Kind() {
	case reflect.Struct:
		schema := &JSONSchema{
			Type:       schemaTypeValue(explainKindObject, nullable),
			Properties: make(map[string]*JSONSchema),
			Additional: false,
		}
		stack = append(stack, typ)
		for field := range typ.Fields() {
			if !field.IsExported() {
				continue
			}
			name, inline, _, skip := explainFieldName(field, "yaml")
			if skip {
				continue
			}
			if inline {
				embedded := reflectLoadSchema(field.Type, stack)
				maps.Copy(schema.Properties, embedded.Properties)
				continue
			}
			if name == "" {
				name, _, _, skip = explainFieldName(field, "json")
				if skip {
					continue
				}
			}
			if name == "" {
				name = snakeCase(field.Name)
			}
			schema.Properties[name] = reflectLoadSchema(field.Type, stack)
		}
		return schema
	case reflect.Slice, reflect.Array:
		return &JSONSchema{
			Type:  schemaTypeValue(explainKindArray, nullable),
			Items: reflectLoadSchema(typ.Elem(), stack),
		}
	case reflect.Map:
		return &JSONSchema{
			Type:       schemaTypeValue(explainKindObject, nullable),
			Additional: reflectLoadSchema(typ.Elem(), stack),
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Chan, reflect.Func,
		reflect.Interface, reflect.Pointer, reflect.String, reflect.UnsafePointer:
		return &JSONSchema{}
	}
	return &JSONSchema{}
}

func escapeJSONPointerToken(value string) string {
	value = strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(value, "/", "~1")
}

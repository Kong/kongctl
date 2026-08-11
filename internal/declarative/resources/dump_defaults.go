package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"go.yaml.in/yaml/v4"
)

type dumpDefaultSource string

const (
	dumpDefaultSourceSDK      dumpDefaultSource = "sdk"
	dumpDefaultSourceOverride dumpDefaultSource = "override"
)

type dumpDefault struct {
	value  any
	kind   reflect.Kind
	typ    string
	source dumpDefaultSource
}

type dumpDefaultRuleKind int

const (
	dumpDefaultRuleOverride dumpDefaultRuleKind = iota
	dumpDefaultRuleExclusion
)

type dumpDefaultRule struct {
	kind   dumpDefaultRuleKind
	value  any
	reason string
}

// WithDumpDefaultOverride supplies a reviewed API default when generated SDK
// metadata is missing or incorrect. The path is relative to the resource.
func WithDumpDefaultOverride(path string, value any, reason string) ResourceRegistrationOption {
	return withDumpDefaultRule(path, dumpDefaultRule{
		kind:   dumpDefaultRuleOverride,
		value:  value,
		reason: reason,
	})
}

// WithDumpDefaultExclusion prevents an SDK default from being omitted when
// omission is not semantically safe. The path is relative to the resource.
func WithDumpDefaultExclusion(path, reason string) ResourceRegistrationOption {
	return withDumpDefaultRule(path, dumpDefaultRule{
		kind:   dumpDefaultRuleExclusion,
		reason: reason,
	})
}

func withDumpDefaultRule(path string, rule dumpDefaultRule) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		path = strings.TrimSpace(path)
		rule.reason = strings.TrimSpace(rule.reason)
		if path == "" {
			return fmt.Errorf("dump default rule path is required")
		}
		if rule.reason == "" {
			return fmt.Errorf("dump default rule %q requires a reason", path)
		}
		if ops.dumpDefaultRules == nil {
			ops.dumpDefaultRules = make(map[string]dumpDefaultRule)
		}
		if _, exists := ops.dumpDefaultRules[path]; exists {
			return fmt.Errorf("dump default rule %q is declared more than once", path)
		}
		ops.dumpDefaultRules[path] = rule
		return nil
	}
}

type dumpSchema struct {
	typ          reflect.Type
	kind         reflect.Kind
	resourceType ResourceType
	fields       map[string]*dumpField
	inline       []*dumpSchema
	union        []dumpUnionBranch
	elem         *dumpSchema
	additional   *dumpSchema
}

type dumpField struct {
	schema            *dumpSchema
	defaultValue      *dumpDefault
	defaultOverrides  map[ResourceType]*dumpDefault
	defaultExclusions map[ResourceType]bool
}

type dumpUnionBranch struct {
	schema        *dumpSchema
	discriminator string
	value         string
}

type dumpSchemaBuilder struct {
	cache map[reflect.Type]*dumpSchema
}

var (
	dumpSchemaOnce sync.Once
	dumpSchemaRoot *dumpSchema
	dumpSchemaErr  error
)

// OmitAPIDefaults removes fields equal to SDK-declared API defaults. All
// reflection and schema construction is intentionally lazy behind this call.
func OmitAPIDefaults(data []byte) ([]byte, error) {
	dumpSchemaOnce.Do(func() {
		dumpSchemaRoot, dumpSchemaErr = buildDumpDefaultSchema()
	})
	if dumpSchemaErr != nil {
		return nil, dumpSchemaErr
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse declarative YAML: %w", err)
	}
	if len(document.Content) == 0 {
		return data, nil
	}

	if err := pruneDumpDefaults(document.Content[0], dumpSchemaRoot, "", nil); err != nil {
		return nil, err
	}

	var result bytes.Buffer
	encoder := yaml.NewEncoder(&result)
	encoder.SetIndent(2)
	encoder.CompactSeqIndent()
	if err := encoder.Encode(&document); err != nil {
		return nil, fmt.Errorf("marshal declarative YAML: %w", err)
	}
	return result.Bytes(), nil
}

func buildDumpDefaultSchema() (*dumpSchema, error) {
	builder := dumpSchemaBuilder{cache: make(map[reflect.Type]*dumpSchema)}
	root, err := builder.build(reflect.TypeFor[ResourceSet]())
	if err != nil {
		return nil, err
	}

	types := RegisteredTypes()
	slices.Sort(types)
	for _, resourceType := range types {
		ops := registry[resourceType]
		schema, err := builder.build(ops.explain.typ)
		if err != nil {
			return nil, fmt.Errorf("build dump defaults for %s: %w", resourceType, err)
		}
		schema.resourceType = resourceType
		if err := applyDumpDefaultRules(resourceType, schema, ops.dumpDefaultRules); err != nil {
			return nil, err
		}
	}

	return root, nil
}

func (b *dumpSchemaBuilder) build(typ reflect.Type) (*dumpSchema, error) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if schema, ok := b.cache[typ]; ok {
		return schema, nil
	}

	schema := &dumpSchema{typ: typ, kind: typ.Kind()}
	b.cache[typ] = schema

	//exhaustive:ignore // Unsupported reflection kinds return an error below.
	switch typ.Kind() {
	case reflect.Struct:
		schema.fields = make(map[string]*dumpField)
		for field := range typ.Fields() {
			if !field.IsExported() {
				continue
			}
			if field.Tag.Get("additionalProperties") == "true" {
				fieldType := derefDumpType(field.Type)
				if fieldType.Kind() == reflect.Map {
					additional, err := b.build(fieldType.Elem())
					if err != nil {
						return nil, err
					}
					schema.additional = additional
				}
				continue
			}

			if field.Tag.Get("union") == "member" {
				branch, err := b.build(field.Type)
				if err != nil {
					return nil, err
				}
				discriminator, value, _ := dumpConstDiscriminator(field.Type)
				schema.union = append(schema.union, dumpUnionBranch{
					schema:        branch,
					discriminator: discriminator,
					value:         value,
				})
				continue
			}

			name, inline, skip := dumpFieldName(field)
			if skip {
				continue
			}
			child, err := b.build(field.Type)
			if err != nil {
				return nil, err
			}
			if inline {
				schema.inline = append(schema.inline, child)
				continue
			}
			defaultValue, err := parseDumpDefault(field)
			if err != nil {
				return nil, err
			}
			schema.fields[name] = &dumpField{schema: child, defaultValue: defaultValue}
		}
	case reflect.Slice, reflect.Array:
		elem, err := b.build(typ.Elem())
		if err != nil {
			return nil, err
		}
		schema.elem = elem
	case reflect.Map:
		additional, err := b.build(typ.Elem())
		if err != nil {
			return nil, err
		}
		schema.additional = additional
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String, reflect.Interface:
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Chan, reflect.Func,
		reflect.UnsafePointer:
		return nil, fmt.Errorf("unsupported declarative field type %s", typ)
	}

	return schema, nil
}

func derefDumpType(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

func dumpFieldName(field reflect.StructField) (name string, inline, skip bool) {
	jsonName, jsonInline, _, jsonSkip := explainFieldName(field, "json")
	yamlName, yamlInline, _, yamlSkip := explainFieldName(field, "yaml")
	if jsonSkip || yamlSkip {
		return "", false, true
	}
	if field.Anonymous && (jsonInline || yamlInline || (jsonName == "" && yamlName == "")) {
		return "", true, false
	}
	name = jsonName
	if name == "" {
		name = yamlName
	}
	if name == "" {
		name = snakeCase(field.Name)
	}
	return name, false, false
}

func parseDumpDefault(field reflect.StructField) (*dumpDefault, error) {
	raw, ok := field.Tag.Lookup("default")
	if !ok {
		return nil, nil
	}
	typ := derefDumpType(field.Type)
	result := &dumpDefault{kind: typ.Kind(), source: dumpDefaultSourceSDK}

	//exhaustive:ignore // Only scalar SDK defaults are supported.
	switch typ.Kind() {
	case reflect.String:
		result.value = raw
		result.typ = "string"
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("parse default for %s: %w", field.Name, err)
		}
		result.value = value
		result.typ = explainKindBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, typ.Bits())
		if err != nil {
			return nil, fmt.Errorf("parse default for %s: %w", field.Name, err)
		}
		result.value = value
		result.typ = "integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(raw, 10, typ.Bits())
		if err != nil {
			return nil, fmt.Errorf("parse default for %s: %w", field.Name, err)
		}
		result.value = value
		result.typ = "integer"
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(raw, typ.Bits())
		if err != nil {
			return nil, fmt.Errorf("parse default for %s: %w", field.Name, err)
		}
		result.value = value
		result.typ = "number"
	default:
		return nil, fmt.Errorf("field %s has unsupported %s default %q", field.Name, typ, raw)
	}
	return result, nil
}

func dumpConstDiscriminator(typ reflect.Type) (string, string, bool) {
	typ = derefDumpType(typ)
	if typ.Kind() != reflect.Struct {
		return "", "", false
	}
	for field := range typ.Fields() {
		value, ok := field.Tag.Lookup("const")
		if !ok {
			continue
		}
		name, _, skip := dumpFieldName(field)
		if skip || name == "" {
			continue
		}
		return name, value, true
	}
	return "", "", false
}

func applyDumpDefaultRules(resourceType ResourceType, schema *dumpSchema, rules map[string]dumpDefaultRule) error {
	paths := make([]string, 0, len(rules))
	for path := range rules {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		fields := lookupDumpFields(schema, strings.Split(path, "."), make(map[*dumpSchema]bool))
		if len(fields) == 0 {
			return fmt.Errorf("dump default rule for %s.%s does not match a declarative field", resourceType, path)
		}
		rule := rules[path]
		for _, field := range fields {
			switch rule.kind {
			case dumpDefaultRuleExclusion:
				if field.defaultValue == nil {
					return fmt.Errorf("dump default exclusion for %s.%s does not match an SDK default", resourceType, path)
				}
				if field.defaultExclusions == nil {
					field.defaultExclusions = make(map[ResourceType]bool)
				}
				field.defaultExclusions[resourceType] = true
			case dumpDefaultRuleOverride:
				value, err := dumpDefaultFromOverride(field.schema.typ, rule.value)
				if err != nil {
					return fmt.Errorf("dump default override for %s.%s: %w", resourceType, path, err)
				}
				if field.defaultOverrides == nil {
					field.defaultOverrides = make(map[ResourceType]*dumpDefault)
				}
				field.defaultOverrides[resourceType] = value
			}
		}
	}
	return nil
}

func dumpDefaultFromOverride(typ reflect.Type, value any) (*dumpDefault, error) {
	typ = derefDumpType(typ)
	valueType := reflect.TypeOf(value)
	if valueType == nil || !valueType.ConvertibleTo(typ) {
		return nil, fmt.Errorf("value of type %T is not compatible with %s", value, typ)
	}
	converted := reflect.ValueOf(value).Convert(typ)
	result := &dumpDefault{kind: typ.Kind(), source: dumpDefaultSourceOverride}
	//exhaustive:ignore // Only scalar registration overrides are supported.
	switch typ.Kind() {
	case reflect.String:
		result.value = converted.String()
		result.typ = "string"
	case reflect.Bool:
		result.value = converted.Bool()
		result.typ = explainKindBoolean
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		result.value = converted.Int()
		result.typ = "integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		result.value = converted.Uint()
		result.typ = "integer"
	case reflect.Float32, reflect.Float64:
		result.value = converted.Float()
		result.typ = "number"
	default:
		return nil, fmt.Errorf("unsupported override type %s", typ)
	}
	return result, nil
}

func lookupDumpFields(schema *dumpSchema, path []string, visiting map[*dumpSchema]bool) []*dumpField {
	if schema == nil || len(path) == 0 || visiting[schema] {
		return nil
	}
	visiting[schema] = true
	defer delete(visiting, schema)

	if schema.kind == reflect.Slice || schema.kind == reflect.Array {
		return lookupDumpFields(schema.elem, path, visiting)
	}
	if field, ok := schema.fields[path[0]]; ok {
		if len(path) == 1 {
			return []*dumpField{field}
		}
		return lookupDumpFields(field.schema, path[1:], visiting)
	}

	var result []*dumpField
	for _, inline := range schema.inline {
		result = append(result, lookupDumpFields(inline, path, visiting)...)
	}
	for _, branch := range schema.union {
		result = append(result, lookupDumpFields(branch.schema, path, visiting)...)
	}
	return result
}

func pruneDumpDefaults(node *yaml.Node, schema *dumpSchema, resourceType ResourceType, path []string) error {
	if node == nil || schema == nil {
		return nil
	}
	if schema.resourceType != "" {
		resourceType = schema.resourceType
		path = nil
	}

	//exhaustive:ignore // Only collection nodes can contain fields to prune.
	switch node.Kind {
	case yaml.SequenceNode:
		for index, child := range node.Content {
			if err := pruneDumpDefaults(child, schema.elem, resourceType, append(path, strconv.Itoa(index))); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			field, err := findDumpField(schema, node, keyNode.Value, path, make(map[*dumpSchema]bool))
			if err != nil {
				return fmt.Errorf("inspect defaults for %s: %w", resourceType, err)
			}
			if field == nil {
				if schema.additional != nil {
					if err := pruneDumpDefaults(
						valueNode,
						schema.additional,
						resourceType,
						append(path, keyNode.Value),
					); err != nil {
						return err
					}
				}
				i += 2
				continue
			}
			if err := pruneDumpDefaults(
				valueNode,
				field.schema,
				resourceType,
				append(path, keyNode.Value),
			); err != nil {
				return err
			}
			if defaultValue := field.effectiveDefault(resourceType); defaultValue != nil &&
				dumpNodeEqualsDefault(valueNode, defaultValue) {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				continue
			}
			i += 2
		}
	}
	return nil
}

func (f *dumpField) effectiveDefault(resourceType ResourceType) *dumpDefault {
	if f == nil || f.defaultExclusions[resourceType] {
		return nil
	}
	if override := f.defaultOverrides[resourceType]; override != nil {
		return override
	}
	return f.defaultValue
}

func selectDumpUnionBranch(node *yaml.Node, schema *dumpSchema, path []string) (*dumpSchema, error) {
	if len(schema.union) == 0 {
		return nil, nil
	}
	var matches []*dumpSchema
	for _, branch := range schema.union {
		if branch.discriminator == "" {
			continue
		}
		value, ok := dumpMappingValue(node, branch.discriminator)
		if ok && value.Value == branch.value {
			matches = append(matches, branch.schema)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if selected := selectDumpUnionWithUnmarshaller(node, schema); selected != nil {
		return selected, nil
	}
	if !dumpSchemaHasDefaults(schema, make(map[*dumpSchema]bool)) {
		return nil, nil
	}
	location := strings.Join(path, ".")
	if location == "" {
		location = "<root>"
	}
	return nil, fmt.Errorf("cannot determine default-bearing union branch at %s", location)
}

func selectDumpUnionWithUnmarshaller(node *yaml.Node, schema *dumpSchema) *dumpSchema {
	instance := reflect.New(schema.typ)
	unmarshaler, ok := instance.Interface().(json.Unmarshaler)
	if !ok {
		return nil
	}

	var value any
	if err := node.Decode(&value); err != nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil || unmarshaler.UnmarshalJSON(data) != nil {
		return nil
	}

	unionValue := instance.Elem()
	for index := range unionValue.NumField() {
		fieldType := schema.typ.Field(index)
		fieldValue := unionValue.Field(index)
		if fieldType.Tag.Get("union") != "member" || fieldValue.Kind() != reflect.Pointer || fieldValue.IsNil() {
			continue
		}
		selectedType := derefDumpType(fieldType.Type)
		for _, branch := range schema.union {
			if branch.schema.typ == selectedType {
				return branch.schema
			}
		}
	}
	return nil
}

func dumpMappingValue(node *yaml.Node, name string) (*yaml.Node, bool) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == name {
			return node.Content[i+1], true
		}
	}
	return nil, false
}

func findDumpField(
	schema *dumpSchema,
	node *yaml.Node,
	name string,
	path []string,
	visiting map[*dumpSchema]bool,
) (*dumpField, error) {
	if schema == nil || visiting[schema] {
		return nil, nil
	}
	visiting[schema] = true
	defer delete(visiting, schema)

	if field := schema.fields[name]; field != nil {
		return field, nil
	}
	for _, inline := range schema.inline {
		field, err := findDumpField(inline, node, name, path, visiting)
		if err != nil || field != nil {
			return field, err
		}
	}

	selected, err := selectDumpUnionBranch(node, schema, path)
	if err != nil {
		return nil, err
	}
	if selected != nil {
		return findDumpField(selected, node, name, path, visiting)
	}

	var candidates []*dumpField
	for _, branch := range schema.union {
		field, err := findDumpField(branch.schema, node, name, path, visiting)
		if err != nil {
			return nil, err
		}
		if field != nil {
			candidates = append(candidates, field)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	first := candidates[0]
	for _, candidate := range candidates[1:] {
		if !sameDumpDefault(first.defaultValue, candidate.defaultValue) {
			return nil, fmt.Errorf("ambiguous defaults for %s.%s", strings.Join(path, "."), name)
		}
	}
	return first, nil
}

func sameDumpDefault(a, b *dumpDefault) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.kind == b.kind && reflect.DeepEqual(a.value, b.value)
}

func dumpNodeEqualsDefault(node *yaml.Node, defaultValue *dumpDefault) bool {
	if node == nil || node.Kind != yaml.ScalarNode || node.Tag == "!!null" {
		return false
	}
	//exhaustive:ignore // Default construction limits this to scalar kinds.
	switch defaultValue.kind {
	case reflect.String:
		return node.Tag == "!!str" && node.Value == defaultValue.value.(string)
	case reflect.Bool:
		if node.Tag != "!!bool" {
			return false
		}
		value, err := strconv.ParseBool(node.Value)
		return err == nil && value == defaultValue.value.(bool)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if node.Tag != "!!int" {
			return false
		}
		value, err := strconv.ParseInt(node.Value, 10, 64)
		return err == nil && value == defaultValue.value.(int64)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if node.Tag != "!!int" {
			return false
		}
		value, err := strconv.ParseUint(node.Value, 10, 64)
		return err == nil && value == defaultValue.value.(uint64)
	case reflect.Float32, reflect.Float64:
		if node.Tag != "!!float" && node.Tag != "!!int" {
			return false
		}
		value, err := strconv.ParseFloat(node.Value, 64)
		return err == nil && value == defaultValue.value.(float64)
	}
	return false
}

func dumpSchemaHasDefaults(schema *dumpSchema, visiting map[*dumpSchema]bool) bool {
	if schema == nil || visiting[schema] {
		return false
	}
	visiting[schema] = true
	defer delete(visiting, schema)
	for _, field := range schema.fields {
		if field.defaultValue != nil || dumpSchemaHasDefaults(field.schema, visiting) {
			return true
		}
	}
	for _, inline := range schema.inline {
		if dumpSchemaHasDefaults(inline, visiting) {
			return true
		}
	}
	for _, branch := range schema.union {
		if dumpSchemaHasDefaults(branch.schema, visiting) {
			return true
		}
	}
	return dumpSchemaHasDefaults(schema.elem, visiting) || dumpSchemaHasDefaults(schema.additional, visiting)
}

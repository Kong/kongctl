package resources

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
)

const (
	jsonSchemaDraft202012 = "https://json-schema.org/draft/2020-12/schema"
	explainKindArray      = "array"
	explainKindObject     = "object"

	explainResourceClassTopLevel = "top-level"
	explainResourceClassChild    = "child"
	explainResourceClassGrouped  = "grouped"
)

type ExplainRegistration struct {
	typ           reflect.Type
	aliases       []string
	fieldHints    map[string]ExplainFieldHint
	schemaBuilder func(ExplainBuildContext) (*ExplainNode, error)
}

type ExplainOption func(*ExplainRegistration)

type ExplainFieldHint struct {
	Description  string
	Default      any
	DefaultFrom  string
	Enum         []any
	FileSample   string
	Literal      string
	Recommended  *bool
	PreferredTag string
	RefKind      string
	Notes        []string
}

type ExplainBuildContext struct {
	ResourceType ResourceType
	Registration ExplainRegistration
}

type ExplainRelation struct {
	ParentAlias   string `json:"parent_alias"   yaml:"parent_alias"`
	ParentType    string `json:"parent_type"    yaml:"parent_type"`
	FieldName     string `json:"field_name"     yaml:"field_name"`
	ChildAlias    string `json:"child_alias"    yaml:"child_alias"`
	ParentRootKey string `json:"parent_root_key" yaml:"parent_root_key"`
}

type ExplainDoc struct {
	ResourceType              ResourceType      `json:"resource_type"               yaml:"resource_type"`
	CanonicalAlias            string            `json:"canonical_alias"             yaml:"canonical_alias"`
	Aliases                   []string          `json:"aliases,omitempty"           yaml:"aliases,omitempty"`
	ResourceClass             string            `json:"resource_class"              yaml:"resource_class"`
	RootKey                   string            `json:"root_key,omitempty"          yaml:"root_key,omitempty"`
	SupportsRoot              bool              `json:"supports_root"               yaml:"supports_root"`
	SupportsNestedDeclaration bool              `json:"supports_nested_declaration" yaml:"supports_nested_declaration"`
	NestedRelations           []ExplainRelation `json:"nested_relations,omitempty"  yaml:"nested_relations,omitempty"`
	ParentRelations           []ExplainRelation `json:"parent_relations,omitempty"  yaml:"parent_relations,omitempty"`
	SupportsKongctl           bool              `json:"supports_kongctl"            yaml:"supports_kongctl"`
	Schema                    *ExplainNode      `json:"-"                           yaml:"-"`
	nestedFields    map[string]ResourceType
}

type ExplainSubject struct {
	Doc               *ExplainDoc
	Node              *ExplainNode
	DisplayPath       string
	FieldPath         []string
	FieldRelativePath []string
	FieldRequired     bool
	FieldRecommended  bool
	ScaffoldSteps     []ExplainScaffoldStep
	ScaffoldOmit      []string
	ScaffoldTrail     []ExplainScaffoldNode
	AncestorTypes     []ResourceType
	ResourceTarget    bool
}

type ExplainScaffoldStep struct {
	Name  string
	Array bool
}

type ExplainScaffoldNode struct {
	Step ExplainScaffoldStep
	Node *ExplainNode
	Omit []string
}

type ExplainNode struct {
	Kind         string
	Description  string
	Default      any
	DefaultFrom  string
	Enum         []any
	Const        any
	Nullable     bool
	Recommended  bool
	PreferredTag string
	RefKind      string
	Notes        []string
	Literal      string
	Properties   []*ExplainField
	propIndex    map[string]*ExplainField
	Items        *ExplainNode
	Additional   *ExplainNode
	OneOf        []*ExplainNode
}

type ExplainField struct {
	Name        string
	Node        *ExplainNode
	Required    bool
	Recommended bool
}

type JSONSchema struct {
	Schema      string                  `json:"$schema,omitempty" yaml:"$schema,omitempty"`
	ID          string                  `json:"$id,omitempty" yaml:"$id,omitempty"`
	Title       string                  `json:"title,omitempty" yaml:"title,omitempty"`
	Description string                  `json:"description,omitempty" yaml:"description,omitempty"`
	Type        any                     `json:"type,omitempty" yaml:"type,omitempty"`
	Properties  map[string]*JSONSchema  `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required    []string                `json:"required,omitempty" yaml:"required,omitempty"`
	Items       *JSONSchema             `json:"items,omitempty" yaml:"items,omitempty"`
	Additional  any                     `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
	OneOf       []*JSONSchema           `json:"oneOf,omitempty" yaml:"oneOf,omitempty"`
	Const       any                     `json:"const,omitempty" yaml:"const,omitempty"`
	Enum        []any                   `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default     any                     `json:"default,omitempty" yaml:"default,omitempty"`
	XResource   any                     `json:"x-kongctl-resource,omitempty" yaml:"x-kongctl-resource,omitempty"`
	XPath       string                  `json:"x-kongctl-path,omitempty" yaml:"x-kongctl-path,omitempty"`
	XRootKey    string                  `json:"x-kongctl-root-key,omitempty" yaml:"x-kongctl-root-key,omitempty"`
	XClass      string                  `json:"x-kongctl-resource-class,omitempty" yaml:"x-kongctl-resource-class,omitempty"` //nolint:lll
	XRefKind    string                  `json:"x-kongctl-ref-kind,omitempty" yaml:"x-kongctl-ref-kind,omitempty"`
	XTag        string                  `json:"x-kongctl-preferred-tag,omitempty" yaml:"x-kongctl-preferred-tag,omitempty"`
	XDefault    string                  `json:"x-kongctl-default-from,omitempty" yaml:"x-kongctl-default-from,omitempty"`
	XNotes      []string                `json:"x-kongctl-notes,omitempty" yaml:"x-kongctl-notes,omitempty"`
	XSubject    *ExplainSchemaSubject   `json:"x-kongctl-subject,omitempty" yaml:"x-kongctl-subject,omitempty"`
	XPlacement  *ExplainSchemaPlacement `json:"x-kongctl-placement,omitempty" yaml:"x-kongctl-placement,omitempty"`
	XRoot       *bool                   `json:"x-kongctl-supports-root,omitempty" yaml:"x-kongctl-supports-root,omitempty"`
	XNestedDecl *bool                   `json:"x-kongctl-supports-nested-declaration,omitempty" yaml:"x-kongctl-supports-nested-declaration,omitempty"` //nolint:lll
}

type ExplainSchemaSubject struct {
	Kind        string `json:"kind" yaml:"kind"`
	Path        string `json:"path" yaml:"path"`
	Required    *bool  `json:"required,omitempty" yaml:"required,omitempty"`
	Recommended *bool  `json:"recommended,omitempty" yaml:"recommended,omitempty"`
}

type ExplainSchemaPlacement struct {
	YAMLPath        string   `json:"yaml_path,omitempty" yaml:"yaml_path,omitempty"`
	RootYAMLPath    string   `json:"root_yaml_path,omitempty" yaml:"root_yaml_path,omitempty"`
	NestedYAMLPath  string   `json:"nested_yaml_path,omitempty" yaml:"nested_yaml_path,omitempty"`
	NestedYAMLPaths []string `json:"nested_yaml_paths,omitempty" yaml:"nested_yaml_paths,omitempty"`
}

type ExplainSchemaResource struct {
	Name          string `json:"name" yaml:"name"`
	ResourceClass string `json:"resource_class" yaml:"resource_class"`
}

var (
	explainDocCache   = make(map[ResourceType]*ExplainDoc)
	explainDocCacheMu sync.RWMutex

	recommendedFieldNames = map[string]struct{}{
		"ref":          {},
		"name":         {},
		"display_name": {},
		"description":  {},
		"version":      {},
		"slug":         {},
		"spec":         {},
		"spec_content": {},
	}
	fileFieldSamples = map[string]string{
		"content":            "./content.txt",
		"css":                "./custom.css",
		"robots":             "./robots.txt",
		"spec":               "./specs/openapi.yaml",
		"spec.content":       "./specs/openapi.yaml",
		"spec_content":       "./specs/openapi.yaml",
		"custom_certificate": "./certs/cert.pem",
		"custom_private_key": "./certs/key.pem",
	}
)

func AutoExplain[R any](opts ...ExplainOption) ExplainRegistration {
	reg := ExplainRegistration{
		typ:        reflect.TypeFor[R](),
		fieldHints: make(map[string]ExplainFieldHint),
	}
	for _, opt := range opts {
		opt(&reg)
	}
	return reg
}

func WithExplainAliases(aliases ...string) ExplainOption {
	return func(reg *ExplainRegistration) {
		reg.aliases = append(reg.aliases, aliases...)
	}
}

func WithExplainFieldHint(path string, hint ExplainFieldHint) ExplainOption {
	return func(reg *ExplainRegistration) {
		if reg.fieldHints == nil {
			reg.fieldHints = make(map[string]ExplainFieldHint)
		}
		reg.fieldHints[path] = mergeExplainFieldHint(reg.fieldHints[path], hint)
	}
}

func WithExplainRecommendedFields(paths ...string) ExplainOption {
	return func(reg *ExplainRegistration) {
		for _, path := range paths {
			hint := reg.fieldHints[path]
			recommended := true
			hint.Recommended = &recommended
			reg.fieldHints[path] = hint
		}
	}
}

func WithExplainSchemaBuilder(fn func(ExplainBuildContext) (*ExplainNode, error)) ExplainOption {
	return func(reg *ExplainRegistration) {
		reg.schemaBuilder = fn
	}
}

func mergeExplainFieldHint(base ExplainFieldHint, override ExplainFieldHint) ExplainFieldHint {
	result := base
	if override.Description != "" {
		result.Description = override.Description
	}
	if override.Default != nil {
		result.Default = override.Default
	}
	if override.DefaultFrom != "" {
		result.DefaultFrom = override.DefaultFrom
	}
	if len(override.Enum) > 0 {
		result.Enum = append([]any(nil), override.Enum...)
	}
	if override.FileSample != "" {
		result.FileSample = override.FileSample
	}
	if override.Literal != "" {
		result.Literal = override.Literal
	}
	if override.Recommended != nil {
		value := *override.Recommended
		result.Recommended = &value
	}
	if override.PreferredTag != "" {
		result.PreferredTag = override.PreferredTag
	}
	if override.RefKind != "" {
		result.RefKind = override.RefKind
	}
	if len(override.Notes) > 0 {
		result.Notes = append([]string(nil), override.Notes...)
	}
	return result
}

func ResolveExplainSubject(path string) (*ExplainSubject, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("resource path is required")
	}

	segments := strings.Split(path, ".")
	doc, ok := explainDocByAlias(strings.TrimSpace(segments[0]))
	if !ok {
		return nil, fmt.Errorf("unsupported resource path %q", path)
	}

	subject := &ExplainSubject{
		Doc:            doc,
		Node:           doc.Schema.clone(),
		DisplayPath:    path,
		ResourceTarget: true,
	}

	if doc.SupportsRoot {
		subject.ScaffoldSteps = []ExplainScaffoldStep{{Name: doc.RootKey, Array: true}}
		subject.ScaffoldTrail = []ExplainScaffoldNode{{
			Step: ExplainScaffoldStep{Name: doc.RootKey, Array: true},
			Node: doc.Schema.clone(),
		}}
	}

	if len(segments) == 1 {
		if !doc.SupportsRoot && len(doc.ParentRelations) > 0 {
			relation := doc.ParentRelations[0]
			subject.ScaffoldSteps = append(subject.ScaffoldSteps,
				ExplainScaffoldStep{Name: relation.ParentRootKey, Array: true},
				ExplainScaffoldStep{Name: relation.FieldName, Array: true},
			)
			if parentDoc, ok := explainDocByType(ResourceType(relation.ParentType)); ok {
				subject.ScaffoldTrail = []ExplainScaffoldNode{
					{
						Step: ExplainScaffoldStep{Name: relation.ParentRootKey, Array: true},
						Node: parentDoc.Schema.clone(),
					},
					{
						Step: ExplainScaffoldStep{Name: relation.FieldName, Array: true},
						Node: doc.Schema.clone(),
						Omit: scaffoldOmitFields([]ResourceType{ResourceType(relation.ParentType)}),
					},
				}
			}
			subject.AncestorTypes = append(subject.AncestorTypes, ResourceType(relation.ParentType))
		}
		subject.ScaffoldOmit = scaffoldOmitFields(subject.AncestorTypes)
		return subject, nil
	}

	currentDoc := doc
	currentNode := subject.Node
	if len(subject.ScaffoldSteps) == 0 {
		subject.ScaffoldSteps = nil
	}

	var ancestors []ResourceType
	var relativePath []string
	for i, segment := range segments[1:] {
		segment = strings.TrimSpace(segment)
		field, ok := currentNode.property(segment)
		if !ok {
			return nil, fmt.Errorf("field %q not found in %q", segment, strings.Join(segments[:i+1], "."))
		}

		nextNode := field.Node
		resourceTarget := false
		if nextNode.Kind == "array" && nextNode.Items != nil && nextNode.Items.Kind == explainKindObject {
			nextNode = nextNode.Items.clone()
			resourceTarget = true
		} else {
			nextNode = nextNode.clone()
		}

		subject.FieldPath = append(subject.FieldPath, segment)
		relativePath = append(relativePath, segment)
		subject.FieldRelativePath = append([]string(nil), relativePath...)
		subject.FieldRequired = field.Required
		subject.FieldRecommended = field.Recommended
		currentNode = nextNode

		if childType, ok := currentDoc.nestedFields[segment]; ok {
			if childDoc, ok := explainDocByType(childType); ok {
				if len(subject.ScaffoldSteps) == 0 {
					subject.ScaffoldSteps = []ExplainScaffoldStep{{Name: currentDoc.RootKey, Array: true}}
				}
				subject.ScaffoldSteps = append(subject.ScaffoldSteps, ExplainScaffoldStep{Name: segment, Array: true})
				ancestors = append(ancestors, currentDoc.ResourceType)
				subject.ScaffoldTrail = append(subject.ScaffoldTrail, ExplainScaffoldNode{
					Step: ExplainScaffoldStep{Name: segment, Array: true},
					Node: childDoc.Schema.clone(),
					Omit: scaffoldOmitFields(ancestors),
				})
				currentDoc = childDoc
				currentNode = childDoc.Schema.clone()
				resourceTarget = true
				relativePath = nil
				subject.FieldRelativePath = nil
			}
		}

		subject.Node = currentNode
		subject.ResourceTarget = resourceTarget
	}

	subject.Doc = currentDoc
	subject.AncestorTypes = ancestors
	subject.ScaffoldOmit = scaffoldOmitFields(ancestors)

	return subject, nil
}

func explainDocByAlias(alias string) (*ExplainDoc, bool) {
	types := RegisteredTypes()
	slices.SortFunc(types, func(a, b ResourceType) int {
		return strings.Compare(string(a), string(b))
	})
	for _, rt := range types {
		doc, err := buildExplainDoc(rt)
		if err != nil {
			continue
		}
		for _, candidate := range doc.Aliases {
			if candidate == alias {
				return doc, true
			}
		}
	}
	return nil, false
}

func explainDocByType(rt ResourceType) (*ExplainDoc, bool) {
	doc, err := buildExplainDoc(rt)
	if err != nil {
		return nil, false
	}
	return doc, true
}

func buildExplainDoc(rt ResourceType) (*ExplainDoc, error) {
	explainDocCacheMu.RLock()
	if doc, ok := explainDocCache[rt]; ok {
		explainDocCacheMu.RUnlock()
		return doc, nil
	}
	explainDocCacheMu.RUnlock()

	ops, ok := registry[rt]
	if !ok {
		return nil, fmt.Errorf("resource type %q is not registered", rt)
	}
	reg := ops.explain
	if reg.typ == nil {
		return nil, fmt.Errorf("resource type %q does not have explain registration", rt)
	}

	node, err := buildExplainSchema(rt, reg)
	if err != nil {
		return nil, err
	}

	rootKey := resourceSetRootKey(reg.typ)
	aliases := []string{string(rt)}
	if rootKey != "" {
		aliases = append(aliases, rootKey)
	}
	aliases = append(aliases, reg.aliases...)
	aliases = slices.Compact(aliases)

	childRelations := explainNestedRelations(rt, reg.typ)
	parentRelations := nestedRelationsFor(rt)

	resourceClass := explainResourceClass(reg.typ, rootKey, parentRelations, childRelations)

	doc := &ExplainDoc{
		ResourceType:              rt,
		CanonicalAlias:            string(rt),
		Aliases:                   aliases,
		ResourceClass:             resourceClass,
		RootKey:                   rootKey,
		SupportsRoot:              rootKey != "",
		SupportsNestedDeclaration: len(parentRelations) > 0,
		NestedRelations:           childRelations,
		ParentRelations:           parentRelations,
		SupportsKongctl:           node.propertyExists("kongctl"),
		Schema:                    node,
		nestedFields:    nestedFieldMap(childRelations),
	}

	explainDocCacheMu.Lock()
	explainDocCache[rt] = doc
	explainDocCacheMu.Unlock()

	return doc, nil
}

func explainResourceClass(
	typ reflect.Type,
	rootKey string,
	parentRelations []ExplainRelation,
	childRelations []ExplainRelation,
) string {
	if len(parentRelations) > 0 {
		return explainResourceClassChild
	}

	var childProbe any
	if typ.Kind() == reflect.Struct {
		childProbe = reflect.New(typ).Interface()
	}
	if _, ok := childProbe.(ResourceWithParent); ok {
		return explainResourceClassChild
	}

	if rootKey == "" && len(childRelations) > 0 {
		return explainResourceClassGrouped
	}

	return explainResourceClassTopLevel
}

func nestedFieldMap(relations []ExplainRelation) map[string]ResourceType {
	result := make(map[string]ResourceType)
	for _, relation := range relations {
		result[relation.FieldName] = ResourceType(relation.ChildAlias)
	}
	return result
}

func buildExplainSchema(rt ResourceType, reg ExplainRegistration) (*ExplainNode, error) {
	ctx := ExplainBuildContext{ResourceType: rt, Registration: reg}
	if reg.schemaBuilder != nil {
		node, err := reg.schemaBuilder(ctx)
		if err != nil {
			return nil, err
		}
		if node != nil {
			return node, nil
		}
	}

	pathHints := defaultExplainHints(rt)
	maps.Copy(pathHints, reg.fieldHints)

	node, err := autoExplainNode(reg.typ, nil, pathHints, nil)
	if err != nil {
		return nil, err
	}

	applyReferenceHints(rt, node)

	return node, nil
}

func defaultExplainHints(rt ResourceType) map[string]ExplainFieldHint {
	hints := make(map[string]ExplainFieldHint)
	hints["ref"] = ExplainFieldHint{Recommended: boolPtr(true)}

	if hasBaseResource(rt) {
		hints["name"] = ExplainFieldHint{
			DefaultFrom: "ref",
			Recommended: boolPtr(true),
		}
	}

	for path, sample := range fileFieldSamples {
		hints[path] = mergeExplainFieldHint(hints[path], ExplainFieldHint{
			FileSample:   sample,
			PreferredTag: "!file",
		})
	}

	return hints
}

func applyReferenceHints(rt ResourceType, node *ExplainNode) {
	doc, ok := registry[rt]
	if !ok {
		return
	}
	instance := reflect.New(doc.explain.typ).Interface()
	if mappings, ok := instance.(ReferenceMapping); ok {
		for path, kind := range mappings.GetReferenceFieldMappings() {
			if fieldNode, ok := node.lookup(strings.Split(path, ".")); ok {
				fieldNode.RefKind = kind
				if fieldNode.PreferredTag == "" {
					fieldNode.PreferredTag = "!ref"
				}
				if fieldNode.Literal == "" {
					fieldNode.Literal = fmt.Sprintf("!ref my-%s", strings.ReplaceAll(kind, "_", "-"))
				}
			}
		}
	}
}

func autoExplainNode(
	typ reflect.Type,
	path []string,
	hints map[string]ExplainFieldHint,
	stack []reflect.Type,
) (*ExplainNode, error) {
	typ = derefExplainType(typ)
	stack = append(stack, typ)

	if typ.Kind() == reflect.Struct {
		node := &ExplainNode{
			Kind:       explainKindObject,
			Properties: []*ExplainField{},
			propIndex:  make(map[string]*ExplainField),
		}

		for field := range typ.Fields() {
			if !field.IsExported() {
				continue
			}

			yamlName, yamlInline, yamlOmit, skip := explainFieldName(field, "yaml")
			if skip {
				continue
			}
			jsonName, _, jsonOmit, _ := explainFieldName(field, "json")
			if field.Anonymous && (yamlInline || jsonName == ",inline" || (yamlName == "" && jsonName == "")) {
				embedded, err := autoExplainNode(field.Type, path, hints, stack)
				if err != nil {
					return nil, err
				}
				for _, embeddedField := range embedded.Properties {
					node.addField(embeddedField)
				}
				continue
			}

			name := yamlName
			if name == "" {
				name = jsonName
			}
			if name == "" {
				name = snakeCase(field.Name)
			}

			fieldPath := append(path, name)
			child, err := autoExplainValueNode(field.Type, fieldPath, hints, stack)
			if err != nil {
				return nil, err
			}

			required := !yamlOmit && !jsonOmit && field.Type.Kind() != reflect.Pointer &&
				field.Type.Kind() != reflect.Slice && field.Type.Kind() != reflect.Map
			if child.Default != nil || child.DefaultFrom != "" {
				required = false
			}
			recommended := required || shouldRecommendField(name, hints, strings.Join(fieldPath, "."))

			fieldNode := &ExplainField{
				Name:        name,
				Node:        child,
				Required:    required,
				Recommended: recommended,
			}
			node.addField(fieldNode)
		}

		return node, nil
	}

	return autoExplainValueNode(typ, path, hints, stack)
}

func autoExplainValueNode(
	typ reflect.Type,
	path []string,
	hints map[string]ExplainFieldHint,
	stack []reflect.Type,
) (*ExplainNode, error) {
	nullable := typ.Kind() == reflect.Pointer
	typ = derefExplainType(typ)

	node := &ExplainNode{
		Nullable: nullable,
	}

	fieldPath := strings.Join(path, ".")
	name := ""
	if len(path) > 0 {
		name = path[len(path)-1]
	}

	switch typ.Kind() {
	case reflect.Struct:
		if slices.Contains(stack, typ) {
			child := recursiveExplainNode(typ)
			child.Nullable = child.Nullable || nullable
			node = child
		} else if resourceType, ok := explainRegisteredResourceType(typ); ok {
			if doc, ok := explainDocByType(resourceType); ok {
				child := doc.Schema.clone()
				child.Nullable = child.Nullable || nullable
				node = child
			} else {
				child, err := autoExplainNode(typ, path, hints, stack)
				if err != nil {
					return nil, err
				}
				child.Nullable = child.Nullable || nullable
				node = child
			}
		} else {
			child, err := autoExplainNode(typ, path, hints, stack)
			if err != nil {
				return nil, err
			}
			child.Nullable = child.Nullable || nullable
			node = child
		}
	case reflect.Slice, reflect.Array:
		child, err := autoExplainValueNode(typ.Elem(), path, hints, stack)
		if err != nil {
			return nil, err
		}
		node.Kind = "array"
		node.Items = child
	case reflect.Map:
		child, err := autoExplainValueNode(typ.Elem(), path, hints, stack)
		if err != nil {
			return nil, err
		}
		node.Kind = explainKindObject
		node.Additional = child
	case reflect.String:
		node.Kind = "string"
	case reflect.Bool:
		node.Kind = "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		node.Kind = "integer"
	case reflect.Float32, reflect.Float64:
		node.Kind = "number"
	case reflect.Interface:
		node.Kind = "any"
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Chan, reflect.Func,
		reflect.Pointer, reflect.UnsafePointer:
		node.Kind = "any"
	}

	if hint, ok := hints[fieldPath]; ok {
		applyExplainFieldHint(node, hint)
	} else if hint, ok := hints[name]; ok {
		applyExplainFieldHint(node, hint)
	}

	if node.Literal == "" {
		node.Literal = explainLiteralFor(node, name)
	}

	return node, nil
}

func recursiveExplainNode(typ reflect.Type) *ExplainNode {
	return &ExplainNode{
		Kind:        "object",
		Description: fmt.Sprintf("Recursive %s object", snakeCase(typ.Name())),
		Notes: []string{
			"schema recursion truncated after the first expansion",
		},
	}
}

func applyExplainFieldHint(node *ExplainNode, hint ExplainFieldHint) {
	if hint.Description != "" {
		node.Description = hint.Description
	}
	if hint.Default != nil {
		node.Default = hint.Default
	}
	if hint.DefaultFrom != "" {
		node.DefaultFrom = hint.DefaultFrom
	}
	if len(hint.Enum) > 0 {
		node.Enum = append([]any(nil), hint.Enum...)
	}
	if hint.FileSample != "" {
		node.Literal = fmt.Sprintf("!file %s", hint.FileSample)
	}
	if hint.Literal != "" {
		node.Literal = hint.Literal
	}
	if hint.Recommended != nil {
		node.Recommended = *hint.Recommended
	}
	if hint.PreferredTag != "" {
		node.PreferredTag = hint.PreferredTag
	}
	if hint.RefKind != "" {
		node.RefKind = hint.RefKind
	}
	if len(hint.Notes) > 0 {
		node.Notes = append([]string(nil), hint.Notes...)
	}
}

func explainLiteralFor(node *ExplainNode, name string) string {
	if len(node.Enum) > 0 {
		if text, ok := node.Enum[0].(string); ok {
			return text
		}
	}
	if node.RefKind != "" {
		return fmt.Sprintf("!ref my-%s", strings.ReplaceAll(node.RefKind, "_", "-"))
	}
	switch node.Kind {
	case "string":
		switch name {
		case "ref":
			return "my-resource"
		case "name":
			return "my-resource"
		case "display_name":
			return "My Resource"
		case "description":
			return "Example description"
		case "version":
			return "v1.0.0"
		case "slug":
			return "my-resource"
		case "host":
			return "example.com"
		case "role_name":
			return "viewer"
		case "entity_type_name":
			return "api"
		case "entity_region":
			return "us"
		case "namespace":
			return "default"
		default:
			return "value"
		}
	case "integer":
		switch name {
		case "port":
			return "80"
		default:
			return "1"
		}
	case "number":
		return "1"
	case "boolean":
		return "false"
	default:
		return ""
	}
}

func RenderExplainText(subject *ExplainSubject, extended bool) string {
	var b strings.Builder

	if subject.isFieldTarget() {
		renderExplainFieldText(&b, subject)
	} else {
		renderExplainResourceBlock(&b, subject.Doc)
		if !extended {
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "FIELD DETAILS: use --extended")
			return strings.TrimRight(b.String(), "\n")
		}
	}

	if subject.Node != nil && subject.Node.Kind == explainKindObject && (!subject.isFieldTarget() || extended) {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "FIELDS")
		renderExplainFields(&b, subject.Node, "", 0)
	}

	return strings.TrimRight(b.String(), "\n")
}

func renderExplainFieldText(b *strings.Builder, subject *ExplainSubject) {
	fmt.Fprintln(b, "FIELD")
	fmt.Fprintf(b, "PATH: %s\n", subject.DisplayPath)
	fmt.Fprintf(b, "TYPE: %s\n", explainTypeLabel(subject.Node))
	fmt.Fprintf(b, "OPTIONAL: %t\n", !subject.FieldRequired)
	if subject.FieldRecommended {
		fmt.Fprintln(b, "RECOMMENDED: yes")
	}
	if subject.Node != nil && subject.Node.DefaultFrom != "" {
		fmt.Fprintf(b, "DEFAULT FROM: %s\n", subject.Node.DefaultFrom)
	}
	if subject.Node != nil && subject.Node.RefKind != "" {
		fmt.Fprintf(b, "REF TARGET: %s\n", subject.Node.RefKind)
	}
	if subject.Node != nil && subject.Node.PreferredTag != "" {
		fmt.Fprintf(b, "PREFERRED TAG: %s\n", subject.Node.PreferredTag)
	}

	if placement := explainFieldPlacement(subject); placement != nil {
		if placement.RootYAMLPath != "" || len(placement.NestedYAMLPaths) > 0 || placement.NestedYAMLPath != "" {
			switch {
			case placement.YAMLPath != "":
				fmt.Fprintf(b, "ROOT YAML PATH: %s\n", placement.YAMLPath)
			case placement.RootYAMLPath != "":
				fmt.Fprintf(b, "ROOT YAML PATH: %s\n", placement.RootYAMLPath)
			}
			if placement.NestedYAMLPath != "" {
				fmt.Fprintf(b, "NESTED YAML PATH: %s\n", placement.NestedYAMLPath)
			}
			for _, path := range placement.NestedYAMLPaths {
				fmt.Fprintf(b, "NESTED YAML PATH: %s\n", path)
			}
		}
	}

	fmt.Fprintln(b)
	renderExplainResourceBlock(b, subject.Doc)
}

func renderExplainFields(b *strings.Builder, node *ExplainNode, path string, depth int) {
	if node == nil {
		return
	}
	if node.Kind == explainKindArray {
		renderExplainFields(b, node.Items, path+"[]", depth)
		return
	}
	if node.Kind == explainKindObject && node.Additional != nil {
		renderExplainFields(b, node.Additional, path+"{}", depth)
	}
	if node.Kind != explainKindObject {
		return
	}
	prefix := strings.Repeat("  ", depth)
	for _, field := range node.Properties {
		fieldPath := field.Name
		if path != "" {
			fieldPath = path + "." + field.Name
		}
		req := "optional"
		if field.Required {
			req = "required"
		}
		fmt.Fprintf(b, "%s- %s: %s %s\n", prefix, field.Name, explainTypeLabel(field.Node), req)
		renderExplainFields(b, field.Node, fieldPath, depth+1)
	}
}

func renderExplainResourceBlock(b *strings.Builder, doc *ExplainDoc) {
	if doc == nil {
		return
	}
	fmt.Fprintln(b, "RESOURCE")
	fmt.Fprintf(b, "RESOURCE CLASS: %s\n", doc.ResourceClass)
	if doc.RootKey != "" {
		fmt.Fprintf(b, "ROOT KEY: %s[]\n", doc.RootKey)
	}
	fmt.Fprintf(b, "SUPPORTS ROOT: %t\n", doc.SupportsRoot)
	fmt.Fprintf(b, "SUPPORTS NESTED DECLARATION: %t\n", doc.SupportsNestedDeclaration)
	if doc.SupportsKongctl {
		fmt.Fprintln(b, "ACCEPTS kongctl metadata: yes")
	} else {
		fmt.Fprintln(b, "ACCEPTS kongctl metadata: no")
	}
	childResources := explainChildResourceNames(doc)
	if len(childResources) > 0 {
		fmt.Fprintf(b, "CHILD RESOURCES: %s\n", strings.Join(childResources, ", "))
	}
}

func explainChildResourceNames(doc *ExplainDoc) []string {
	if doc == nil || len(doc.NestedRelations) == 0 {
		return nil
	}
	names := make([]string, 0, len(doc.NestedRelations))
	for _, relation := range doc.NestedRelations {
		names = append(names, relation.FieldName)
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func RenderExplainSchema(subject *ExplainSubject) *JSONSchema {
	schema := subject.Node.toJSONSchema()
	schema.Schema = jsonSchemaDraft202012
	schema.ID = fmt.Sprintf("kongctl://declarative/%s", strings.ReplaceAll(subject.DisplayPath, ".", "/"))
	schema.Title = fmt.Sprintf("kongctl declarative schema: %s", subject.DisplayPath)
	if subject.isFieldTarget() {
		schema.XSubject = &ExplainSchemaSubject{
			Kind:        "field",
			Path:        subject.DisplayPath,
			Required:    boolPtr(subject.FieldRequired),
			Recommended: boolPtr(subject.FieldRecommended),
		}
		schema.XPlacement = explainFieldPlacement(subject)
		schema.XResource = &ExplainSchemaResource{
			Name:          subject.Doc.CanonicalAlias,
			ResourceClass: subject.Doc.ResourceClass,
		}
		return schema
	}
	schema.XResource = subject.Doc.CanonicalAlias
	schema.XPath = subject.DisplayPath
	if subject.Doc.RootKey != "" {
		schema.XRootKey = subject.Doc.RootKey
	}
	schema.XClass = subject.Doc.ResourceClass
	schema.XRoot = boolPtr(subject.Doc.SupportsRoot)
	schema.XNestedDecl = boolPtr(subject.Doc.SupportsNestedDeclaration)
	return schema
}

func explainFieldPlacement(subject *ExplainSubject) *ExplainSchemaPlacement {
	if subject == nil || !subject.isFieldTarget() || len(subject.FieldRelativePath) == 0 {
		return nil
	}

	requestedPath := explainPlacementPath(subject.ScaffoldSteps, subject.FieldRelativePath)
	rootPath := explainRootFieldPath(subject.Doc, subject.FieldRelativePath)
	nestedPaths := explainNestedFieldPaths(subject.Doc, subject.FieldRelativePath)

	placement := &ExplainSchemaPlacement{}
	if len(subject.AncestorTypes) > 0 {
		placement.NestedYAMLPath = requestedPath
		if rootPath != "" && rootPath != requestedPath {
			placement.RootYAMLPath = rootPath
		}
		placement.NestedYAMLPaths = filterExplainPaths(nestedPaths, requestedPath)
	} else {
		placement.YAMLPath = requestedPath
		placement.NestedYAMLPaths = filterExplainPaths(nestedPaths, requestedPath)
	}

	if !placement.hasAnyPath() {
		return nil
	}

	return placement
}

func (s *ExplainSubject) isFieldTarget() bool {
	return s != nil && !s.ResourceTarget
}

func (p *ExplainSchemaPlacement) hasAnyPath() bool {
	return p != nil && (p.YAMLPath != "" || p.RootYAMLPath != "" || p.NestedYAMLPath != "" ||
		len(p.NestedYAMLPaths) > 0)
}

func explainPlacementPath(steps []ExplainScaffoldStep, relativePath []string) string {
	parts := make([]string, 0, len(steps)+len(relativePath))
	for _, step := range steps {
		name := step.Name
		if step.Array {
			name += "[]"
		}
		parts = append(parts, name)
	}
	parts = append(parts, relativePath...)
	return strings.Join(parts, ".")
}

func explainRootFieldPath(doc *ExplainDoc, relativePath []string) string {
	if doc == nil || !doc.SupportsRoot || doc.RootKey == "" {
		return ""
	}
	parts := []string{doc.RootKey + "[]"}
	parts = append(parts, relativePath...)
	return strings.Join(parts, ".")
}

func explainNestedFieldPaths(doc *ExplainDoc, relativePath []string) []string {
	if doc == nil || len(doc.ParentRelations) == 0 {
		return nil
	}
	paths := make([]string, 0, len(doc.ParentRelations))
	for _, relation := range doc.ParentRelations {
		parts := []string{explainRelationParentPath(relation), relation.FieldName + "[]"}
		parts = append(parts, relativePath...)
		paths = append(paths, strings.Join(parts, "."))
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

func explainRelationParentPath(relation ExplainRelation) string {
	if relation.ParentRootKey != "" {
		return relation.ParentRootKey + "[]"
	}
	return relation.ParentAlias
}

func filterExplainPaths(paths []string, current string) []string {
	var filtered []string
	for _, path := range paths {
		if path != "" && path != current {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

func RenderScaffoldYAML(subject *ExplainSubject) (string, error) {
	if !subject.ResourceTarget || subject.Node == nil || subject.Node.Kind != "object" {
		return "", fmt.Errorf("scaffold supports resource objects only")
	}

	trail := append([]ExplainScaffoldNode(nil), subject.ScaffoldTrail...)
	if len(trail) == 0 {
		if subject.Doc.RootKey != "" {
			trail = append(trail, ExplainScaffoldNode{
				Step: ExplainScaffoldStep{Name: subject.Doc.RootKey, Array: true},
				Node: subject.Doc.Schema.clone(),
			})
		} else if len(subject.Doc.NestedRelations) > 0 {
			relation := subject.Doc.NestedRelations[0]
			if parentDoc, ok := explainDocByType(ResourceType(relation.ParentType)); ok {
				trail = append(trail,
					ExplainScaffoldNode{
						Step: ExplainScaffoldStep{Name: relation.ParentRootKey, Array: true},
						Node: parentDoc.Schema.clone(),
					},
					ExplainScaffoldNode{
						Step: ExplainScaffoldStep{Name: relation.FieldName, Array: true},
						Node: subject.Doc.Schema.clone(),
						Omit: scaffoldOmitFields([]ResourceType{ResourceType(relation.ParentType)}),
					},
				)
			}
		}
	}

	var lines []string
	renderScaffoldTrail(linesAppender(&lines), trail, 0)
	return strings.Join(lines, "\n") + "\n", nil
}

type scaffoldWriter func(string)

func linesAppender(lines *[]string) scaffoldWriter {
	return func(line string) {
		*lines = append(*lines, line)
	}
}

func renderScaffoldTrail(write scaffoldWriter, trail []ExplainScaffoldNode, depth int) {
	if len(trail) == 0 {
		return
	}

	current := trail[0]
	indent := strings.Repeat("  ", depth)
	write(indent + current.Step.Name + ":")
	omit := scaffoldOmitSet(current.Omit)
	if len(trail) > 1 {
		for field := range scaffoldFocusedOmitFields(current.Node, "ref", trail[1].Step.Name) {
			omit[field] = struct{}{}
		}
		omit[trail[1].Step.Name] = struct{}{}
	}
	if current.Step.Array {
		write(indent + "  - " + firstScaffoldLine(current.Node, omit))
		renderScaffoldObject(write, current.Node, depth+2, omit, true)
	} else {
		renderScaffoldObject(write, current.Node, depth+1, omit, false)
	}
	if len(trail) > 1 {
		renderNestedTrail(write, trail[1:], depth+2)
	}
}

func renderNestedTrail(write scaffoldWriter, trail []ExplainScaffoldNode, depth int) {
	if len(trail) == 0 {
		return
	}

	current := trail[0]
	indent := strings.Repeat("  ", depth)
	omit := scaffoldOmitSet(current.Omit)
	if len(trail) > 1 {
		omit[trail[1].Step.Name] = struct{}{}
	}
	write(indent + current.Step.Name + ":")
	if current.Step.Array {
		write(indent + "  - " + firstScaffoldLine(current.Node, omit))
		renderScaffoldObject(write, current.Node, depth+2, omit, true)
	} else {
		renderScaffoldObject(write, current.Node, depth+1, omit, false)
	}
	if len(trail) > 1 {
		renderNestedTrail(write, trail[1:], depth+2)
	}
}

func firstScaffoldLine(node *ExplainNode, omit map[string]struct{}) string {
	if field := firstScaffoldField(node, omit); field != nil {
		return fmt.Sprintf("%s: %s", field.Name, scaffoldLiteral(field.Node))
	}
	if node.Kind != "object" {
		return scaffoldLiteral(node)
	}
	return "{}"
}

func firstScaffoldField(node *ExplainNode, omit map[string]struct{}) *ExplainField {
	if node.Kind != "object" {
		return nil
	}
	fields := node.Properties
	for _, field := range fields {
		if _, ok := omit[field.Name]; ok {
			continue
		}
		if field.Required || field.Recommended {
			return field
		}
	}
	return nil
}

func renderScaffoldObject(
	write scaffoldWriter,
	node *ExplainNode,
	depth int,
	omit map[string]struct{},
	skipFirst bool,
) {
	skipped := ""
	if skipFirst {
		if field := firstScaffoldField(node, omit); field != nil {
			skipped = field.Name
		}
	}
	for _, field := range node.Properties {
		if _, ok := omit[field.Name]; ok {
			continue
		}
		if skipped != "" && field.Name == skipped {
			continue
		}

		required := field.Required || field.Recommended
		renderScaffoldField(write, depth, field, omit, !required)
	}
}

func renderScaffoldField(write scaffoldWriter, depth int, field *ExplainField, omit map[string]struct{}, comment bool) {
	indent := strings.Repeat("  ", depth)
	commentPrefix := ""
	if comment {
		commentPrefix = "# "
	}

	if field.Node.Kind == explainKindObject && field.Node.Additional == nil && len(field.Node.OneOf) == 0 {
		write(indent + commentPrefix + field.Name + ":")
		renderCommentedBlock(write, depth+1, field.Node, omit, comment)
		return
	}

	if field.Node.Kind == "array" {
		write(indent + commentPrefix + field.Name + ":")
		itemLine := scaffoldLiteral(field.Node.Items)
		if field.Node.Items != nil && field.Node.Items.Kind == explainKindObject {
			write(strings.Repeat("  ", depth+1) + commentPrefix + "- " + firstScaffoldLine(field.Node.Items, omit))
			renderCommentedBlock(write, depth+2, field.Node.Items, omit, comment)
		} else {
			write(strings.Repeat("  ", depth+1) + commentPrefix + "- " + itemLine)
		}
		return
	}

	write(indent + commentPrefix + field.Name + ": " + scaffoldLiteral(field.Node))
}

func renderCommentedBlock(write scaffoldWriter, depth int, node *ExplainNode, omit map[string]struct{}, comment bool) {
	if node == nil || node.Kind != "object" {
		return
	}
	for _, child := range node.Properties {
		if _, ok := omit[child.Name]; ok {
			continue
		}
		renderScaffoldField(write, depth, child, omit, comment || !child.Required && !child.Recommended)
	}
}

func scaffoldLiteral(node *ExplainNode) string {
	if node == nil {
		return "null"
	}
	if node.Literal != "" {
		return node.Literal
	}
	switch node.Kind {
	case "string", "integer", "number", "boolean":
		return ""
	case "array":
		if node.Items != nil {
			return scaffoldLiteral(node.Items)
		}
		return "[]"
	case "object":
		if node.Additional != nil {
			return "{}"
		}
		return "{}"
	default:
		return "value"
	}
}

func (n *ExplainNode) property(name string) (*ExplainField, bool) {
	if n == nil || n.propIndex == nil {
		return nil, false
	}
	field, ok := n.propIndex[name]
	return field, ok
}

func (n *ExplainNode) propertyExists(name string) bool {
	_, ok := n.property(name)
	return ok
}

func (n *ExplainNode) addField(field *ExplainField) {
	if field == nil {
		return
	}
	if n.propIndex == nil {
		n.propIndex = make(map[string]*ExplainField)
	}
	n.Properties = append(n.Properties, field)
	n.propIndex[field.Name] = field
}

func (n *ExplainNode) lookup(path []string) (*ExplainNode, bool) {
	current := n
	for _, segment := range path {
		field, ok := current.property(segment)
		if !ok {
			return nil, false
		}
		current = field.Node
		if current.Kind == "array" && current.Items != nil {
			current = current.Items
		}
	}
	return current, true
}

func (n *ExplainNode) clone() *ExplainNode {
	if n == nil {
		return nil
	}
	cloned := &ExplainNode{
		Kind:         n.Kind,
		Description:  n.Description,
		Default:      n.Default,
		DefaultFrom:  n.DefaultFrom,
		Enum:         append([]any(nil), n.Enum...),
		Const:        n.Const,
		Nullable:     n.Nullable,
		Recommended:  n.Recommended,
		PreferredTag: n.PreferredTag,
		RefKind:      n.RefKind,
		Notes:        append([]string(nil), n.Notes...),
		Literal:      n.Literal,
		Items:        n.Items.clone(),
		Additional:   n.Additional.clone(),
		propIndex:    make(map[string]*ExplainField),
	}
	for _, child := range n.Properties {
		clonedField := &ExplainField{
			Name:        child.Name,
			Node:        child.Node.clone(),
			Required:    child.Required,
			Recommended: child.Recommended,
		}
		cloned.addField(clonedField)
	}
	for _, branch := range n.OneOf {
		cloned.OneOf = append(cloned.OneOf, branch.clone())
	}
	return cloned
}

func (n *ExplainNode) toJSONSchema() *JSONSchema {
	if n == nil {
		return nil
	}

	schema := &JSONSchema{
		Description: n.Description,
		Default:     n.Default,
		Const:       n.Const,
		Enum:        append([]any(nil), n.Enum...),
		XRefKind:    n.RefKind,
		XTag:        n.PreferredTag,
		XDefault:    n.DefaultFrom,
		XNotes:      append([]string(nil), n.Notes...),
	}

	schema.Type = schemaTypeValue(n.Kind, n.Nullable)

	switch n.Kind {
	case "object":
		if n.Additional != nil {
			schema.Additional = n.Additional.toJSONSchema()
		} else {
			schema.Additional = false
		}
		if len(n.Properties) > 0 {
			schema.Properties = make(map[string]*JSONSchema, len(n.Properties))
		}
		for _, field := range n.Properties {
			schema.Properties[field.Name] = field.Node.toJSONSchema()
			if field.Required {
				schema.Required = append(schema.Required, field.Name)
			}
		}
	case "array":
		schema.Items = n.Items.toJSONSchema()
	case "any":
		schema.Type = nil
	}

	for _, branch := range n.OneOf {
		schema.OneOf = append(schema.OneOf, branch.toJSONSchema())
	}

	return schema
}

func schemaTypeValue(kind string, nullable bool) any {
	if kind == "any" || kind == "" {
		if nullable {
			return []string{"null"}
		}
		return nil
	}
	if !nullable {
		return kind
	}
	return []string{kind, "null"}
}

func nestedRelationsFor(target ResourceType) []ExplainRelation {
	var relations []ExplainRelation
	for _, parentType := range RegisteredTypes() {
		parentOps, ok := registry[parentType]
		if !ok || parentOps.explain.typ == nil {
			continue
		}
		for _, relation := range explainNestedRelations(parentType, parentOps.explain.typ) {
			if relation.ChildAlias == string(target) {
				relations = append(relations, relation)
			}
		}
	}
	return relations
}

func explainNestedRelations(parentType ResourceType, parent reflect.Type) []ExplainRelation {
	parent = derefExplainType(parent)
	var relations []ExplainRelation
	for field := range parent.Fields() {
		if !field.IsExported() {
			continue
		}
		name, _, _, skip := explainFieldName(field, "yaml")
		if skip || name == "" {
			continue
		}
		childType, ok := explainRegisteredResourceType(field.Type)
		if !ok {
			continue
		}
		relations = append(relations, ExplainRelation{
			ParentAlias:   string(parentType),
			ParentType:    string(parentType),
			FieldName:     name,
			ChildAlias:    string(childType),
			ParentRootKey: resourceSetRootKey(parent),
		})
	}
	return relations
}

func explainRegisteredResourceType(typ reflect.Type) (ResourceType, bool) {
	typ = derefExplainType(typ)
	if typ.Kind() == reflect.Slice {
		typ = derefExplainType(typ.Elem())
	}
	for rt, ops := range registry {
		if ops.explain.typ == typ {
			return rt, true
		}
	}
	return "", false
}

func resourceSetRootKey(resourceType reflect.Type) string {
	resourceType = derefExplainType(resourceType)
	rsType := reflect.TypeFor[ResourceSet]()
	for field := range rsType.Fields() {
		fieldType := derefExplainType(field.Type)
		if fieldType.Kind() != reflect.Slice {
			continue
		}
		if derefExplainType(fieldType.Elem()) != resourceType {
			continue
		}
		tag := field.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			return ""
		}
		name := strings.Split(tag, ",")[0]
		if name == "-" {
			return ""
		}
		return name
	}
	return ""
}

func explainFieldName(field reflect.StructField, tagName string) (name string, inline bool, omitempty bool, skip bool) {
	tag := field.Tag.Get(tagName)
	if tag == "-" {
		return "", false, false, true
	}
	if field.Anonymous && strings.Contains(tag, "inline") {
		return "", true, false, false
	}
	if tag == "" {
		return "", false, false, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, part := range parts[1:] {
		switch part {
		case "inline":
			inline = true
		case "omitempty":
			omitempty = true
		}
	}
	return name, inline, omitempty, false
}

func derefExplainType(typ reflect.Type) reflect.Type {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

func boolPtr(value bool) *bool {
	return &value
}

func hasBaseResource(rt ResourceType) bool {
	ops, ok := registry[rt]
	if !ok || ops.explain.typ == nil {
		return false
	}
	typ := derefExplainType(ops.explain.typ)
	for field := range typ.Fields() {
		if field.Anonymous && derefExplainType(field.Type) == reflect.TypeFor[BaseResource]() {
			return true
		}
	}
	return false
}

func shouldRecommendField(name string, hints map[string]ExplainFieldHint, path string) bool {
	if hint, ok := hints[path]; ok && hint.Recommended != nil {
		return *hint.Recommended
	}
	if hint, ok := hints[name]; ok && hint.Recommended != nil {
		return *hint.Recommended
	}
	_, ok := recommendedFieldNames[name]
	return ok
}

func explainTypeLabel(node *ExplainNode) string {
	if node == nil {
		return "any"
	}
	switch node.Kind {
	case "array":
		if node.Items == nil {
			return "array"
		}
		return fmt.Sprintf("array[%s]", explainTypeLabel(node.Items))
	case "object":
		if node.Additional != nil {
			return fmt.Sprintf("map[string]%s", explainTypeLabel(node.Additional))
		}
		return "object"
	default:
		if node.Nullable {
			return node.Kind + "|null"
		}
		return node.Kind
	}
}

func scaffoldOmitFields(ancestors []ResourceType) []string {
	var fields []string
	for _, ancestor := range ancestors {
		name := string(ancestor)
		fields = append(fields, name)
		if idx := strings.LastIndex(name, "_"); idx >= 0 {
			fields = append(fields, name[idx+1:])
		}
	}
	return slices.Compact(fields)
}

func scaffoldOmitSet(fields []string) map[string]struct{} {
	set := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		set[field] = struct{}{}
	}
	return set
}

func scaffoldFocusedOmitFields(node *ExplainNode, keep ...string) map[string]struct{} {
	if node == nil || node.Kind != "object" {
		return nil
	}
	keepSet := make(map[string]struct{}, len(keep))
	for _, field := range keep {
		if field == "" {
			continue
		}
		keepSet[field] = struct{}{}
	}
	if _, ok := keepSet["ref"]; !ok && node.propertyExists("ref") {
		keepSet["ref"] = struct{}{}
	}

	omit := make(map[string]struct{}, len(node.Properties))
	for _, field := range node.Properties {
		if _, ok := keepSet[field.Name]; ok {
			continue
		}
		omit[field.Name] = struct{}{}
	}
	return omit
}

func snakeCase(input string) string {
	var b strings.Builder
	for i, r := range input {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

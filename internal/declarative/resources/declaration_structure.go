package resources

import (
	"reflect"
	"strings"
)

type nestedResourceField struct {
	name         string
	resourceType ResourceType
	array        bool
}

// nestedResourceFields discovers declaration containment without assigning
// lifecycle or sync ownership semantics.
func nestedResourceFields(parent reflect.Type) []nestedResourceField {
	var fields []nestedResourceField
	for field := range derefExplainType(parent).Fields() {
		if !field.IsExported() {
			continue
		}
		name, _, _, skip := explainFieldName(field, "yaml")
		if skip || name == "" {
			continue
		}
		kind, ok := registeredResourceType(field.Type)
		if !ok {
			continue
		}
		fields = append(fields, nestedResourceField{
			name:         name,
			resourceType: kind,
			array:        derefExplainType(field.Type).Kind() == reflect.Slice,
		})
	}
	return fields
}

func registeredResourceType(typ reflect.Type) (ResourceType, bool) {
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

// resourceSetDeclarationPath includes grouping objects, but stops at resource
// collections: structural children are not additional root declarations.
func resourceSetDeclarationPath(resourceType reflect.Type) []string {
	visiting := make(map[reflect.Type]bool)
	var find func(reflect.Type) []string
	find = func(container reflect.Type) []string {
		container = derefExplainType(container)
		if visiting[container] {
			return nil
		}
		visiting[container] = true
		defer delete(visiting, container)
		for field := range container.Fields() {
			name, _, _, skip := explainFieldName(field, "yaml")
			if !field.IsExported() || skip || name == "" {
				continue
			}
			typ := derefExplainType(field.Type)
			if typ.Kind() == reflect.Slice {
				if derefExplainType(typ.Elem()) == resourceType {
					return []string{name}
				}
				continue
			}
			if typ.Kind() == reflect.Struct {
				if _, registered := registeredResourceType(typ); !registered {
					if path := find(typ); path != nil {
						return append([]string{name}, path...)
					}
				}
			}
		}
		return nil
	}
	return find(reflect.TypeFor[ResourceSet]())
}

// Maps are a collection shape for scope capture, while explain has its own
// path handling for maps and keeps using nestedResourceFields.
func nestedSyncResourceFields(parent reflect.Type) []nestedResourceField {
	fields := nestedResourceFields(parent)
	for field := range derefExplainType(parent).Fields() {
		name, _, _, skip := explainFieldName(field, "yaml")
		typ := derefExplainType(field.Type)
		if !field.IsExported() || skip || name == "" || typ.Kind() != reflect.Map {
			continue
		}
		if kind, ok := registeredResourceType(typ.Elem()); ok {
			fields = append(fields, nestedResourceField{name: name, resourceType: kind})
		}
	}
	return fields
}

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

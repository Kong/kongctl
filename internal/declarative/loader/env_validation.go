package loader

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag inspection
)

func validateEnvTagStringFields(rawContent []byte) error {
	var node yaml.Node
	if err := yaml.Unmarshal(rawContent, &node); err != nil {
		return err
	}

	return validateEnvNode(&node, reflect.TypeOf(temporaryParseResult{}), nil)
}

func validateEnvNode(node *yaml.Node, targetType reflect.Type, path []string) error {
	if node == nil {
		return nil
	}

	targetType = derefType(targetType)
	if targetType == nil {
		return nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil
		}
		return validateEnvNode(node.Content[0], targetType, path)
	case yaml.MappingNode:
		//exhaustive:ignore
		switch targetType.Kind() {
		case reflect.Struct:
			for i := 0; i+1 < len(node.Content); i += 2 {
				keyNode := node.Content[i]
				valueNode := node.Content[i+1]

				fieldType, ok := lookupStructFieldType(targetType, keyNode.Value)
				if !ok {
					continue
				}

				nextPath := append(path, keyNode.Value)
				if valueNode.Tag == "!env" {
					if !isEnvStringFieldType(fieldType) {
						return fmt.Errorf(
							"!env currently supports string-typed fields only (field: %s)",
							strings.Join(nextPath, "."),
						)
					}
					continue
				}

				if err := validateEnvNode(valueNode, fieldType, nextPath); err != nil {
					return err
				}
			}
		case reflect.Map:
			elemType := derefType(targetType.Elem())
			for i := 0; i+1 < len(node.Content); i += 2 {
				valueNode := node.Content[i+1]
				if valueNode.Tag == "!env" {
					if !isEnvStringFieldType(elemType) {
						return fmt.Errorf(
							"!env currently supports string-typed fields only (field: %s)",
							strings.Join(path, "."),
						)
					}
					continue
				}
				if err := validateEnvNode(valueNode, elemType, path); err != nil {
					return err
				}
			}
		default:
			return nil
		}
	case yaml.SequenceNode:
		if targetType.Kind() != reflect.Slice && targetType.Kind() != reflect.Array {
			return nil
		}

		elemType := derefType(targetType.Elem())
		for _, child := range node.Content {
			if child.Tag == "!env" {
				if !isEnvStringFieldType(elemType) {
					return fmt.Errorf(
						"!env currently supports string-typed fields only (field: %s)",
						strings.Join(path, "."),
					)
				}
				continue
			}
			if err := validateEnvNode(child, elemType, path); err != nil {
				return err
			}
		}
	case yaml.ScalarNode, yaml.AliasNode:
		return nil
	default:
		return nil
	}

	return nil
}

func lookupStructFieldType(targetType reflect.Type, name string) (reflect.Type, bool) {
	targetType = derefType(targetType)
	if targetType == nil || targetType.Kind() != reflect.Struct {
		return nil, false
	}

	if field, ok := targetType.FieldByName(name); ok && field.PkgPath == "" {
		return matchedFieldType(field, name)
	}

	if pascalName := toPascalCase(name); pascalName != name {
		if field, ok := targetType.FieldByName(pascalName); ok && field.PkgPath == "" {
			return matchedFieldType(field, name)
		}
	}

	for i := 0; i < targetType.NumField(); i++ {
		field := targetType.Field(i)
		if field.PkgPath != "" {
			continue
		}

		if matchesStructuredFieldName(field, name) {
			return field.Type, true
		}

		if field.Anonymous || field.Tag.Get("union") == "member" || structuredFieldInline(field) {
			if nestedType, ok := lookupStructFieldType(field.Type, name); ok {
				return nestedType, true
			}
		}
	}

	return nil, false
}

func matchedFieldType(field reflect.StructField, name string) (reflect.Type, bool) {
	if field.Tag.Get("union") == "member" {
		return lookupStructFieldType(field.Type, name)
	}

	return field.Type, true
}

func matchesStructuredFieldName(field reflect.StructField, name string) bool {
	if yamlName, _, yamlSkip := parseStructuredFieldTag(field.Tag.Get("yaml")); !yamlSkip && yamlName == name {
		return true
	}

	if jsonName, _, jsonSkip := parseStructuredFieldTag(field.Tag.Get("json")); !jsonSkip && jsonName == name {
		return true
	}

	return false
}

func structuredFieldInline(field reflect.StructField) bool {
	_, yamlInline, yamlSkip := parseStructuredFieldTag(field.Tag.Get("yaml"))
	if !yamlSkip && yamlInline {
		return true
	}

	_, jsonInline, jsonSkip := parseStructuredFieldTag(field.Tag.Get("json"))
	return !jsonSkip && jsonInline
}

func isEnvStringFieldType(fieldType reflect.Type) bool {
	fieldType = derefType(fieldType)
	return fieldType != nil && fieldType.Kind() == reflect.String
}

func derefType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

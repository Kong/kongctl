package loader

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func (l *Loader) collectDeferredEnvSources(actual, placeholder *resources.ResourceSet) error {
	if actual == nil || placeholder == nil {
		return nil
	}

	return visitResourceSetResources(placeholder, func(resource resources.Resource) error {
		resourceRef := resource.GetRef()
		if tags.IsEnvPlaceholder(resourceRef) {
			return fmt.Errorf("!env tags are not supported on resource refs")
		}

		return walkDeferredEnvPlaceholders(reflect.ValueOf(resource), nil, func(path string, placeholder string) error {
			switch path {
			case "/ref":
				return fmt.Errorf("!env tags are not supported on resource refs")
			case "/kongctl/namespace":
				return fmt.Errorf("!env tags are not supported on kongctl.namespace")
			}

			actual.AddEnvSource(resourceRef, path, placeholder)
			return nil
		})
	})
}

func visitResourceSetResources(rs *resources.ResourceSet, fn func(resources.Resource) error) error {
	if rs == nil {
		return nil
	}

	value := reflect.ValueOf(rs).Elem()
	resourceType := reflect.TypeOf((*resources.Resource)(nil)).Elem()

	for i := range value.NumField() {
		fieldValue := value.Field(i)
		fieldType := value.Type().Field(i)
		if fieldType.PkgPath != "" || fieldType.Tag.Get("yaml") == "-" {
			continue
		}

		switch fieldValue.Kind() {
		case reflect.Slice:
			for j := range fieldValue.Len() {
				item := fieldValue.Index(j)
				if !item.IsValid() {
					continue
				}

				if item.CanAddr() && item.Addr().Type().Implements(resourceType) {
					if err := fn(item.Addr().Interface().(resources.Resource)); err != nil {
						return err
					}
					continue
				}

				if item.Type().Implements(resourceType) {
					if err := fn(item.Interface().(resources.Resource)); err != nil {
						return err
					}
				}
			}
		case reflect.Ptr:
			if fieldValue.IsNil() || !fieldValue.Type().Implements(resourceType) {
				continue
			}
			if err := fn(fieldValue.Interface().(resources.Resource)); err != nil {
				return err
			}
		}
	}

	return nil
}

func walkDeferredEnvPlaceholders(value reflect.Value, path []string, visit func(string, string) error) error {
	if !value.IsValid() {
		return nil
	}

	for value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.String:
		strValue := value.String()
		if tags.IsEnvPlaceholder(strValue) {
			return visit(pointerPath(path), strValue)
		}
		return nil
	case reflect.Struct:
		valueType := value.Type()
		for i := range value.NumField() {
			field := valueType.Field(i)
			if field.PkgPath != "" {
				continue
			}

			fieldName, inline, skip := deferredEnvFieldName(field)
			if skip {
				continue
			}

			nextPath := path
			if !inline && fieldName != "" {
				nextPath = append(nextPath, fieldName)
			}

			if err := walkDeferredEnvPlaceholders(value.Field(i), nextPath, visit); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		for _, key := range value.MapKeys() {
			keyStr := fmt.Sprintf("%v", key.Interface())
			if err := walkDeferredEnvPlaceholders(value.MapIndex(key), append(path, keyStr), visit); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice, reflect.Array:
		for i := range value.Len() {
			if err := walkDeferredEnvPlaceholders(value.Index(i), append(path, strconv.Itoa(i)), visit); err != nil {
				return err
			}
		}
		return nil
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		return walkDeferredEnvPlaceholders(value.Elem(), path, visit)
	default:
		return nil
	}
}

func deferredEnvFieldName(field reflect.StructField) (name string, inline bool, skip bool) {
	if field.Tag.Get("union") == "member" {
		return "", true, false
	}

	if jsonName, jsonInline, jsonSkip := parseStructuredFieldTag(field.Tag.Get("json")); !jsonSkip {
		if jsonInline {
			return "", true, false
		}
		if jsonName != "" {
			return jsonName, false, false
		}
	}

	yamlName, yamlInline, yamlSkip := parseStructuredFieldTag(field.Tag.Get("yaml"))
	if yamlSkip {
		return "", false, true
	}
	if yamlInline {
		return "", true, false
	}
	if yamlName != "" {
		return yamlName, false, false
	}

	if field.Anonymous {
		return "", true, false
	}

	return field.Name, false, false
}

func parseStructuredFieldTag(tag string) (name string, inline bool, skip bool) {
	if tag == "-" {
		return "", false, true
	}
	if tag == "" {
		return "", false, false
	}

	parts := strings.Split(tag, ",")
	if parts[0] != "" {
		name = parts[0]
	}
	for _, part := range parts[1:] {
		if part == "inline" {
			inline = true
		}
	}
	return name, inline, false
}

var jsonPointerEscaper = strings.NewReplacer("~", "~0", "/", "~1")

func pointerPath(segments []string) string {
	if len(segments) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, segment := range segments {
		builder.WriteByte('/')
		builder.WriteString(jsonPointerEscaper.Replace(segment))
	}
	return builder.String()
}

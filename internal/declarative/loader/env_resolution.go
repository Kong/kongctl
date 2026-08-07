package loader

import (
	"fmt"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/secrets"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func resolveDefaultEnvPlaceholders(defaults *resources.FileDefaults) error {
	if defaults == nil {
		return nil
	}
	return resolveEnvPlaceholders(reflect.ValueOf(defaults), nil, func(string) bool { return true })
}

func resolveOrdinaryEnvPlaceholders(rs *resources.ResourceSet) error {
	return visitResourceSetResources(rs, func(resource resources.Resource) error {
		return resolveEnvPlaceholders(reflect.ValueOf(resource), nil, func(path string) bool {
			_, isSecret := secrets.Match(resource.GetType(), path)
			return !isSecret
		})
	})
}

func resolveEnvPlaceholders(value reflect.Value, path []string, shouldResolve func(string) bool) error {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	//exhaustive:ignore
	switch value.Kind() {
	case reflect.String:
		placeholder := value.String()
		if !tags.IsEnvPlaceholder(placeholder) || !shouldResolve(pointerPath(path)) {
			return nil
		}
		resolved, err := tags.ResolveEnvPlaceholder(placeholder)
		if err != nil {
			return err
		}
		if !value.CanSet() {
			return fmt.Errorf("cannot set resolved environment value at %s", pointerPath(path))
		}
		value.SetString(resolved)
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
			if err := resolveEnvPlaceholders(value.Field(i), nextPath, shouldResolve); err != nil {
				return err
			}
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			entry := reflect.New(value.Type().Elem()).Elem()
			entry.Set(value.MapIndex(key))
			if err := resolveEnvPlaceholders(
				entry,
				append(path, fmt.Sprintf("%v", key.Interface())),
				shouldResolve,
			); err != nil {
				return err
			}
			value.SetMapIndex(key, entry)
		}
	case reflect.Slice, reflect.Array:
		for i := range value.Len() {
			if err := resolveEnvPlaceholders(value.Index(i), append(path, fmt.Sprintf("%d", i)), shouldResolve); err != nil {
				return err
			}
		}
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		entry := reflect.New(value.Elem().Type()).Elem()
		entry.Set(value.Elem())
		if err := resolveEnvPlaceholders(entry, path, shouldResolve); err != nil {
			return err
		}
		if value.CanSet() {
			value.Set(entry)
		}
	}
	return nil
}

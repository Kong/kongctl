package loader

import (
	"fmt"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/secrets"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func (l *Loader) collectSecretSources(actual, placeholder *resources.ResourceSet) error {
	if actual == nil || placeholder == nil {
		return nil
	}

	return visitResourceSetResources(placeholder, func(resource resources.Resource) error {
		resourceRef := resource.GetRef()
		return walkConfiguredStrings(reflect.ValueOf(resource), nil, func(path, value string) error {
			capability, supported := secrets.Match(resource.GetType(), path)
			if tags.IsSecretPlaceholder(value) {
				if !supported {
					return fmt.Errorf(
						"resource %s %q field %s is not a reviewed write-only field and cannot use !secret",
						resource.GetType(), resourceRef, path,
					)
				}
				if !capability.Create {
					return fmt.Errorf(
						"resource %s %q field %s does not support secret writes on create",
						resource.GetType(), resourceRef, path,
					)
				}
				expression, err := tags.ParseSecretPlaceholder(value)
				if err != nil {
					return fmt.Errorf("resource %s %q field %s has an invalid !secret declaration: %w",
						resource.GetType(), resourceRef, path, err)
				}
				actual.AddSecretSource(resourceRef, path, expression, false)
				return nil
			}

			if !supported || value == "" {
				return nil
			}
			if tags.IsEnvPlaceholder(value) {
				expression, err := tags.SecretExpressionFromEnvPlaceholder(value)
				if err != nil {
					return fmt.Errorf("resource %s %q field %s has an invalid !env declaration: %w",
						resource.GetType(), resourceRef, path, err)
				}
				actual.AddSecretSource(resourceRef, path, expression, true)
				return nil
			}

			return fmt.Errorf(
				"resource %s %q field %s is write-only and requires !secret with a deferred source",
				resource.GetType(), resourceRef, path,
			)
		})
	})
}

func walkConfiguredStrings(value reflect.Value, path []string, visit func(string, string) error) error {
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
		return visit(pointerPath(path), value.String())
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
			if err := walkConfiguredStrings(value.Field(i), nextPath, visit); err != nil {
				return err
			}
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			if err := walkConfiguredStrings(
				value.MapIndex(key),
				append(path, fmt.Sprintf("%v", key.Interface())),
				visit,
			); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range value.Len() {
			if err := walkConfiguredStrings(value.Index(i), append(path, fmt.Sprintf("%d", i)), visit); err != nil {
				return err
			}
		}
	case reflect.Interface:
		if !value.IsNil() {
			return walkConfiguredStrings(value.Elem(), path, visit)
		}
	}
	return nil
}

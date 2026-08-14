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

	if err := visitResourceSetResources(placeholder, func(resource resources.Resource) error {
		resourceRef := resource.GetRef()
		return walkConfiguredStrings(reflect.ValueOf(resource), nil, func(path, value string) (bool, error) {
			capability, supported := secrets.Match(resource.GetType(), path)
			if tags.IsSecretPlaceholder(value) {
				if !supported {
					return false, fmt.Errorf(
						"resource %s %q field %s is not a reviewed write-only field and cannot use !secret",
						resource.GetType(), resourceRef, path,
					)
				}
				if !capability.Create {
					return false, fmt.Errorf(
						"resource %s %q field %s does not support secret writes on create",
						resource.GetType(), resourceRef, path,
					)
				}
				expression, err := tags.ParseSecretPlaceholder(value)
				if err != nil {
					return false, fmt.Errorf("resource %s %q field %s has an invalid !secret declaration: %w",
						resource.GetType(), resourceRef, path, err)
				}
				actual.AddSecretSource(resourceRef, path, expression, false)
				return true, nil
			}

			if !supported || value == "" {
				return false, nil
			}
			if tags.IsEnvPlaceholder(value) {
				expression, err := tags.SecretExpressionFromEnvPlaceholder(value)
				if err != nil {
					return false, fmt.Errorf("resource %s %q field %s has an invalid !env declaration: %w",
						resource.GetType(), resourceRef, path, err)
				}
				actual.AddSecretSource(resourceRef, path, expression, true)
				return false, nil
			}
			if secrets.IsVaultReference(value) {
				return false, nil
			}

			return false, fmt.Errorf(
				"resource %s %q field %s is write-only and requires !secret with a deferred source",
				resource.GetType(), resourceRef, path,
			)
		})
	}); err != nil {
		return err
	}

	return walkConfiguredStrings(reflect.ValueOf(placeholder), nil, func(path, value string) (bool, error) {
		if tags.IsSecretPlaceholder(value) {
			return false, fmt.Errorf(
				"field %s is not a reviewed write-only field and cannot use !secret",
				path,
			)
		}
		return false, nil
	})
}

func walkConfiguredStrings(
	value reflect.Value,
	path []string,
	visit func(string, string) (bool, error),
) error {
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
		clearValue, err := visit(pointerPath(path), value.String())
		if err != nil || !clearValue {
			return err
		}
		if !value.CanSet() {
			return fmt.Errorf("cannot clear processed !secret placeholder at %s", pointerPath(path))
		}
		value.SetString("")
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
			if err := walkConfiguredStrings(value.Field(i), nextPath, visit); err != nil {
				return err
			}
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			mapValue := reflect.New(value.Type().Elem()).Elem()
			mapValue.Set(value.MapIndex(key))
			if err := walkConfiguredStrings(
				mapValue,
				append(path, fmt.Sprintf("%v", key.Interface())),
				visit,
			); err != nil {
				return err
			}
			value.SetMapIndex(key, mapValue)
		}
	case reflect.Slice, reflect.Array:
		for i := range value.Len() {
			if err := walkConfiguredStrings(value.Index(i), append(path, fmt.Sprintf("%d", i)), visit); err != nil {
				return err
			}
		}
	case reflect.Interface:
		if !value.IsNil() {
			interfaceValue := reflect.New(value.Elem().Type()).Elem()
			interfaceValue.Set(value.Elem())
			if err := walkConfiguredStrings(interfaceValue, path, visit); err != nil {
				return err
			}
			if value.CanSet() {
				value.Set(interfaceValue)
			}
		}
	}
	return nil
}

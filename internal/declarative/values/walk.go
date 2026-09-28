// Package values traverses declarative payload values without interpreting map keys.
package values

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Transform visits string values in maps, lists and SDK structures. Paths use
// JSON Pointer escaping; a numeric segment addresses an index only in a list.
// The callback can return a scalar of a different type in an interface slot.
func Transform(value any, visit func(path, value string) (any, error)) error {
	return transform(reflect.ValueOf(value), "", visit)
}

// Child appends a JSON Pointer segment.
func Child(path, key string) string {
	return path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

func transform(v reflect.Value, path string, visit func(string, string) (any, error)) error {
	if !v.IsValid() {
		return nil
	}
	//exhaustive:ignore
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			return transform(v.Elem(), path, visit)
		}
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		if v.Elem().Kind() == reflect.String && v.CanSet() {
			replacement, err := visit(path, v.Elem().String())
			if err != nil {
				return fmt.Errorf("field %s: %w", path, err)
			}
			if s, ok := replacement.(string); ok && s == v.Elem().String() {
				return nil
			}
			v.Set(reflect.ValueOf(replacement))
			return nil
		}
		editable := reflect.New(v.Elem().Type()).Elem()
		editable.Set(v.Elem())
		if err := transform(editable, path, visit); err != nil {
			return err
		}
		if v.CanSet() {
			v.Set(editable)
		}
	case reflect.String:
		replacement, err := visit(path, v.String())
		if err != nil {
			return fmt.Errorf("field %s: %w", path, err)
		}
		s, ok := replacement.(string)
		if !ok {
			return fmt.Errorf("field %s: reference value is incompatible with string destination", path)
		}
		if v.CanSet() {
			v.SetString(s)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			f := v.Type().Field(i)
			if !f.IsExported() {
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			next := path
			if name != "" || (!f.Anonymous && f.Tag.Get("union") != "member") {
				if name == "" {
					name = f.Name
				}
				next = Child(path, name)
			}
			if err := transform(v.Field(i), next, visit); err != nil {
				return err
			}
		}
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return nil
		}
		keys := v.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(a.String(), b.String()) })
		for _, key := range keys {
			editable := reflect.New(v.Type().Elem()).Elem()
			editable.Set(v.MapIndex(key))
			if err := transform(editable, Child(path, key.String()), visit); err != nil {
				return err
			}
			v.SetMapIndex(key, editable)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			if err := transform(v.Index(i), Child(path, strconv.Itoa(i)), visit); err != nil {
				return err
			}
		}
	default:
	}
	return nil
}

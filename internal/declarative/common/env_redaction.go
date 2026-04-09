package common

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/tags"
)

const DeferredEnvRedactedDisplay = "[redacted from !env]"

func FormatPlanDisplayValue(value any) string {
	return fmt.Sprintf("%v", SanitizeDeferredEnvValue(value))
}

func SanitizeDeferredEnvValue(value any) any {
	sanitized, changed := sanitizeDeferredEnvReflect(reflect.ValueOf(value))
	if !changed {
		return value
	}
	if !sanitized.IsValid() {
		return nil
	}
	return sanitized.Interface()
}

func HasDeferredEnvReference(ref planner.ReferenceInfo) bool {
	return tags.IsEnvPlaceholder(ref.Ref) ||
		slices.ContainsFunc(ref.Refs, tags.IsEnvPlaceholder)
}

func sanitizeDeferredEnvReflect(value reflect.Value) (reflect.Value, bool) {
	if !value.IsValid() {
		return reflect.Value{}, false
	}

	//exhaustive:ignore
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value, false
		}
		return sanitizeDeferredEnvReflect(value.Elem())
	case reflect.Ptr:
		if value.IsNil() {
			return value, false
		}
		sanitized, changed := sanitizeDeferredEnvReflect(value.Elem())
		if !changed {
			return value, false
		}
		ptr := reflect.New(value.Type().Elem())
		if !tags.AssignReflectValue(ptr.Elem(), sanitized) {
			return value, false
		}
		return ptr, true
	case reflect.String:
		if !tags.IsEnvPlaceholder(value.String()) {
			return value, false
		}
		return reflect.ValueOf(DeferredEnvRedactedDisplay).Convert(value.Type()), true
	case reflect.Map:
		copied := reflect.MakeMapWithSize(value.Type(), value.Len())
		changed := false
		iter := value.MapRange()
		for iter.Next() {
			sanitized, childChanged := sanitizeDeferredEnvReflect(iter.Value())
			if childChanged {
				changed = true
				if !tags.SetMapValue(copied, iter.Key(), sanitized) {
					return value, false
				}
				continue
			}
			copied.SetMapIndex(iter.Key(), iter.Value())
		}
		if !changed {
			return value, false
		}
		return copied, true
	case reflect.Slice:
		copied := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		changed := false
		for i := range value.Len() {
			sanitized, childChanged := sanitizeDeferredEnvReflect(value.Index(i))
			if childChanged {
				changed = true
				if !tags.AssignReflectValue(copied.Index(i), sanitized) {
					return value, false
				}
				continue
			}
			copied.Index(i).Set(value.Index(i))
		}
		if !changed {
			return value, false
		}
		return copied, true
	case reflect.Array:
		copied := reflect.New(value.Type()).Elem()
		changed := false
		for i := range value.Len() {
			sanitized, childChanged := sanitizeDeferredEnvReflect(value.Index(i))
			if childChanged {
				changed = true
				if !tags.AssignReflectValue(copied.Index(i), sanitized) {
					return value, false
				}
				continue
			}
			copied.Index(i).Set(value.Index(i))
		}
		if !changed {
			return value, false
		}
		return copied, true
	case reflect.Struct:
		if value.Type() == reflect.TypeOf(planner.FieldChange{}) {
			current := value.Interface().(planner.FieldChange)
			current.Old = SanitizeDeferredEnvValue(current.Old)
			current.New = SanitizeDeferredEnvValue(current.New)
			return reflect.ValueOf(current), true
		}
		copied := reflect.New(value.Type()).Elem()
		copied.Set(value)
		changed := false
		for i := range value.NumField() {
			field := value.Field(i)
			target := copied.Field(i)
			if !target.CanSet() {
				continue
			}
			sanitized, childChanged := sanitizeDeferredEnvReflect(field)
			if !childChanged {
				continue
			}
			changed = true
			if !tags.AssignReflectValue(target, sanitized) {
				return value, false
			}
		}
		if !changed {
			return value, false
		}
		return copied, true
	default:
		return value, false
	}
}

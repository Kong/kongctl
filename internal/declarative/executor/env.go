package executor

import (
	"maps"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
)

func (e *Executor) resolveDeferredEnvPlaceholders(change *planner.PlannedChange) error {
	if change == nil {
		return nil
	}

	fields, err := resolveDeferredEnvValue(change.Fields)
	if err != nil {
		return err
	}
	if resolvedFields, ok := fields.(map[string]any); ok {
		change.Fields = resolvedFields
	}

	for field, refInfo := range change.References {
		resolvedRef, err := resolveDeferredEnvReference(refInfo)
		if err != nil {
			return err
		}
		change.References[field] = resolvedRef
	}

	return nil
}

func resolveDeferredEnvReference(refInfo planner.ReferenceInfo) (planner.ReferenceInfo, error) {
	if refInfo.IsArray {
		if len(refInfo.ResolvedIDs) < len(refInfo.Refs) {
			refInfo.ResolvedIDs = append(refInfo.ResolvedIDs, make([]string, len(refInfo.Refs)-len(refInfo.ResolvedIDs))...)
		}

		for i, ref := range refInfo.Refs {
			if !tags.IsEnvPlaceholder(ref) {
				continue
			}

			resolved, err := tags.ResolveEnvPlaceholder(ref)
			if err != nil {
				return refInfo, err
			}

			refInfo.Refs[i] = resolved
			if util.IsValidUUID(resolved) {
				refInfo.ResolvedIDs[i] = resolved
			} else {
				refInfo.ResolvedIDs[i] = ""
			}
		}

		return refInfo, nil
	}

	if !tags.IsEnvPlaceholder(refInfo.Ref) {
		return refInfo, nil
	}

	resolved, err := tags.ResolveEnvPlaceholder(refInfo.Ref)
	if err != nil {
		return refInfo, err
	}

	refInfo.Ref = resolved
	refInfo.LookupFields = nil
	if util.IsValidUUID(resolved) {
		refInfo.ID = resolved
	} else {
		refInfo.ID = ""
	}

	return refInfo, nil
}

func resolveDeferredEnvValue(value any) (any, error) {
	switch typed := value.(type) {
	case planner.FieldChange:
		resolvedNew, err := resolveDeferredEnvValue(typed.New)
		if err != nil {
			return nil, err
		}
		typed.New = resolvedNew
		return typed, nil
	case map[string]any:
		_, hasOld := typed["old"]
		_, hasNew := typed["new"]
		if hasOld && hasNew {
			resolvedNew, err := resolveDeferredEnvValue(typed["new"])
			if err != nil {
				return nil, err
			}
			copied := maps.Clone(typed)
			copied["new"] = resolvedNew
			return copied, nil
		}
	}

	return resolveDeferredEnvReflect(reflect.ValueOf(value))
}

func resolveDeferredEnvReflect(value reflect.Value) (any, error) {
	if !value.IsValid() {
		return nil, nil
	}

	//exhaustive:ignore
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return nil, nil
		}
		return resolveDeferredEnvReflect(value.Elem())
	case reflect.Ptr:
		if value.IsNil() {
			return value.Interface(), nil
		}
		resolved, err := resolveDeferredEnvReflect(value.Elem())
		if err != nil {
			return nil, err
		}
		ptr := reflect.New(value.Type().Elem())
		if resolved == nil {
			return ptr.Interface(), nil
		}
		if !tags.AssignReflectValue(ptr.Elem(), reflect.ValueOf(resolved)) {
			return value.Interface(), nil
		}
		return ptr.Interface(), nil
	case reflect.String:
		current := value.String()
		if !tags.IsEnvPlaceholder(current) {
			return current, nil
		}
		return tags.ResolveEnvPlaceholder(current)
	case reflect.Map:
		resolved := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			entry, err := resolveDeferredEnvReflect(iter.Value())
			if err != nil {
				return nil, err
			}
			if entry == nil {
				resolved.SetMapIndex(iter.Key(), reflect.Zero(value.Type().Elem()))
				continue
			}
			entryValue := reflect.ValueOf(entry)
			if !tags.SetMapValue(resolved, iter.Key(), entryValue) {
				resolved.SetMapIndex(iter.Key(), iter.Value())
			}
		}
		return resolved.Interface(), nil
	case reflect.Slice:
		resolved := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := range value.Len() {
			entry, err := resolveDeferredEnvReflect(value.Index(i))
			if err != nil {
				return nil, err
			}
			if entry == nil {
				continue
			}
			if !tags.AssignReflectValue(resolved.Index(i), reflect.ValueOf(entry)) {
				resolved.Index(i).Set(value.Index(i))
			}
		}
		return resolved.Interface(), nil
	case reflect.Array:
		resolved := reflect.New(value.Type()).Elem()
		for i := range value.Len() {
			entry, err := resolveDeferredEnvReflect(value.Index(i))
			if err != nil {
				return nil, err
			}
			if entry == nil {
				continue
			}
			if !tags.AssignReflectValue(resolved.Index(i), reflect.ValueOf(entry)) {
				resolved.Index(i).Set(value.Index(i))
			}
		}
		return resolved.Interface(), nil
	case reflect.Struct:
		if value.Type() == reflect.TypeOf(planner.FieldChange{}) {
			current := value.Interface().(planner.FieldChange)
			resolvedNew, err := resolveDeferredEnvValue(current.New)
			if err != nil {
				return nil, err
			}
			current.New = resolvedNew
			return current, nil
		}
		resolved := reflect.New(value.Type()).Elem()
		resolved.Set(value)
		for i := range value.NumField() {
			field := value.Field(i)
			target := resolved.Field(i)
			if !target.CanSet() {
				continue
			}
			entry, err := resolveDeferredEnvReflect(field)
			if err != nil {
				return nil, err
			}
			if entry == nil {
				continue
			}
			if !tags.AssignReflectValue(target, reflect.ValueOf(entry)) {
				target.Set(field)
			}
		}
		return resolved.Interface(), nil
	default:
		return value.Interface(), nil
	}
}

func actualRefForExecution(ref string) (string, error) {
	actualRef := ref
	if tags.IsEnvPlaceholder(actualRef) {
		resolved, err := tags.ResolveEnvPlaceholder(actualRef)
		if err != nil {
			return "", err
		}
		actualRef = resolved
	}
	if tags.IsRefPlaceholder(actualRef) {
		parsedRef, _, ok := tags.ParseRefPlaceholder(actualRef)
		if ok && parsedRef != "" {
			actualRef = parsedRef
		}
	}
	return actualRef, nil
}

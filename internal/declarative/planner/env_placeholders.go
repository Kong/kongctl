package planner

import (
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

var jsonPointerDecoder = strings.NewReplacer("~1", "/", "~0", "~")

const deferredEnvWarningMessage = "" +
	"Contains deferred !env values. Execution resolves them from the current " +
	"environment, so executed values may differ from the plan."

func (p *Planner) addUnresolvedReferenceWarnings(plan *Plan, rs *resources.ResourceSet) {
	if plan == nil {
		return
	}

	for _, change := range plan.Changes {
		var envSources map[string]string
		if rs != nil {
			envSources = rs.GetEnvSources(change.ResourceRef)
		}

		for field, ref := range change.References {
			if ref.ID != "[unknown]" {
				continue
			}
			if referenceFieldUsesDeferredEnv(envSources, field) {
				continue
			}
			plan.AddWarning(change.ID, "Reference "+field+"="+ref.Ref+" will be resolved during execution")
		}
	}
}

func (p *Planner) applyDeferredEnvPlaceholders(plan *Plan, rs *resources.ResourceSet) {
	if plan == nil || rs == nil || !rs.HasEnvSources() {
		return
	}

	for i := range plan.Changes {
		change := &plan.Changes[i]
		envSources := rs.GetEnvSources(change.ResourceRef)
		if len(envSources) == 0 {
			continue
		}

		paths := slices.Sorted(maps.Keys(envSources))

		applied := false
		for _, path := range paths {
			segments := decodeJSONPointer(path)
			if len(segments) == 0 {
				continue
			}

			placeholder := envSources[path]
			if applyDeferredEnvFieldPlaceholder(change.Fields, segments, placeholder) {
				applied = true
			}
			if applyDeferredEnvChangedFieldPlaceholder(change.ChangedFields, segments, placeholder) {
				applied = true
			}
			if applyDeferredEnvReferencePlaceholder(change, segments, placeholder) {
				applied = true
			}
		}

		if applied {
			plan.AddWarning(change.ID, deferredEnvWarningMessage)
		}
	}
}

func referenceFieldUsesDeferredEnv(envSources map[string]string, field string) bool {
	if len(envSources) == 0 {
		return false
	}

	for path := range envSources {
		segments := decodeJSONPointer(path)
		for _, candidate := range referenceFieldCandidates(segments) {
			if candidate == field {
				return true
			}
		}
	}

	return false
}

func applyDeferredEnvFieldPlaceholder(fields map[string]any, segments []string, placeholder string) bool {
	if len(fields) == 0 || len(segments) == 0 {
		return false
	}

	field := segments[0]
	current, ok := fields[field]
	if !ok {
		return false
	}

	updated, changed := applyDeferredEnvPlaceholderValue(current, segments[1:], placeholder)
	if !changed {
		return false
	}

	fields[field] = updated
	return true
}

func applyDeferredEnvChangedFieldPlaceholder(
	changedFields map[string]FieldChange,
	segments []string,
	placeholder string,
) bool {
	if len(changedFields) == 0 || len(segments) == 0 {
		return false
	}

	field := segments[0]
	current, ok := changedFields[field]
	if !ok {
		return false
	}

	updated, changed := applyDeferredEnvPlaceholderValue(current, segments[1:], placeholder)
	if !changed {
		return false
	}

	fieldChange, ok := updated.(FieldChange)
	if !ok {
		return false
	}
	changedFields[field] = fieldChange
	return true
}

func applyDeferredEnvPlaceholderValue(value any, segments []string, placeholder string) (any, bool) {
	if len(segments) == 0 {
		switch typed := value.(type) {
		case FieldChange:
			typed.New = placeholder
			return typed, true
		case map[string]any:
			if isFieldChangeMap(typed) {
				copied := maps.Clone(typed)
				copied["new"] = placeholder
				return copied, true
			}
		}

		return placeholder, true
	}

	switch typed := value.(type) {
	case FieldChange:
		updated, changed := applyDeferredEnvPlaceholderValue(typed.New, segments, placeholder)
		if !changed {
			return value, false
		}
		typed.New = updated
		return typed, true
	case map[string]any:
		if isFieldChangeMap(typed) {
			updated, changed := applyDeferredEnvPlaceholderValue(typed["new"], segments, placeholder)
			if !changed {
				return value, false
			}
			copied := maps.Clone(typed)
			copied["new"] = updated
			return copied, true
		}
	}

	reflected, changed := applyDeferredEnvPlaceholderReflect(reflect.ValueOf(value), segments, placeholder)
	if !changed {
		return value, false
	}
	return reflected.Interface(), true
}

func applyDeferredEnvPlaceholderReflect(
	value reflect.Value,
	segments []string,
	placeholder string,
) (reflect.Value, bool) {
	if !value.IsValid() {
		return reflect.Value{}, false
	}

	//exhaustive:ignore
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value, false
		}
		return applyDeferredEnvPlaceholderReflect(value.Elem(), segments, placeholder)
	case reflect.Ptr:
		if value.IsNil() {
			return value, false
		}
		updated, changed := applyDeferredEnvPlaceholderReflect(value.Elem(), segments, placeholder)
		if !changed {
			return value, false
		}
		ptr := reflect.New(value.Type().Elem())
		if !tags.AssignReflectValue(ptr.Elem(), updated) {
			return value, false
		}
		return ptr, true
	case reflect.Map:
		if len(segments) == 0 || value.Type().Key().Kind() != reflect.String {
			return value, false
		}
		key := reflect.ValueOf(segments[0]).Convert(value.Type().Key())
		current := value.MapIndex(key)
		if !current.IsValid() {
			return value, false
		}
		updated, changed := applyDeferredEnvPlaceholderReflect(current, segments[1:], placeholder)
		if !changed {
			return value, false
		}
		copied := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			copied.SetMapIndex(iter.Key(), iter.Value())
		}
		if !tags.SetMapValue(copied, key, updated) {
			return value, false
		}
		return copied, true
	case reflect.Slice:
		if len(segments) == 0 {
			return value, false
		}
		index, err := strconv.Atoi(segments[0])
		if err != nil || index < 0 || index >= value.Len() {
			return value, false
		}
		updated, changed := applyDeferredEnvPlaceholderReflect(value.Index(index), segments[1:], placeholder)
		if !changed {
			return value, false
		}
		copied := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		reflect.Copy(copied, value)
		if !tags.AssignReflectValue(copied.Index(index), updated) {
			return value, false
		}
		return copied, true
	case reflect.Array:
		if len(segments) == 0 {
			return value, false
		}
		index, err := strconv.Atoi(segments[0])
		if err != nil || index < 0 || index >= value.Len() {
			return value, false
		}
		copied := reflect.New(value.Type()).Elem()
		reflect.Copy(copied, value)
		updated, changed := applyDeferredEnvPlaceholderReflect(copied.Index(index), segments[1:], placeholder)
		if !changed {
			return value, false
		}
		if !tags.AssignReflectValue(copied.Index(index), updated) {
			return value, false
		}
		return copied, true
	case reflect.Struct:
		if len(segments) == 0 {
			return value, false
		}
		fieldIndex, fieldValue, ok := deferredEnvStructField(value, segments[0])
		if !ok {
			return value, false
		}
		updated, changed := applyDeferredEnvPlaceholderReflect(fieldValue, segments[1:], placeholder)
		if !changed {
			return value, false
		}
		copied := reflect.New(value.Type()).Elem()
		copied.Set(value)
		if !tags.AssignReflectValue(copied.Field(fieldIndex), updated) {
			return value, false
		}
		return copied, true
	case reflect.String:
		if len(segments) != 0 {
			return value, false
		}
		return reflect.ValueOf(placeholder).Convert(value.Type()), true
	default:
		return value, false
	}
}

func applyDeferredEnvReferencePlaceholder(change *PlannedChange, segments []string, placeholder string) bool {
	if change == nil || len(change.References) == 0 || len(segments) == 0 {
		return false
	}

	for _, candidate := range referenceFieldCandidates(segments) {
		refInfo, ok := change.References[candidate]
		if !ok {
			continue
		}

		if refInfo.IsArray {
			index, ok := firstNumericSegment(segments[1:])
			if !ok || index < 0 || index >= len(refInfo.Refs) {
				return false
			}
			refInfo.Refs[index] = placeholder
			refInfo.ResolvedIDs = nil
			refInfo.LookupArrays = nil
			change.References[candidate] = refInfo
			return true
		}

		refInfo.Ref = placeholder
		refInfo.ID = ""
		refInfo.LookupFields = nil
		change.References[candidate] = refInfo
		return true
	}

	return false
}

func referenceFieldCandidates(segments []string) []string {
	if len(segments) == 0 {
		return nil
	}

	candidates := make([]string, 0, 2)
	dottedParts := make([]string, 0, len(segments))
	for _, segment := range segments {
		if _, err := strconv.Atoi(segment); err == nil {
			continue
		}
		dottedParts = append(dottedParts, segment)
	}
	if len(dottedParts) > 1 {
		candidates = append(candidates, strings.Join(dottedParts, "."))
	}
	candidates = append(candidates, segments[0])
	return uniqueDeferredEnvStrings(candidates)
}

func decodeJSONPointer(path string) []string {
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return nil
	}

	parts := strings.Split(trimmed, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		segments = append(segments, jsonPointerDecoder.Replace(part))
	}
	return segments
}

func firstNumericSegment(segments []string) (int, bool) {
	for _, segment := range segments {
		index, err := strconv.Atoi(segment)
		if err == nil {
			return index, true
		}
	}
	return 0, false
}

func uniqueDeferredEnvStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func deferredEnvStructField(value reflect.Value, segment string) (int, reflect.Value, bool) {
	valueType := value.Type()
	for i := range value.NumField() {
		field := valueType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, skip := deferredEnvStructFieldName(field)
		if skip || name != segment {
			continue
		}
		return i, value.Field(i), true
	}
	return 0, reflect.Value{}, false
}

func deferredEnvStructFieldName(field reflect.StructField) (string, bool) {
	if name, ok := parsedStructFieldName(field.Tag.Get("json")); ok {
		return name, false
	}
	if name, ok := parsedStructFieldName(field.Tag.Get("yaml")); ok {
		return name, false
	}
	if field.Anonymous {
		return "", true
	}
	return field.Name, false
}

func parsedStructFieldName(tag string) (string, bool) {
	if tag == "" || tag == "-" {
		return "", false
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return "", false
	}
	return name, true
}

func isFieldChangeMap(value map[string]any) bool {
	_, hasOld := value["old"]
	_, hasNew := value["new"]
	return hasOld && hasNew
}

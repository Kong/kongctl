package resources

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/values"
)

// Discover top-level label maps through SDK embeddings and union variants.
// Ordinary fields such as config and child collections are separate payloads.
func labelFieldPaths(typ reflect.Type) [][]int {
	var paths [][]int
	for i := range typ.NumField() {
		field := typ.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "labels" {
			if field.Type != reflect.TypeFor[map[string]string]() &&
				field.Type != reflect.TypeFor[map[string]*string]() {
				panic("unsupported label map type: " + field.Type.String())
			}
			paths = append(paths, []int{i})
			continue
		}
		if !field.Anonymous && field.Tag.Get("union") != "member" {
			continue
		}
		childType := field.Type
		if childType.Kind() == reflect.Pointer {
			childType = childType.Elem()
		}
		if childType.Kind() == reflect.Struct {
			for _, path := range labelFieldPaths(childType) {
				paths = append(paths, append([]int{i}, path...))
			}
		}
	}
	return paths
}

func labelChildFields(typ reflect.Type) []int {
	var fields []int
	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Struct &&
			reflect.PointerTo(field.Type.Elem()).Implements(reflect.TypeFor[Resource]()) {
			fields = append(fields, i)
		}
	}
	return fields
}

// ValidateLabels applies Konnect label rules to all registered declarations.
// Call after nested extraction and defaulting so roots have effective metadata.
func (rs *ResourceSet) ValidateLabels() error {
	if rs == nil {
		return nil
	}
	kinds := RegisteredTypes()
	slices.Sort(kinds)
	for _, kind := range kinds {
		ops := registry[kind]
		var validationErr error
		ops.forEach(rs, func(resource Resource) bool {
			validationErr = rs.validateResourceLabels(resource, resource.GetRef(), "")
			return validationErr == nil
		})
		if validationErr != nil {
			return validationErr
		}
	}
	return nil
}

func (rs *ResourceSet) validateResourceLabels(resource Resource, sourceRef, sourcePath string) error {
	if external, ok := resource.(interface{ IsExternal() bool }); ok && external.IsExternal() {
		return nil
	}
	ops := registry[resource.GetType()]
	maxCount := labels.MaxCount
	if ops.namespace != nil {
		participant := ops.namespace.participant(resource)
		maxCount-- // Namespace is always sent on managed parents.
		if (*participant.Meta).IsProtected() {
			maxCount--
		}
	}
	value := reflect.ValueOf(resource).Elem()
	for _, path := range ops.labelFields {
		field, err := value.FieldByIndexErr(path)
		if err != nil { // Inactive SDK union variant.
			continue
		}
		var values map[string]string
		switch value := field.Interface().(type) {
		case map[string]string:
			values = value
		case map[string]*string:
			values = labels.NormalizeLabels(value)
		}
		if ops.namespace != nil {
			values = labels.GetUserLabels(values)
		}
		deferred := make(map[string]bool)
		for key, value := range values {
			path := sourcePath + "/labels/" + escapeJSONPointerToken(key)
			if rs.GetEnvSources(sourceRef)[path] != "" || rs.LiteralSources[sourceRef][path] != "" {
				continue
			}
			_, _, reference := tags.ParseRefPlaceholder(value)
			_, lookup := tags.ParseExternalPlaceholder(value)
			deferred[key] = reference || lookup
		}
		if err := labels.ValidateLabelsWithDeferredValues(values, maxCount, deferred); err != nil {
			return fmt.Errorf("invalid %s %q labels: %w", resource.GetType(), resource.GetRef(), err)
		}
	}
	// Some children, including Event Gateway policies, remain nested after load.
	for _, index := range ops.labelChildren {
		children := value.Field(index)
		name, _, _ := strings.Cut(value.Type().Field(index).Tag.Get("json"), ",")
		childPath := values.Child(sourcePath, name)
		for i := range children.Len() {
			if err := rs.validateResourceLabels(children.Index(i).Addr().Interface().(Resource),
				sourceRef, values.Child(childPath, strconv.Itoa(i))); err != nil {
				return err
			}
		}
	}
	return nil
}

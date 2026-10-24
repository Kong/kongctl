package resources

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/require"
)

func TestLabelDiscoveryMatchesEveryResourceSchema(t *testing.T) {
	schema, err := RenderLoadSchema(LoadSchemaProfileShape)
	require.NoError(t, err)
	var hasLabels func(*JSONSchema) bool
	hasLabels = func(node *JSONSchema) bool {
		if node.Properties["labels"] != nil {
			return true
		}
		return slices.ContainsFunc(node.OneOf, hasLabels)
	}
	for _, kind := range RegisteredTypes() {
		if hasLabels(schema.Defs[string(kind)]) {
			require.NotEmpty(t, registry[kind].labelFields, "label validation missing for %s", kind)
		}
	}
}

// Exercise every concrete label map, including all SDK union variants, through
// the same resource-set validation pass used by the loader.
func TestRegisteredResourceLabels(t *testing.T) {
	for _, kind := range RegisteredTypes() {
		ops := registry[kind]
		for i, path := range ops.labelFields {
			t.Run(fmt.Sprintf("%s/%d", kind, i), func(t *testing.T) {
				resource := reflect.New(ops.explain.typ).Interface().(Resource)
				field := labelFieldForTest(resource, path)
				setTestLabels(field, map[string]string{"owner": "has space"})
				rs := &ResourceSet{}
				appendResourceForLabelTest(t, rs, resource)
				require.ErrorContains(t, rs.ValidateLabels(), "owner")
				setTestLabels(field, map[string]string{"owner": "Platform-1"})
				rs = &ResourceSet{}
				appendResourceForLabelTest(t, rs, resource)
				require.NoError(t, rs.ValidateLabels())

				maxCount := labels.MaxCount
				if ops.namespace != nil {
					maxCount--
				}
				for _, count := range []int{maxCount, maxCount + 1} {
					values := make(map[string]string, count)
					for j := range count {
						values[fmt.Sprintf("label%d", j)] = "value"
					}
					setTestLabels(field, values)
					rs = &ResourceSet{}
					appendResourceForLabelTest(t, rs, resource)
					if count == maxCount {
						require.NoError(t, rs.ValidateLabels())
					} else {
						require.ErrorContains(t, rs.ValidateLabels(), "too many labels")
					}
				}
			})
		}
	}
}

func TestProtectedResourceLabelBudgets(t *testing.T) {
	for _, kind := range RegisteredTypes() {
		ops := registry[kind]
		if ops.namespace == nil || len(ops.labelFields) == 0 {
			continue
		}
		t.Run(string(kind), func(t *testing.T) {
			resource := reflect.New(ops.explain.typ).Interface().(Resource)
			field := labelFieldForTest(resource, ops.labelFields[0])
			participant := ops.namespace.participant(resource)
			protected := true
			*participant.Meta = &KongctlMeta{Protected: &protected}
			values := make(map[string]string)
			for i := range 48 {
				values[fmt.Sprintf("label%d", i)] = "value"
			}
			setTestLabels(field, values)
			rs := &ResourceSet{}
			appendResourceForLabelTest(t, rs, resource)
			require.NoError(t, rs.ValidateLabels())
			values["extra"] = "value"
			setTestLabels(field, values)
			rs = &ResourceSet{}
			appendResourceForLabelTest(t, rs, resource)
			require.ErrorContains(t, rs.ValidateLabels(), "maximum is 48")
		})
	}
}

func labelFieldForTest(resource Resource, path []int) reflect.Value {
	value := reflect.ValueOf(resource)
	for _, index := range path {
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				value.Set(reflect.New(value.Type().Elem()))
			}
			value = value.Elem()
		}
		value = value.Field(index)
	}
	return value
}

func setTestLabels(field reflect.Value, values map[string]string) {
	if field.Type() == reflect.TypeFor[map[string]*string]() {
		field.Set(reflect.ValueOf(labels.DenormalizeLabels(values)))
	} else {
		field.Set(reflect.ValueOf(values))
	}
}

func appendResourceForLabelTest(t *testing.T, rs *ResourceSet, resource Resource) {
	t.Helper()
	value := reflect.ValueOf(rs).Elem()
	resourceType := reflect.TypeOf(resource).Elem()
	for _, field := range value.Fields() {
		if field.CanSet() && field.Kind() == reflect.Slice && field.Type().Elem() == resourceType {
			field.Set(reflect.Append(field, reflect.ValueOf(resource).Elem()))
			return
		}
	}
	t.Fatalf("no flattened storage for %s", resource.GetType())
}

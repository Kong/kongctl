package resources

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/declarative/values"
)

// IsRelationshipPath identifies declared relationships, whose established
// parent/ref/name semantics are handled separately from arbitrary values.
func IsRelationshipPath(resource Resource, path string) bool {
	// AI Gateway name-based associations predate relationship descriptors.
	//exhaustive:ignore
	switch resource.GetType() {
	case ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayConsumer, ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayModel, ResourceTypeAIGatewayMCPServer:
		if strings.HasPrefix(path, "/"+aiGatewayAgentFieldPolicies+"/") {
			return true
		}
	}
	if resource.GetType() == ResourceTypeAIGatewayConsumerGroup &&
		strings.HasPrefix(path, "/"+aiGatewayConsumerGroupFieldConsumers+"/") {
		return true
	}
	for _, descriptor := range RelationshipDescriptorsFor(resource) {
		prefix := "/" + strings.ReplaceAll(descriptor.FieldPath, ".", "/")
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

// ReferenceTarget finds a declaration without choosing arbitrarily between
// declarations that share a ref in different scopes or resource types.
func (rs *ResourceSet) ReferenceTarget(ref string) (Resource, error) {
	var found Resource
	for _, resource := range rs.AllResources() {
		if resource.GetRef() != ref {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("ambiguous reference target %q", ref)
		}
		found = resource
	}
	if found == nil {
		return nil, fmt.Errorf("resource not found: %s", ref)
	}
	return found, nil
}

// ResolvePayloadReference resolves configuration-known scalar selectors and
// known identities. An unknown identity remains an expression until execution.
func (rs *ResourceSet) ResolvePayloadReference(expression string) (any, error) {
	return rs.resolvePayloadReference(expression, nil)
}

func (rs *ResourceSet) resolvePayloadReference(expression string, stack []string) (any, error) {
	ref, selector, ok := tags.ParseRefPlaceholder(expression)
	if !ok || ref == "" || selector == "" {
		return nil, fmt.Errorf("invalid reference expression")
	}
	if slices.Contains(stack, expression) {
		return nil, fmt.Errorf("circular reference involving %q selector %q", ref, selector)
	}
	target, err := rs.ReferenceTarget(ref)
	if err != nil {
		return nil, err
	}
	if selector == "id" || selector == "ID" {
		if id := target.GetKonnectID(); id != "" && id != UnknownReferenceID {
			return id, nil
		}
		return expression, nil
	}
	path := referenceSelectorPointer(target, selector)
	if _, secret := rs.GetSecretSources(ref)[path]; secret {
		return nil, fmt.Errorf("reference %q: selector %q selects a write-only secret", ref, selector)
	}
	v := reflect.ValueOf(target)
	for part := range strings.SplitSeq(selector, ".") {
		v = referenceField(v, part)
		if !v.IsValid() {
			return nil, fmt.Errorf("reference %q: unknown or unavailable selector %q", ref, selector)
		}
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil, fmt.Errorf("reference %q: selector %q is unset", ref, selector)
		}
		v = v.Elem()
	}
	//exhaustive:ignore
	switch v.Kind() {
	case reflect.String:
		if rs.GetEnvSources(ref)[path] != "" || rs.LiteralSources[ref][path] != "" {
			return v.String(), nil
		}
		if tags.IsSecretPlaceholder(v.String()) {
			return nil, fmt.Errorf("reference %q: selector %q selects a write-only secret", ref, selector)
		}
		if tags.IsRefPlaceholder(v.String()) {
			return rs.resolvePayloadReference(v.String(), append(stack, expression))
		}
		return v.String(), nil
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return v.Interface(), nil
	default:
		return nil, fmt.Errorf("reference %q: selector %q must select a scalar value", ref, selector)
	}
}

func referenceSelectorPointer(target Resource, selector string) string {
	path := ""
	v := reflect.ValueOf(target)
	for part := range strings.SplitSeq(selector, ".") {
		var name string
		v, name = referenceFieldName(v, part)
		path = values.Child(path, name)
	}
	return path
}

// PayloadReferenceEnvSource preserves deferred environment provenance when a
// configuration-known field is copied to another payload destination.
func (rs *ResourceSet) PayloadReferenceEnvSource(expression string) string {
	return rs.payloadReferenceSource(expression, rs.EnvSources)
}

// PayloadReferenceLiteralSource finds literal provenance through selector chains.
func (rs *ResourceSet) PayloadReferenceLiteralSource(expression string) string {
	return rs.payloadReferenceSource(expression, rs.LiteralSources)
}

func (rs *ResourceSet) payloadReferenceSource(expression string, sources map[string]map[string]string) string {
	seen := make(map[string]bool)
	for !seen[expression] {
		seen[expression] = true
		ref, selector, ok := tags.ParseRefPlaceholder(expression)
		if !ok {
			return ""
		}
		target, err := rs.ReferenceTarget(ref)
		if err != nil {
			return ""
		}
		if source := sources[ref][referenceSelectorPointer(target, selector)]; source != "" {
			return source
		}
		v := reflect.ValueOf(target)
		for part := range strings.SplitSeq(selector, ".") {
			v = referenceField(v, part)
		}
		for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
			v = v.Elem()
		}
		if !v.IsValid() || v.Kind() != reflect.String {
			return ""
		}
		expression = v.String()
	}
	return ""
}

// AddLiteralSource records an explicitly literal value at a JSON Pointer path.
func (rs *ResourceSet) AddLiteralSource(ref, path, value string) {
	if rs.LiteralSources == nil {
		rs.LiteralSources = make(map[string]map[string]string)
	}
	if rs.LiteralSources[ref] == nil {
		rs.LiteralSources[ref] = make(map[string]string)
	}
	rs.LiteralSources[ref][path] = value
}

func referenceField(v reflect.Value, name string) reflect.Value {
	result, _ := referenceFieldName(v, name)
	return result
}

func referenceFieldName(v reflect.Value, name string) (reflect.Value, string) {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return reflect.Value{}, name
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return reflect.Value{}, name
	}
	if v.Kind() == reflect.Map && v.Type().Key().Kind() == reflect.String {
		return v.MapIndex(reflect.ValueOf(name).Convert(v.Type().Key())), name
	}
	if v.Kind() != reflect.Struct {
		return reflect.Value{}, name
	}
	for i := range v.NumField() {
		f := v.Type().Field(i)
		tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if !f.IsExported() || tag == "-" {
			continue
		}
		if tag == name || f.Name == name {
			if tag == "" {
				tag = f.Name
			}
			return v.Field(i), tag
		}
		if (f.Anonymous || f.Tag.Get("union") == "member") && tag == "" {
			if child, childName := referenceFieldName(v.Field(i), name); child.IsValid() {
				return child, childName
			}
		}
	}
	return reflect.Value{}, name
}

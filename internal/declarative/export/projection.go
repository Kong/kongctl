package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/secrets"
)

// Shape describes a declaration's registered document location and parent link.
type Shape struct {
	Collection  []string
	ParentField string
	Resource    resources.Resource
}

// DeclarationShape uses the registry and manifest schema rather than a second
// resource inventory. Identity's grouped document hierarchy is retained.
func DeclarationShape(kind resources.ResourceType) (Shape, error) {
	resource, err := resources.NewDeclaration(kind)
	if err != nil {
		return Shape{}, err
	}
	element := reflect.TypeOf(resource).Elem()
	collection := declarationPath(reflect.TypeFor[resources.ResourceSet](), element)
	if len(collection) == 0 {
		return Shape{}, fmt.Errorf("no declaration collection for %s", kind)
	}
	parentField := ""
	for _, relationship := range resources.RelationshipDescriptorsForType(kind) {
		if relationship.Kind == resources.RelationshipKindKongctlParentSelector && relationship.RootOnly {
			parentField = relationship.FieldPath
			break
		}
	}

	return Shape{Collection: collection, ParentField: parentField, Resource: resource}, nil
}

func declarationPath(container, element reflect.Type) []string {
	for field := range container.Fields() {
		key, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if key == "" || key == "-" {
			continue
		}
		if field.Type.Kind() == reflect.Slice && field.Type.Elem() == element {
			return []string{key}
		}
		if field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct {
			if path := declarationPath(field.Type.Elem(), element); path != nil {
				return append([]string{key}, path...)
			}
		}
	}
	return nil
}

// Project decodes public API fields into the registered declaration, excluding
// server metadata while retaining declaration fields. Callers choose whether
// to omit secrets according to their export contract.
func Project(kind resources.ResourceType, payload any) (Shape, error) {
	shape, err := DeclarationShape(kind)
	if err != nil {
		return shape, err
	}
	fields, err := PublicFields(payload)
	if err != nil {
		return shape, err
	}
	for _, key := range []string{
		resources.SchemaFieldID, resources.SchemaFieldCreatedAt,
		resources.SchemaFieldUpdatedAt, resources.SchemaFieldManagedBy, resources.SchemaFieldKonnectManaged,
	} {
		delete(fields, key)
	}
	if kind == resources.ResourceTypeAIGatewayModel {
		resources.OmitAIGatewayModelDatastoreConnections(fields)
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return shape, fmt.Errorf("encode declarative resource: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(shape.Resource); err != nil {
		return shape, fmt.Errorf("map declarative resource: %w", err)
	}
	return shape, nil
}

func PublicFields(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("cannot encode declarative resource snapshot")
	}
	var fields map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&fields); err != nil {
		return nil, fmt.Errorf("cannot decode declarative resource snapshot")
	}
	return fields, nil
}

// The reviewed write-only catalog is shared with loading and execution. Remove
// values even if a backend happens to return them; retain public array siblings.
func OmitSecrets(kind resources.ResourceType, value any, path string) {
	switch node := value.(type) {
	case map[string]any:
		for key, field := range node {
			pointer := path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
			if _, secret := secrets.Match(kind, pointer); secret {
				delete(node, key)
				continue
			}
			OmitSecrets(kind, field, pointer)
		}
	case []any:
		for i, field := range node {
			OmitSecrets(kind, field, path+"/"+strconv.Itoa(i))
		}
	}
}

// ProjectAs is the typed entry point for dump collectors. Adoption uses Project
// when the selected resource type is known at runtime.
func ProjectAs[R any, P interface {
	*R
	resources.Resource
}](payload any) (R, error) {
	var zero R
	shape, err := Project(P(new(R)).GetType(), payload)
	if err != nil {
		return zero, err
	}
	resource, ok := shape.Resource.(P)
	if !ok {
		return zero, fmt.Errorf("registered declaration has unexpected type %T", shape.Resource)
	}
	return *resource, nil
}

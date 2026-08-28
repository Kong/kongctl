package resources

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/maturity"
)

// ResourceRegistry provides a central lookup for resource type metadata and iteration.
// This enables adding new resources without modifying switch statements across the codebase.
// Adding a new resource:
//  1. Define the resource struct and embed BaseResource
//  2. Add init() with registerResourceType() in the resource file
//  3. Add the field to ResourceSet
//     For example:
//       APIs []APIResource `yaml:"apis,omitempty" json:"apis,omitempty"`

// resourceOps provides operations for a specific resource type within a ResourceSet.
type resourceOps struct {
	get                       func(rs *ResourceSet) []Resource
	append                    func(dest, src *ResourceSet)
	forEach                   func(rs *ResourceSet, fn func(Resource) bool) bool
	count                     func(rs *ResourceSet) int
	explain                   ExplainRegistration
	dumpDefaultRules          map[string]dumpDefaultRule
	maturity                  *maturity.Metadata
	operationMaturity         map[Operation]maturity.Metadata
	external                  *ExternalResolutionRegistration
	materializeExternal       func(rs *ResourceSet, ref, id, parentRef string) (Resource, error)
	externalUnsupportedReason string
}

// ExternalResolutionRegistration describes the selectors and scope supported
// by a resource type's external lookup adapter.
type ExternalResolutionRegistration struct {
	Selectors              []string
	ParentType             ResourceType
	ParentFieldPath        string
	AllowAnyStringSelector bool
}

// WithExternalUnsupportedReason records why a relationship target cannot yet
// participate in external resolution. Relationship contract tests require
// either this option or an external capability registration for every target.
func WithExternalUnsupportedReason(reason string) ResourceRegistrationOption {
	return func(ops *resourceOps) error {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			return fmt.Errorf("external unsupported reason cannot be empty")
		}
		if ops.external != nil {
			return fmt.Errorf("external capability and unsupported reason are mutually exclusive")
		}
		ops.externalUnsupportedReason = reason
		return nil
	}
}

// registry maps resource types to their operations.
// Registered via init() in each resource file.
var registry = make(map[ResourceType]resourceOps)

// registerResourceType registers a resource type with a single slice pointer function.
// All resource operations are derived automatically.
//
// Type parameters:
//   - R: the concrete resource struct type (e.g., PortalResource)
//   - RPtr: pointer-to-R that implements Resource interface (e.g., *PortalResource)
//
// Usage:
//
//	func init() {
//	    registerResourceType(ResourceTypePortal, func(rs *ResourceSet) *[]PortalResource {
//	        return &rs.Portals
//	    })
//	}
func registerResourceType[R any, RPtr interface {
	*R
	Resource
}](rt ResourceType,
	getSlicePtr func(*ResourceSet) *[]R,
	explain ExplainRegistration,
	options ...ResourceRegistrationOption,
) {
	registerResourceTypeWithSliceAccessors[R, RPtr](rt, getSlicePtr, getSlicePtr, explain, options...)
}

func registerExternalResourceType[R any, RPtr interface {
	*R
	ExternallyResolvableResource
}](rt ResourceType,
	getSlicePtr func(*ResourceSet) *[]R,
	explain ExplainRegistration,
	external ExternalResolutionRegistration,
	options ...ResourceRegistrationOption,
) {
	registerResourceTypeWithSliceAccessors[R, RPtr](rt, getSlicePtr, getSlicePtr, explain, options...)
	ops := registry[rt]
	if ops.externalUnsupportedReason != "" {
		panic("register resource type " + string(rt) + ": external capability and unsupported reason are mutually exclusive")
	}
	ops.external = &external
	ops.materializeExternal = externalMaterializer[R, RPtr](rt, getSlicePtr, external)
	registry[rt] = ops
}

func registerExternalResourceTypeWithSliceAccessors[R any, RPtr interface {
	*R
	ExternallyResolvableResource
}](rt ResourceType,
	getSlicePtr func(*ResourceSet) *[]R,
	ensureSlicePtr func(*ResourceSet) *[]R,
	explain ExplainRegistration,
	external ExternalResolutionRegistration,
	options ...ResourceRegistrationOption,
) {
	registerResourceTypeWithSliceAccessors[R, RPtr](rt, getSlicePtr, ensureSlicePtr, explain, options...)
	ops := registry[rt]
	if ops.externalUnsupportedReason != "" {
		panic("register resource type " + string(rt) + ": external capability and unsupported reason are mutually exclusive")
	}
	ops.external = &external
	ops.materializeExternal = externalMaterializer[R, RPtr](rt, ensureSlicePtr, external)
	registry[rt] = ops
}

func externalMaterializer[R any, RPtr interface {
	*R
	ExternallyResolvableResource
}](
	rt ResourceType,
	ensureSlicePtr func(*ResourceSet) *[]R,
	external ExternalResolutionRegistration,
) func(*ResourceSet, string, string, string) (Resource, error) {
	return func(rs *ResourceSet, ref, id, parentRef string) (Resource, error) {
		if rs == nil {
			return nil, fmt.Errorf("cannot materialize external %s in a nil resource set", rt)
		}
		if strings.TrimSpace(ref) == "" || strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("cannot materialize external %s without ref and ID", rt)
		}
		if existing, ok := rs.GetResourceByRef(ref); ok {
			if existing.GetType() != rt {
				return nil, fmt.Errorf("external ref %q is already used by %s, expected %s", ref, existing.GetType(), rt)
			}
			if existing.GetKonnectID() != "" && existing.GetKonnectID() != id {
				return nil, fmt.Errorf(
					"external %s %q has Konnect ID %q, expected %q",
					rt,
					ref,
					existing.GetKonnectID(),
					id,
				)
			}
			return existing, nil
		}

		var value R
		resource := RPtr(&value)
		if err := setMaterializedField(resource, SchemaFieldRef, ref); err != nil {
			return nil, fmt.Errorf("materialize external %s ref: %w", rt, err)
		}
		if err := setMaterializedExternalBlock(resource, &ExternalBlock{ID: id}); err != nil {
			return nil, fmt.Errorf("materialize external %s block: %w", rt, err)
		}
		if external.ParentType != "" {
			if external.ParentFieldPath == "" {
				return nil, fmt.Errorf("external %s registration has parent type %s without parent field", rt, external.ParentType)
			}
			if strings.TrimSpace(parentRef) == "" {
				return nil, fmt.Errorf("cannot materialize external %s without %s parent ref", rt, external.ParentType)
			}
			if err := setMaterializedField(resource, external.ParentFieldPath, parentRef); err != nil {
				return nil, fmt.Errorf("materialize external %s parent: %w", rt, err)
			}
		}
		resource.SetKonnectID(id)
		slice := ensureSlicePtr(rs)
		*slice = append(*slice, value)
		materialized := RPtr(&(*slice)[len(*slice)-1])
		return materialized, nil
	}
}

func setMaterializedExternalBlock(resource Resource, external *ExternalBlock) error {
	field, err := materializedFieldByPath(resource, "_external")
	if err != nil {
		return err
	}
	if field.Type() != reflect.TypeFor[*ExternalBlock]() || !field.CanSet() {
		return fmt.Errorf("field _external is not a settable external block")
	}
	field.Set(reflect.ValueOf(external))
	return nil
}

func setMaterializedField(resource Resource, path, value string) error {
	field, err := materializedFieldByPath(resource, path)
	if err != nil {
		return err
	}
	if field.Kind() != reflect.String || !field.CanSet() {
		return fmt.Errorf("field %s is not a settable string", path)
	}
	field.SetString(value)
	return nil
}

func materializedFieldByPath(resource Resource, path string) (reflect.Value, error) {
	current := reflect.ValueOf(resource)
	for part := range strings.SplitSeq(path, ".") {
		for current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return reflect.Value{}, fmt.Errorf("field %s is nil", path)
			}
			current = current.Elem()
		}
		current = materializedTaggedField(current, part)
		if !current.IsValid() {
			return reflect.Value{}, fmt.Errorf("field %s not found", path)
		}
	}
	return current, nil
}

func materializedTaggedField(value reflect.Value, name string) reflect.Value {
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	typ := value.Type()
	for i := range value.NumField() {
		field := typ.Field(i)
		for _, tagName := range []string{"yaml", "json"} {
			tag, _, _ := strings.Cut(field.Tag.Get(tagName), ",")
			if tag == name {
				return value.Field(i)
			}
		}
	}
	for i := range value.NumField() {
		if !typ.Field(i).Anonymous {
			continue
		}
		field := value.Field(i)
		for field.Kind() == reflect.Pointer && !field.IsNil() {
			field = field.Elem()
		}
		if result := materializedTaggedField(field, name); result.IsValid() {
			return result
		}
	}
	return reflect.Value{}
}

func registerResourceTypeWithSliceAccessors[R any, RPtr interface {
	*R
	Resource
}](rt ResourceType,
	getSlicePtr func(*ResourceSet) *[]R,
	ensureSlicePtr func(*ResourceSet) *[]R,
	explain ExplainRegistration,
	options ...ResourceRegistrationOption,
) {
	ops := resourceOps{
		get: func(rs *ResourceSet) []Resource {
			slicePtr := getSlicePtr(rs)
			if slicePtr == nil {
				return nil
			}
			return sliceToResources[R, RPtr](*slicePtr)
		},
		append: func(dest, src *ResourceSet) {
			srcPtr := getSlicePtr(src)
			if srcPtr == nil || len(*srcPtr) == 0 {
				return
			}
			destPtr := ensureSlicePtr(dest)
			*destPtr = append(*destPtr, *srcPtr...)
		},
		forEach: func(rs *ResourceSet, fn func(Resource) bool) bool {
			slicePtr := getSlicePtr(rs)
			if slicePtr == nil {
				return true
			}
			slice := *slicePtr // e.g., rs.Portals (the actual slice, not pointer)
			for i := range slice {
				// Get pointer to element and convert to Resource interface
				// This avoids allocating a new slice of Resource and allows direct iteration.
				// Explanation:
				// eg; For slice []PortalResource => slice[i] is a "PortalResource"
				// Thus, &slice[i] is a pointer to PortalResource ("*PortalResource")
				//
				// *PortalResource implements Resource, so we are explicitly converting *PortalResource to Resource interface.
				// (*PortalResource)(&slice[i]) -> Resource
				resource := RPtr(&slice[i])
				if !fn(resource) {
					return false // callback requested stop
				}
			}
			return true
		},
		count: func(rs *ResourceSet) int {
			slicePtr := getSlicePtr(rs)
			if slicePtr == nil {
				return 0
			}
			return len(*slicePtr)
		},
		explain: explain,
	}
	for _, option := range options {
		if err := option(&ops); err != nil {
			panic("register resource type " + string(rt) + ": " + err.Error())
		}
	}
	registry[rt] = ops
}

// sliceToResources converts a typed slice to []Resource using generics.
// R is the concrete resource struct type (e.g., PortalResource)
// RPtr is pointer-to-R that implements Resource (e.g., *PortalResource)
func sliceToResources[R any, RPtr interface {
	*R
	Resource
}](slice []R) []Resource {
	result := make([]Resource, len(slice))
	for i := range slice {
		result[i] = RPtr(&slice[i])
	}
	return result
}

// AllResources returns all resources in the ResourceSet as a slice of Resource interface.
// Uses registered accessors to collect resources from all typed slices.
// Resources not yet registered in the registry will not be included.
//
// NOTE: This allocates a new slice. For iteration without allocation, use ForEachResource.
func (rs *ResourceSet) AllResources() []Resource {
	// Pre-allocate with known capacity to reduce allocations
	total := rs.ResourceCount()
	result := make([]Resource, 0, total)
	for _, ops := range registry {
		result = append(result, ops.get(rs)...)
	}
	return result
}

// ForEachResource iterates over all resources without allocating a slice.
// The callback returns false to stop iteration early.
// Returns true if all resources were visited, false if stopped early.
func (rs *ResourceSet) ForEachResource(fn func(Resource) bool) bool {
	// Registry map is used for aggregate operations where visit order
	// is irrelevant. Thus, we are not defining an iteration order here.
	for _, ops := range registry {
		if !ops.forEach(rs, fn) {
			return false
		}
	}
	return true
}

// ResourceCount returns the total number of resources across all registered types.
// Time complexity - O(number of resource types)
func (rs *ResourceSet) ResourceCount() int {
	total := 0
	for _, ops := range registry {
		total += ops.count(rs)
	}
	return total
}

// IsEmpty returns true if the ResourceSet contains no resources.
func (rs *ResourceSet) IsEmpty() bool {
	for _, ops := range registry {
		if ops.count(rs) > 0 {
			return false
		}
	}
	return true
}

// AllResourcesByType returns resources of a specific type from the ResourceSet.
// Returns nil if the resource type is not registered.
func (rs *ResourceSet) AllResourcesByType(rt ResourceType) []Resource {
	ops, ok := registry[rt]
	if !ok {
		return nil
	}
	return ops.get(rs)
}

// AppendAll appends all resources from src to dest for all registered types.
func (rs *ResourceSet) AppendAll(src *ResourceSet) {
	for _, ops := range registry {
		ops.append(rs, src)
	}
}

// IsRegistered returns true if a resource type is registered in the registry.
func IsRegistered(rt ResourceType) bool {
	_, ok := registry[rt]
	return ok
}

// RegisteredTypes returns all registered resource types.
func RegisteredTypes() []ResourceType {
	types := make([]ResourceType, 0, len(registry))
	for rt := range registry {
		types = append(types, rt)
	}
	return types
}

// ExternalResolutionFor returns external lookup capability metadata.
func ExternalResolutionFor(rt ResourceType) (ExternalResolutionRegistration, bool) {
	ops, ok := registry[rt]
	if !ok || ops.external == nil {
		return ExternalResolutionRegistration{}, false
	}
	result := *ops.external
	result.Selectors = append([]string(nil), result.Selectors...)
	return result, true
}

// MaterializeExternalResource adds an externally managed resource identity to
// the ResourceSet using the constructor registered for its resource type.
func MaterializeExternalResource(
	rs *ResourceSet,
	rt ResourceType,
	ref string,
	id string,
	parentRef string,
) (Resource, error) {
	ops, ok := registry[rt]
	if !ok || ops.external == nil || ops.materializeExternal == nil {
		return nil, fmt.Errorf("resource type %s does not support external materialization", rt)
	}
	return ops.materializeExternal(rs, ref, id, parentRef)
}

// ExternalUnsupportedReason reports the reviewed reason a relationship target
// does not currently support external resolution.
func ExternalUnsupportedReason(rt ResourceType) (string, bool) {
	ops, ok := registry[rt]
	if !ok || strings.TrimSpace(ops.externalUnsupportedReason) == "" {
		return "", false
	}
	return ops.externalUnsupportedReason, true
}

// ExternalResolvableTypes returns all resource types with the external capability contract.
func ExternalResolvableTypes() []ResourceType {
	result := make([]ResourceType, 0)
	for rt, ops := range registry {
		if ops.external != nil {
			result = append(result, rt)
		}
	}
	slices.Sort(result)
	return result
}

// ExternalResolvableTypesByScope returns capabilities partitioned by whether
// their lookup requires a resolved parent identity.
func ExternalResolvableTypesByScope(scoped bool) []ResourceType {
	result := make([]ResourceType, 0)
	for _, resourceType := range ExternalResolvableTypes() {
		capability, _ := ExternalResolutionFor(resourceType)
		if (capability.ParentType != "") == scoped {
			result = append(result, resourceType)
		}
	}
	return result
}

package resources

// ResourceRegistry provides a central lookup for resource type metadata and iteration.
// This enables adding new resources without modifying switch statements across the codebase.
// Adding a new resource:
//  1. Define the resource struct (embed BaseResource or BaseResourceCore)
//  2. Add init() with registerResourceType() in the resource file
//  3. Add the field to ResourceSet

// resourceOps provides operations for a specific resource type within a ResourceSet.
type resourceOps struct {
	get     func(rs *ResourceSet) []Resource
	append  func(dest, src *ResourceSet)
	forEach func(rs *ResourceSet, fn func(Resource) bool) bool
	count   func(rs *ResourceSet) int
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
	getSlicePtr func(*ResourceSet) *[]R) {
	registry[rt] = resourceOps{
		get: func(rs *ResourceSet) []Resource {
			return sliceToResources[R, RPtr](*getSlicePtr(rs))
		},
		append: func(dest, src *ResourceSet) {
			destPtr := getSlicePtr(dest)
			*destPtr = append(*destPtr, *getSlicePtr(src)...)
		},
		forEach: func(rs *ResourceSet, fn func(Resource) bool) bool {
			slice := *getSlicePtr(rs) // e.g., rs.Portals (the actual slice, not pointer)
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
			return len(*getSlicePtr(rs))
		},
	}
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

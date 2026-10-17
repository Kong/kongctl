package resources

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceRegistrationRejectsDuplicatesBeforeInventoryWrites(t *testing.T) {
	storage := func(rs *ResourceSet) *[]PortalResource { return &rs.Portals }
	explain := AutoExplain[PortalResource]()
	external := ExternalResolutionRegistration{Selectors: []string{"id"}}
	for _, entry := range []struct {
		name     string
		register func(...ResourceRegistrationOption)
	}{
		{"ordinary", func(options ...ResourceRegistrationOption) {
			registerResourceType(ResourceTypePortal, storage, explain, options...)
		}},
		{"separate accessors", func(options ...ResourceRegistrationOption) {
			registerResourceTypeWithSliceAccessors(ResourceTypePortal, storage, storage, explain, options...)
		}},
		{"external", func(options ...ResourceRegistrationOption) {
			registerExternalResourceType(ResourceTypePortal, storage, explain, external, options...)
		}},
		{"external separate accessors", func(options ...ResourceRegistrationOption) {
			registerExternalResourceTypeWithSliceAccessors(ResourceTypePortal, storage, storage, explain, external, options...)
		}},
	} {
		for _, withInventories := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/inventories=%t", entry.name, withInventories), func(t *testing.T) {
				isolateBaseRegistrationInventories(t)
				registerExternalResourceType(ResourceTypePortal, storage, explain, external)
				original := registry[ResourceTypePortal]
				var options []ResourceRegistrationOption
				if withInventories {
					options = append(options, func(ops *resourceOps) error {
						ops.namespace = &namespaceRegistration{
							order: 1,
							visit: func(*ResourceSet, func(NamespaceParticipant) error) error { return nil },
						}
						ops.load = &childLoadRegistration{
							parent: ResourceTypeAPI, family: ResourceTypeAPI, extractOrder: 1, validateOrder: 1,
							extract: func(*ResourceSet, Resource) {}, validate: func(*ResourceSet) error { return nil },
						}
						return nil
					})
				}
				require.PanicsWithValue(t, "duplicate resource type registration: "+string(ResourceTypePortal), func() {
					entry.register(options...)
				})
				require.Len(t, registry, 1)
				require.Same(
					t,
					original.external,
					registry[ResourceTypePortal].external,
					"original capabilities must survive",
				)
				require.Nil(t, registry[ResourceTypePortal].namespace)
				require.Nil(t, registry[ResourceTypePortal].load)
				require.Empty(t, namespaceRegistrations)
				require.Empty(t, childExtractors)
				require.Empty(t, childValidators)
				rs := &ResourceSet{Portals: []PortalResource{{BaseResource: BaseResource{Ref: "retained"}}}}
				got := registry[ResourceTypePortal].get(rs)
				require.Len(t, got, 1)
				require.Equal(t, "retained", got[0].GetRef())
			})
		}
	}
}

func TestResourceRegistrationAllowsDistinctKinds(t *testing.T) {
	isolateBaseRegistrationInventories(t)
	registerResourceType(
		ResourceTypePortal,
		func(rs *ResourceSet) *[]PortalResource { return &rs.Portals },
		AutoExplain[PortalResource](),
	)
	registerExternalResourceType(
		ResourceTypeAPI,
		func(rs *ResourceSet) *[]APIResource { return &rs.APIs },
		AutoExplain[APIResource](),
		ExternalResolutionRegistration{Selectors: []string{"id"}},
	)
	require.Len(t, registry, 2)
	rs := &ResourceSet{Portals: []PortalResource{{}}, APIs: []APIResource{{}}}
	require.Equal(t, 1, registry[ResourceTypePortal].count(rs))
	require.Equal(t, 1, registry[ResourceTypeAPI].count(rs))
	require.NotNil(t, registry[ResourceTypeAPI].external)
}

func TestRegisteredStorageExplainAndKindAgree(t *testing.T) {
	for _, kind := range RegisteredTypes() {
		t.Run(string(kind), func(t *testing.T) {
			ops := registry[kind]
			require.NotNil(t, ops.explain.typ)
			path := resourceSetDeclarationPath(ops.explain.typ)
			require.NotEmpty(t, path)
			rs := &ResourceSet{}
			value := reflect.ValueOf(rs).Elem()
			// Some grouped declarations normalize into an untagged root slice.
			for _, field := range value.Fields() {
				if field.Type() == reflect.SliceOf(ops.explain.typ) {
					field.Set(reflect.MakeSlice(field.Type(), 1, 1))
				}
			}
			for _, name := range path {
				for value.Kind() == reflect.Pointer {
					if value.IsNil() {
						value.Set(reflect.New(value.Type().Elem()))
					}
					value = value.Elem()
				}
				value = materializedTaggedField(value, name)
				require.True(t, value.IsValid(), "declaration field %s", name)
			}
			require.Equal(t, reflect.Slice, value.Kind())
			value.Set(reflect.MakeSlice(value.Type(), 1, 1))
			items := ops.get(rs)
			require.Len(t, items, 1, "storage must expose the declared explain type")
			require.Equal(t, reflect.PointerTo(ops.explain.typ), reflect.TypeOf(items[0]))
			require.Equal(t, kind, items[0].GetType())
		})
	}
}

func isolateBaseRegistrationInventories(t *testing.T) {
	t.Helper()
	// Sequential tests restore all globals before parallel resource tests resume.
	original, namespace, extractors, validators := registry, namespaceRegistrations, childExtractors, childValidators
	registry = make(map[ResourceType]resourceOps)
	namespaceRegistrations = nil
	childExtractors = make(map[ResourceType][]childLoadRegistration)
	childValidators = make(map[ResourceType][]childLoadRegistration)
	t.Cleanup(func() {
		registry, namespaceRegistrations, childExtractors, childValidators = original, namespace, extractors, validators
	})
}

package adopt

import (
	"maps"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestManagedByAdoptionAcrossIdentityResources(t *testing.T) {
	const directory = "731559d3-39bd-48c7-b33c-eb192847eb20"
	const principal = "731559d3-39bd-48c7-b33c-eb192847eb21"
	const identity = "731559d3-39bd-48c7-b33c-eb192847eb22"
	for _, family := range []struct {
		name string
		ref  bool
		run  func(*testing.T, map[string]string, bool) (map[string]string, int, error)
	}{
		{name: "directory", run: func(t *testing.T, metadata map[string]string, overwrite bool,
		) (map[string]string, int, error) {
			current := fixture.Directory("default", "", false)
			current.ManagedBy = metadata
			api := &fixture.DirectoryAPI{Current: current}
			_, err := adoptIdentityDirectory(t.Context(), api, current.ID, "team-a", overwrite)
			return api.Replaced.ManagedBy, api.Replaces, err
		}},
		{name: "group", run: func(t *testing.T, metadata map[string]string, overwrite bool,
		) (map[string]string, int, error) {
			current := fixture.Group("group", "", false)
			current.ManagedBy = metadata
			api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{directory: {current}}}
			_, err := adoptIdentityGroup(t.Context(), api, directory, current.ID, "team-a", overwrite)
			if len(api.Writes) == 0 {
				return nil, 0, err
			}
			return api.Writes[0].ManagedBy, len(api.Writes), err
		}},
		{name: "basic-auth", run: func(t *testing.T, metadata map[string]string, overwrite bool,
		) (map[string]string, int, error) {
			current := fixture.BasicAuth("username", "", false)
			current.ManagedBy = metadata
			api := &fixture.BasicAuthAPI{Parents: map[string][]helpers.IdentityBasicAuth{directory + "/" + principal: {current}}}
			_, err := adoptIdentityBasicAuth(t.Context(), api, directory, principal, current.ID, "", "team-a", overwrite)
			if len(api.Writes) == 0 {
				return nil, 0, err
			}
			return api.Writes[0].ManagedBy, len(api.Writes), err
		}},
		{name: "principal", ref: true, run: func(t *testing.T, metadata map[string]string, overwrite bool,
		) (map[string]string, int, error) {
			current := kkComps.KongPrincipal{ID: principal, ManagedBy: metadata}
			api := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directory: {current}}}
			_, err := adoptIdentityPrincipal(t.Context(), api, directory, principal, "ref", "team-a", overwrite)
			if len(api.Writes) == 0 {
				return nil, 0, err
			}
			return api.Writes[0].ManagedBy, len(api.Writes), err
		}},
		{name: "principal-identity", ref: true, run: func(t *testing.T, metadata map[string]string, overwrite bool,
		) (map[string]string, int, error) {
			current := kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
				ID: identity, Key: "employee", Value: "123", ManagedBy: metadata,
			})
			api := &fixture.PrincipalIdentitiesAPI{
				Parents: map[string][]kkComps.KongPrincipalIdentity{directory + "/" + principal: {current}},
			}
			_, err := adoptPrincipalIdentity(t.Context(), api, directory, principal, identity, "ref", "team-a", overwrite)
			if len(api.Replaces) == 0 {
				return nil, 0, err
			}
			return api.Replaces[0].KongPrincipalIdentityBody.KongPrincipalIdentityCustom.ManagedBy, len(api.Replaces), err
		}},
	} {
		t.Run(family.name, func(t *testing.T) {
			for _, overwrite := range []bool{false, true} {
				for _, metadata := range []map[string]string{
					{resources.ManagedByToolKey: "another-tool"},
					{"product": "another-product"},
					{resources.ManagedByToolKey: resources.ManagedByTool, "product": "another-product"},
				} {
					_, writes, err := family.run(t, maps.Clone(metadata), overwrite)
					require.ErrorContains(t, err, "unrecognized managed_by metadata")
					require.Zero(t, writes)
				}
				metadata := map[string]string{
					resources.ChildProtectedKey: "true", resources.ManagedByToolKey: resources.ManagedByTool,
					resources.ManagedByVersionKey: "old",
				}
				adopted, writes, err := family.run(t, metadata, overwrite)
				require.NoError(t, err)
				require.Equal(t, 1, writes)
				require.Equal(t, "true", adopted[resources.ChildProtectedKey])
				require.Equal(t, "team-a", adopted[resources.ChildNamespaceKey])
			}
			metadata := resources.BuildChildManagedBy("previous", true, "old")
			metadata["outside"] = "keep"
			_, writes, err := family.run(t, maps.Clone(metadata), false)
			require.ErrorContains(t, err, "--overwrite-namespace")
			require.Zero(t, writes)
			adopted, writes, err := family.run(t, metadata, true)
			if family.ref {
				require.ErrorContains(t, err, "5-entry")
				require.Zero(t, writes, "the final map must include kongctl-ref before capacity validation")
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, writes)
				require.Equal(t, "keep", adopted["outside"])
			}
		})
	}
}

package dump

import (
	"cmp"
	"context"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

const resourceIdentityDirectories = "identity.directories"

func identityDirectoryCollector() declarativeCollector {
	return rootCollector(
		resourceIdentityDirectories,
		func(ctx context.Context, d *declarativeDumpContext) ([]resources.IdentityDirectoryResource, error) {
			values, err := d.stateClient.ListIdentityDirectories(ctx)
			if err != nil {
				return nil, err
			}
			values = filterByNameOrID(values, d.filter, func(value state.IdentityDirectory) (string, string) {
				return value.Name, value.ID
			})
			result := make([]resources.IdentityDirectoryResource, 0, len(values))
			for _, current := range values {
				result = append(result, resources.IdentityDirectoryResource{
					BaseResource: resources.BaseResource{
						Ref:     current.Name,
						Kongctl: kongctlMetaFromLabels(current.NormalizedLabels),
					},
					Name: current.Name, Description: &current.Description,
					AllowedControlPlanes:  slices.Clone(current.AllowedControlPlanes),
					AllowAllControlPlanes: current.AllowAllControlPlanes, TTLSecs: current.TTLSecs,
					NegativeTTLSecs: current.NegativeTTLSecs, VaultEnabled: current.VaultEnabled,
					Labels: labels.GetUserLabels(current.Labels),
				})
			}
			slices.SortFunc(result, func(a, b resources.IdentityDirectoryResource) int { return cmp.Compare(a.Name, b.Name) })
			return result, nil
		},
		func(rs *resources.ResourceSet) *[]resources.IdentityDirectoryResource {
			if rs.Identity == nil {
				rs.Identity = &resources.IdentityResource{}
			}
			return &rs.Identity.Directories
		}, nil,
	)
}

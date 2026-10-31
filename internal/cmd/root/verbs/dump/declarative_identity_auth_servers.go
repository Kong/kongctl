package dump

import (
	"cmp"
	"context"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

const resourceIdentityAuthServers = "identity.auth_servers"

func identityAuthServerCollector() declarativeCollector {
	return rootCollector(resourceIdentityAuthServers,
		func(ctx context.Context, d *declarativeDumpContext) ([]resources.IdentityAuthServerResource, error) {
			values, err := d.stateClient.ListIdentityAuthServers(ctx)
			if err != nil {
				return nil, err
			}
			values = filterByNameOrID(values, d.filter, func(value state.IdentityAuthServer) (string, string) {
				return value.Name, value.ID
			})
			result := make([]resources.IdentityAuthServerResource, 0, len(values))
			for _, value := range values {
				a := resources.IdentityAuthServerResource{
					BaseResource: resources.BaseResource{Ref: value.ID, Kongctl: kongctlMetaFromLabels(value.NormalizedLabels)},
					Name:         value.Name, Audience: value.Audience, Description: &value.Description,
					SigningAlgorithm: value.SigningAlgorithm, TrustedOrigins: slices.Clone(value.TrustedOrigins),
					Labels: labels.GetUserLabels(value.Labels),
				}
				if a.TrustedOrigins == nil {
					a.TrustedOrigins = []string{}
				}
				if a.Labels == nil {
					a.Labels = map[string]string{}
				}
				if value.Dcr != nil {
					a.DCR = &resources.IdentityAuthServerDCR{DefaultAccessTokenDuration: value.Dcr.DefaultAccessTokenDuration}
				}
				result = append(result, a)
			}
			slices.SortFunc(result, func(a, b resources.IdentityAuthServerResource) int { return cmp.Compare(a.Name, b.Name) })
			return result, nil
		}, func(rs *resources.ResourceSet) *[]resources.IdentityAuthServerResource {
			if rs.Identity == nil {
				rs.Identity = &resources.IdentityResource{}
			}
			return &rs.Identity.AuthServers
		}, nil)
}

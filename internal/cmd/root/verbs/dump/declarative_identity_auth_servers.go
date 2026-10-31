package dump

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
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
		}, collectIdentityAuthServerChildren,
		resources.ResourceTypeIdentityAuthServerScope, resources.ResourceTypeIdentityAuthServerClaim)
}

func collectIdentityAuthServerChildren(ctx context.Context, d *declarativeDumpContext,
	servers []resources.IdentityAuthServerResource,
) error {
	for i := range servers {
		server := &servers[i]
		scopes, err := d.stateClient.ListIdentityAuthServerScopes(ctx, server.Ref)
		if err != nil {
			return err
		}
		claims, err := d.stateClient.ListIdentityAuthServerClaims(ctx, server.Ref)
		if err != nil {
			return err
		}
		server.Scopes = make([]resources.IdentityAuthServerScopeResource, 0, len(scopes))
		server.Claims = make([]resources.IdentityAuthServerClaimResource, 0, len(claims))
		scopeIDs := make(map[string]bool)
		for _, scope := range scopes {
			scopeIDs[scope.ID] = true
			server.Scopes = append(server.Scopes, resources.IdentityAuthServerScopeResource{
				IdentityAuthServerChild: resources.IdentityAuthServerChild{Ref: scope.ID, Name: scope.Name},
				Description:             &scope.Description, Default: scope.Default,
				IncludeInMetadata: scope.IncludeInMetadata, Enabled: scope.Enabled,
			})
		}
		names := make(map[string]bool)
		for _, claim := range claims {
			if names[claim.Name] {
				return fmt.Errorf("cannot dump auth server %q: duplicate claim name %q", server.Name, claim.Name)
			}
			names[claim.Name] = true
			links := make([]string, 0, len(claim.IncludeInScopes))
			for _, id := range claim.IncludeInScopes {
				if !scopeIDs[id] {
					return fmt.Errorf("claim %q references an unobserved scope", claim.Name)
				}
				links = append(links, tags.RefPlaceholderPrefix+id+"#id")
			}
			server.Claims = append(server.Claims, resources.IdentityAuthServerClaimResource{
				IdentityAuthServerChild: resources.IdentityAuthServerChild{Ref: claim.ID, Name: claim.Name},
				Value:                   claim.Value, IncludeInToken: claim.IncludeInToken,
				IncludeInAllScopes: claim.IncludeInAllScopes,
				IncludeInScopes:    links, Enabled: claim.Enabled,
			})
		}
		slices.SortFunc(server.Scopes, func(a, b resources.IdentityAuthServerScopeResource) int {
			return cmp.Compare(a.Name, b.Name)
		})
		slices.SortFunc(server.Claims, func(a, b resources.IdentityAuthServerClaimResource) int {
			return cmp.Compare(a.Name, b.Name)
		})
	}
	return nil
}

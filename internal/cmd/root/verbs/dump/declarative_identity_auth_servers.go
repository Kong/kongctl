package dump

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
					BaseResource: resources.BaseResource{
						Ref:     value.ID,
						Kongctl: kongctlMetaFromLabels(value.NormalizedLabels),
					},
					Name: value.Name, Audience: value.Audience, Description: &value.Description,
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
					a.DCR = &resources.IdentityAuthServerDCR{
						DefaultAccessTokenDuration: value.Dcr.DefaultAccessTokenDuration,
					}
				}
				result = append(result, a)
			}
			slices.SortFunc(
				result,
				func(a, b resources.IdentityAuthServerResource) int { return cmp.Compare(a.Name, b.Name) },
			)
			return result, nil
		}, func(rs *resources.ResourceSet) *[]resources.IdentityAuthServerResource {
			if rs.Identity == nil {
				rs.Identity = &resources.IdentityResource{}
			}
			return &rs.Identity.AuthServers
		}, collectIdentityAuthServerChildren,
		resources.ResourceTypeIdentityAuthServerScope, resources.ResourceTypeIdentityAuthServerClaim,
		resources.ResourceTypeIdentityAuthServerClient)
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
		clients, err := d.stateClient.ListIdentityAuthServerClients(ctx, server.Ref)
		if err != nil {
			return err
		}
		server.Clients = make([]resources.IdentityAuthServerClientResource, 0, len(clients))
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
		clientNames := make(map[string]bool)
		for _, client := range clients {
			if clientNames[client.Name] {
				return fmt.Errorf("cannot dump auth server %q: duplicate client name %q", server.Name, client.Name)
			}
			clientNames[client.Name] = true
			links := make([]string, 0, len(client.AllowScopes))
			for _, id := range client.AllowScopes {
				if !scopeIDs[id] {
					return fmt.Errorf("client %q references an unobserved scope", client.Name)
				}
				links = append(links, tags.RefPlaceholderPrefix+id+"#id")
			}
			server.Clients = append(server.Clients, identityClientForDump(client, links))
		}
		slices.SortFunc(
			server.Clients,
			func(a, b resources.IdentityAuthServerClientResource) int { return cmp.Compare(a.Name, b.Name) },
		)
	}
	return nil
}

func identityClientForDump(
	client kkComps.AuthServerClient,
	scopes []string,
) resources.IdentityAuthServerClientResource {
	userLabels := labels.GetUserLabels(client.Labels)
	if userLabels == nil {
		userLabels = map[string]string{}
	}
	redirects := slices.Clone(client.RedirectUris)
	if redirects == nil {
		redirects = []string{}
	}
	return resources.IdentityAuthServerClientResource{
		BaseResource: resources.BaseResource{Ref: client.ID, Kongctl: kongctlMetaFromLabels(client.Labels)},
		Name:         client.Name, GrantTypes: append([]kkComps.AuthServerGrantType{}, client.GrantTypes...),
		ResponseTypes: append([]kkComps.ResponseType{}, client.ResponseTypes...), RedirectURIs: redirects,
		LoginURI: client.LoginURI, AccessTokenDuration: client.AccessTokenDuration, IDTokenDuration: client.IDTokenDuration,
		AllowAllScopes: client.AllowAllScopes, AllowScopes: scopes, TokenEndpointAuthMethod: client.TokenEndpointAuthMethod,
		Labels: userLabels,
	}
}

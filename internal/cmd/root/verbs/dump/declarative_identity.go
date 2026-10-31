package dump

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
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
						Ref:     current.ID,
						Kongctl: kongctlMetaFromLabels(current.NormalizedLabels),
					},
					Name: current.Name, Description: &current.Description,
					AllowedControlPlanes:  slices.Clone(current.AllowedControlPlanes),
					AllowAllControlPlanes: current.AllowAllControlPlanes, TTLSecs: current.TTLSecs,
					NegativeTTLSecs: current.NegativeTTLSecs, VaultEnabled: current.VaultEnabled,
					Labels: labels.GetUserLabels(current.Labels),
				})
			}
			slices.SortFunc(
				result,
				func(a, b resources.IdentityDirectoryResource) int { return cmp.Compare(a.Name, b.Name) },
			)
			return result, nil
		},
		func(rs *resources.ResourceSet) *[]resources.IdentityDirectoryResource {
			if rs.Identity == nil {
				rs.Identity = &resources.IdentityResource{}
			}
			return &rs.Identity.Directories
		}, func(ctx context.Context, d *declarativeDumpContext, values []resources.IdentityDirectoryResource) error {
			seen := make(map[string]bool)
			for i := range values {
				principals, err := d.stateClient.ListIdentityPrincipals(ctx, values[i].Ref)
				if err != nil {
					return err
				}
				for _, current := range principals {
					ref := current.ID
					if current.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool {
						if err := resources.ValidateRef(current.GetName()); err != nil {
							return fmt.Errorf(
								"principal %s has invalid persisted ref; explicitly adopt it before dumping",
								current.ID,
							)
						}
						ref = current.GetName()
					}
					if seen[ref] {
						return fmt.Errorf(
							"principal ref %q occurs in multiple directories or namespaces; filter the directory dump",
							ref,
						)
					}
					seen[ref] = true
					data, err := json.Marshal(current.Metadata)
					if err != nil {
						return err
					}
					metadata := make(map[string]any)
					decoder := json.NewDecoder(bytes.NewReader(data))
					decoder.UseNumber()
					if err := decoder.Decode(&metadata); err != nil {
						return err
					}
					principal := resources.IdentityPrincipalResource{
						BaseResource: resources.BaseResource{
							Ref:     ref,
							Kongctl: kongctlMetaFromLabels(current.NormalizedLabels),
						},
						DisplayName: current.DisplayName, Description: &current.Description, Metadata: metadata, Labels: current.Labels,
						Active: current.Active,
					}
					if current.ManagedBy[resources.ManagedByToolKey] != resources.ManagedByTool {
						principal.External = &resources.ExternalBlock{ID: current.ID}
						principal.Kongctl = nil
					}
					if current.Type != nil {
						principal.Type = new(string(*current.Type))
					} else {
						principal.DeclareUnclassified()
					}
					identities, err := d.stateClient.ListIdentityPrincipalIdentities(ctx, values[i].Ref, current.ID)
					if err != nil {
						return err
					}
					for _, identity := range identities {
						data, err := json.Marshal(identity)
						if err != nil {
							return err
						}
						var declaration resources.IdentityPrincipalIdentityResource
						if err := json.Unmarshal(data, &declaration); err != nil {
							return err
						}
						declaration.Ref = identity.ID
						if identity.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool {
							declaration.Ref = identity.ManagedBy[resources.ManagedByRefKey]
							if err := resources.ValidateRef(declaration.Ref); err != nil {
								return fmt.Errorf(
									"identity %s has invalid persisted ref; explicitly adopt it before dumping",
									identity.ID,
								)
							}
							owned := state.NormalizeIdentityPrincipalIdentity(identity)
							declaration.Kongctl = kongctlMetaFromLabels(owned.NormalizedLabels)
						} else {
							declaration.External = &resources.ExternalBlock{ID: identity.ID}
						}
						if identity.WorkspaceID != nil {
							// Q25: preserve the inspected association without promising
							// declarative writes for an unverified workspace contract.
							declaration.External = &resources.ExternalBlock{ID: identity.ID}
							declaration.Kongctl = nil
						}
						if seen[declaration.Ref] {
							return fmt.Errorf(
								"identity ref %q is ambiguous in the dump; filter the dump",
								declaration.Ref,
							)
						}
						seen[declaration.Ref] = true
						principal.Identities = append(principal.Identities, declaration)
					}
					slices.SortFunc(
						principal.Identities,
						func(a, b resources.IdentityPrincipalIdentityResource) int { return cmp.Compare(a.Ref, b.Ref) },
					)
					values[i].Principals = append(values[i].Principals, principal)
				}
				slices.SortFunc(
					values[i].Principals,
					func(a, b resources.IdentityPrincipalResource) int { return cmp.Compare(a.Ref, b.Ref) },
				)
			}
			return nil
		}, resources.ResourceTypeIdentityPrincipal, resources.ResourceTypeIdentityPrincipalIdentity,
	)
}

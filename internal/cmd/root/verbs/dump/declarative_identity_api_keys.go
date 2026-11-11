package dump

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
)

func collectIdentityAPIKeys(ctx context.Context, d *declarativeDumpContext, kind resources.ResourceType,
	directory, parent string, seen map[string]bool,
) ([]resources.IdentityAPIKeyFields, error) {
	all, err := d.stateClient.ListIdentityAPIKeys(ctx, kind, directory, parent)
	if err != nil {
		return nil, err
	}
	indexed, err := state.IndexIdentityAPIKeys(all)
	if err != nil {
		return nil, err
	}
	var refs []string
	for ref := range indexed {
		refs = append(refs, ref)
	}
	slices.Sort(refs)
	var result []resources.IdentityAPIKeyFields
	for _, ref := range refs {
		if _, err := state.FindIdentityAPIKey(all, ref); err != nil {
			return nil, err
		}
		if seen[ref] {
			return nil, fmt.Errorf("API-key ref %q is ambiguous in the dump; filter the directory", ref)
		}
		seen[ref] = true
		value := indexed[ref]
		if kind == resources.ResourceTypeIdentityGroupAPIKey &&
			value.ManagedBy[resources.IdentityAPIKeyCreateTypeKey] == resources.IdentityAPIKeyImportedType {
			return nil, fmt.Errorf("imported group API keys are outside declarative support")
		}
		fields := resources.IdentityAPIKeyFields{
			Ref:       ref,
			Type:      value.ManagedBy[resources.IdentityAPIKeyCreateTypeKey],
			Name:      value.Name,
			ExpiresAt: value.ExpiresAt,
		}
		if value.ExpiresAt == nil {
			fields.DeclareNoExpiration()
		}
		result = append(result, fields)
	}
	return result, nil
}

func collectExternalPrincipalAPIKeys(ctx context.Context, d *declarativeDumpContext,
	directory, parent string, seen map[string]bool,
) ([]resources.IdentityPrincipalAPIKeyResource, error) {
	all, err := d.stateClient.ListIdentityAPIKeys(ctx, resources.ResourceTypeIdentityPrincipalAPIKey, directory, parent)
	if err != nil {
		return nil, err
	}
	var result []resources.IdentityPrincipalAPIKeyResource
	for _, value := range all {
		namespace := value.ManagedBy[resources.ChildNamespaceKey]
		if namespace == "" {
			continue
		}
		ref, owned, err := resources.IdentityAPIKeyRef(value.ManagedBy)
		if err != nil {
			return nil, err
		}
		if !owned {
			continue
		}
		if _, err := state.FindIdentityAPIKey(all, ref, namespace); err != nil {
			return nil, err
		}
		if seen[ref] {
			return nil, fmt.Errorf("API-key ref %q is ambiguous in the dump; filter the directory", ref)
		}
		seen[ref] = true
		key := resources.IdentityPrincipalAPIKeyResource{
			IdentityAPIKeyFields: resources.IdentityAPIKeyFields{
				Ref: ref, Type: value.ManagedBy[resources.IdentityAPIKeyCreateTypeKey],
				Name: value.Name, ExpiresAt: value.ExpiresAt,
			},
			Kongctl: &resources.KongctlMeta{Namespace: new(namespace)},
		}
		if value.ExpiresAt == nil {
			key.DeclareNoExpiration()
		}
		result = append(result, key)
	}
	slices.SortFunc(result, func(a, b resources.IdentityPrincipalAPIKeyResource) int {
		return strings.Compare(a.Ref, b.Ref)
	})
	return result, nil
}

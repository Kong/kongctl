package dump

import (
	"context"
	"fmt"
	"slices"

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
		fields := resources.IdentityAPIKeyFields{
			Ref:       ref,
			Type:      resources.IdentityAPIKeyGeneratedType,
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

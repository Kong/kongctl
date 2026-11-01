package state

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
)

func IndexIdentityAPIKeys(values []IdentityAPIKey) (map[string]IdentityAPIKey, error) {
	result := make(map[string]IdentityAPIKey)
	for _, value := range values {
		ref, owned, err := resources.IdentityAPIKeyRef(value.ManagedBy)
		if err != nil {
			return nil, err
		}
		if !owned {
			continue
		}
		if _, duplicate := result[ref]; duplicate {
			return nil, fmt.Errorf("multiple API keys match persisted ref %q", ref)
		}
		result[ref] = value
	}
	return result, nil
}

func FindIdentityAPIKey(values []IdentityAPIKey, ref string) (*IdentityAPIKey, error) {
	indexed, err := IndexIdentityAPIKeys(values)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		if value.ManagedBy[resources.ManagedByToolKey] == resources.ManagedByTool &&
			value.ManagedBy[resources.ManagedByRefKey] == ref {
			_, owned, err := resources.IdentityAPIKeyRef(value.ManagedBy)
			if err != nil {
				return nil, err
			}
			if !owned {
				return nil, fmt.Errorf("API-key ref %q collides with incompatible or legacy ownership metadata", ref)
			}
		}
	}
	if value, ok := indexed[ref]; ok {
		return &value, nil
	}
	return nil, nil
}

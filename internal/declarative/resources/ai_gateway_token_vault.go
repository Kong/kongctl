package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/tags"
)

// AIGatewayTokenVaultFields returns writable Token Vault fields in their JSON representation.
func AIGatewayTokenVaultFields(
	datastores []kkComps.AIGatewayDatastoreRef,
	vault any,
) (map[string]any, error) {
	fields, err := marshalObjectToMap(
		map[string]any{"datastores": datastores, "token_vault": vault},
		"AI Gateway Token Vault configuration",
	)
	if err != nil {
		return nil, err
	}
	if datastores == nil {
		delete(fields, "datastores")
	}
	if fields["token_vault"] == nil {
		delete(fields, "token_vault")
	}
	return fields, nil
}

// validateAIGatewayTokenVault enforces the datastore cardinality contract.
func validateAIGatewayTokenVault(stores []kkComps.AIGatewayDatastoreRef, required bool) error {
	if stores != nil && len(stores) != 1 {
		return fmt.Errorf("datastores must contain exactly one token-vault datastore reference")
	}
	if required && len(stores) != 1 {
		return fmt.Errorf(
			"token_vault requires exactly one token-vault datastore reference in datastores",
		)
	}
	for _, store := range stores {
		if store.Name == "" {
			return fmt.Errorf("datastores[].name is required")
		}
	}
	return nil
}

func validateAIGatewayTokenVaultStoreTypes(rs *ResourceSet, child ResourceWithParent) error {
	kind := child.GetType()
	if kind != ResourceTypeAIGatewayProvider && kind != ResourceTypeAIGatewayAuthStrategy &&
		kind != ResourceTypeAIGatewayMCPServer {
		return nil
	}
	payloadResource, ok := child.(interface {
		MutablePayloadMap() (map[string]any, error)
	})
	if !ok {
		return nil
	}
	payload, err := payloadResource.MutablePayloadMap()
	if err != nil {
		return err
	}
	stores, _ := payload["datastores"].([]any)
	parent := child.GetParentRef()
	if parent == nil {
		return nil
	}
	for _, raw := range stores {
		store, _ := raw.(map[string]any)
		name, _ := store["name"].(string)
		var target *AIGatewayDatastoreResource
		if ref, _, ok := tags.ParseRefPlaceholder(name); ok {
			target = rs.GetAIGatewayDatastoreByRef(ref)
		} else {
			for i := range rs.AIGatewayDatastores {
				candidate := &rs.AIGatewayDatastores[i]
				if NormalizeResourceRef(candidate.AIGateway) == NormalizeResourceRef(parent.Ref) && candidate.Name() == name {
					target = candidate
					break
				}
			}
		}
		if target != nil &&
			(target.DatastoreType() != "token-vault" ||
				NormalizeResourceRef(target.AIGateway) != NormalizeResourceRef(parent.Ref)) {
			return fmt.Errorf(
				"%s %q requires a token-vault datastore under the same ai_gateway",
				child.GetType(),
				child.GetRef(),
			)
		}
	}
	return nil
}

func configureAIGatewayTokenVaultExplain(node *ExplainNode) {
	auth, ok := node.lookup([]string{"token_vault", "cache", "redis", "cloud_authentication"})
	if !ok {
		return
	}
	branches := auth.OneOf
	if len(branches) == 0 {
		branches = []*ExplainNode{auth}
	}
	for _, branch := range branches {
		for _, field := range []string{"secret_access_key", "client_secret", "service_account_json"} {
			explainReplacePath(
				branch,
				[]string{field},
				explainSecretEnvNode("TOKEN_VAULT_CACHE_SECRET"),
			)
		}
	}
	explainRefreshUnionAggregate(auth)
}

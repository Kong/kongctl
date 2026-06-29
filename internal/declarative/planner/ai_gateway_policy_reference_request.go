package planner

import (
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func normalizeAIGatewayPolicyNameReferencesForRequest(payload map[string]any, rs *resources.ResourceSet) {
	if payload == nil {
		return
	}

	rawPolicies, ok := payload[FieldPolicies]
	if !ok {
		return
	}

	switch policies := rawPolicies.(type) {
	case []any:
		for i, rawPolicy := range policies {
			policyRef, ok := rawPolicy.(string)
			if !ok {
				continue
			}
			policies[i] = aiGatewayPolicyNameReferenceForRequest(policyRef, rs)
		}
	case []string:
		normalized := make([]any, len(policies))
		for i, policyRef := range policies {
			normalized[i] = aiGatewayPolicyNameReferenceForRequest(policyRef, rs)
		}
		payload[FieldPolicies] = normalized
	}
}

func aiGatewayPolicyNameReferenceForRequest(policyRef string, rs *resources.ResourceSet) string {
	targetRef, field, ok := tags.ParseRefPlaceholder(policyRef)
	if !ok || field != FieldName {
		return policyRef
	}

	if rs != nil {
		if policy := rs.GetAIGatewayPolicyByRef(targetRef); policy != nil {
			if policy.Name != "" {
				return policy.Name
			}
			if policy.Ref != "" {
				return policy.Ref
			}
		}
	}

	return targetRef
}

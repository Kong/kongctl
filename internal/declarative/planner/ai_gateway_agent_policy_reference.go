package planner

import (
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func normalizeAIGatewayAgentPolicyReferencesForRequest(payload map[string]any, rs *resources.ResourceSet) {
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
			policies[i] = aiGatewayAgentPolicyReferenceForRequest(policyRef, rs)
		}
	case []string:
		normalized := make([]any, len(policies))
		for i, policyRef := range policies {
			normalized[i] = aiGatewayAgentPolicyReferenceForRequest(policyRef, rs)
		}
		payload[FieldPolicies] = normalized
	}
}

func aiGatewayAgentPolicyReferenceForRequest(policyRef string, rs *resources.ResourceSet) string {
	targetRef, _, ok := tags.ParseRefPlaceholder(policyRef)
	if !ok {
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

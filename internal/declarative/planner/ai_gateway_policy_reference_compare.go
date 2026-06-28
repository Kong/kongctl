package planner

import (
	"maps"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func normalizeAIGatewayPolicyReferencesForComparison(
	currentPayload map[string]any,
	desiredPayload map[string]any,
	rs *resources.ResourceSet,
) (map[string]any, map[string]any) {
	currentCompare := clonePayloadMap(currentPayload)
	desiredCompare := clonePayloadMap(desiredPayload)

	_, currentHasPolicies := currentCompare[FieldPolicies]
	_, desiredHasPolicies := desiredCompare[FieldPolicies]
	if !currentHasPolicies || !desiredHasPolicies {
		return currentCompare, desiredCompare
	}

	aliases := aiGatewayPolicyReferenceAliases(rs)
	currentCompare[FieldPolicies] = normalizeAIGatewayPolicyReferenceList(currentCompare[FieldPolicies], aliases)
	desiredCompare[FieldPolicies] = normalizeAIGatewayPolicyReferenceList(desiredCompare[FieldPolicies], aliases)
	return currentCompare, desiredCompare
}

func clonePayloadMap(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	clone := make(map[string]any, len(payload))
	maps.Copy(clone, payload)
	return clone
}

func aiGatewayPolicyReferenceAliases(rs *resources.ResourceSet) map[string]string {
	aliases := make(map[string]string)
	if rs == nil {
		return aliases
	}

	for _, policy := range rs.AIGatewayPolicies {
		canonical := firstNonEmpty(policy.Ref, policy.Name, policy.GetKonnectID())
		if canonical == "" {
			continue
		}
		for _, alias := range []string{policy.Ref, policy.Name, policy.GetKonnectID()} {
			if alias != "" {
				aliases[alias] = canonical
			}
		}
	}
	return aliases
}

func normalizeAIGatewayPolicyReferenceList(raw any, aliases map[string]string) any {
	switch policies := raw.(type) {
	case []any:
		normalized := make([]any, len(policies))
		for i, policy := range policies {
			if policyRef, ok := policy.(string); ok {
				normalized[i] = canonicalAIGatewayPolicyReference(policyRef, aliases)
				continue
			}
			normalized[i] = policy
		}
		return normalized
	case []string:
		normalized := make([]any, len(policies))
		for i, policyRef := range policies {
			normalized[i] = canonicalAIGatewayPolicyReference(policyRef, aliases)
		}
		return normalized
	default:
		return raw
	}
}

func canonicalAIGatewayPolicyReference(policyRef string, aliases map[string]string) string {
	if parsedRef, _, ok := tags.ParseRefPlaceholder(policyRef); ok {
		policyRef = parsedRef
	}
	if canonical := aliases[policyRef]; canonical != "" {
		return canonical
	}
	return policyRef
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

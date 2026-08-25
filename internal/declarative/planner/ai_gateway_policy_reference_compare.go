package planner

import (
	"maps"
	"slices"
	"strings"

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
	currentCompare[FieldPolicies] = normalizeAIGatewayReferenceList(currentCompare[FieldPolicies], aliases)
	desiredCompare[FieldPolicies] = normalizeAIGatewayReferenceList(desiredCompare[FieldPolicies], aliases)
	return currentCompare, desiredCompare
}

func normalizeAIGatewayAuthStrategyReferencesForComparison(
	currentPayload map[string]any,
	desiredPayload map[string]any,
	rs *resources.ResourceSet,
) (map[string]any, map[string]any) {
	currentCompare := clonePayloadMap(currentPayload)
	desiredCompare := clonePayloadMap(desiredPayload)

	currentAccess, currentOK := currentCompare[FieldAccess].(map[string]any)
	desiredAccess, desiredOK := desiredCompare[FieldAccess].(map[string]any)
	if !currentOK || !desiredOK {
		return currentCompare, desiredCompare
	}
	currentAccess = clonePayloadMap(currentAccess)
	desiredAccess = clonePayloadMap(desiredAccess)
	// Konnect currently mirrors auth strategy references into the deprecated
	// identity_providers response field. Treat the canonical field as authoritative.
	if _, ok := currentAccess[FieldAuthStrategies]; ok {
		delete(currentAccess, FieldIdentityProviders)
	}
	if _, ok := desiredAccess[FieldAuthStrategies]; ok {
		delete(desiredAccess, FieldIdentityProviders)
	}
	currentCompare[FieldAccess] = currentAccess
	desiredCompare[FieldAccess] = desiredAccess
	_, currentHasProviders := currentAccess[FieldAuthStrategies]
	_, desiredHasProviders := desiredAccess[FieldAuthStrategies]
	if !currentHasProviders || !desiredHasProviders {
		return currentCompare, desiredCompare
	}

	aliases := aiGatewayAuthStrategyReferenceAliases(rs)
	currentAccess[FieldAuthStrategies] = normalizeAIGatewayReferenceList(
		currentAccess[FieldAuthStrategies], aliases,
	)
	desiredAccess[FieldAuthStrategies] = normalizeAIGatewayReferenceList(
		desiredAccess[FieldAuthStrategies], aliases,
	)
	currentCompare[FieldAccess] = currentAccess
	desiredCompare[FieldAccess] = desiredAccess
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

func aiGatewayAuthStrategyReferenceAliases(rs *resources.ResourceSet) map[string]string {
	aliases := make(map[string]string)
	if rs == nil {
		return aliases
	}

	for _, provider := range rs.AIGatewayAuthStrategies {
		canonical := firstNonEmpty(provider.Ref, provider.Name, provider.GetKonnectID())
		if canonical == "" {
			continue
		}
		for _, alias := range []string{provider.Ref, provider.Name, provider.GetKonnectID()} {
			if alias != "" {
				aliases[alias] = canonical
			}
		}
	}
	return aliases
}

func normalizeAIGatewayReferenceList(raw any, aliases map[string]string) any {
	switch references := raw.(type) {
	case []any:
		normalized := make([]any, len(references))
		for i, reference := range references {
			if referenceValue, ok := reference.(string); ok {
				normalized[i] = canonicalAIGatewayReference(referenceValue, aliases)
				continue
			}
			normalized[i] = reference
		}
		sortAIGatewayReferences(normalized)
		return normalized
	case []string:
		normalized := make([]any, len(references))
		for i, reference := range references {
			normalized[i] = canonicalAIGatewayReference(reference, aliases)
		}
		sortAIGatewayReferences(normalized)
		return normalized
	default:
		return raw
	}
}

func sortAIGatewayReferences(references []any) {
	for _, reference := range references {
		if _, ok := reference.(string); !ok {
			return
		}
	}
	slices.SortStableFunc(references, func(a, b any) int {
		return strings.Compare(a.(string), b.(string))
	})
}

func canonicalAIGatewayReference(reference string, aliases map[string]string) string {
	if parsedRef, _, ok := tags.ParseRefPlaceholder(reference); ok {
		reference = parsedRef
	}
	if canonical := aliases[reference]; canonical != "" {
		return canonical
	}
	return reference
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

package planner

import (
	"maps"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
)

func normalizeAIGatewayModelVectorDBForComparison(
	current, desired map[string]any,
) {
	vectorConfig := func(payload map[string]any) map[string]any {
		config, _ := payload[FieldConfig].(map[string]any)
		balancer, _ := config[FieldBalancer].(map[string]any)
		vector, _ := balancer["vectordb"].(map[string]any)
		return vector
	}
	currentVector, desiredVector := vectorConfig(current), vectorConfig(desired)
	if currentVector[FieldType] != "pgvector" || desiredVector[FieldType] != "pgvector" {
		return
	}
	if aiGatewayModelSharesDatastore(current, desired) {
		// A datastore supplies connection fields omitted by the model. Project
		// only those fields; search settings and explicit overrides remain owned.
		observedVector := maps.Clone(currentVector)
		resources.OmitAIGatewayModelDatastoreConnections(current)
		for key, value := range desiredVector {
			if observed, exists := observedVector[key]; exists {
				currentVector[key] = observed
				if key == FieldSSL {
					currentVector[key] = projectAIGatewayPolicyConfigValue(observed, value)
				}
			}
		}
	} else {
		// Reviewed API defaults from AIGatewayModelVectorDBConfigPgVector.
		// The SDK patch preserves request omission; it does not change the API
		// defaults. Keep non-default observed values as real drift.
		pruneAIGatewayPgvectorDefaults(currentVector, desiredVector, map[string]any{
			"host": "127.0.0.1", "database": "kong-pgvector", "port": float64(5432),
			"user": "postgres", "timeout": float64(5000),
		})
		currentSSL, _ := currentVector[FieldSSL].(map[string]any)
		desiredSSL, _ := desiredVector[FieldSSL].(map[string]any)
		pruneAIGatewayPgvectorDefaults(currentSSL, desiredSSL, map[string]any{
			"enabled": true, "required": true, "verify": true, "version": "tlsv1_2",
		})
	}
	pruneEmptyContainersMissingFromPeer(current, desired)
}

func pruneAIGatewayPgvectorDefaults(current, desired, defaults map[string]any) {
	for key, value := range defaults {
		if _, explicit := desired[key]; !explicit && reflect.DeepEqual(current[key], value) {
			delete(current, key)
		}
	}
}

func aiGatewayModelSharesDatastore(current, desired map[string]any) bool {
	currentStores, _ := current[FieldDatastores].([]any)
	desiredStores, _ := desired[FieldDatastores].([]any)
	currentNames := make(map[string]bool, len(currentStores))
	for _, raw := range currentStores {
		store, _ := raw.(map[string]any)
		name, _ := store[FieldName].(string)
		if name != "" {
			currentNames[name] = true
		}
	}
	for _, raw := range desiredStores {
		store, _ := raw.(map[string]any)
		name, _ := store[FieldName].(string)
		if currentNames[name] {
			return true
		}
	}
	return false
}

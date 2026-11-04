package planner

import (
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
)

func normalizeAIGatewayModelVectorDBForComparison(
	current, desired map[string]any,
	rs *resources.ResourceSet,
	gatewayRef string,
) {
	vectorConfig := func(payload map[string]any) map[string]any {
		config, _ := payload[FieldConfig].(map[string]any)
		balancer, _ := config["balancer"].(map[string]any)
		vector, _ := balancer["vectordb"].(map[string]any)
		return vector
	}
	currentVector, desiredVector := vectorConfig(current), vectorConfig(desired)
	if currentVector[FieldType] != "pgvector" || desiredVector[FieldType] != "pgvector" {
		return
	}
	if aiGatewayModelUsesDeclaredVectorDatastore(desired, rs, gatewayRef) {
		// A datastore supplies connection fields omitted by the model. Project
		// only those fields; search settings and explicit overrides remain owned.
		for _, key := range []string{"host", "database", "port", "user", "password", "timeout", "ssl"} {
			if value, explicit := desiredVector[key]; explicit {
				if key == "ssl" {
					currentVector[key] = projectAIGatewayPolicyConfigValue(currentVector[key], value)
				}
			} else {
				delete(currentVector, key)
			}
		}
	} else {
		// The SDK preserves omission, while the API still supplies inline
		// pgvector defaults. Keep non-default observed values as real drift.
		pruneAIGatewayPgvectorDefaults(currentVector, desiredVector, map[string]any{
			"host": "127.0.0.1", "database": "kong-pgvector", "port": float64(5432),
			"user": "postgres", "timeout": float64(5000),
		})
		currentSSL, _ := currentVector["ssl"].(map[string]any)
		desiredSSL, _ := desiredVector["ssl"].(map[string]any)
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

func aiGatewayModelUsesDeclaredVectorDatastore(
	payload map[string]any,
	rs *resources.ResourceSet,
	gatewayRef string,
) bool {
	if rs == nil {
		return false
	}
	ids := aiGateway23GatewayIDs(rs)
	stores, _ := payload[FieldDatastores].([]any)
	for _, raw := range stores {
		store, _ := raw.(map[string]any)
		for _, declared := range rs.AIGatewayDatastores {
			if declared.DatastoreType() == "vectordb" && store[FieldName] == declared.Name() &&
				aiGateway23SameGateway(ids, declared.AIGateway, gatewayRef) {
				return true
			}
		}
	}
	return false
}

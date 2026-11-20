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

// normalizeAIGatewaySessionAffinityForComparison prevents spurious diffs on
// config.balancer.session_affinity: literal API defaults (enabled, ttl) omitted
// from the manifest, and redis connection fields inherited from a datastore
// referenced with purpose "session-affinity" (mirroring the pgvector handling
// above for vectordb connections).
func normalizeAIGatewaySessionAffinityForComparison(current, desired map[string]any) {
	balancerOf := func(payload map[string]any) map[string]any {
		config, _ := payload[FieldConfig].(map[string]any)
		balancer, _ := config[FieldBalancer].(map[string]any)
		return balancer
	}
	currentBalancer, desiredBalancer := balancerOf(current), balancerOf(desired)
	currentAffinity, _ := currentBalancer[FieldSessionAffinity].(map[string]any)
	desiredAffinity, _ := desiredBalancer[FieldSessionAffinity].(map[string]any)
	if currentAffinity == nil && desiredAffinity == nil {
		return
	}

	pruneAIGatewayPgvectorDefaults(currentAffinity, desiredAffinity, map[string]any{
		"enabled": true, "ttl": float64(21600),
	})
	pruneAIGatewayPgvectorDefaults(desiredAffinity, currentAffinity, map[string]any{
		"enabled": true, "ttl": float64(21600),
	})

	currentStorage, _ := currentAffinity["storage"].(map[string]any)
	desiredStorage, _ := desiredAffinity["storage"].(map[string]any)
	switch {
	case currentStorage["type"] == "redis" && desiredStorage["type"] == "redis" &&
		aiGatewayModelSharesDatastore(current, desired):
		// A referenced datastore overrides the inline connection details entirely (the
		// API rejects relying on both), so an inline value on either side is moot. Unlike
		// vectordb, this storage type's SDK struct fills unset connection fields with
		// literal defaults on marshal rather than omitting them, so desiredStorage having
		// a key does not mean the user wrote it. Drop the whole connection surface from
		// both sides rather than risk treating a filled-in default as observed drift.
		resources.OmitAIGatewayModelDatastoreConnections(current)
		for _, key := range []string{
			"host", "port", "database", "username", "password", "ssl", "ssl_verify", "server_name",
			"connect_timeout", "read_timeout", "send_timeout", "connection_is_proxied",
			"cluster", "sentinel", "cloud_authentication", "keepalive",
		} {
			delete(desiredStorage, key)
		}
	case currentStorage["type"] == "local":
		// Reviewed API default: omitted storage resolves to local, per-node state.
		pruneAIGatewayPgvectorDefaults(currentStorage, desiredStorage, map[string]any{"type": "local"})
	case currentStorage["type"] == "redis" && desiredStorage["type"] == "redis":
		pruneAIGatewayPgvectorDefaults(currentStorage, desiredStorage, map[string]any{
			"port": float64(6379), "database": float64(0),
			"connect_timeout": float64(2000), "read_timeout": float64(2000), "send_timeout": float64(2000),
			"connection_is_proxied": false, "ssl": true, "ssl_verify": true,
		})
	}

	// Bottom-up: collapsing an inner container to empty only lets the next
	// level up see it as droppable once the peer at that level also lacks it.
	pruneEmptyContainersMissingFromPeer(currentStorage, desiredStorage)
	pruneEmptyContainersMissingFromPeer(desiredStorage, currentStorage)
	pruneEmptyContainersMissingFromPeer(currentAffinity, desiredAffinity)
	pruneEmptyContainersMissingFromPeer(desiredAffinity, currentAffinity)
	pruneEmptyContainersMissingFromPeer(currentBalancer, desiredBalancer)
	pruneEmptyContainersMissingFromPeer(desiredBalancer, currentBalancer)
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

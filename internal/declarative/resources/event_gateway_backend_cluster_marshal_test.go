package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func TestEventGatewayBackendClusterResourceMarshalJSONIncludesMetadata(t *testing.T) {
	desc := "cluster"
	meta := int64(60)
	auth := kkComps.CreateBackendClusterAuthenticationSchemeAnonymous(kkComps.BackendClusterAuthenticationAnonymous{})

	cluster := EventGatewayBackendClusterResource{
		CreateBackendClusterRequest: kkComps.CreateBackendClusterRequest{
			Name:                          "backend",
			Description:                   &desc,
			Authentication:                auth,
			BootstrapServers:              []string{"host:9092"},
			TLS:                           kkComps.BackendClusterTLS{Enabled: true},
			MetadataUpdateIntervalSeconds: &meta,
			Labels:                        map[string]string{"env": "test"},
		},
		Ref:          "cluster-ref",
		EventGateway: "gateway-ref",
	}

	raw, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if payload["ref"] != "cluster-ref" {
		t.Fatalf("expected ref %q, got %v", "cluster-ref", payload["ref"])
	}
	if payload["event_gateway"] != "gateway-ref" {
		t.Fatalf("expected event_gateway %q, got %v", "gateway-ref", payload["event_gateway"])
	}
	if payload["name"] != "backend" {
		t.Fatalf("expected name %q, got %v", "backend", payload["name"])
	}
}

package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func TestAPIImplementationResourceMarshalJSONIncludesMetadata(t *testing.T) {
	service := kkComps.APIImplementationService{
		ID:             "svc-id",
		ControlPlaneID: "cp-id",
	}
	impl := kkComps.CreateAPIImplementationServiceReference(kkComps.ServiceReference{Service: &service})

	resource := APIImplementationResource{
		APIImplementation: impl,
		Ref:               "impl-ref",
		API:               "api-ref",
	}

	raw, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if payload["ref"] != "impl-ref" {
		t.Fatalf("expected ref %q, got %v", "impl-ref", payload["ref"])
	}
	if payload["api"] != "api-ref" {
		t.Fatalf("expected api %q, got %v", "api-ref", payload["api"])
	}

	serviceVal, ok := payload["service"].(map[string]any)
	if !ok {
		t.Fatalf("expected service payload, got %v", payload["service"])
	}
	if serviceVal["id"] != "svc-id" {
		t.Fatalf("expected service id %q, got %v", "svc-id", serviceVal["id"])
	}
	if serviceVal["control_plane_id"] != "cp-id" {
		t.Fatalf("expected control_plane_id %q, got %v", "cp-id", serviceVal["control_plane_id"])
	}
}

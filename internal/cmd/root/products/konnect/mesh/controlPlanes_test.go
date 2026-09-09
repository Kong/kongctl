package mesh

import (
	"encoding/json"
	"testing"
)

// The Konnect control plane list uses a different envelope from the one a
// control plane's own API uses for resource lists, so both are decoded.
func TestControlPlanesResponseDecoding(t *testing.T) {
	body := `{
	  "data": [
	    {"id":"11111111-1111-1111-1111-111111111111","name":"prod","version":"v3",
	     "labels":{"team":"mesh"},"created_at":"2026-09-07T10:29:26Z",
	     "features":[{"type":"MeshCreation","meshCreation":{"enabled":false}}]},
	    {"id":"22222222-2222-2222-2222-222222222222","name":"legacy","version":"v0"}
	  ],
	  "meta": {"page": {"number": 1, "size": 100, "total": 2}}
	}`

	var payload controlPlanesResponse
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}

	if len(payload.Data) != 2 {
		t.Fatalf("expected 2 control planes, got %d", len(payload.Data))
	}
	if payload.Meta.Page.Total != 2 {
		t.Errorf("total = %d, want 2", payload.Meta.Page.Total)
	}

	first := payload.Data[0]
	if first.Name != "prod" || first.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("unexpected first control plane: %+v", first)
	}
	// Version is the API line, not the control plane's version. A v0 entry is
	// a 2.14 control plane, which this command surface does not support.
	if first.Version != "v3" || payload.Data[1].Version != "v0" {
		t.Errorf("unexpected API lines: %q, %q", first.Version, payload.Data[1].Version)
	}
	if first.Labels["team"] != "mesh" {
		t.Errorf("labels were dropped: %v", first.Labels)
	}
	// Features vary in shape and are passed through rather than modelled.
	if len(first.Features) != 1 {
		t.Errorf("features were dropped: %v", first.Features)
	}
}

// Absent optional fields must not fail the decode: only id and name are
// dependably present.
func TestControlPlanesResponseTolerantOfMissingFields(t *testing.T) {
	var payload controlPlanesResponse
	err := json.Unmarshal([]byte(`{"data":[{"id":"x","name":"y"}],"meta":{}}`), &payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.Data[0].Version != "" || payload.Data[0].Labels != nil {
		t.Errorf("expected zero values, got %+v", payload.Data[0])
	}
}

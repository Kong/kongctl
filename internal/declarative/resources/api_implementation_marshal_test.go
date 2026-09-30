package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, apiImplementationTypeService, payload["type"])

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

func TestAPIImplementationResourceMarshalJSONControlPlane(t *testing.T) {
	resource := APIImplementationResource{
		APIImplementation: kkComps.CreateAPIImplementationControlPlaneReference(kkComps.ControlPlaneReference{
			ControlPlane: &kkComps.APIImplementationControlPlaneInput{ID: "cp-id"},
		}),
		Ref: "impl-ref",
		API: "api-ref",
	}

	raw, err := json.Marshal(resource)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.Equal(t, apiImplementationTypeControlPlane, payload["type"])
	assert.Equal(t, "cp-id", payload["control_plane"].(map[string]any)["control_plane_id"])
}

func TestAPIImplementationResourceUnmarshalJSONVariants(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  kkComps.APIImplementationType
		wantCPID  string
		wantSvcID string
		wantErr   string
	}{
		{
			name:     "control plane with type",
			input:    `{"ref":"impl","type":"control_plane","control_plane":{"control_plane_id":"cp"}}`,
			wantType: kkComps.APIImplementationTypeControlPlaneReference,
			wantCPID: "cp",
		},
		{
			name:     "control plane without type",
			input:    `{"ref":"impl","control_plane":{"control_plane_id":"cp"}}`,
			wantType: kkComps.APIImplementationTypeControlPlaneReference,
			wantCPID: "cp",
		},
		{
			name:      "service without type",
			input:     `{"ref":"impl","service":{"id":"svc","control_plane_id":"cp"}}`,
			wantType:  kkComps.APIImplementationTypeServiceReference,
			wantSvcID: "svc",
		},
		{
			name:    "mismatched type",
			input:   `{"ref":"impl","type":"service","control_plane":{"control_plane_id":"cp"}}`,
			wantErr: "does not match",
		},
		{
			name: "both payloads",
			input: `{"ref":"impl","service":{"id":"svc","control_plane_id":"cp"},` +
				`"control_plane":{"control_plane_id":"cp"}}`,
			wantErr: "exactly one",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resource APIImplementationResource
			err := json.Unmarshal([]byte(tt.input), &resource)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, resource.Type)
			if tt.wantCPID != "" {
				assert.Equal(t, tt.wantCPID, resource.getControlPlane().ID)
			}
			if tt.wantSvcID != "" {
				assert.Equal(t, tt.wantSvcID, resource.getService().ID)
			}
		})
	}
}

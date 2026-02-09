package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func TestAPIPublicationResourceMarshalJSONIncludesMetadata(t *testing.T) {
	vis := kkComps.APIPublicationVisibilityPublic
	autoApprove := true

	pub := APIPublicationResource{
		APIPublication: kkComps.APIPublication{
			AuthStrategyIds:          []string{"strategy-1"},
			AutoApproveRegistrations: &autoApprove,
			Visibility:               &vis,
		},
		Ref:      "pub-ref",
		API:      "api-ref",
		PortalID: "portal-ref",
	}

	raw, err := json.Marshal(pub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if payload["ref"] != "pub-ref" {
		t.Fatalf("expected ref %q, got %v", "pub-ref", payload["ref"])
	}
	if payload["portal_id"] != "portal-ref" {
		t.Fatalf("expected portal_id %q, got %v", "portal-ref", payload["portal_id"])
	}
	if payload["api"] != "api-ref" {
		t.Fatalf("expected api %q, got %v", "api-ref", payload["api"])
	}

	auth, ok := payload["auth_strategy_ids"].([]any)
	if !ok || len(auth) != 1 || auth[0] != "strategy-1" {
		t.Fatalf("unexpected auth_strategy_ids payload: %v", payload["auth_strategy_ids"])
	}
}

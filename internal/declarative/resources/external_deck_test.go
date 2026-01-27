package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestExternalBlockValidateDeckRequires(t *testing.T) {
	selectorName := &ExternalSelector{MatchFields: map[string]string{"name": "svc"}}

	external := &ExternalBlock{
		Selector: selectorName,
		Requires: map[string]any{"deck": map[string]any{"files": []string{"gateway-service.yaml"}}},
	}

	err := external.Validate()
	require.ErrorContains(t, err, "_external.requires is no longer supported")
}

func TestControlPlaneValidateDeckConfig(t *testing.T) {
	cp := ControlPlaneResource{
		CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
			Name: "cp",
		},
		Ref:  "cp",
		Deck: &DeckConfig{Files: []string{"gateway-service.yaml"}},
	}

	require.NoError(t, cp.Validate())
}

func TestPortalValidateRejectsExternalRequires(t *testing.T) {
	portal := PortalResource{
		Ref: "portal",
		External: &ExternalBlock{
			Selector: &ExternalSelector{MatchFields: map[string]string{"name": "portal"}},
			Requires: map[string]any{"deck": map[string]any{"files": []string{"gateway-service.yaml"}}},
		},
	}

	require.ErrorContains(t, portal.Validate(), "_external.requires is no longer supported")
}

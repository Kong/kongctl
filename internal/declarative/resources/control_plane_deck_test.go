package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneValidateDeckConfig(t *testing.T) {
	cp := ControlPlaneResource{
		CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
			Name: "cp",
		},
		BaseResource: BaseResource{Ref: "cp"},
		Deck:         &DeckConfig{Files: []string{"gateway-service.yaml"}},
	}

	require.NoError(t, cp.Validate())
}

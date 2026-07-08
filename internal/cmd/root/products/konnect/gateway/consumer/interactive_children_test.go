package consumer

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneIDFromParentTypedNilPointer(t *testing.T) {
	var cp *kkComps.ControlPlane

	id, err := controlPlaneIDFromParent(cp)

	require.Error(t, err)
	require.Empty(t, id)
}

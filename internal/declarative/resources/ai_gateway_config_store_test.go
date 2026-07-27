package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIGatewayConfigStoreValidateDisplayName(t *testing.T) {
	t.Run("accepts API-supported characters", func(t *testing.T) {
		displayName := "Support.Store_1~-Updated"
		resource := AIGatewayConfigStoreResource{
			BaseResource: BaseResource{Ref: "support-store"},
			AIGateway:    "support-gateway",
			Name:         "support-store",
			DisplayName:  &displayName,
		}

		require.NoError(t, resource.Validate())
	})

	t.Run("rejects spaces", func(t *testing.T) {
		displayName := "Support Store"
		resource := AIGatewayConfigStoreResource{
			BaseResource: BaseResource{Ref: "support-store"},
			AIGateway:    "support-gateway",
			Name:         "support-store",
			DisplayName:  &displayName,
		}

		err := resource.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "display_name")
	})
}

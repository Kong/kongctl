package portal

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestFindApplicationByName(t *testing.T) {
	apps := []kkComps.Application{
		{
			Type: kkComps.ApplicationTypeKeyAuthApplication,
			KeyAuthApplication: &kkComps.KeyAuthApplication{
				ID:   "11111111-1111-1111-1111-111111111111",
				Name: "checkout-app",
			},
		},
		{
			Type: kkComps.ApplicationTypeClientCredentialsApplication,
			ClientCredentialsApplication: &kkComps.ClientCredentialsApplication{
				ID:   "22222222-2222-2222-2222-222222222222",
				Name: "payments-gateway",
			},
		},
	}

	t.Run("matches by name ignoring case", func(t *testing.T) {
		result := findApplicationByName(apps, "Checkout-App")
		require.NotNil(t, result)
		require.Equal(t, "checkout-app", result.KeyAuthApplication.GetName())
	})

	t.Run("matches by identifier", func(t *testing.T) {
		result := findApplicationByName(apps, "22222222-2222-2222-2222-222222222222")
		require.NotNil(t, result)
		require.Equal(t, "payments-gateway", result.ClientCredentialsApplication.GetName())
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		require.Nil(t, findApplicationByName(apps, "unknown"))
	})
}

func TestMatchIDFromApplication(t *testing.T) {
	tests := []struct {
		name string
		app  kkComps.Application
		want string
	}{
		{
			name: "key auth",
			app: kkComps.Application{
				Type: kkComps.ApplicationTypeKeyAuthApplication,
				KeyAuthApplication: &kkComps.KeyAuthApplication{
					ID: "key-auth-id",
				},
			},
			want: "key-auth-id",
		},
		{
			name: "client credentials",
			app: kkComps.Application{
				Type: kkComps.ApplicationTypeClientCredentialsApplication,
				ClientCredentialsApplication: &kkComps.ClientCredentialsApplication{
					ID: "client-credentials-id",
				},
			},
			want: "client-credentials-id",
		},
		{
			name: "unknown type",
			app: kkComps.Application{
				Type: "custom",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, matchID(tt.app))
		})
	}
}

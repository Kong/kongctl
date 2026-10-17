package helpers_test

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestFeaturePaginationAndULIDResolution(t *testing.T) {
	server := fixture.NewFeatureServer(
		t,
		kkComps.Feature{ID: fixture.FeatureID, Key: "first"},
		kkComps.Feature{ID: "01G65Z755AFWAKHE12NY0CQ9FJ", Key: "second"},
	)
	api := server.SDK.GetFeaturesAPI()
	values, err := helpers.ListAllFeatures(t.Context(), api, 1)
	require.NoError(t, err)
	require.Len(t, values, 2)
	for _, id := range []string{"first", fixture.FeatureID} {
		m, err := helpers.ResolveFeature(t.Context(), api, id, 1)
		require.NoError(t, err)
		require.Equal(t, fixture.FeatureID, m.ID)
	}
	_, err = helpers.ResolveFeature(t.Context(), api, "display-name", 1)
	require.ErrorContains(t, err, "API key or ULID")
}

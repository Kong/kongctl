package dump

import (
	"encoding/json"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/fieldcontracts"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

// Inspect exported pointers without the SDK's separate wire-default marshaler.
type (
	portalVisibilityExportProjection kkComps.CreatePortal
	portalVisibilityListFixture      kkComps.ListPortalsResponsePortal
	portalVisibilityDetailFixture    kkComps.PortalResponse
)

func TestPortalVisibilityContractExport(t *testing.T) {
	fields := []string{
		fieldcontracts.FieldDefaultAPIVisibility, fieldcontracts.FieldDefaultPageVisibility,
	}
	for _, field := range fields {
		for _, value := range []any{nil, "", "public", "private", "future-value"} {
			t.Run(fmt.Sprintf("%s/%v", field, value), func(t *testing.T) {
				data, err := json.Marshal(map[string]any{field: value})
				require.NoError(t, err)
				var list portalVisibilityListFixture
				var detail portalVisibilityDetailFixture
				require.NoError(t, json.Unmarshal(data, &list))
				require.NoError(t, json.Unmarshal(data, &detail))
				for _, resource := range []resources.PortalResource{
					mapPortalToDeclarativeResource(kkComps.ListPortalsResponsePortal(list)),
					mapPortalResponseToDeclarativeResource(kkComps.PortalResponse(detail)),
				} {
					data, err := json.Marshal(portalVisibilityExportProjection(resource.CreatePortal))
					require.NoError(t, err)
					var actual map[string]any
					require.NoError(t, json.Unmarshal(data, &actual))
					expected := value
					if value == "" {
						expected = nil
					}
					require.Equal(t, expected, actual[field])
					for _, other := range fields {
						if other != field {
							require.Nil(t, actual[other])
						}
					}
				}
			})
		}
	}
}

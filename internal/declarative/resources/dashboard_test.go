package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardResourceValidate(t *testing.T) {
	valid := DashboardResource{
		BaseResource: BaseResource{Ref: "traffic-summary"},
		Name:         "Traffic Summary",
		Definition: kkComps.Dashboard{
			Tiles: []kkComps.Tile{},
		},
	}

	require.NoError(t, valid.Validate())
}

func TestDashboardResourceValidateRequiresFields(t *testing.T) {
	tests := []struct {
		name    string
		dash    DashboardResource
		wantErr string
	}{
		{
			name: "ref",
			dash: DashboardResource{
				Name: "Traffic Summary",
				Definition: kkComps.Dashboard{
					Tiles: []kkComps.Tile{},
				},
			},
			wantErr: "invalid dashboard ref",
		},
		{
			name: "name",
			dash: DashboardResource{
				BaseResource: BaseResource{Ref: "traffic-summary"},
				Definition: kkComps.Dashboard{
					Tiles: []kkComps.Tile{},
				},
			},
			wantErr: "name is required for dashboard traffic-summary",
		},
		{
			name: "definition",
			dash: DashboardResource{
				BaseResource: BaseResource{Ref: "traffic-summary"},
				Name:         "Traffic Summary",
			},
			wantErr: "definition is required for dashboard traffic-summary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dash.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestDashboardResourceUnmarshalRejectsUnknownFields(t *testing.T) {
	var dash DashboardResource
	err := json.Unmarshal([]byte(`{
		"ref": "traffic-summary",
		"name": "Traffic Summary",
		"definition": {"tiles": []},
		"unexpected": true
	}`), &dash)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
)

func TestExtractPortalFieldsIncludesSIPREnabled(t *testing.T) {
	tests := []struct {
		name  string
		value bool
	}{
		{name: "enabled", value: true},
		{name: "disabled", value: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			portal := resources.PortalResource{
				CreatePortal: kkComps.CreatePortal{
					Name:        "customer-portal",
					SiprEnabled: &tt.value,
				},
			}

			fields := extractPortalFields(portal)

			assert.Equal(t, tt.value, fields[FieldSIPREnabled])
		})
	}
}

func TestShouldUpdatePortalSIPREnabled(t *testing.T) {
	tests := []struct {
		name        string
		current     *bool
		desired     *bool
		wantUpdate  bool
		wantNew     any
		wantOld     any
		wantChanged bool
	}{
		{
			name:        "enable",
			current:     new(false),
			desired:     new(true),
			wantUpdate:  true,
			wantNew:     true,
			wantOld:     false,
			wantChanged: true,
		},
		{
			name:        "disable",
			current:     new(true),
			desired:     new(false),
			wantUpdate:  true,
			wantNew:     false,
			wantOld:     true,
			wantChanged: true,
		},
		{
			name:    "unchanged",
			current: new(true),
			desired: new(true),
		},
		{
			name:    "omitted preserves current value",
			current: new(true),
		},
		{
			name:        "missing current value differs from desired",
			desired:     new(true),
			wantUpdate:  true,
			wantNew:     true,
			wantChanged: true,
		},
	}

	planner := &portalPlannerImpl{BasePlanner: &BasePlanner{}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := state.Portal{
				ListPortalsResponsePortal: kkComps.ListPortalsResponsePortal{
					SiprEnabled: tt.current,
				},
			}
			desired := resources.PortalResource{
				CreatePortal: kkComps.CreatePortal{SiprEnabled: tt.desired},
			}

			needsUpdate, updates, changedFields := planner.shouldUpdatePortal(current, desired)

			assert.Equal(t, tt.wantUpdate, needsUpdate)
			if tt.wantUpdate {
				assert.Equal(t, tt.wantNew, updates[FieldSIPREnabled])
			} else {
				assert.NotContains(t, updates, FieldSIPREnabled)
			}
			change, changed := changedFields[FieldSIPREnabled]
			assert.Equal(t, tt.wantChanged, changed)
			if tt.wantChanged {
				assert.Equal(t, tt.wantOld, change.Old)
				assert.Equal(t, tt.wantNew, change.New)
			}
		})
	}
}

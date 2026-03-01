package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldUpdateListener_FullPayloadAndChangedFields(t *testing.T) {
	oldDesc := "old description"
	newDesc := "new description"
	oldPort := "80"
	newPort := "443"

	current := state.EventGatewayListener{
		EventGatewayListener: kkComps.EventGatewayListener{
			Name:        "listener-a",
			Description: &oldDesc,
			Addresses:   []string{"0.0.0.0"},
			Ports: []kkComps.EventGatewayListenerPort{
				{
					Type: kkComps.EventGatewayListenerPortTypeStr,
					Str:  &oldPort,
				},
			},
			Labels: map[string]string{
				"team": "platform",
			},
		},
	}

	desired := resources.EventGatewayListenerResource{
		CreateEventGatewayListenerRequest: kkComps.CreateEventGatewayListenerRequest{
			Name:        "listener-a",
			Description: &newDesc,
			Addresses:   []string{"0.0.0.0"},
			Ports: []kkComps.EventGatewayListenerPort{
				{
					Type: kkComps.EventGatewayListenerPortTypeStr,
					Str:  &newPort,
				},
			},
			Labels: map[string]string{
				"team": "platform",
			},
		},
		Ref: "listener-a",
	}

	p := &Planner{}
	needsUpdate, updateFields, changedFields := p.shouldUpdateListener(current, desired)

	require.True(t, needsUpdate)

	// Listener updates use PUT payloads, so unchanged required fields are included.
	assert.Equal(t, "listener-a", updateFields["name"])
	assert.Equal(t, []string{"0.0.0.0"}, updateFields["addresses"])
	assert.Equal(t, []string{"443"}, updateFields["ports"])
	assert.Equal(t, "new description", updateFields["description"])

	// changed_fields should contain only actual deltas.
	require.Contains(t, changedFields, "description")
	require.Contains(t, changedFields, "ports")
	assert.NotContains(t, changedFields, "name")
	assert.NotContains(t, changedFields, "addresses")

	assert.Equal(t, oldDesc, changedFields["description"].Old)
	assert.Equal(t, newDesc, changedFields["description"].New)
	assert.Equal(t, []string{"80"}, changedFields["ports"].Old)
	assert.Equal(t, []string{"443"}, changedFields["ports"].New)
}

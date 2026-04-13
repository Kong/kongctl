package planner

import (
	"log/slog"
	"os"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStaticKeyPlanner returns a minimal Planner suitable for unit tests.
func newTestStaticKeyPlanner() *Planner {
	return &Planner{
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

// newTestStaticKeyPlan returns a minimal Plan with the given mode.
func newTestStaticKeyPlan() *Plan {
	return &Plan{
		Metadata: PlanMetadata{Mode: "apply"},
	}
}

// ---------------------------------------------------------------------------
// doesStaticKeyNeedChange
// ---------------------------------------------------------------------------

func TestDoesStaticKeyNeedChange_NoChanges(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()

	desc := "a description"
	current := state.EventGatewayStaticKey{
		EventGatewayStaticKey: kkComps.EventGatewayStaticKey{
			ID:          "key-123",
			Name:        "my-key",
			Description: &desc,
			Labels:      map[string]string{"env": "prod"},
		},
	}
	desired := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:        "my-key",
			Value:       "secret",
			Description: &desc,
			Labels:      map[string]string{"env": "prod"},
		},
		Ref: "my-key-ref",
	}

	assert.False(t, p.doesStaticKeyNeedChange(current, desired))
}

func TestDoesStaticKeyNeedChange_DescriptionChanged(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()

	oldDesc := "old description"
	newDesc := "new description"
	current := state.EventGatewayStaticKey{
		EventGatewayStaticKey: kkComps.EventGatewayStaticKey{
			ID:          "key-123",
			Name:        "my-key",
			Description: &oldDesc,
		},
	}
	desired := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:        "my-key",
			Value:       "secret",
			Description: &newDesc,
		},
		Ref: "my-key-ref",
	}

	assert.True(t, p.doesStaticKeyNeedChange(current, desired))
}

func TestDoesStaticKeyNeedChange_LabelsChanged(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()

	current := state.EventGatewayStaticKey{
		EventGatewayStaticKey: kkComps.EventGatewayStaticKey{
			ID:     "key-123",
			Name:   "my-key",
			Labels: map[string]string{"env": "staging"},
		},
	}
	desired := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:   "my-key",
			Value:  "secret",
			Labels: map[string]string{"env": "prod"},
		},
		Ref: "my-key-ref",
	}

	assert.True(t, p.doesStaticKeyNeedChange(current, desired))
}

func TestDoesStaticKeyNeedChange_ValueChangedNotDetected(t *testing.T) {
	t.Parallel()

	// The value field is write-only; the API never returns it.
	// A value change cannot be detected and must NOT trigger a replace.
	p := newTestStaticKeyPlanner()

	current := state.EventGatewayStaticKey{
		EventGatewayStaticKey: kkComps.EventGatewayStaticKey{
			ID:   "key-123",
			Name: "my-key",
		},
	}
	desired := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:  "my-key",
			Value: "changed-secret",
		},
		Ref: "my-key-ref",
	}

	assert.False(t, p.doesStaticKeyNeedChange(current, desired),
		"value changes are undetectable and must not trigger a replace")
}

// ---------------------------------------------------------------------------
// planStaticKeyCreate
// ---------------------------------------------------------------------------

func TestPlanStaticKeyCreate_WithExistingGateway(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()
	plan := newTestStaticKeyPlan()

	desc := "key description"
	key := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:        "my-key",
			Value:       "s3cr3t",
			Description: &desc,
			Labels:      map[string]string{"team": "platform"},
		},
		Ref: "my-key-ref",
	}

	p.planStaticKeyCreate("default", "gw-ref", "my-gateway", "gw-id", key, nil, plan)

	require.Len(t, plan.Changes, 1)
	c := plan.Changes[0]
	assert.Equal(t, ActionCreate, c.Action)
	assert.Equal(t, ResourceTypeEventGatewayStaticKey, c.ResourceType)
	assert.Equal(t, "my-key-ref", c.ResourceRef)
	assert.Equal(t, "my-key", c.Fields["name"])
	assert.Equal(t, "s3cr3t", c.Fields["value"])
	assert.Equal(t, "key description", c.Fields["description"])
	assert.Equal(t, map[string]string{"team": "platform"}, c.Fields["labels"])
	require.NotNil(t, c.Parent)
	assert.Equal(t, "gw-id", c.Parent.ID)
	assert.Equal(t, "gw-ref", c.Parent.Ref)
}

func TestPlanStaticKeyCreate_ForNewGateway_HasReference(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()
	plan := newTestStaticKeyPlan()

	key := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:  "my-key",
			Value: "val",
		},
		Ref: "my-key-ref",
	}

	// gatewayID="" signals gateway doesn't exist yet
	p.planStaticKeyCreate("default", "gw-ref", "my-gateway", "", key, []string{"dep-1"}, plan)

	require.Len(t, plan.Changes, 1)
	c := plan.Changes[0]
	assert.Nil(t, c.Parent, "parent should not be set when gateway doesn't exist yet")
	require.Contains(t, c.References, "event_gateway_id")
	assert.Equal(t, "gw-ref", c.References["event_gateway_id"].Ref)
	assert.Contains(t, c.DependsOn, "dep-1")
}

// ---------------------------------------------------------------------------
// planStaticKeyDelete
// ---------------------------------------------------------------------------

func TestPlanStaticKeyDelete_ReturnsChangeID(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()
	plan := newTestStaticKeyPlan()

	changeID := p.planStaticKeyDelete("gw-ref", "my-gateway", "gw-id", "key-id-1", "my-key", plan)

	require.NotEmpty(t, changeID)
	require.Len(t, plan.Changes, 1)
	c := plan.Changes[0]
	assert.Equal(t, ActionDelete, c.Action)
	assert.Equal(t, ResourceTypeEventGatewayStaticKey, c.ResourceType)
	assert.Equal(t, "key-id-1", c.ResourceID)
	assert.Equal(t, changeID, c.ID)
}

// ---------------------------------------------------------------------------
// planStaticKeyCreatesForNewGateway
// ---------------------------------------------------------------------------

func TestPlanStaticKeyCreatesForNewGateway_WithGatewayDependency(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()
	plan := newTestStaticKeyPlan()

	keys := []resources.EventGatewayStaticKeyResource{
		{
			EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
				Name:  "key-a",
				Value: "val-a",
			},
			Ref: "key-a",
		},
		{
			EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
				Name:  "key-b",
				Value: "val-b",
			},
			Ref: "key-b",
		},
	}

	p.planStaticKeyCreatesForNewGateway("ns", "gw-ref", "my-gateway", "gw-change-id", keys, plan)

	require.Len(t, plan.Changes, 2)
	for _, c := range plan.Changes {
		assert.Equal(t, ActionCreate, c.Action)
		assert.Contains(t, c.DependsOn, "gw-change-id",
			"each static key create must depend on gateway create")
	}
}

func TestPlanStaticKeyCreatesForNewGateway_NoGatewayChangeID(t *testing.T) {
	t.Parallel()

	p := newTestStaticKeyPlanner()
	plan := newTestStaticKeyPlan()

	key := resources.EventGatewayStaticKeyResource{
		EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{
			Name:  "key-a",
			Value: "val",
		},
		Ref: "key-a",
	}

	p.planStaticKeyCreatesForNewGateway(
		"ns", "gw-ref", "my-gateway", "",
		[]resources.EventGatewayStaticKeyResource{key}, plan)

	require.Len(t, plan.Changes, 1)
	assert.Empty(t, plan.Changes[0].DependsOn)
}

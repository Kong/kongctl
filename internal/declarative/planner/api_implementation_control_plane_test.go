package planner

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanAPIImplementationCreateControlPlane(t *testing.T) {
	planner := NewPlanner(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	planner.resources = &resources.ResourceSet{}
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
	implementation := resources.APIImplementationResource{
		Ref: "implementation",
		APIImplementation: kkComps.CreateAPIImplementationControlPlaneReference(kkComps.ControlPlaneReference{
			ControlPlane: &kkComps.APIImplementationControlPlaneInput{ID: "control-plane-id"},
		}),
	}

	planner.planAPIImplementationCreate("default", "api", "api-id", implementation, nil, plan)
	require.Len(t, plan.Changes, 1)
	controlPlane := plan.Changes[0].Fields[FieldControlPlane].(map[string]any)
	assert.Equal(t, "control-plane-id", controlPlane[FieldControlPlaneID])
}

func TestPlanAPIImplementationDeleteControlPlane(t *testing.T) {
	planner := NewPlanner(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	implementation := state.APIImplementation{
		ID: "implementation-id",
		ControlPlane: &struct{ ID string }{
			ID: "control-plane-id",
		},
	}

	planner.planAPIImplementationDelete("default", "api", "api-id", implementation, plan)
	require.Len(t, plan.Changes, 1)
	assert.Equal(t, ActionDelete, plan.Changes[0].Action)
	controlPlane := plan.Changes[0].Fields[FieldControlPlane].(map[string]any)
	assert.Equal(t, "control-plane-id", controlPlane[FieldControlPlaneID])
}

func TestResolveAPIImplementationControlPlaneReferencePreservesForwardReference(t *testing.T) {
	planner := NewPlanner(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	controlPlane := resources.ControlPlaneResource{BaseResource: resources.BaseResource{Ref: "control-plane"}}

	resolved, err := planner.resolveAPIImplementationControlPlaneReference(
		fmt.Sprintf("%scontrol-plane#id", tags.RefPlaceholderPrefix),
		map[string]*resources.ControlPlaneResource{"control-plane": &controlPlane},
		"implementation",
	)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%scontrol-plane#id", tags.RefPlaceholderPrefix), resolved)
}

func TestDeckControlPlaneRefFromFields(t *testing.T) {
	fields := map[string]any{FieldControlPlane: map[string]any{
		FieldControlPlaneID: fmt.Sprintf("%scontrol-plane#id", tags.RefPlaceholderPrefix),
	}}
	assert.Equal(t, "control-plane", deckControlPlaneRefFromFields(fields, map[string]string{"control-plane": "deck"}))
}

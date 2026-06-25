package executor

import (
	"context"
	"testing"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestEventGatewayControlPlaneAdapterMapCreateFieldsMinRuntimeVersion(t *testing.T) {
	fields := map[string]any{
		planner.FieldName:              "event-gateway",
		planner.FieldMinRuntimeVersion: "1.2",
	}

	var create components.CreateGatewayRequest
	err := (&EventGatewayControlPlaneControlPlaneAdapter{}).MapCreateFields(
		context.Background(),
		&ExecutionContext{Namespace: "default"},
		fields,
		&create,
	)

	require.NoError(t, err)
	require.NotNil(t, create.MinRuntimeVersion)
	require.Equal(t, "1.2", *create.MinRuntimeVersion)
}

func TestEventGatewayControlPlaneAdapterMapUpdateFieldsMinRuntimeVersion(t *testing.T) {
	fields := map[string]any{
		planner.FieldMinRuntimeVersion: "1.2",
	}

	var update components.UpdateGatewayRequest
	err := (&EventGatewayControlPlaneControlPlaneAdapter{}).MapUpdateFields(
		context.Background(),
		&ExecutionContext{Namespace: "default"},
		fields,
		&update,
		nil,
	)

	require.NoError(t, err)
	require.NotNil(t, update.MinRuntimeVersion)
	require.Equal(t, "1.2", *update.MinRuntimeVersion)
}

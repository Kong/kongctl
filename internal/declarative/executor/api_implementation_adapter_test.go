package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIImplementationAdapterMapCreateFields(t *testing.T) {
	tests := []struct {
		name     string
		fields   map[string]any
		wantType kkComps.APIImplementationType
		wantErr  string
	}{
		{
			name: "service",
			fields: map[string]any{planner.FieldService: map[string]any{
				planner.FieldID: "service-id", planner.FieldControlPlaneID: "control-plane-id",
			}},
			wantType: kkComps.APIImplementationTypeServiceReference,
		},
		{
			name: "control plane",
			fields: map[string]any{planner.FieldControlPlane: map[string]any{
				planner.FieldControlPlaneID: "control-plane-id",
			}},
			wantType: kkComps.APIImplementationTypeControlPlaneReference,
		},
		{name: "neither", fields: map[string]any{}, wantErr: "exactly one"},
		{
			name: "both",
			fields: map[string]any{
				planner.FieldService:      map[string]any{},
				planner.FieldControlPlane: map[string]any{},
			},
			wantErr: "exactly one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request kkComps.APIImplementation
			err := NewAPIImplementationAdapter(nil).MapCreateFields(
				context.Background(), nil, tt.fields, &request,
			)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, request.Type)
		})
	}
}

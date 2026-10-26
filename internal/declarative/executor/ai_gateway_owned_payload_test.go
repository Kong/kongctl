package executor

import (
	"errors"
	"maps"
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"

	"github.com/stretchr/testify/require"
)

type failingOwnedChildMarshaler struct {
	cause error
}

func (m failingOwnedChildMarshaler) MarshalJSON() ([]byte, error) {
	return nil, m.cause
}

func TestOwnedChildPayloadErrorsDoNotExposeCredentials(t *testing.T) {
	const credential = "synthetic-credential-must-not-appear"
	cause := errors.New("invalid credential: " + credential)
	for _, tc := range []struct {
		name, message string
		request       any
	}{
		{
			name: "custom marshaler error", message: "cannot encode AI Gateway child payload",
			request: failingOwnedChildMarshaler{cause: cause},
		},
		{
			name: "non-object JSON", message: "cannot decode AI Gateway child payload",
			request: credential,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := ownedChildPayload(tc.request)
			require.Nil(t, payload)
			require.EqualError(t, err, tc.message)
			require.NotContains(t, err.Error(), credential)
			require.NotErrorIs(t, err, cause)
		})
	}
}

func TestOwnedChildUnprotectPreservesOmittedLabels(t *testing.T) {
	currentMetadata := resources.BuildChildManagedBy(nil, nil, "team-a", true, "dev")
	requestedMetadata := resources.BuildChildManagedBy(currentMetadata, nil, "team-a", false, "dev")
	current := map[string]any{
		planner.FieldName: "policy", planner.FieldType: "rate-limiting",
		planner.FieldGlobal: true, planner.FieldConfig: map[string]any{}, planner.FieldManagedBy: currentMetadata,
	}
	desired := maps.Clone(current)
	desired[planner.FieldManagedBy] = requestedMetadata
	change := planner.PlannedChange{
		Namespace: "team-a", Fields: desired,
		ChangedFields: map[string]planner.FieldChange{planner.FieldManagedBy: {Old: currentMetadata, New: requestedMetadata}},
		Protection:    planner.ProtectionChange{Old: true, New: false},
	}
	payload, err := prepareOwnedMappedChildUpdate("AI Gateway policy", current, desired, NewExecutionContext(&change))
	require.NoError(t, err)
	require.NotContains(t, payload, planner.FieldLabels)
}

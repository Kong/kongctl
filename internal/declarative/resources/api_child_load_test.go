package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIChildLoadRegistrations(t *testing.T) {
	want := []ResourceType{
		ResourceTypeAPIVersion,
		ResourceTypeAPIPublication,
		ResourceTypeAPIImplementation,
		ResourceTypeAPIDocument,
	}

	tests := []struct {
		name          string
		registrations []childLoadRegistration
	}{
		// Extraction order also determines per-parent validation order.
		{"extraction", childExtractors[ResourceTypeAPI]},
		{"validation", childValidators[ResourceTypeAPI]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make([]ResourceType, 0, len(tt.registrations))
			for _, registration := range tt.registrations {
				got = append(got, registration.kind)
			}

			require.Equal(t, want, got)
		})
	}
}

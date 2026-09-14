package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestControlPlaneChildLoadRegistrations(t *testing.T) {
	want := []ResourceType{
		ResourceTypeGatewayService,
		ResourceTypeControlPlaneDataPlaneCertificate,
	}

	tests := []struct {
		name          string
		registrations []childLoadRegistration
	}{
		{"extraction", childExtractors[ResourceTypeControlPlane]},
		{"validation", childValidators[ResourceTypeControlPlane]},
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

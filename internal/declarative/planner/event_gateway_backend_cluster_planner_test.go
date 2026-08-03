package planner

import (
	"testing"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestCompareTLSSettingsTreatsOmittedVersionsAsAPIDefaults(t *testing.T) {
	tests := []struct {
		name    string
		current []components.TLSVersions
	}{
		{
			name: "service materializes defaults",
			current: []components.TLSVersions{
				components.TLSVersionsTls12,
				components.TLSVersionsTls13,
			},
		},
		{
			name:    "service omits defaults",
			current: nil,
		},
	}
	desired := components.BackendClusterTLS{Enabled: false}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := components.BackendClusterTLS{
				Enabled:     false,
				TLSVersions: tt.current,
			}

			require.True(t, compareTLSSettings(current, desired))
		})
	}
}

func TestCompareTLSSettingsPreservesExplicitVersions(t *testing.T) {
	current := components.BackendClusterTLS{
		Enabled: false,
		TLSVersions: []components.TLSVersions{
			components.TLSVersionsTls12,
			components.TLSVersionsTls13,
		},
	}

	tests := []struct {
		name     string
		desired  []components.TLSVersions
		expected bool
	}{
		{
			name: "matching",
			desired: []components.TLSVersions{
				components.TLSVersionsTls12,
				components.TLSVersionsTls13,
			},
			expected: true,
		},
		{
			name:     "different",
			desired:  []components.TLSVersions{components.TLSVersionsTls13},
			expected: false,
		},
		{
			name:     "explicitly empty",
			desired:  []components.TLSVersions{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desired := components.BackendClusterTLS{
				Enabled:     false,
				TLSVersions: tt.desired,
			}

			require.Equal(t, tt.expected, compareTLSSettings(current, desired))
		})
	}
}

func TestCompareTLSSettingsTreatsOmittedInsecureSkipVerifyAsFalse(t *testing.T) {
	falseValue := false
	trueValue := true

	tests := []struct {
		name     string
		current  *bool
		desired  *bool
		expected bool
	}{
		{
			name:     "service materializes false",
			current:  &falseValue,
			expected: true,
		},
		{
			name:     "desired explicitly sets false",
			desired:  &falseValue,
			expected: true,
		},
		{
			name:     "matching true values",
			current:  &trueValue,
			desired:  &trueValue,
			expected: true,
		},
		{
			name:     "service true differs from omitted desired value",
			current:  &trueValue,
			expected: false,
		},
		{
			name:     "service false differs from desired true",
			current:  &falseValue,
			desired:  &trueValue,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := components.BackendClusterTLS{InsecureSkipVerify: tt.current}
			desired := components.BackendClusterTLS{InsecureSkipVerify: tt.desired}

			require.Equal(t, tt.expected, compareTLSSettings(current, desired))
		})
	}
}

package planner

import (
	"encoding/json"
	"testing"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestInternalSDKBackendAuthenticationPreservesAWSIAM(t *testing.T) {
	input := `{"type":"sasl_aws_iam","sasl_aws_iam":{"type":"assume_role",
		"assume_role":{"arn":"arn:aws:iam::123456789012:role/example"}}}`
	var current components.BackendClusterAuthenticationSensitiveDataAwareScheme
	var desired components.BackendClusterAuthenticationScheme
	require.NoError(t, json.Unmarshal([]byte(input), &current))
	require.NoError(t, json.Unmarshal([]byte(input), &desired))
	require.True(t, compareAuthenticationSchemes(current, desired))
	data, err := json.Marshal(backendClusterAuthenticationFields(desired))
	require.NoError(t, err)
	require.JSONEq(t, input, string(data))
	desired.BackendClusterAuthenticationSaslAwsIam.SaslAwsIam.
		BackendClusterAuthenticationSaslAwsIamAssumeRole.AssumeRole.Arn = "arn:aws:iam::123456789012:role/changed"
	require.False(t, compareAuthenticationSchemes(current, desired))
}

func TestShouldUpdateBackendCluster_EmptyLabelKeyChanged(t *testing.T) {
	t.Parallel()

	current := state.EventGatewayBackendCluster{
		BackendCluster: components.BackendCluster{
			Labels: map[string]string{"old": ""},
			Authentication: components.BackendClusterAuthenticationSensitiveDataAwareScheme{
				Type: components.BackendClusterAuthenticationSensitiveDataAwareSchemeTypeAnonymous,
			},
		},
	}
	desired := resources.EventGatewayBackendClusterResource{
		CreateBackendClusterRequest: components.CreateBackendClusterRequest{
			Labels: map[string]string{"new": ""},
			Authentication: components.BackendClusterAuthenticationScheme{
				Type: components.BackendClusterAuthenticationSchemeTypeAnonymous,
			},
		},
	}

	p := &Planner{}
	needsUpdate, updates, changes := p.shouldUpdateBackendCluster(current, desired)
	require.True(t, needsUpdate)
	require.Equal(t, desired.Labels, updates[FieldLabels])
	require.Equal(t, map[string]FieldChange{
		FieldLabels: {Old: current.Labels, New: desired.Labels},
	}, changes)
}

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

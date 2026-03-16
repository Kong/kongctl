package resources

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventGatewayClusterPolicyResource_UnmarshalJSON_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		errContains string
	}{
		{
			name:        "missing type field",
			input:       `{"ref":"p","config":{"rules":[{"principal":"*","resource":"*","permission":"allow"}]}}`,
			errContains: "cluster policy requires 'type' field",
		},
		{
			name:        "wrong type value",
			input:       `{"ref":"p","type":"unknown","config":{"rules":[{"principal":"*","resource":"*","permission":"allow"}]}}`,
			errContains: "cluster policy 'type' must be 'acls'",
		},
		{
			name:        "type is not a string",
			input:       `{"ref":"p","type":42,"config":{"rules":[{"principal":"*","resource":"*","permission":"allow"}]}}`,
			errContains: "cluster policy 'type' must be a string",
		},
		{
			name:        "missing config field",
			input:       `{"ref":"p","type":"acls"}`,
			errContains: "cluster policy requires 'config' field",
		},
		{
			name:        "config is not an object",
			input:       `{"ref":"p","type":"acls","config":"bad"}`,
			errContains: "cluster policy 'config' must be an object",
		},
		{
			name:        "missing config.rules field",
			input:       `{"ref":"p","type":"acls","config":{}}`,
			errContains: "cluster policy config requires 'rules' field",
		},
		{
			name:        "config.rules is not an array",
			input:       `{"ref":"p","type":"acls","config":{"rules":"bad"}}`,
			errContains: "cluster policy config 'rules' must be an array",
		},
		{
			name:        "config.rules is empty",
			input:       `{"ref":"p","type":"acls","config":{"rules":[]}}`,
			errContains: "cluster policy config 'rules' must have at least one element",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var policy EventGatewayClusterPolicyResource
			err := json.Unmarshal([]byte(tt.input), &policy)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestEventGatewayClusterPolicyResource_UnmarshalJSON_ValidPolicy(t *testing.T) {
	input := `{
		"ref": "my-policy",
		"virtual_cluster": "vc-ref",
		"type": "acls",
		"name": "deny-all",
		"config": {
			"rules": [
				{
					"action": "deny",
					"resource_type": "transactional_id",
					"operations": [{"name": "describe_configs"}],
					"resource_names": [{"match": "*"}]
				}
			]
		}
	}`

	var policy EventGatewayClusterPolicyResource
	err := json.Unmarshal([]byte(input), &policy)
	require.NoError(t, err)

	assert.Equal(t, "my-policy", policy.Ref)
	assert.Equal(t, "vc-ref", policy.VirtualCluster)
	require.NotNil(t, policy.EventGatewayACLsPolicy)
	require.NotNil(t, policy.EventGatewayACLsPolicy.Name)
	assert.Equal(t, "deny-all", *policy.EventGatewayACLsPolicy.Name)
}

func TestEventGatewayClusterPolicyResource_UnmarshalJSON_RejectsKongctlMetadata(t *testing.T) {
	input := `{"ref":"p","type":"acls","kongctl":{"some":"meta"},"config":{"rules":[{}]}}`

	var policy EventGatewayClusterPolicyResource
	err := json.Unmarshal([]byte(input), &policy)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kongctl metadata not supported on child resources")
}

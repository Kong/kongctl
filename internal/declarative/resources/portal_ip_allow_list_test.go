package resources

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPortalIPAllowListResourceValidate(t *testing.T) {
	tests := []struct {
		name        string
		resource    PortalIPAllowListResource
		expectedErr string
	}{
		{
			name: "valid IP and CIDR values",
			resource: PortalIPAllowListResource{
				Ref:        "portal-allow-list",
				AllowedIPs: []string{"192.0.2.10", "2001:db8::/32", "198.51.100.0/24"},
			},
		},
		{
			name: "requires allowed_ips",
			resource: PortalIPAllowListResource{
				Ref: "portal-allow-list",
			},
			expectedErr: "allowed_ips must contain at least one IP address or CIDR block",
		},
		{
			name: "rejects invalid IP value",
			resource: PortalIPAllowListResource{
				Ref:        "portal-allow-list",
				AllowedIPs: []string{"not-an-ip"},
			},
			expectedErr: "allowed_ips[0] must be an IP address or CIDR block: \"not-an-ip\"",
		},
		{
			name: "rejects duplicate values",
			resource: PortalIPAllowListResource{
				Ref:        "portal-allow-list",
				AllowedIPs: []string{"192.0.2.10", "192.0.2.10"},
			},
			expectedErr: "allowed_ips[1] duplicates \"192.0.2.10\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.resource.Validate()
			if tt.expectedErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.expectedErr)
		})
	}
}

func TestPortalIPAllowListResourceRejectsKongctlMetadata(t *testing.T) {
	var resource PortalIPAllowListResource
	err := json.Unmarshal([]byte(`{
		"ref": "portal-allow-list",
		"allowed_ips": ["192.0.2.10"],
		"kongctl": {"protected": true}
	}`), &resource)

	require.EqualError(t, err, "kongctl metadata not supported on portal IP allow lists")
}

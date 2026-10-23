package loader

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoaderControlPlaneLabels(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		count     int
		protected bool
		wantErr   bool
	}{
		{name: "space in value", key: "owner", value: "has space", wantErr: true},
		{name: "at in value", key: "owner", value: "user@example", wantErr: true},
		{name: "leading dash", key: "owner", value: "-team", wantErr: true},
		{name: "trailing punctuation", key: "owner", value: "team_", wantErr: true},
		{name: "empty value", key: "owner", wantErr: true},
		{name: "long value", key: "owner", value: strings.Repeat("a", 64), wantErr: true},
		{name: "long key", key: strings.Repeat("a", 64), value: "team", wantErr: true},
		{name: "space in key", key: "cost centre", value: "team", wantErr: true},
		{name: "leading punctuation in key", key: "-owner", value: "team", wantErr: true},
		{name: "unicode value", key: "owner", value: "café", wantErr: true},
		{name: "reserved prefix", key: "konnect_owner", value: "team", wantErr: true},
		{name: "reserved kong prefix", key: "kong_owner", value: "team", wantErr: true},
		{name: "reserved mesh prefix", key: "mesh_owner", value: "team", wantErr: true},
		{name: "reserved kic prefix", key: "kic_owner", value: "team", wantErr: true},
		{name: "reserved lowercase kongctl prefix", key: "kongctl-owner", value: "team", wantErr: true},
		{name: "valid boundaries", key: strings.Repeat("a", 63), value: strings.Repeat("Z", 63)},
		{name: "valid punctuation", key: "cost_centre", value: "A-b_c.123"},
		{name: "single character", key: "a", value: "1"},
		{name: "49 user labels", count: 49},
		{name: "50 user labels", count: 50, wantErr: true},
		{name: "48 protected user labels", count: 48, protected: true},
		{name: "49 protected user labels", count: 49, protected: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input strings.Builder
			fmt.Fprintf(&input, `control_planes:
  - ref: cp-1
    name: cp-1
    kongctl:
      namespace: demo
      protected: %t
    labels:
`, tt.protected)
			if tt.count > 0 {
				for i := range tt.count {
					fmt.Fprintf(&input, "      label%d: value\n", i)
				}
			} else {
				fmt.Fprintf(&input, "      %q: %q\n", tt.key, tt.value)
			}
			_, err := New().LoadFile(writeLoaderTestFile(t, input.String()))
			if tt.wantErr {
				require.ErrorContains(t, err, "control_plane")
				require.ErrorContains(t, err, "label")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentityDirectoryResourceValidateTTLBounds(t *testing.T) {
	validMin := IdentityDirectoryMinTTLSecs
	validMax := IdentityDirectoryMaxTTLSecs
	tooLow := IdentityDirectoryMinTTLSecs - 1
	tooHigh := IdentityDirectoryMaxTTLSecs + 1

	tests := []struct {
		name    string
		update  func(*IdentityDirectoryResource)
		wantErr string
	}{
		{
			name: "allows omitted TTL fields",
		},
		{
			name: "allows minimum TTL fields",
			update: func(directory *IdentityDirectoryResource) {
				directory.TTLSecs = &validMin
				directory.NegativeTTLSecs = &validMin
			},
		},
		{
			name: "allows maximum TTL fields",
			update: func(directory *IdentityDirectoryResource) {
				directory.TTLSecs = &validMax
				directory.NegativeTTLSecs = &validMax
			},
		},
		{
			name: "rejects ttl_secs below minimum",
			update: func(directory *IdentityDirectoryResource) {
				directory.TTLSecs = &tooLow
			},
			wantErr: "ttl_secs must be between 300 and 86400",
		},
		{
			name: "rejects ttl_secs above maximum",
			update: func(directory *IdentityDirectoryResource) {
				directory.TTLSecs = &tooHigh
			},
			wantErr: "ttl_secs must be between 300 and 86400",
		},
		{
			name: "rejects negative_ttl_secs below minimum",
			update: func(directory *IdentityDirectoryResource) {
				directory.NegativeTTLSecs = &tooLow
			},
			wantErr: "negative_ttl_secs must be between 300 and 86400",
		},
		{
			name: "rejects negative_ttl_secs above maximum",
			update: func(directory *IdentityDirectoryResource) {
				directory.NegativeTTLSecs = &tooHigh
			},
			wantErr: "negative_ttl_secs must be between 300 and 86400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directory := IdentityDirectoryResource{
				BaseResource: BaseResource{Ref: "directory"},
				Name:         "directory",
			}
			if tt.update != nil {
				tt.update(&directory)
			}

			err := directory.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

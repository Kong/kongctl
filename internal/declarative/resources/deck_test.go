package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeckConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *DeckConfig
		wantErr bool
	}{
		{
			name:   "valid config",
			config: &DeckConfig{Files: []string{"gateway-service.yaml"}},
		},
		{
			name:    "missing files",
			config:  &DeckConfig{},
			wantErr: true,
		},
		{
			name:    "file cannot be flag",
			config:  &DeckConfig{Files: []string{"--foo"}},
			wantErr: true,
		},
		{
			name:    "flag must be flag",
			config:  &DeckConfig{Files: []string{"gateway-service.yaml"}, Flags: []string{"not-a-flag"}},
			wantErr: true,
		},
		{
			name:    "flag cannot include konnect auth",
			config:  &DeckConfig{Files: []string{"gateway-service.yaml"}, Flags: []string{"--konnect-token=abc"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		err := tt.config.Validate()
		if tt.wantErr {
			require.Error(t, err, tt.name)
			continue
		}
		require.NoError(t, err, tt.name)
	}
}

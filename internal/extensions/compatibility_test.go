package extensions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		manifest   Manifest
		cliVersion string
		want       CompatibilityResult
		wantErr    string
	}{
		{
			name:       "no compatibility metadata",
			manifest:   Manifest{},
			cliVersion: "0.20.0",
			want: CompatibilityResult{
				Compatible:     true,
				CurrentVersion: "0.20.0",
			},
		},
		{
			name: "minimum version allows same version",
			manifest: Manifest{Compatibility: Compatibility{
				MinVersion: "0.20.0",
			}},
			cliVersion: "0.20.0",
			want: CompatibilityResult{
				Compatible:     true,
				CurrentVersion: "0.20.0",
				Constraint:     ">= 0.20.0",
			},
		},
		{
			name: "minimum version rejects older version",
			manifest: Manifest{Compatibility: Compatibility{
				MinVersion: "0.20.0",
			}},
			cliVersion: "0.19.9",
			want: CompatibilityResult{
				Compatible:     false,
				CurrentVersion: "0.19.9",
				Constraint:     ">= 0.20.0",
			},
		},
		{
			name: "maximum version is inclusive",
			manifest: Manifest{Compatibility: Compatibility{
				MaxVersion: "0.25.0",
			}},
			cliVersion: "0.25.0",
			want: CompatibilityResult{
				Compatible:     true,
				CurrentVersion: "0.25.0",
				Constraint:     "<= 0.25.0",
			},
		},
		{
			name: "maximum version rejects newer version",
			manifest: Manifest{Compatibility: Compatibility{
				MaxVersion: "0.25.0",
			}},
			cliVersion: "0.25.1",
			want: CompatibilityResult{
				Compatible:     false,
				CurrentVersion: "0.25.1",
				Constraint:     "<= 0.25.0",
			},
		},
		{
			name: "wildcard maximum constrains major lane",
			manifest: Manifest{Compatibility: Compatibility{
				MinVersion: "0.20.0",
				MaxVersion: "0.x",
			}},
			cliVersion: "0.50.0",
			want: CompatibilityResult{
				Compatible:     true,
				CurrentVersion: "0.50.0",
				Constraint:     ">= 0.20.0, 0.x",
			},
		},
		{
			name: "wildcard maximum rejects next major",
			manifest: Manifest{Compatibility: Compatibility{
				MinVersion: "0.20.0",
				MaxVersion: "0.x",
			}},
			cliVersion: "1.0.0",
			want: CompatibilityResult{
				Compatible:     false,
				CurrentVersion: "1.0.0",
				Constraint:     ">= 0.20.0, 0.x",
			},
		},
		{
			name: "prefixed versions",
			manifest: Manifest{Compatibility: Compatibility{
				MinVersion: "v0.20.0",
			}},
			cliVersion: "v0.21.0",
			want: CompatibilityResult{
				Compatible:     true,
				CurrentVersion: "v0.21.0",
				Constraint:     ">= v0.20.0",
			},
		},
		{
			name: "unknown development version",
			manifest: Manifest{Compatibility: Compatibility{
				MinVersion: "0.20.0",
			}},
			cliVersion: "dev",
			want: CompatibilityResult{
				Compatible:     true,
				Unknown:        true,
				CurrentVersion: "dev",
				Constraint:     ">= 0.20.0",
			},
		},
		{
			name: "invalid cli version",
			manifest: Manifest{Compatibility: Compatibility{
				MinVersion: "0.20.0",
			}},
			cliVersion: "not-a-version",
			wantErr:    "parse kongctl version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckCompatibility(tt.manifest, tt.cliVersion)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEnsureCompatibleReturnsActionableError(t *testing.T) {
	manifest := Manifest{
		Publisher: "kong",
		Name:      "debug",
		Compatibility: Compatibility{
			MinVersion: "9.0.0",
		},
	}

	err := EnsureCompatible(manifest, "1.0.0")

	require.ErrorContains(t, err, "extension kong/debug is not compatible")
	require.ErrorContains(t, err, "Required: >= 9.0.0")
	require.ErrorContains(t, err, "Current:  1.0.0")
}

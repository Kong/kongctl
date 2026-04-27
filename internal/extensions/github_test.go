package extensions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGitHubSource(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		ref       string
		wantOK    bool
		wantOwner string
		wantRepo  string
		wantRef   string
		wantErr   string
	}{
		{
			name:      "owner repo",
			source:    "kong/kongctl-ext-debug",
			ref:       "v1.0.0",
			wantOK:    true,
			wantOwner: "kong",
			wantRepo:  "kongctl-ext-debug",
			wantRef:   "v1.0.0",
		},
		{
			name:      "https URL",
			source:    "https://github.com/Kong/kongctl-ext-debug.git",
			wantOK:    true,
			wantOwner: "Kong",
			wantRepo:  "kongctl-ext-debug",
		},
		{
			name:      "ssh URL",
			source:    "git@github.com:kong/kongctl-ext-debug.git",
			wantOK:    true,
			wantOwner: "kong",
			wantRepo:  "kongctl-ext-debug",
		},
		{
			name:   "local path",
			source: "./extensions/debug",
			wantOK: false,
		},
		{
			name:    "invalid owner",
			source:  "bad_owner/repo",
			wantOK:  true,
			wantErr: "invalid GitHub source",
		},
		{
			name:   "too many path segments",
			source: "kong/team/repo",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, ok, err := ParseGitHubSource(tt.source, tt.ref)

			require.Equal(t, tt.wantOK, ok)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantOwner, source.Owner)
			require.Equal(t, tt.wantRepo, source.Repo)
			require.Equal(t, tt.wantRef, source.Ref)
		})
	}
}

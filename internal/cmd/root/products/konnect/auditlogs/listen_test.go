package auditlogs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildEndpointFromPublicURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		baseURL   string
		path      string
		want      string
		expectErr bool
	}{
		{
			name:    "base with root path",
			baseURL: "https://example.ngrok.app",
			path:    "/audit-logs",
			want:    "https://example.ngrok.app/audit-logs",
		},
		{
			name:    "base with existing path",
			baseURL: "https://example.ngrok.app/forwarded",
			path:    "/audit-logs",
			want:    "https://example.ngrok.app/forwarded/audit-logs",
		},
		{
			name:      "invalid base URL",
			baseURL:   "://broken",
			path:      "/audit-logs",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := buildEndpointFromPublicURL(tt.baseURL, tt.path)
			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoader_LoadsGroupedAuditLogWebhookDestinations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit-logs.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
audit-logs:
  destinations:
    - ref: foo
      _external:
        selector:
          matchFields:
            name: foo
`), 0o600))

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AuditLogs.Destinations, 1)

	destination := rs.AuditLogs.Destinations[0]
	require.Equal(t, "foo", destination.GetRef())
	require.NotNil(t, destination.External)
	require.NotNil(t, destination.External.Selector)
	require.Equal(t, "foo", destination.External.Selector.MatchFields["name"])
}

package get

import (
	"testing"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/stretchr/testify/require"
)

func TestAuditLogPullCommandForms(t *testing.T) {
	t.Parallel()

	command, err := NewGetCmd()
	require.NoError(t, err)
	for _, path := range [][]string{{"audit-logs"}, {"konnect", "audit-logs"}} {
		found, _, err := command.Find(path)
		require.NoError(t, err)
		require.Equal(t, "audit-logs", found.Name())
		require.Equal(t, []string{"audit-log"}, found.Aliases)
		require.Contains(t, found.Flag("since").Usage, "30s, 15m, 2h, 24h, 168h, 1h30m")
		require.Contains(t, found.Flag("start-time").Usage, "2026-08-23T14:00:00Z")
		require.Contains(t, found.Flag("start-time").Usage, "2026-08-23T09:00:00-05:00")
		require.Contains(t, found.Flag("end-time").Usage, "2026-08-24T14:00:00Z")
		require.Contains(t, found.Flag("end-time").Usage, "2026-08-24T09:00:00-05:00")
		require.NotNil(t, found.RunE)
		for _, flag := range []string{
			"start-time", "end-time", "since", "type", "limit", "page-size", "follow", "poll-interval",
		} {
			require.NotNilf(t, found.Flag(flag), "expected %v to expose --%s", path, flag)
		}
		require.Equal(t, "100", found.Flag("page-size").DefValue)
		require.Contains(t, cmdcommon.AllowedOutputFormats(found), "jsonl")
		for _, childName := range []string{"destinations", "destination", "webhook"} {
			child, _, err := found.Find([]string{childName})
			require.NoError(t, err)
			require.NotContains(t, cmdcommon.AllowedOutputFormats(child), "jsonl")
			require.Error(t, cmdcommon.ValidateOutputFormat(child, "jsonl"))
		}
	}
}

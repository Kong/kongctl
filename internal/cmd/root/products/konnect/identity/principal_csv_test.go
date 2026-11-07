package identity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/stretchr/testify/require"
)

func TestPrincipalCSVInspectionRequiresExplicitSourceAndMode(t *testing.T) {
	command, err := newPrincipalCSVInspectCommand(nil, nil)
	require.NoError(t, err)
	require.Error(t, command.ValidateRequiredFlags())
	require.NoError(t, command.Flags().Set("file", "principals.csv"))
	require.Error(t, command.ValidateRequiredFlags())
	require.NoError(t, command.Flags().Set("metadata-mode", "merge"))
	require.NoError(t, command.ValidateRequiredFlags())
	require.Error(t, command.Args(command, []string{"extra"}))
	group, err := NewIdentityCmd(verbs.Inspect, nil, nil)
	require.NoError(t, err)
	child, _, err := group.Find([]string{"principal-csv"})
	require.NoError(t, err)
	require.Equal(t, "principal-csv", child.Name())
}

func TestPrincipalCSVInspectionReportsCountsWithoutRowValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "principals.csv")
	require.NoError(
		t,
		os.WriteFile(
			path,
			[]byte("ref,display_name,metadata\nworker,private-display-name,\"{\"\"team\"\":\"\"private-team\"\"}\"\n"),
			0o600,
		),
	)
	value, err := loader.InspectPrincipalCSV(path, "merge")
	require.NoError(t, err)
	require.Equal(t, 1, value.Rows)
	require.False(t, value.RemoteStateChecked)
	var text bytes.Buffer
	require.NoError(t, printPrincipalCSVSummary(&text, value))
	require.Contains(t, text.String(), "remote_state_checked")
	require.NotContains(t, text.String(), "private-")
	data, err := json.Marshal(value)
	require.NoError(t, err)
	require.NotContains(t, string(data), "private-")
	_, err = loader.InspectPrincipalCSV(path, "invalid")
	require.Error(t, err)
}

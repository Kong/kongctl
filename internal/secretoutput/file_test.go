package secretoutput

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReservedOutputIsPrivateExclusiveAndTracksCapture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issued.json")
	metadata := Metadata{AuthServerID: "server", ClientID: "client"}
	output, err := Reserve(path, metadata)
	require.NoError(t, err)
	t.Cleanup(func() { _ = output.Close() })
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Zero(t, info.Mode().Perm()&0o077)
	_, err = Reserve(path, metadata)
	require.Error(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var pending record
	require.NoError(t, json.Unmarshal(data, &pending))
	require.Equal(t, "creation_not_confirmed", pending.Status)
	require.Empty(t, pending.Secret)
	metadata.SecretID = "secret-id"
	require.NoError(t, output.Capture(metadata, "private-material"))
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	var captured record
	require.NoError(t, json.Unmarshal(data, &captured))
	require.Equal(t, record{Metadata: metadata, Status: "captured", Secret: "private-material"}, captured)
	require.NoError(t, output.Close())
	_, err = Reserve(path, metadata)
	require.Error(t, err, "capture never overwrites an earlier credential")
}

func TestOutputRefusesSymlinksAndRetainsFailureEvidence(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, []byte("preserve"), 0o600))
	require.NoError(t, os.Symlink(target, link))
	_, err := Reserve(link, Metadata{})
	require.Error(t, err)
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "preserve", string(data))
	for _, failure := range []string{"missing-secret", "write-failure", "replaced-path"} {
		t.Run(failure, func(t *testing.T) {
			path := filepath.Join(dir, failure)
			output, err := Reserve(path, Metadata{ClientID: "client"})
			require.NoError(t, err)
			t.Cleanup(func() { _ = output.Close() })
			value := ""
			if failure == "write-failure" {
				require.NoError(t, output.file.Close())
				value = "never-in-error"
			}
			if failure == "replaced-path" {
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.WriteFile(path, []byte("preserve-replacement"), 0o600))
				value = "never-in-error"
			}
			err = output.Capture(Metadata{ClientID: "client", SecretID: "created-id"}, value)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "never-in-error")
			_, err = os.Stat(path)
			require.NoError(t, err, "capture failures retain the file for recovery")
			if failure == "replaced-path" {
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, "preserve-replacement", string(data))
			}
			if failure == "missing-secret" {
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				var missing record
				require.NoError(t, json.Unmarshal(data, &missing))
				require.Equal(t, "secret_missing", missing.Status)
				require.Equal(t, "created-id", missing.SecretID)
			}
		})
	}
}

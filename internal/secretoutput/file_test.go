package secretoutput

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReportHistoryUsesNumberedPrivateJSONFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "report.json")
	for sequence := range 3 {
		metadata := Metadata{DryRun: new(sequence == 0)}
		output, err := ReserveNextReport(path, metadata)
		require.NoError(t, err)
		expected := path
		if sequence != 0 {
			expected = filepath.Join(directory, fmt.Sprintf("report.%d.json", sequence))
		}
		require.Equal(t, expected, output.Path())
		require.NoError(t, output.CaptureReport(metadata, []byte(`{"counts":{}}`)))
	}
	for sequence := range 3 {
		path := filepath.Join(directory, "report.json")
		if sequence != 0 {
			path = filepath.Join(directory, fmt.Sprintf("report.%d.json", sequence))
		}
		contents, err := os.ReadFile(path)
		require.NoError(t, err)
		var value record
		require.NoError(t, json.Unmarshal(contents, &value))
		require.Equal(t, new(sequence == 0), value.DryRun)
		require.Equal(t, "captured", value.Status)
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestReportHistoryRejectsSymlinksAndDirectories(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.json")
	require.NoError(t, os.WriteFile(target, []byte("preserved"), 0o600))
	link := filepath.Join(directory, "link.json")
	require.NoError(t, os.Symlink(target, link))
	for _, path := range []string{directory, link} {
		_, err := ReserveNextReport(path, Metadata{})
		require.ErrorContains(t, err, "not a regular file")
	}
	contents, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "preserved", string(contents))
}

func TestPrivateReportCapturePreservesCompleteOrNonJSONResponses(t *testing.T) {
	for _, body := range [][]byte{[]byte(`{"warnings":[{"detail":"private employee"}],"future":true}`), {0xff, '<'}} {
		path := filepath.Join(t.TempDir(), "report.json")
		metadata := Metadata{DirectoryID: "directory", DryRun: new(false), RequestSHA256: "digest"}
		file, err := ReserveReport(path, metadata)
		require.NoError(t, err)
		pending, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(pending), "request_not_confirmed")
		_, err = ReserveReport(path, metadata)
		require.Error(t, err)
		err = file.CaptureReport(metadata, body)
		require.NoError(t, err)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var value record
		require.NoError(t, json.Unmarshal(data, &value))
		require.Equal(t, metadata, value.Metadata)
		if json.Valid(body) {
			require.JSONEq(t, string(body), string(value.Report))
		} else {
			require.Equal(t, body, value.ResponseBody)
			require.Equal(t, "response_not_json", value.Status)
		}
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		require.NoError(t, file.Close())
	}
}

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

func TestEnrollmentCaptureRetainsCapabilityOnlyInExclusivePrivateRecord(t *testing.T) {
	for _, empty := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "enrollment.json")
		metadata := Metadata{DirectoryID: "directory", ProviderID: "provider", CredentialID: "credential"}
		output, err := Reserve(path, metadata)
		require.NoError(t, err)
		t.Cleanup(func() { _ = output.Close() })
		value := "https://enrollment.example.test/consent?token=private-capability"
		if empty {
			value = ""
		}
		err = output.CaptureEnrollmentURL(metadata, value)
		if empty {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var captured record
		require.NoError(t, json.Unmarshal(data, &captured))
		require.Equal(t, metadata, captured.Metadata)
		require.Equal(t, value, captured.EnrollmentURL)
		require.Empty(t, captured.Secret)
		if empty {
			require.Equal(t, "enrollment_url_missing", captured.Status)
		} else {
			require.Equal(t, "captured", captured.Status)
		}
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Zero(t, info.Mode().Perm()&0o077)
		_, err = Reserve(path, metadata)
		require.Error(t, err)
	}
}

func TestNilOutputCanBeClosed(t *testing.T) {
	var output *File
	require.NoError(t, output.Close())
}

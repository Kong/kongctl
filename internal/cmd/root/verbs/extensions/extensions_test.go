package extensions

import (
	"bytes"
	"context"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/extensions"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestConfirmRemoteInstallTrustRequiresYes(t *testing.T) {
	helper, in, out := newTrustPromptTestHelper(t)
	_, err := in.WriteString("yes\n")
	require.NoError(t, err)

	confirmed, err := confirmRemoteInstallTrust(
		helper,
		testFetchedGitHubSource(),
		testRemoteCandidate(),
		testPackageObservation(),
		false,
	)

	require.NoError(t, err)
	require.True(t, confirmed)
	require.Contains(t, out.String(), "Remote extension trust confirmation")
	require.Contains(t, out.String(), "Package SHA256")
	require.Contains(t, out.String(), "Type 'yes' to confirm")
}

func TestConfirmRemoteInstallTrustRejectsDecline(t *testing.T) {
	helper, in, _ := newTrustPromptTestHelper(t)
	_, err := in.WriteString("no\n")
	require.NoError(t, err)

	confirmed, err := confirmRemoteInstallTrust(
		helper,
		testFetchedGitHubSource(),
		testRemoteCandidate(),
		testPackageObservation(),
		false,
	)

	require.ErrorContains(t, err, "extension install cancelled")
	require.False(t, confirmed)
}

func TestConfirmRemoteInstallTrustSkipsPromptWithYesFlag(t *testing.T) {
	helper, _, out := newTrustPromptTestHelper(t)

	confirmed, err := confirmRemoteInstallTrust(
		helper,
		testFetchedGitHubSource(),
		testRemoteCandidate(),
		testPackageObservation(),
		true,
	)

	require.NoError(t, err)
	require.True(t, confirmed)
	require.Empty(t, out.String())
}

func TestConfirmRemoteInstallTrustRejectsStructuredOutputWithoutYes(t *testing.T) {
	helper, _, _ := newTrustPromptTestHelper(t)
	cmd := helper.GetCmd()
	cmd.Flags().String(cmdcommon.OutputFlagName, "", "Output format")
	require.NoError(t, cmd.Flags().Set(cmdcommon.OutputFlagName, cmdcommon.JSON.String()))

	confirmed, err := confirmRemoteInstallTrust(
		helper,
		testFetchedGitHubSource(),
		testRemoteCandidate(),
		testPackageObservation(),
		false,
	)

	require.ErrorContains(t, err, "structured output")
	require.False(t, confirmed)
}

func TestConfirmRemoteUpgradeTrustShowsCurrentAndTarget(t *testing.T) {
	helper, in, out := newTrustPromptTestHelper(t)
	_, err := in.WriteString("yes\n")
	require.NoError(t, err)

	confirmed, err := confirmRemoteUpgradeTrust(
		helper,
		testInstalledRemoteExtension(),
		testFetchedGitHubSource(),
		testRemoteCandidate(),
		testPackageObservation(),
		false,
	)

	require.NoError(t, err)
	require.True(t, confirmed)
	require.Contains(t, out.String(), "Current")
	require.Contains(t, out.String(), "Target")
	require.Contains(t, out.String(), "Do you want to upgrade this extension?")
}

func TestParseUpgradeExtensionTarget(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		wantID     string
		wantTarget string
		wantErr    string
	}{
		{
			name:   "id only",
			value:  "kong/debug",
			wantID: "kong/debug",
		},
		{
			name:       "tag target",
			value:      "kong/debug@v0.2.0",
			wantID:     "kong/debug",
			wantTarget: "v0.2.0",
		},
		{
			name:       "version target",
			value:      "kong/debug@0.2.0",
			wantID:     "kong/debug",
			wantTarget: "0.2.0",
		},
		{
			name:       "latest target",
			value:      "kong/debug@latest",
			wantID:     "kong/debug",
			wantTarget: "latest",
		},
		{
			name:    "missing target",
			value:   "kong/debug@",
			wantErr: "target is required",
		},
		{
			name:    "invalid id",
			value:   "debug",
			wantErr: "publisher/name",
		},
		{
			name:    "extra at",
			value:   "kong/debug@v1@other",
			wantErr: "must not contain @",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, target, err := parseUpgradeExtensionTarget(tt.value)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantID, id)
			require.Equal(t, tt.wantTarget, target)
		})
	}
}

func newTrustPromptTestHelper(t *testing.T) (cmdpkg.Helper, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	streams, in, out, _ := iostreams.NewTestIOStreams()
	cmd := &cobra.Command{Use: "install extension"}
	cmd.SetContext(context.WithValue(context.Background(), iostreams.StreamsKey, streams))
	return cmdpkg.BuildHelper(cmd, nil), in, out
}

func testFetchedGitHubSource() extensions.FetchedGitHubSource {
	return extensions.FetchedGitHubSource{
		SourceType: extensions.SourceTypeGitHubReleaseAsset,
		Repository: "kong/kongctl-ext-debug",
		URL:        "https://github.com/kong/kongctl-ext-debug",
		Ref:        "v0.1.0",
		ReleaseTag: "v0.1.0",
		AssetName:  "kongctl-ext-debug-universal.tar.gz",
		AssetURL:   "https://github.com/kong/kongctl-ext-debug/releases/download/v0.1.0/kongctl-ext-debug-universal.tar.gz",
	}
}

func testRemoteCandidate() extensions.Extension {
	return extensions.Extension{
		ID: "kong/debug",
		Manifest: extensions.Manifest{
			Publisher: "kong",
			Name:      "debug",
			Version:   "0.1.0",
			Runtime: extensions.Runtime{
				Command: "kongctl-ext-debug",
			},
		},
		CommandPaths: []extensions.CommandPath{
			{
				Path: []extensions.PathSegment{
					{Name: "get"},
					{Name: "debug-info"},
				},
			},
		},
	}
}

func testInstalledRemoteExtension() extensions.Extension {
	ext := testRemoteCandidate()
	ext.InstallType = extensions.InstallTypeInstalled
	ext.Install = &extensions.InstallState{
		Source: extensions.SourceState{
			Type:       extensions.SourceTypeGitHubReleaseAsset,
			Repository: "kong/kongctl-ext-debug",
			Ref:        "v0.1.0",
			ReleaseTag: "v0.1.0",
		},
		PackageHash: "old-package-sha",
	}
	return ext
}

func testPackageObservation() extensions.PackageObservation {
	return extensions.PackageObservation{
		Manifest: extensions.Manifest{
			Version: "0.1.0",
		},
		ManifestHash:   "manifest-sha",
		RuntimeHash:    "runtime-sha",
		PackageHash:    "package-sha",
		RuntimeCommand: "kongctl-ext-debug",
	}
}

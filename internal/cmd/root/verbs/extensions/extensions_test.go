package extensions

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/extensions"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

var ansiEscapeSequencePattern = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

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
	require.Contains(t, out.String(), "Remote extension trust warning!!")
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
	require.Contains(t, stripANSIEscapeSequences(out.String()), "  Target: kong/kongctl-ext-debug@v0.1.0\n"+
		"  Extension name: kong/debug\n")
	require.Contains(t, out.String(), "Do you want to upgrade this extension?")
}

func TestWriteRemoteTrustPromptKeepsAssetURLAndCompactsHashes(t *testing.T) {
	var out bytes.Buffer
	fetched := testFetchedGitHubSource()
	fetched.AssetURL = "https://github.com/kong/kongctl-ext-debug/releases/download/v0.1.0/" +
		"kongctl-ext-debug-universal-with-a-very-long-name.tar.gz"
	observation := testPackageObservation()
	fullHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	observation.PackageHash = fullHash
	observation.ManifestHash = fullHash
	observation.RuntimeHash = fullHash

	err := writeRemoteTrustPrompt(&out, "upgrade", &extensions.Extension{}, fetched, testRemoteCandidate(), observation)

	require.NoError(t, err)
	plain := stripANSIEscapeSequences(out.String())
	require.NotContains(t, out.String(), fullHash)
	require.Contains(t, out.String(), abbreviateTrustHash(fullHash))
	require.Contains(t, plain, "  Asset URL: "+fetched.AssetURL)
	require.Contains(t, plain, "  Executable: kongctl-ext-debug\n")
	require.Contains(t, plain, "  Executable SHA256: "+abbreviateTrustHash(fullHash)+"\n")
	require.NotContains(t, plain, "\n    https://")
}

func TestWriteRemoteTrustPromptUsesShortTopCopy(t *testing.T) {
	var out bytes.Buffer

	err := writeRemoteTrustPrompt(
		&out,
		"install",
		nil,
		testFetchedGitHubSource(),
		testRemoteCandidate(),
		testPackageObservation(),
	)

	require.NoError(t, err)
	plain := stripANSIEscapeSequences(out.String())
	require.Contains(t, plain, "! Remote extension trust warning!!\n")
	require.Contains(t, plain, "  Source: kong/kongctl-ext-debug@v0.1.0\n"+
		"  Extension name: kong/debug\n")

	shortLines := []string{
		"Remote extension trust warning!!",
		"This extension is executable code.",
		"Install it only if you trust the source.",
		"Review the package before installing.",
		"Do you want to install this extension?",
		"Type 'yes' to confirm:",
	}
	for _, line := range shortLines {
		require.LessOrEqual(t, len(line), 40, line)
		require.Contains(t, plain, line)
	}
}

func stripANSIEscapeSequences(value string) string {
	return ansiEscapeSequencePattern.ReplaceAllString(value, "")
}

func TestExtensionDisplayVersionPrefersManifestVersion(t *testing.T) {
	ext := testInstalledRemoteExtension()
	ext.Manifest.Version = "0.1.0"
	ext.Install.Source.ReleaseTag = "v0.2.0"

	require.Equal(t, "0.1.0", extensionDisplayVersion(ext))
}

func TestExtensionDisplayVersionFallsBackToReleaseTag(t *testing.T) {
	ext := testInstalledRemoteExtension()
	ext.Manifest.Version = ""
	ext.Install.Source.ReleaseTag = "v0.2.0"

	require.Equal(t, "v0.2.0", extensionDisplayVersion(ext))
}

func TestExtensionDisplayVersionFallsBackToSourceRefOrCommit(t *testing.T) {
	ext := testInstalledRemoteExtension()
	ext.Manifest.Version = ""
	ext.Install.Source = extensions.SourceState{
		Type:           extensions.SourceTypeGitHubSource,
		Repository:     "kong/kongctl-ext-debug",
		Ref:            "main",
		ResolvedCommit: "0123456789abcdef0123456789abcdef01234567",
	}
	require.Equal(t, "main", extensionDisplayVersion(ext))

	ext.Install.Source.Ref = ""
	require.Equal(t, "0123456789ab", extensionDisplayVersion(ext))
}

func TestWriteListSummaryUsesSourceVersionFallback(t *testing.T) {
	var out bytes.Buffer
	ext := testInstalledRemoteExtension()
	ext.Manifest.Version = ""
	ext.Install.Source.ReleaseTag = "v0.2.0"

	err := writeListSummary(&out, []extensions.Extension{ext}, "0.20.0")

	require.NoError(t, err)
	require.Contains(t, out.String(), "v0.2.0")
	require.NotContains(t, out.String(), "unversioned")
}

func TestWriteListSummaryMarksIncompatibleExtensions(t *testing.T) {
	var out bytes.Buffer
	ext := testInstalledRemoteExtension()
	ext.Manifest.Compatibility.MinVersion = "9.0.0"

	err := writeListSummary(&out, []extensions.Extension{ext}, "1.0.0")

	require.NoError(t, err)
	plain := stripANSIEscapeSequences(out.String())
	require.Contains(t, plain, "!  kong/debug  installed  0.1.0  incompatible\n")
}

func TestWriteExtensionSummaryShowsCompatibility(t *testing.T) {
	var out bytes.Buffer
	ext := testInstalledRemoteExtension()
	ext.Manifest.Compatibility.MinVersion = "0.20.0"
	ext.Manifest.Compatibility.MaxVersion = "0.x"

	err := writeExtensionSummary(&out, ext, "0.25.0")

	require.NoError(t, err)
	plain := stripANSIEscapeSequences(out.String())
	require.Contains(t, plain, "  Compatibility: compatible\n")
	require.Contains(t, plain, "  Requires: >= 0.20.0, 0.x\n")
	require.Contains(t, plain, "  Current kongctl: 0.25.0\n")
}

func TestWriteExtensionSummaryShowsUnknownCompatibilityForDevelopmentVersion(t *testing.T) {
	var out bytes.Buffer
	ext := testInstalledRemoteExtension()
	ext.Manifest.Compatibility.MinVersion = "0.20.0"

	err := writeExtensionSummary(&out, ext, "dev")

	require.NoError(t, err)
	plain := stripANSIEscapeSequences(out.String())
	require.Contains(t, plain, "  Compatibility: unknown\n")
	require.Contains(t, plain, "  Requires: >= 0.20.0\n")
	require.Contains(t, plain, "  Current kongctl: dev\n")
}

func TestUpgradeExtensionCommandSupportsUpgradeAll(t *testing.T) {
	cmd := newUpgradeExtensionCmd()

	require.Equal(t, "extension [publisher/name|owner/repo[@tag|ref|version]]", cmd.Use)
	require.Contains(t, cmd.Aliases, "extensions")
	require.NoError(t, cmd.Args(cmd, []string{}))
	require.NoError(t, cmd.Args(cmd, []string{"kong/debug"}))
	require.Error(t, cmd.Args(cmd, []string{"kong/debug", "kong/other"}))
}

func TestUpgradeAllSkipReason(t *testing.T) {
	tests := []struct {
		name string
		ext  extensions.Extension
		want string
	}{
		{
			name: "GitHub release asset",
			ext:  testInstalledRemoteExtension(),
		},
		{
			name: "linked",
			ext: extensions.Extension{
				ID:          "kong/linked",
				InstallType: extensions.InstallTypeLinked,
			},
			want: "linked extension",
		},
		{
			name: "local path",
			ext: extensions.Extension{
				ID:          "kong/local",
				InstallType: extensions.InstallTypeInstalled,
				Install: &extensions.InstallState{
					Source: extensions.SourceState{Type: extensions.SourceTypeLocalPath},
				},
			},
			want: "local path install",
		},
		{
			name: "GitHub source clone",
			ext: extensions.Extension{
				ID:          "kong/source",
				InstallType: extensions.InstallTypeInstalled,
				Install: &extensions.InstallState{
					Source: extensions.SourceState{Type: extensions.SourceTypeGitHubSource},
				},
			},
			want: "GitHub source clone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := upgradeAllSkipReason(tt.ext)
			if tt.want == "" {
				require.Empty(t, reason)
				return
			}
			require.Contains(t, reason, tt.want)
		})
	}
}

func TestWriteUpgradeAllSummary(t *testing.T) {
	var out bytes.Buffer

	err := writeUpgradeAllSummary(&out, upgradeAllExtensionResult{
		Upgraded: []string{"kong/debug"},
		UpToDate: []string{"kong/cowsay"},
		Skipped: []upgradeAllExtensionEntry{{
			ID:     "kong/local",
			Reason: "local path install",
		}},
		Failed: []upgradeAllExtensionEntry{{
			ID:    "kong/broken",
			Error: "fetch failed",
		}},
	})

	require.NoError(t, err)
	plain := stripANSIEscapeSequences(out.String())
	require.Contains(t, plain, "Extension upgrades\n")
	require.Contains(t, plain, "kong/debug  upgraded\n")
	require.Contains(t, plain, "kong/cowsay  up to date\n")
	require.Contains(t, plain, "kong/local  skipped  local path install\n")
	require.Contains(t, plain, "kong/broken  failed  fetch failed\n")
	require.Contains(t, plain, "Summary: 1 upgraded, 1 up to date, 1 skipped, 1 failed\n")
}

func TestParseUpgradeExtensionTarget(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		wantSelector string
		wantTarget   string
		wantErr      string
	}{
		{
			name:         "id only",
			value:        "kong/debug",
			wantSelector: "kong/debug",
		},
		{
			name:         "tag target",
			value:        "kong/debug@v0.2.0",
			wantSelector: "kong/debug",
			wantTarget:   "v0.2.0",
		},
		{
			name:         "version target",
			value:        "kong/debug@0.2.0",
			wantSelector: "kong/debug",
			wantTarget:   "0.2.0",
		},
		{
			name:         "latest target",
			value:        "kong/debug@latest",
			wantSelector: "kong/debug",
			wantTarget:   "latest",
		},
		{
			name:         "GitHub repository target",
			value:        "kong/kongctl-ext-debug@v0.2.0",
			wantSelector: "kong/kongctl-ext-debug",
			wantTarget:   "v0.2.0",
		},
		{
			name:         "GitHub URL",
			value:        "https://github.com/kong/kongctl-ext-debug",
			wantSelector: "https://github.com/kong/kongctl-ext-debug",
		},
		{
			name:         "GitHub URL target",
			value:        "https://github.com/kong/kongctl-ext-debug@v0.2.0",
			wantSelector: "https://github.com/kong/kongctl-ext-debug",
			wantTarget:   "v0.2.0",
		},
		{
			name:         "SSH URL",
			value:        "git@github.com:kong/kongctl-ext-debug.git",
			wantSelector: "git@github.com:kong/kongctl-ext-debug.git",
		},
		{
			name:         "SSH URL target",
			value:        "git@github.com:kong/kongctl-ext-debug.git@v0.2.0",
			wantSelector: "git@github.com:kong/kongctl-ext-debug.git",
			wantTarget:   "v0.2.0",
		},
		{
			name:    "missing target",
			value:   "kong/debug@",
			wantErr: "target is required",
		},
		{
			name:    "missing selector",
			value:   "",
			wantErr: "required",
		},
		{
			name:    "extra at",
			value:   "kong/debug@v1@other",
			wantErr: "must not contain @",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, target, err := parseUpgradeExtensionTarget(tt.value)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantSelector, selector)
			require.Equal(t, tt.wantTarget, target)
		})
	}
}

func TestMatchUpgradeExtensionByGitHubRepository(t *testing.T) {
	matches := []extensions.Extension{
		{
			ID:          "kong/debug",
			InstallType: extensions.InstallTypeInstalled,
			Install: &extensions.InstallState{
				Source: extensions.SourceState{
					Type:       extensions.SourceTypeGitHubReleaseAsset,
					Repository: "Kong/kongctl-ext-debug",
				},
			},
		},
		{
			ID:          "kong/local",
			InstallType: extensions.InstallTypeInstalled,
			Install: &extensions.InstallState{
				Source: extensions.SourceState{
					Type: extensions.SourceTypeLocalPath,
				},
			},
		},
		{
			ID:          "kong/linked-debug",
			InstallType: extensions.InstallTypeLinked,
			Install: &extensions.InstallState{
				Source: extensions.SourceState{
					Type:       extensions.SourceTypeGitHubReleaseAsset,
					Repository: "kong/kongctl-ext-debug",
				},
			},
		},
	}

	ext, found, err := matchUpgradeExtensionByGitHubRepository(matches, "kong/kongctl-ext-debug")

	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "kong/debug", ext.ID)
}

func TestMatchUpgradeExtensionByGitHubRepositoryRejectsAmbiguousMatches(t *testing.T) {
	matches := []extensions.Extension{
		{
			ID:          "kong/one",
			InstallType: extensions.InstallTypeInstalled,
			Install: &extensions.InstallState{
				Source: extensions.SourceState{
					Type:       extensions.SourceTypeGitHubReleaseAsset,
					Repository: "kong/repo",
				},
			},
		},
		{
			ID:          "kong/two",
			InstallType: extensions.InstallTypeInstalled,
			Install: &extensions.InstallState{
				Source: extensions.SourceState{
					Type:       extensions.SourceTypeGitHubSource,
					Repository: "kong/repo",
				},
			},
		},
	}

	_, found, err := matchUpgradeExtensionByGitHubRepository(matches, "kong/repo")

	require.ErrorContains(t, err, "multiple installed extensions")
	require.False(t, found)
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

package extensions

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	extensioncore "github.com/kong/kongctl/internal/extensions"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/theme"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.yaml.in/yaml/v4"
)

type installExtensionOptions struct {
	source string
	ref    string
	yes    bool
}

type uninstallExtensionOptions struct {
	id         string
	removeData bool
}

type linkExtensionOptions struct {
	source string
}

type upgradeExtensionOptions struct {
	id     string
	target string
	yes    bool
}

func NewInstallExtensionCmd() *cobra.Command {
	opts := &installExtensionOptions{}
	cmd := &cobra.Command{
		Use:   "extension <source>",
		Short: i18n.T("root.verbs.install.extension.short", "Install a kongctl CLI extension"),
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			opts.source = args[0]
			return runInstallExtension(command, args, *opts)
		},
	}
	cmd.Flags().StringVar(&opts.ref, "ref", "", "GitHub release tag, branch, or source ref to install.")
	cmd.Flags().BoolVar(&opts.yes, "yes", false, "Accept the remote extension trust prompt.")
	return cmd
}

func NewListExtensionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "extensions",
		Aliases: []string{"extension"},
		Short:   i18n.T("root.verbs.list.extensions.short", "List installed kongctl CLI extensions"),
		Args:    cobra.NoArgs,
		RunE:    runListExtensions,
	}
	return cmd
}

func NewLinkCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "link",
		Short: i18n.T("root.verbs.link.short", "Link locally developed features"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, verbs.Link))
		},
	}
	cmd.AddCommand(newLinkExtensionCmd())
	return cmd, nil
}

func NewUninstallCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: i18n.T("root.verbs.uninstall.short", "Uninstall features"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, verbs.Uninstall))
		},
	}
	cmd.AddCommand(newUninstallExtensionCmd())
	return cmd, nil
}

func NewUpgradeCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: i18n.T("root.verbs.upgrade.short", "Upgrade kongctl features"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, verbs.Upgrade))
		},
	}
	cmd.AddCommand(newUpgradeExtensionCmd())
	return cmd, nil
}

func NewGetExtensionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extension <publisher/name>",
		Short: i18n.T("root.verbs.get.extension.short", "Get a kongctl CLI extension"),
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return runGetExtension(command, args, args[0])
		},
	}
	return cmd
}

func newLinkExtensionCmd() *cobra.Command {
	opts := &linkExtensionOptions{}
	cmd := &cobra.Command{
		Use:   "extension <path>",
		Short: i18n.T("root.verbs.link.extension.short", "Link a local development CLI extension"),
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			opts.source = args[0]
			return runLinkExtension(command, args, *opts)
		},
	}
	return cmd
}

func newUninstallExtensionCmd() *cobra.Command {
	opts := &uninstallExtensionOptions{}
	cmd := &cobra.Command{
		Use:   "extension <publisher/name>",
		Short: i18n.T("root.verbs.uninstall.extension.short", "Uninstall a kongctl CLI extension"),
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			opts.id = args[0]
			return runUninstallExtension(command, args, *opts)
		},
	}
	cmd.Flags().BoolVar(&opts.removeData, "remove-data", false,
		"Remove the extension-owned data directory in addition to host install/link records.")
	return cmd
}

func newUpgradeExtensionCmd() *cobra.Command {
	opts := &upgradeExtensionOptions{}
	cmd := &cobra.Command{
		Use:   "extension <publisher/name[@tag|ref|version]>",
		Short: i18n.T("root.verbs.upgrade.extension.short", "Upgrade a kongctl CLI extension"),
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, target, err := parseUpgradeExtensionTarget(args[0])
			if err != nil {
				return &cmdpkg.ConfigurationError{Err: err}
			}
			opts.id = id
			opts.target = target
			return runUpgradeExtension(command, args, *opts)
		},
	}
	cmd.Flags().BoolVar(&opts.yes, "yes", false, "Accept the remote extension trust prompt.")
	return cmd
}

func runInstallExtension(command *cobra.Command, args []string, opts installExtensionOptions) error {
	helper := cmdpkg.BuildHelper(command, args)
	store, err := extensioncore.DefaultStore()
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to resolve extension store", err)
	}
	if _, err := os.Stat(opts.source); err != nil {
		if !os.IsNotExist(err) {
			return &cmdpkg.ConfigurationError{Err: err}
		}
		return runInstallGitHubExtension(command, args, opts, store)
	}
	if strings.TrimSpace(opts.ref) != "" {
		return &cmdpkg.ConfigurationError{Err: errors.New("--ref is only supported for GitHub extension sources")}
	}
	candidate, err := extensioncore.LoadLocalExtension(opts.source, extensioncore.InstallTypeInstalled)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	if err := extensioncore.ValidateExtensionCommands(command.Root(), candidate); err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	version, err := cliVersion(helper)
	if err != nil {
		return err
	}
	result, err := store.InstallLocal(opts.source, version, time.Now())
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to install extension", err)
	}
	return writeCommandResult(helper, result, func() error {
		return writeInstallSummary(helper.GetStreams().Out, result, opts.source)
	})
}

func runInstallGitHubExtension(
	command *cobra.Command,
	args []string,
	opts installExtensionOptions,
	store extensioncore.Store,
) error {
	helper := cmdpkg.BuildHelper(command, args)
	githubSource, ok, err := extensioncore.ParseGitHubSource(opts.source, opts.ref)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	if !ok {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf("extension source %q does not exist", opts.source)}
	}
	fetched, err := extensioncore.FetchGitHubSource(helper.GetContext(), githubSource, store.TempDir())
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to fetch GitHub extension", err)
	}
	defer fetched.Cleanup()

	candidate, err := extensioncore.LoadLocalExtension(fetched.Dir, extensioncore.InstallTypeInstalled)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	if err := extensioncore.ValidateExtensionCommands(command.Root(), candidate); err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	observation, err := extensioncore.ObservePackage(fetched.Dir)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	trustConfirmed, err := confirmRemoteInstallTrust(helper, fetched, candidate, observation, opts.yes)
	if err != nil {
		return err
	}
	version, err := cliVersion(helper)
	if err != nil {
		return err
	}
	result, err := store.InstallGitHubSource(fetched.Dir, fetched, version, time.Now(), trustConfirmed)
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to install extension", err)
	}
	return writeCommandResult(helper, result, func() error {
		return writeInstallSummary(helper.GetStreams().Out, result, fetched.Repository)
	})
}

func runLinkExtension(command *cobra.Command, args []string, opts linkExtensionOptions) error {
	helper := cmdpkg.BuildHelper(command, args)
	store, err := extensioncore.DefaultStore()
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to resolve extension store", err)
	}
	candidate, err := extensioncore.LoadLocalExtension(opts.source, extensioncore.InstallTypeLinked)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	if err := extensioncore.ValidateExtensionCommands(command.Root(), candidate); err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	version, err := cliVersion(helper)
	if err != nil {
		return err
	}
	ext, err := store.LinkLocal(opts.source, version, time.Now())
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to link extension", err)
	}
	return writeCommandResult(helper, ext, func() error {
		return writeLinkSummary(helper.GetStreams().Out, ext, opts.source)
	})
}

func runListExtensions(command *cobra.Command, args []string) error {
	helper := cmdpkg.BuildHelper(command, args)
	store, err := extensioncore.DefaultStore()
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to resolve extension store", err)
	}
	extensions, err := store.List()
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to list extensions", err)
	}
	return writeCommandResult(helper, extensions, func() error {
		return writeListSummary(helper.GetStreams().Out, extensions)
	})
}

func runGetExtension(command *cobra.Command, args []string, id string) error {
	helper := cmdpkg.BuildHelper(command, args)
	store, err := extensioncore.DefaultStore()
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to resolve extension store", err)
	}
	ext, err := store.Get(id)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	return writeCommandResult(helper, ext, func() error {
		return writeExtensionSummary(helper.GetStreams().Out, ext)
	})
}

func runUninstallExtension(command *cobra.Command, args []string, opts uninstallExtensionOptions) error {
	helper := cmdpkg.BuildHelper(command, args)
	store, err := extensioncore.DefaultStore()
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to resolve extension store", err)
	}
	result, err := store.Uninstall(opts.id, opts.removeData)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	return writeCommandResult(helper, result, func() error {
		return writeUninstallSummary(helper.GetStreams().Out, result)
	})
}

func runUpgradeExtension(command *cobra.Command, args []string, opts upgradeExtensionOptions) error {
	helper := cmdpkg.BuildHelper(command, args)
	store, err := extensioncore.DefaultStore()
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to resolve extension store", err)
	}
	ext, err := store.Get(opts.id)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	if ext.InstallType == extensioncore.InstallTypeLinked {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"extension %s is linked; linked extensions read directly from the linked working tree", ext.ID,
		)}
	}
	if ext.Install == nil {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf("extension %s is missing install metadata", ext.ID)}
	}
	switch ext.Install.Source.Type {
	case extensioncore.SourceTypeLocalPath:
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"extension %s was installed from a local path; reinstall it from the source path to upgrade", ext.ID,
		)}
	case extensioncore.SourceTypeGitHubReleaseAsset:
		return runUpgradeGitHubReleaseExtension(command, args, store, ext, opts)
	case extensioncore.SourceTypeGitHubSource, "":
		return runUpgradeGitHubSourceExtension(command, args, store, ext, opts)
	default:
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"extension %s has unsupported source type %q", ext.ID, ext.Install.Source.Type,
		)}
	}
}

func runUpgradeGitHubReleaseExtension(
	command *cobra.Command,
	args []string,
	store extensioncore.Store,
	current extensioncore.Extension,
	opts upgradeExtensionOptions,
) error {
	helper := cmdpkg.BuildHelper(command, args)
	repository := strings.TrimSpace(current.Install.Source.Repository)
	if repository == "" {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"extension %s is missing its GitHub repository; reinstall it before upgrading", current.ID,
		)}
	}
	target := normalizedUpgradeTarget(opts.target)
	githubSource, ok, err := extensioncore.ParseGitHubSource(repository, target)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	if !ok {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"extension %s has invalid GitHub repository %q", current.ID, repository,
		)}
	}

	fetched, err := extensioncore.FetchGitHubReleaseAsset(helper.GetContext(), githubSource, store.TempDir())
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to fetch GitHub release artifact", err)
	}
	defer fetched.Cleanup()

	candidate, observation, err := validateRemoteUpgradeCandidate(command, current, fetched.Dir)
	if err != nil {
		return err
	}
	if extensionPackageMatchesInstall(current, fetched, observation) {
		result := installResultFromExtension(current)
		return writeCommandResult(helper, result, func() error {
			return writeUpgradeUpToDateSummary(helper.GetStreams().Out, current)
		})
	}

	trustConfirmed, err := confirmRemoteUpgradeTrust(helper, current, fetched, candidate, observation, opts.yes)
	if err != nil {
		return err
	}
	version, err := cliVersion(helper)
	if err != nil {
		return err
	}
	result, err := store.InstallGitHubSource(fetched.Dir, fetched, version, time.Now(), trustConfirmed)
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to upgrade extension", err)
	}
	return writeCommandResult(helper, result, func() error {
		return writeUpgradeSummary(helper.GetStreams().Out, result.Extension, current)
	})
}

func runUpgradeGitHubSourceExtension(
	command *cobra.Command,
	args []string,
	store extensioncore.Store,
	current extensioncore.Extension,
	opts upgradeExtensionOptions,
) error {
	helper := cmdpkg.BuildHelper(command, args)
	if normalizedUpgradeTarget(opts.target) == "" {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"extension %s was installed from a GitHub source clone; upgrade it with %s upgrade extension %s@<tag|ref|commit>",
			current.ID,
			meta.CLIName,
			current.ID,
		)}
	}
	repository := strings.TrimSpace(current.Install.Source.Repository)
	if repository == "" {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"extension %s is missing its GitHub repository; reinstall it before upgrading", current.ID,
		)}
	}
	githubSource, ok, err := extensioncore.ParseGitHubSource(repository, opts.target)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	if !ok {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"extension %s has invalid GitHub repository %q", current.ID, repository,
		)}
	}

	fetched, err := extensioncore.FetchGitHubSourceClone(helper.GetContext(), githubSource, store.TempDir())
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to fetch GitHub source", err)
	}
	defer fetched.Cleanup()

	candidate, observation, err := validateRemoteUpgradeCandidate(command, current, fetched.Dir)
	if err != nil {
		return err
	}
	if extensionPackageMatchesInstall(current, fetched, observation) {
		result := installResultFromExtension(current)
		return writeCommandResult(helper, result, func() error {
			return writeUpgradeUpToDateSummary(helper.GetStreams().Out, current)
		})
	}

	trustConfirmed, err := confirmRemoteUpgradeTrust(helper, current, fetched, candidate, observation, opts.yes)
	if err != nil {
		return err
	}
	version, err := cliVersion(helper)
	if err != nil {
		return err
	}
	result, err := store.InstallGitHubSource(fetched.Dir, fetched, version, time.Now(), trustConfirmed)
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to upgrade extension", err)
	}
	return writeCommandResult(helper, result, func() error {
		return writeUpgradeSummary(helper.GetStreams().Out, result.Extension, current)
	})
}

func validateRemoteUpgradeCandidate(
	command *cobra.Command,
	current extensioncore.Extension,
	sourceRoot string,
) (extensioncore.Extension, extensioncore.PackageObservation, error) {
	candidate, err := extensioncore.LoadLocalExtension(sourceRoot, extensioncore.InstallTypeInstalled)
	if err != nil {
		return extensioncore.Extension{}, extensioncore.PackageObservation{}, &cmdpkg.ConfigurationError{Err: err}
	}
	if candidate.ID != current.ID {
		return extensioncore.Extension{}, extensioncore.PackageObservation{}, &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"upgrade candidate id %q does not match installed extension %q", candidate.ID, current.ID,
		)}
	}
	if err := extensioncore.ValidateExtensionCommands(command.Root(), candidate); err != nil {
		return extensioncore.Extension{}, extensioncore.PackageObservation{}, &cmdpkg.ConfigurationError{Err: err}
	}
	observation, err := extensioncore.ObservePackage(sourceRoot)
	if err != nil {
		return extensioncore.Extension{}, extensioncore.PackageObservation{}, &cmdpkg.ConfigurationError{Err: err}
	}
	return candidate, observation, nil
}

func extensionPackageMatchesInstall(
	current extensioncore.Extension,
	fetched extensioncore.FetchedGitHubSource,
	observation extensioncore.PackageObservation,
) bool {
	if current.Install == nil {
		return false
	}
	switch fetched.SourceType {
	case extensioncore.SourceTypeGitHubReleaseAsset:
		currentTag := current.Install.Source.ReleaseTag
		if currentTag == "" {
			currentTag = current.Install.Source.Ref
		}
		targetTag := fetched.ReleaseTag
		if targetTag == "" {
			targetTag = fetched.Ref
		}
		return currentTag != "" &&
			currentTag == targetTag &&
			current.Install.PackageHash != "" &&
			current.Install.PackageHash == observation.PackageHash
	case extensioncore.SourceTypeGitHubSource:
		return fetched.ResolvedCommit != "" &&
			current.Install.Source.ResolvedCommit == fetched.ResolvedCommit &&
			current.Install.PackageHash != "" &&
			current.Install.PackageHash == observation.PackageHash
	default:
		return false
	}
}

func installResultFromExtension(ext extensioncore.Extension) extensioncore.InstallResult {
	result := extensioncore.InstallResult{Extension: ext}
	if ext.Install != nil {
		result.ManifestHash = ext.Install.ManifestHash
		result.RuntimeHash = ext.Install.RuntimeHash
		result.PackageHash = ext.Install.PackageHash
	}
	return result
}

func parseUpgradeExtensionTarget(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	id, target, hasTarget := strings.Cut(value, "@")
	id = strings.TrimSpace(id)
	target = strings.TrimSpace(target)
	if id == "" {
		return "", "", fmt.Errorf("extension id is required")
	}
	if err := extensioncore.ValidateExtensionID(id); err != nil {
		return "", "", err
	}
	if hasTarget && target == "" {
		return "", "", fmt.Errorf("extension upgrade target is required after @")
	}
	if strings.Contains(target, "@") {
		return "", "", fmt.Errorf("extension upgrade target must not contain @")
	}
	return id, target, nil
}

func normalizedUpgradeTarget(target string) string {
	target = strings.TrimSpace(target)
	if strings.EqualFold(target, "latest") {
		return ""
	}
	return target
}

func confirmRemoteInstallTrust(
	helper cmdpkg.Helper,
	fetched extensioncore.FetchedGitHubSource,
	candidate extensioncore.Extension,
	observation extensioncore.PackageObservation,
	yes bool,
) (bool, error) {
	return confirmRemoteExtensionTrust(helper, "install", nil, fetched, candidate, observation, yes)
}

func confirmRemoteUpgradeTrust(
	helper cmdpkg.Helper,
	current extensioncore.Extension,
	fetched extensioncore.FetchedGitHubSource,
	candidate extensioncore.Extension,
	observation extensioncore.PackageObservation,
	yes bool,
) (bool, error) {
	return confirmRemoteExtensionTrust(helper, "upgrade", &current, fetched, candidate, observation, yes)
}

func confirmRemoteExtensionTrust(
	helper cmdpkg.Helper,
	action string,
	current *extensioncore.Extension,
	fetched extensioncore.FetchedGitHubSource,
	candidate extensioncore.Extension,
	observation extensioncore.PackageObservation,
	yes bool,
) (bool, error) {
	if yes {
		return true, nil
	}
	if explicitOutputRequested(helper.GetCmd()) {
		return false, &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"remote extension %s confirmation is not available with structured output; use --yes to accept",
			action,
		)}
	}

	streams := helper.GetStreams()
	if err := writeRemoteTrustPrompt(streams.Out, action, current, fetched, candidate, observation); err != nil {
		return false, err
	}

	input := streams.In
	if f, ok := input.(*os.File); ok && f.Fd() == os.Stdin.Fd() {
		if tty, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0); err == nil {
			defer tty.Close()
			input = tty
		}
	}

	reader := bufio.NewReader(input)
	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		lineCh <- line
	}()

	ctx := helper.GetCmd().Context()
	if ctx == nil {
		ctx = context.Background()
	}
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
		return false, cmdpkg.PrepareExecutionErrorMsg(helper, "extension "+action+" cancelled")
	case <-sigCh:
		return false, cmdpkg.PrepareExecutionErrorMsg(helper, "extension "+action+" cancelled")
	case err := <-errCh:
		_ = err
		return false, cmdpkg.PrepareExecutionErrorMsg(helper, "extension "+action+" cancelled")
	case line := <-lineCh:
		if strings.ToLower(strings.TrimSpace(line)) != "yes" {
			return false, cmdpkg.PrepareExecutionErrorMsg(helper, "extension "+action+" cancelled")
		}
		return true, nil
	}
}

func writeRemoteTrustPrompt(
	w io.Writer,
	action string,
	current *extensioncore.Extension,
	fetched extensioncore.FetchedGitHubSource,
	candidate extensioncore.Extension,
	observation extensioncore.PackageObservation,
) error {
	ui := extensionUI()
	source := sourceStateFromFetched(fetched)
	if _, err := fmt.Fprintf(w, "%s %s %s\n",
		ui.warning.Render("!"),
		ui.strong.Render("Remote extension trust confirmation"),
		candidate.ID,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w,
		"  This extension is executable code. "+remoteTrustActionTitle(action)+
			" it only if you trust the source and reviewed the package.",
	); err != nil {
		return err
	}
	if current != nil && current.Install != nil {
		writeField(w, ui, "Current", installSourceLabel(current.Install.Source, current.ID))
		writeField(w, ui, "Target", installSourceLabel(source, fetched.Repository))
	} else {
		writeField(w, ui, "Source", installSourceLabel(source, fetched.Repository))
	}
	writeField(w, ui, "Source type", remoteSourceTypeLabel(fetched.SourceType))
	writeOptionalField(w, ui, "Release tag", fetched.ReleaseTag)
	if fetched.SourceType != extensioncore.SourceTypeGitHubReleaseAsset {
		writeOptionalField(w, ui, "Source ref", source.Ref)
	}
	writeOptionalField(w, ui, "Resolved commit", fetched.ResolvedCommit)
	writeOptionalField(w, ui, "Asset", fetched.AssetName)
	writeOptionalField(w, ui, "Asset URL", fetched.AssetURL)
	writeOptionalField(w, ui, "Version", observation.Manifest.Version)
	writeField(w, ui, "Runtime", observation.RuntimeCommand)
	writeField(w, ui, "Package SHA256", observation.PackageHash)
	writeField(w, ui, "Manifest SHA256", observation.ManifestHash)
	writeField(w, ui, "Runtime SHA256", observation.RuntimeHash)
	writeCommands(w, ui, candidate.CommandPaths)
	_, err := fmt.Fprintf(w, "\nDo you want to %s this extension? Type 'yes' to confirm: ", action)
	return err
}

func remoteTrustActionTitle(action string) string {
	if action == "" {
		return "Install"
	}
	return strings.ToUpper(action[:1]) + action[1:]
}

func remoteSourceTypeLabel(sourceType string) string {
	switch sourceType {
	case extensioncore.SourceTypeGitHubReleaseAsset:
		return "GitHub release asset"
	case extensioncore.SourceTypeGitHubSource, "":
		return "GitHub source clone"
	default:
		return sourceType
	}
}

func sourceStateFromFetched(fetched extensioncore.FetchedGitHubSource) extensioncore.SourceState {
	sourceType := fetched.SourceType
	if sourceType == "" {
		sourceType = extensioncore.SourceTypeGitHubSource
	}
	return extensioncore.SourceState{
		Type:           sourceType,
		Repository:     fetched.Repository,
		URL:            fetched.URL,
		Ref:            fetched.Ref,
		ResolvedCommit: fetched.ResolvedCommit,
		ReleaseTag:     fetched.ReleaseTag,
		AssetName:      fetched.AssetName,
		AssetURL:       fetched.AssetURL,
	}
}

func writeCommandResult(helper cmdpkg.Helper, value any, writeText func() error) error {
	format := cmdcommon.TEXT
	if explicitOutputRequested(helper.GetCmd()) {
		var err error
		format, err = helper.GetOutputFormat()
		if err != nil {
			return err
		}
	}
	switch format {
	case cmdcommon.JSON:
		encoder := json.NewEncoder(helper.GetStreams().Out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	case cmdcommon.YAML:
		encoder := yaml.NewEncoder(helper.GetStreams().Out)
		encoder.SetIndent(2)
		defer func() {
			_ = encoder.Close()
		}()
		return encoder.Encode(value)
	case cmdcommon.TEXT:
		return writeText()
	default:
		return writeText()
	}
}

func explicitOutputRequested(command *cobra.Command) bool {
	for current := command; current != nil; current = current.Parent() {
		for _, flags := range []*pflag.FlagSet{
			current.Flags(),
			current.PersistentFlags(),
			current.InheritedFlags(),
		} {
			if flag := flags.Lookup(cmdcommon.OutputFlagName); flag != nil && flag.Changed {
				return true
			}
		}
	}
	return false
}

func writeInstallSummary(w io.Writer, result extensioncore.InstallResult, source string) error {
	ext := result.Extension
	ui := extensionUI()
	if _, err := fmt.Fprintf(w, "%s %s %s\n", ui.success.Render("✓"), ui.strong.Render("Installed"), ext.ID); err != nil {
		return err
	}
	if ext.Install != nil {
		writeField(w, ui, "Source", installSourceLabel(ext.Install.Source, source))
		writeOptionalField(w, ui, "Asset", ext.Install.Source.AssetName)
	} else {
		writeField(w, ui, "Source", source)
	}
	writeField(w, ui, "Runtime", ext.Manifest.Runtime.Command)
	writeOptionalField(w, ui, "Version", ext.Manifest.Version)
	writeCommands(w, ui, ext.CommandPaths)
	writeField(w, ui, "Next", meta.CLIName+" "+extensioncore.CommandPathString(ext.CommandPaths[0])+" --help")
	return nil
}

func writeLinkSummary(w io.Writer, ext extensioncore.Extension, source string) error {
	ui := extensionUI()
	if _, err := fmt.Fprintf(w, "%s %s %s\n", ui.success.Render("✓"), ui.strong.Render("Linked"), ext.ID); err != nil {
		return err
	}
	writeField(w, ui, "Path", source)
	writeField(w, ui, "Runtime", ext.Manifest.Runtime.Command)
	writeOptionalField(w, ui, "Version", ext.Manifest.Version)
	writeCommands(w, ui, ext.CommandPaths)
	writeField(w, ui, "Next", meta.CLIName+" "+extensioncore.CommandPathString(ext.CommandPaths[0])+" --help")
	return nil
}

func writeListSummary(w io.Writer, extensions []extensioncore.Extension) error {
	ui := extensionUI()
	if len(extensions) == 0 {
		_, err := fmt.Fprintf(w, "%s No extensions installed or linked.\n  %s\n",
			ui.muted.Render("•"),
			ui.muted.Render("Try: "+meta.CLIName+" install extension <path>"),
		)
		return err
	}
	if _, err := fmt.Fprintln(w, ui.heading.Render("Extensions")); err != nil {
		return err
	}
	for _, ext := range extensions {
		version := ext.Manifest.Version
		if version == "" {
			version = "unversioned"
		}
		if _, err := fmt.Fprintf(w, "%s %s  %s  %s\n",
			ui.success.Render("✓"),
			ui.strong.Render(ext.ID),
			ui.muted.Render(string(ext.InstallType)),
			ui.muted.Render(version),
		); err != nil {
			return err
		}
		writeCommands(w, ui, ext.CommandPaths)
	}
	return nil
}

func writeExtensionSummary(w io.Writer, ext extensioncore.Extension) error {
	ui := extensionUI()
	if _, err := fmt.Fprintf(w, "%s %s\n", ui.heading.Render("Extension"), ui.strong.Render(ext.ID)); err != nil {
		return err
	}
	writeField(w, ui, "Name", ext.Manifest.Name)
	writeField(w, ui, "Publisher", ext.Manifest.Publisher)
	writeField(w, ui, "Type", string(ext.InstallType))
	writeOptionalField(w, ui, "Version", ext.Manifest.Version)
	writeOptionalField(w, ui, "Summary", ext.Manifest.Summary)
	switch ext.InstallType {
	case extensioncore.InstallTypeInstalled:
		writeOptionalField(w, ui, "Package", ext.PackageDir)
		if ext.Install != nil {
			writeOptionalField(w, ui, "Source", installSourceLabel(ext.Install.Source, ""))
			writeOptionalField(w, ui, "Asset", ext.Install.Source.AssetName)
		}
	case extensioncore.InstallTypeLinked:
		writeOptionalField(w, ui, "Path", ext.LinkedDir)
	}
	writeField(w, ui, "Runtime", ext.Manifest.Runtime.Command)
	writeCommands(w, ui, ext.CommandPaths)
	return nil
}

func writeUninstallSummary(w io.Writer, result extensioncore.UninstallResult) error {
	ui := extensionUI()
	if _, err := fmt.Fprintf(w, "%s %s %s\n",
		ui.success.Render("✓"),
		ui.strong.Render("Uninstalled"),
		result.ID,
	); err != nil {
		return err
	}
	if result.RemovedData {
		writeField(w, ui, "Data", "removed")
	} else {
		writeField(w, ui, "Data", "preserved")
	}
	return nil
}

func writeUpgradeSummary(w io.Writer, upgraded, previous extensioncore.Extension) error {
	ui := extensionUI()
	if _, err := fmt.Fprintf(w, "%s %s %s\n",
		ui.success.Render("✓"),
		ui.strong.Render("Upgraded"),
		upgraded.ID,
	); err != nil {
		return err
	}
	if previous.Install != nil {
		writeField(w, ui, "From", installSourceLabel(previous.Install.Source, previous.ID))
	}
	if upgraded.Install != nil {
		writeField(w, ui, "To", installSourceLabel(upgraded.Install.Source, upgraded.ID))
		writeOptionalField(w, ui, "Asset", upgraded.Install.Source.AssetName)
	}
	writeField(w, ui, "Runtime", upgraded.Manifest.Runtime.Command)
	writeOptionalField(w, ui, "Version", upgraded.Manifest.Version)
	writeCommands(w, ui, upgraded.CommandPaths)
	return nil
}

func writeUpgradeUpToDateSummary(w io.Writer, ext extensioncore.Extension) error {
	ui := extensionUI()
	if _, err := fmt.Fprintf(w, "%s %s %s\n",
		ui.success.Render("✓"),
		ui.strong.Render("Extension is up to date"),
		ext.ID,
	); err != nil {
		return err
	}
	if ext.Install != nil {
		writeField(w, ui, "Current", installSourceLabel(ext.Install.Source, ext.ID))
	}
	writeOptionalField(w, ui, "Version", ext.Manifest.Version)
	return nil
}

func writeCommands(w io.Writer, ui extensionUIStyles, paths []extensioncore.CommandPath) {
	if len(paths) == 0 {
		return
	}
	fmt.Fprintf(w, "  %s\n", ui.label.Render("Commands"))
	for _, path := range paths {
		fmt.Fprintf(w, "    %s %s %s\n",
			ui.muted.Render("•"),
			ui.command.Render(meta.CLIName),
			ui.command.Render(extensioncore.CommandPathString(path)),
		)
	}
}

func writeField(w io.Writer, ui extensionUIStyles, label, value string) {
	fmt.Fprintf(w, "  %s %s\n", ui.label.Render(label+":"), value)
}

func writeOptionalField(w io.Writer, ui extensionUIStyles, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	writeField(w, ui, label, value)
}

func installSourceLabel(source extensioncore.SourceState, fallback string) string {
	switch source.Type {
	case extensioncore.SourceTypeGitHubReleaseAsset:
		value := source.Repository
		tag := source.ReleaseTag
		if tag == "" {
			tag = source.Ref
		}
		if tag != "" {
			value += "@" + tag
		}
		if value != "" {
			return value
		}
	case extensioncore.SourceTypeGitHubSource:
		value := source.Repository
		if source.Ref != "" {
			value += "@" + source.Ref
		} else if source.ResolvedCommit != "" {
			value += "@" + source.ResolvedCommit
		}
		if value != "" {
			return value
		}
	case extensioncore.SourceTypeLocalPath:
		if source.Path != "" {
			return source.Path
		}
	}
	return fallback
}

type extensionUIStyles struct {
	heading lipgloss.Style
	strong  lipgloss.Style
	label   lipgloss.Style
	muted   lipgloss.Style
	success lipgloss.Style
	warning lipgloss.Style
	command lipgloss.Style
}

func extensionUI() extensionUIStyles {
	palette := theme.Current()
	return extensionUIStyles{
		heading: palette.ForegroundStyle(theme.ColorPrimary).Bold(true),
		strong:  palette.ForegroundStyle(theme.ColorTextPrimary).Bold(true),
		label:   palette.ForegroundStyle(theme.ColorTextSecondary).Bold(true),
		muted:   palette.ForegroundStyle(theme.ColorTextMuted),
		success: palette.ForegroundStyle(theme.ColorSuccess).Bold(true),
		warning: palette.ForegroundStyle(theme.ColorWarning).Bold(true),
		command: palette.ForegroundStyle(theme.ColorAccent),
	}
}

func cliVersion(helper cmdpkg.Helper) (string, error) {
	buildInfo, err := helper.GetBuildInfo()
	if err != nil {
		return "", err
	}
	if buildInfo != nil && strings.TrimSpace(buildInfo.Version) != "" {
		return buildInfo.Version, nil
	}
	return meta.DefaultCLIVersion, nil
}

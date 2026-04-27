package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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
}

type uninstallExtensionOptions struct {
	id         string
	removeData bool
}

type linkExtensionOptions struct {
	source string
}

type upgradeExtensionOptions struct {
	id string
	to string
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
	cmd.Flags().StringVar(&opts.ref, "ref", "", "GitHub branch or tag to install for owner/repo sources.")
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
		Use:   "extension <publisher/name>",
		Short: i18n.T("root.verbs.upgrade.extension.short", "Upgrade a kongctl CLI extension"),
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			opts.id = args[0]
			return runUpgradeExtension(command, args, *opts)
		},
	}
	cmd.Flags().StringVar(&opts.to, "to", "", "Explicit tag, ref, or version target for source-backed upgrades.")
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
	version, err := cliVersion(helper)
	if err != nil {
		return err
	}
	result, err := store.InstallGitHubSource(fetched.Dir, fetched, version, time.Now())
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
	if ext.Install != nil && ext.Install.Source.Type == "local_path" {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf(
			"extension %s was installed from a local path; reinstall it from the source path to upgrade", ext.ID,
		)}
	}
	return &cmdpkg.ConfigurationError{Err: errors.New(
		"remote extension upgrade is not implemented yet",
	)}
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
	writeField(w, ui, "Source", source)
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

type extensionUIStyles struct {
	heading lipgloss.Style
	strong  lipgloss.Style
	label   lipgloss.Style
	muted   lipgloss.Style
	success lipgloss.Style
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

package extensions

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	annotationExtensionID  = "kongctl.extension.id"
	annotationContribution = "kongctl.extension.contribution"
	annotationSynthetic    = "kongctl.extension.synthetic"
)

type SplitArgsResult struct {
	Remaining       []string
	ProfileOverride string
	ShowHelp        bool
}

func RegisterInstalledCommands(root *cobra.Command, store Store) error {
	extensions, err := store.List()
	if err != nil {
		return err
	}
	return RegisterCommands(root, store, extensions)
}

func RegisterCommands(root *cobra.Command, store Store, extensions []Extension) error {
	for _, ext := range extensions {
		for _, path := range ext.CommandPaths {
			if err := addContribution(root, store, ext, path); err != nil {
				return fmt.Errorf("register extension %s path %q: %w", ext.ID, CommandPathString(path), err)
			}
		}
	}
	return nil
}

func ValidateExtensionCommands(root *cobra.Command, ext Extension) error {
	for _, path := range ext.CommandPaths {
		if err := validateContribution(root, ext, path); err != nil {
			return fmt.Errorf("extension %s path %q: %w", ext.ID, CommandPathString(path), err)
		}
	}
	return nil
}

func addContribution(root *cobra.Command, store Store, ext Extension, contribution CommandPath) error {
	if err := validateContribution(root, ext, contribution); err != nil {
		return err
	}
	parent, err := rootForContribution(root, ext, contribution, true)
	if err != nil {
		return err
	}

	for i := 1; i < len(contribution.Path); i++ {
		segment := contribution.Path[i]
		terminal := i == len(contribution.Path)-1
		child := findChildByName(parent, segment.Name)
		if child == nil {
			child = newSyntheticCommand(ext, contribution, segment, terminal, store)
			parent.AddCommand(child)
		} else if !syntheticOwnedBy(child, ext.ID) {
			return fmt.Errorf("segment %q collides with existing command %q", segment.Name, child.CommandPath())
		} else {
			mergeAliases(child, segment.Aliases)
			if terminal {
				configureTerminalCommand(child, ext, contribution, store)
			}
		}
		parent = child
	}

	if len(contribution.Path) == 1 {
		configureTerminalCommand(parent, ext, contribution, store)
	}

	return nil
}

func validateContribution(root *cobra.Command, ext Extension, contribution CommandPath) error {
	parent, err := rootForContribution(root, ext, contribution, false)
	if err != nil {
		return err
	}
	if parent == nil {
		return nil
	}
	for i := 1; i < len(contribution.Path); i++ {
		segment := contribution.Path[i]
		if err := validateSegmentAgainstParent(parent, ext.ID, segment); err != nil {
			return err
		}
		if existing := findChildByName(parent, segment.Name); existing != nil && syntheticOwnedBy(existing, ext.ID) {
			parent = existing
			continue
		}
		break
	}
	return nil
}

func rootForContribution(
	root *cobra.Command,
	ext Extension,
	contribution CommandPath,
	create bool,
) (*cobra.Command, error) {
	if len(contribution.Path) == 0 {
		return nil, fmt.Errorf("command path is empty")
	}
	rootSegment := contribution.Path[0]
	if IsOpenBuiltInRoot(rootSegment.Name) {
		if len(rootSegment.Aliases) > 0 {
			return nil, fmt.Errorf("built-in root segment %q cannot declare aliases", rootSegment.Name)
		}
		command := findChildByName(root, rootSegment.Name)
		if command == nil {
			return nil, fmt.Errorf("built-in root command %q is not registered", rootSegment.Name)
		}
		return command, nil
	}
	if IsClosedBuiltInRoot(rootSegment.Name) {
		return nil, fmt.Errorf("built-in root command %q is closed to extension contributions", rootSegment.Name)
	}
	if err := validateSegmentAgainstParent(root, ext.ID, rootSegment); err != nil {
		return nil, err
	}
	command := findChildByName(root, rootSegment.Name)
	if command != nil {
		if syntheticOwnedBy(command, ext.ID) {
			return command, nil
		}
		return nil, fmt.Errorf("custom root %q collides with existing command %q", rootSegment.Name, command.CommandPath())
	}
	if !create {
		return nil, nil
	}
	command = newSyntheticCommand(ext, contribution, rootSegment, len(contribution.Path) == 1, NewStore(""))
	root.AddCommand(command)
	return command, nil
}

func validateSegmentAgainstParent(parent *cobra.Command, extensionID string, segment PathSegment) error {
	for _, name := range append([]string{segment.Name}, segment.Aliases...) {
		existing := findChildByNameOrAlias(parent, name)
		if existing == nil || syntheticOwnedBy(existing, extensionID) {
			continue
		}
		return fmt.Errorf("segment or alias %q collides with existing command %q", name, existing.CommandPath())
	}
	return nil
}

func newSyntheticCommand(
	ext Extension,
	contribution CommandPath,
	segment PathSegment,
	terminal bool,
	store Store,
) *cobra.Command {
	command := &cobra.Command{
		Use:     segment.Name,
		Aliases: append([]string(nil), segment.Aliases...),
		Short:   extensionShort(ext.ID, contribution),
		Long:    extensionLong(ext.ID, contribution),
		Example: strings.Join(contribution.Examples, "\n"),
		Annotations: map[string]string{
			annotationExtensionID: ext.ID,
			annotationSynthetic:   "true",
		},
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	if terminal {
		configureTerminalCommand(command, ext, contribution, store)
	}
	return command
}

func configureTerminalCommand(command *cobra.Command, ext Extension, contribution CommandPath, store Store) {
	command.Use = contribution.Path[len(contribution.Path)-1].Name + " [args] [flags]"
	command.Short = extensionShort(ext.ID, contribution)
	command.Long = extensionLong(ext.ID, contribution)
	command.Example = strings.Join(contribution.Examples, "\n")
	command.DisableFlagParsing = true
	command.SilenceUsage = true
	ensureExtensionHostFlags(command)
	if command.Annotations == nil {
		command.Annotations = map[string]string{}
	}
	command.Annotations[annotationExtensionID] = ext.ID
	command.Annotations[annotationContribution] = contribution.ID
	command.Annotations[annotationSynthetic] = "true"
	command.RunE = func(command *cobra.Command, args []string) error {
		return runExtensionCommand(command, args, store, ext, contribution)
	}
}

func runExtensionCommand(
	command *cobra.Command,
	args []string,
	store Store,
	ext Extension,
	contribution CommandPath,
) error {
	helper := cmdpkg.BuildHelper(command, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	buildInfo, err := helper.GetBuildInfo()
	if err != nil {
		return err
	}

	split, err := SplitExtensionArgs(command, args, cfg)
	if err != nil {
		return err
	}
	if split.ShowHelp {
		return PrintExtensionHelp(command.OutOrStdout(), ext.ID, contribution)
	}

	originalArgs := append(CommandPathNames(contribution), args...)
	return store.Dispatch(
		helper.GetContext(),
		helper.GetStreams(),
		cfg,
		buildInfo,
		ext,
		contribution,
		originalArgs,
		split.Remaining,
		split.ProfileOverride,
	)
}

func SplitExtensionArgs(command *cobra.Command, args []string, cfg config.Hook) (SplitArgsResult, error) {
	result := SplitArgsResult{Remaining: make([]string, 0, len(args))}
	hostFlags := collectHostFlags(command)
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			result.Remaining = append(result.Remaining, args[i+1:]...)
			return result, nil
		}
		if token == "--help" || token == "-h" {
			result.ShowHelp = true
			continue
		}
		if strings.HasPrefix(token, "--") && token != "--" {
			nameValue := strings.TrimPrefix(token, "--")
			name, value, hasValue := strings.Cut(nameValue, "=")
			flag := hostFlags.Lookup(name)
			if flag == nil {
				result.Remaining = append(result.Remaining, token)
				continue
			}
			if !hasValue {
				if flag.Value.Type() == "bool" {
					value = "true"
				} else {
					var err error
					value, i, err = longFlagValue(args, i, name)
					if err != nil {
						return result, err
					}
				}
			}
			if err := applyHostFlag(flag, value, cfg, &result); err != nil {
				return result, err
			}
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			nameValue := strings.TrimPrefix(token, "-")
			if nameValue == "" {
				result.Remaining = append(result.Remaining, token)
				continue
			}
			shorthand := nameValue[:1]
			flag := hostFlags.ShorthandLookup(shorthand)
			if flag == nil {
				result.Remaining = append(result.Remaining, token)
				continue
			}
			value, nextIndex, err := shorthandFlagValue(args, i, nameValue, flag)
			if err != nil {
				return result, err
			}
			i = nextIndex
			if err := applyHostFlag(flag, value, cfg, &result); err != nil {
				return result, err
			}
			continue
		}
		result.Remaining = append(result.Remaining, token)
	}
	return result, nil
}

func ensureExtensionHostFlags(command *cobra.Command) {
	if findFlag(command, jqoutput.FlagName) != nil {
		return
	}
	jqoutput.AddFlags(command.Flags())
}

func findFlag(command *cobra.Command, name string) *pflag.Flag {
	for current := command; current != nil; current = current.Parent() {
		for _, flags := range []*pflag.FlagSet{
			current.Flags(),
			current.PersistentFlags(),
			current.InheritedFlags(),
		} {
			if flags == nil {
				continue
			}
			if flag := flags.Lookup(name); flag != nil {
				return flag
			}
		}
	}
	return nil
}

func longFlagValue(args []string, index int, name string) (string, int, error) {
	next := index + 1
	if next >= len(args) {
		return "", index, fmt.Errorf("flag --%s requires a value", name)
	}
	return args[next], next, nil
}

func shorthandFlagValue(args []string, index int, nameValue string, flag *pflag.Flag) (string, int, error) {
	shorthand := nameValue[:1]
	if len(nameValue) > 1 {
		return nameValue[1:], index, nil
	}
	if flag.Value.Type() == "bool" {
		return "true", index, nil
	}
	next := index + 1
	if next >= len(args) {
		return "", index, fmt.Errorf("flag -%s requires a value", shorthand)
	}
	return args[next], next, nil
}

func PrintExtensionHelp(w io.Writer, extensionID string, contribution CommandPath) error {
	if _, err := fmt.Fprintf(w, "Usage:\n  %s\n\n", contribution.Usage); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s\n\nExtension: %s\n", contribution.Summary, extensionID); err != nil {
		return err
	}
	if contribution.Description != "" {
		if _, err := fmt.Fprintf(w, "\n%s\n", contribution.Description); err != nil {
			return err
		}
	}
	if len(contribution.Args) > 0 {
		if _, err := fmt.Fprintln(w, "\nArguments:"); err != nil {
			return err
		}
		for _, arg := range contribution.Args {
			required := ""
			if arg.Required {
				required = " (required)"
			}
			if _, err := fmt.Fprintf(w, "  %s%s\t%s\n", arg.Name, required, arg.Description); err != nil {
				return err
			}
		}
	}
	if len(contribution.Flags) > 0 {
		if _, err := fmt.Fprintln(w, "\nExtension Flags:"); err != nil {
			return err
		}
		for _, flag := range contribution.Flags {
			typeHint := ""
			if flag.Type != "" {
				typeHint = " " + flag.Type
			}
			if _, err := fmt.Fprintf(w, "  --%s%s\t%s\n", flag.Name, typeHint, flag.Description); err != nil {
				return err
			}
		}
	}
	if err := printExtensionHostFlags(w); err != nil {
		return err
	}
	if len(contribution.Examples) > 0 {
		if _, err := fmt.Fprintln(w, "\nExamples:"); err != nil {
			return err
		}
		for _, example := range contribution.Examples {
			if _, err := fmt.Fprintf(w, "  %s\n", example); err != nil {
				return err
			}
		}
	}
	return nil
}

func printExtensionHostFlags(w io.Writer) error {
	hostFlags := []struct {
		flag        string
		description string
	}{
		{"-o, --output string", "Output format: text, json, or yaml"},
		{"--jq string", "Filter JSON or YAML output using a jq expression"},
		{"-r, --jq-raw-output", "Output string jq results without JSON quotes"},
		{"--jq-color string", "Color mode for jq output: auto, always, or never"},
		{"--jq-color-theme string", "Color theme for jq output"},
		{"-p, --profile string", "Configuration profile to use"},
		{"--color-theme string", "kongctl color theme"},
	}
	if _, err := fmt.Fprintln(w, "\nHost Flags:"); err != nil {
		return err
	}
	for _, hostFlag := range hostFlags {
		if _, err := fmt.Fprintf(w, "  %s\t%s\n", hostFlag.flag, hostFlag.description); err != nil {
			return err
		}
	}
	return nil
}

func collectHostFlags(command *cobra.Command) *pflag.FlagSet {
	flags := pflag.NewFlagSet(command.Name(), pflag.ContinueOnError)
	flags.SortFlags = false
	addFlags := func(source *pflag.FlagSet) {
		if source == nil {
			return
		}
		source.VisitAll(func(flag *pflag.Flag) {
			if flags.Lookup(flag.Name) != nil {
				return
			}
			if flag.Shorthand != "" && flags.ShorthandLookup(flag.Shorthand) != nil {
				return
			}
			flags.AddFlag(flag)
		})
	}
	for current := command; current != nil; current = current.Parent() {
		addFlags(current.LocalNonPersistentFlags())
		addFlags(current.PersistentFlags())
	}
	return flags
}

func applyHostFlag(flag *pflag.Flag, value string, cfg config.Hook, result *SplitArgsResult) error {
	if err := flag.Value.Set(value); err != nil {
		return err
	}
	flag.Changed = true
	switch flag.Name {
	case cmdcommon.ProfileFlagName:
		result.ProfileOverride = value
	case cmdcommon.OutputFlagName:
		cfg.SetString(cmdcommon.OutputConfigPath, value)
	case cmdcommon.LogLevelFlagName:
		cfg.SetString(cmdcommon.LogLevelConfigPath, value)
	case cmdcommon.LogFileFlagName:
		cfg.SetString(cmdcommon.LogFileConfigPath, value)
	case cmdcommon.ColorThemeFlagName:
		cfg.SetString(cmdcommon.ColorThemeConfigPath, value)
	case jqoutput.FlagName:
		cfg.SetString(jqoutput.DefaultExpressionConfigPath, value)
	case jqoutput.ColorFlagName:
		cfg.SetString(jqoutput.ColorEnabledConfigPath, value)
	case jqoutput.ColorThemeFlagName:
		cfg.SetString(jqoutput.ColorThemeConfigPath, value)
	case jqoutput.RawOutputFlagName:
		rawOutput, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Set(jqoutput.RawOutputConfigPath, rawOutput)
	case konnectcommon.BaseURLFlagName:
		cfg.SetString(konnectcommon.BaseURLConfigPath, value)
	case konnectcommon.RegionFlagName:
		cfg.SetString(konnectcommon.RegionConfigPath, value)
	case konnectcommon.PATFlagName:
		cfg.SetString(konnectcommon.PATConfigPath, value)
	case konnectcommon.RequestPageSizeFlagName:
		pageSize, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.Set(konnectcommon.RequestPageSizeConfigPath, pageSize)
	}
	return nil
}

func findChildByName(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}

func findChildByNameOrAlias(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
		if slices.Contains(child.Aliases, name) {
			return child
		}
	}
	return nil
}

func syntheticOwnedBy(command *cobra.Command, extensionID string) bool {
	if command == nil || command.Annotations == nil {
		return false
	}
	return command.Annotations[annotationSynthetic] == "true" &&
		command.Annotations[annotationExtensionID] == extensionID
}

func mergeAliases(command *cobra.Command, aliases []string) {
	seen := map[string]struct{}{}
	for _, alias := range command.Aliases {
		seen[alias] = struct{}{}
	}
	for _, alias := range aliases {
		if _, ok := seen[alias]; ok {
			continue
		}
		command.Aliases = append(command.Aliases, alias)
		seen[alias] = struct{}{}
	}
}

func extensionShort(extensionID string, contribution CommandPath) string {
	summary := strings.TrimSpace(contribution.Summary)
	if summary == "" {
		summary = fmt.Sprintf("Run %s extension command", extensionID)
	}
	return summary + " [extension: " + extensionID + "]"
}

func extensionLong(extensionID string, contribution CommandPath) string {
	var b strings.Builder
	description := strings.TrimSpace(contribution.Description)
	if description == "" {
		description = strings.TrimSpace(contribution.Summary)
	}
	if description != "" {
		fmt.Fprintln(&b, description)
		fmt.Fprintln(&b)
	}
	fmt.Fprintf(&b, "Extension: %s\n", extensionID)
	fmt.Fprintf(&b, "Usage: %s\n", strings.Replace(contribution.Usage, "kongctl", meta.CLIName, 1))
	return strings.TrimSpace(b.String())
}

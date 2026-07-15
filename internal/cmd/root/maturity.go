package root

import (
	"fmt"
	"slices"
	"strings"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type maturityHelpEntry struct {
	name  string
	level maturity.Level
}

type maturityValueHelp struct {
	name   string
	levels map[maturity.Level][]string
}

func maturityCommandDescription(parent, child *cobra.Command) (string, error) {
	if child == nil {
		return "", nil
	}
	description := child.Short
	if parent == nil {
		return description, nil
	}
	parentMaturity, err := maturity.ResolveCommand(parent)
	if err != nil {
		return "", err
	}
	childMaturity, err := maturity.ResolveCommand(child)
	if err != nil {
		return "", err
	}
	if childMaturity.Effective.Level.LessThan(parentMaturity.Effective.Level) {
		description += " [" + childMaturity.Effective.Level.DisplayName() + "]"
	}
	return description, nil
}

func maturityUsage(command *cobra.Command) (string, error) {
	commandMaturity, err := maturity.ResolveCommand(command)
	if err != nil {
		return "", err
	}

	flagEntries, flagValues, err := maturityFlagHelp(command, commandMaturity)
	if err != nil {
		return "", err
	}
	argumentEntries, argumentValues, err := maturityArgumentHelp(command, commandMaturity)
	if err != nil {
		return "", err
	}

	if commandMaturity.Effective.Level == maturity.LevelGA && len(flagEntries) == 0 && len(flagValues) == 0 &&
		len(argumentEntries) == 0 && len(argumentValues) == 0 {
		return "", nil
	}

	var b strings.Builder
	fmt.Fprintln(&b, "Maturity:")
	if commandMaturity.Effective.Level != maturity.LevelGA {
		fmt.Fprintf(&b, "  %s\n", commandMaturity.Effective.Level.DisplayName())
		writeMaturityMessage(&b, commandMaturity.Effective.Message, "  ")
		if reference := strings.TrimSpace(commandMaturity.Effective.ReferenceURL); reference != "" {
			fmt.Fprintf(&b, "  Learn more: %s\n", reference)
		}
	}
	for _, entry := range flagEntries {
		fmt.Fprintf(&b, "  --%s: %s\n", entry.name, entry.level.DisplayName())
	}
	for _, entry := range argumentEntries {
		fmt.Fprintf(&b, "  %s: %s\n", displayArgumentName(entry.name), entry.level.DisplayName())
	}
	writeMaturityValueHelp(&b, flagValues, func(name string) string { return "--" + name })
	writeMaturityValueHelp(&b, argumentValues, displayArgumentName)
	return strings.TrimRight(b.String(), "\n"), nil
}

func maturityFlagHelp(
	command *cobra.Command,
	commandMaturity maturity.Resolution,
) ([]maturityHelpEntry, []maturityValueHelp, error) {
	flags := visibleMaturityFlags(command)
	entries := make([]maturityHelpEntry, 0)
	valueEntries := make([]maturityValueHelp, 0)
	for _, flag := range flags {
		resolved, err := maturity.ResolveFlag(command, flag.Name)
		if err != nil {
			return nil, nil, err
		}
		if resolved.Effective.Level.LessThan(commandMaturity.Effective.Level) {
			entries = append(entries, maturityHelpEntry{name: flag.Name, level: resolved.Effective.Level})
		}
		declaredValues, err := maturity.DeclaredFlagValues(flag)
		if err != nil {
			return nil, nil, err
		}
		valueHelp := maturityValueHelp{name: flag.Name, levels: make(map[maturity.Level][]string)}
		for value := range declaredValues {
			valueMaturity, err := maturity.ResolveFlagValue(command, flag.Name, value)
			if err != nil {
				return nil, nil, err
			}
			if valueMaturity.Effective.Level.LessThan(resolved.Effective.Level) {
				valueHelp.levels[valueMaturity.Effective.Level] = append(
					valueHelp.levels[valueMaturity.Effective.Level], value,
				)
			}
		}
		if len(valueHelp.levels) > 0 {
			valueEntries = append(valueEntries, valueHelp)
		}
	}
	slices.SortFunc(entries, func(a, b maturityHelpEntry) int { return strings.Compare(a.name, b.name) })
	slices.SortFunc(valueEntries, func(a, b maturityValueHelp) int { return strings.Compare(a.name, b.name) })
	return entries, valueEntries, nil
}

func maturityArgumentHelp(
	command *cobra.Command,
	commandMaturity maturity.Resolution,
) ([]maturityHelpEntry, []maturityValueHelp, error) {
	declaredArguments, err := maturity.DeclaredArguments(command)
	if err != nil {
		return nil, nil, err
	}
	declaredValues, err := maturity.DeclaredArgumentValues(command)
	if err != nil {
		return nil, nil, err
	}
	names := make(map[string]struct{}, len(declaredArguments)+len(declaredValues))
	for name := range declaredArguments {
		names[name] = struct{}{}
	}
	for name := range declaredValues {
		names[name] = struct{}{}
	}

	entries := make([]maturityHelpEntry, 0)
	valueEntries := make([]maturityValueHelp, 0)
	for name := range names {
		resolved, err := maturity.ResolveArgument(command, name)
		if err != nil {
			return nil, nil, err
		}
		if resolved.Effective.Level.LessThan(commandMaturity.Effective.Level) {
			entries = append(entries, maturityHelpEntry{name: name, level: resolved.Effective.Level})
		}
		valueHelp := maturityValueHelp{name: name, levels: make(map[maturity.Level][]string)}
		for value := range declaredValues[name] {
			valueMaturity, err := maturity.ResolveArgumentValue(command, name, value)
			if err != nil {
				return nil, nil, err
			}
			if valueMaturity.Effective.Level.LessThan(resolved.Effective.Level) {
				valueHelp.levels[valueMaturity.Effective.Level] = append(
					valueHelp.levels[valueMaturity.Effective.Level], value,
				)
			}
		}
		if len(valueHelp.levels) > 0 {
			valueEntries = append(valueEntries, valueHelp)
		}
	}
	slices.SortFunc(entries, func(a, b maturityHelpEntry) int { return strings.Compare(a.name, b.name) })
	slices.SortFunc(valueEntries, func(a, b maturityValueHelp) int { return strings.Compare(a.name, b.name) })
	return entries, valueEntries, nil
}

func visibleMaturityFlags(command *cobra.Command) []*pflag.Flag {
	if command == nil {
		return nil
	}
	hidden := cmdcommon.HiddenInheritedFlags(command)
	if cmdcommon.IsOutputFormatValidationSkipped(command) {
		hidden[cmdcommon.OutputFlagName] = struct{}{}
	}
	byName := make(map[string]*pflag.Flag)
	visit := func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		if _, ok := hidden[flag.Name]; ok {
			return
		}
		byName[flag.Name] = flag
	}
	command.LocalFlags().VisitAll(visit)
	command.InheritedFlags().VisitAll(visit)
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	slices.Sort(names)
	result := make([]*pflag.Flag, 0, len(names))
	for _, name := range names {
		result = append(result, byName[name])
	}
	return result
}

func writeMaturityMessage(b *strings.Builder, message, indent string) {
	for line := range strings.Lines(strings.TrimSpace(message)) {
		line = strings.TrimSpace(line)
		if line != "" {
			fmt.Fprintln(b, indent+line)
		}
	}
}

func writeMaturityValueHelp(
	b *strings.Builder,
	entries []maturityValueHelp,
	displayName func(string) string,
) {
	for _, entry := range entries {
		fmt.Fprintf(b, "  %s values:\n", displayName(entry.name))
		for _, level := range []maturity.Level{maturity.LevelBeta, maturity.LevelTechPreview} {
			values := entry.levels[level]
			if len(values) == 0 {
				continue
			}
			slices.Sort(values)
			fmt.Fprintf(b, "    %s: %s\n", level.DisplayName(), strings.Join(values, ", "))
		}
	}
}

func displayArgumentName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "<") || strings.HasPrefix(name, "[") {
		return name
	}
	return "<" + name + ">"
}

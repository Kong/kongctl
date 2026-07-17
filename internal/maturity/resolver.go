package maturity

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ResolveCommand resolves command maturity through its ancestor hierarchy.
func ResolveCommand(command *cobra.Command) (Resolution, error) {
	if command == nil {
		return Resolution{}, fmt.Errorf("command is required")
	}
	lineage := make([]*cobra.Command, 0, 4)
	for current := command; current != nil; current = current.Parent() {
		lineage = append(lineage, current)
	}
	slices.Reverse(lineage)

	resolved := DefaultResolution()
	for _, current := range lineage {
		annotations, err := readCommandAnnotations(current)
		if err != nil {
			return Resolution{}, err
		}
		resolved = ResolveDeclaration(resolved, annotations.Command, Source{
			Kind: KindCommand,
			Path: current.CommandPath(),
		})
	}

	annotations, err := readCommandAnnotations(command)
	if err != nil {
		return Resolution{}, err
	}
	resolved.Declared = annotations.Command
	return resolved, nil
}

// ResolveFlag resolves a visible flag against the command's effective maturity.
func ResolveFlag(command *cobra.Command, flagName string) (Resolution, error) {
	flagName, err := requiredName(flagName, "flag name")
	if err != nil {
		return Resolution{}, err
	}
	if command == nil {
		return Resolution{}, fmt.Errorf("command is required")
	}
	flag := command.Flag(flagName)
	if flag == nil {
		return Resolution{}, fmt.Errorf("flag --%s is not available on command %q", flagName, command.CommandPath())
	}
	parent, err := ResolveCommand(command)
	if err != nil {
		return Resolution{}, err
	}
	annotations, err := readFlagAnnotations(flag)
	if err != nil {
		return Resolution{}, err
	}
	owner := flagOwner(command, flag)
	return ResolveDeclaration(parent, annotations.Flag, Source{
		Kind: KindFlag,
		Path: owner.CommandPath(),
		Name: flag.Name,
	}), nil
}

// ResolveArgument resolves a named positional argument against command maturity.
func ResolveArgument(command *cobra.Command, argumentName string) (Resolution, error) {
	argumentName, err := requiredName(argumentName, "argument name")
	if err != nil {
		return Resolution{}, err
	}
	parent, err := ResolveCommand(command)
	if err != nil {
		return Resolution{}, err
	}
	annotations, err := readCommandAnnotations(command)
	if err != nil {
		return Resolution{}, err
	}
	var declared *Metadata
	if metadata, ok := annotations.Arguments[argumentName]; ok {
		declared = new(metadata)
	}
	return ResolveDeclaration(parent, declared, Source{
		Kind: KindArgument,
		Path: command.CommandPath(),
		Name: argumentName,
	}), nil
}

// ResolveFlagValue resolves an accepted flag value against its flag maturity.
func ResolveFlagValue(command *cobra.Command, flagName, value string) (Resolution, error) {
	flagName, err := requiredName(flagName, "flag name")
	if err != nil {
		return Resolution{}, err
	}
	value, err = requiredName(value, "flag value")
	if err != nil {
		return Resolution{}, err
	}
	parent, err := ResolveFlag(command, flagName)
	if err != nil {
		return Resolution{}, err
	}
	flag := command.Flag(flagName)
	annotations, err := readFlagAnnotations(flag)
	if err != nil {
		return Resolution{}, err
	}
	var declared *Metadata
	if metadata, ok := annotations.Values[value]; ok {
		declared = new(metadata)
	}
	owner := flagOwner(command, flag)
	return ResolveDeclaration(parent, declared, Source{
		Kind:  KindFlagValue,
		Path:  owner.CommandPath(),
		Name:  flag.Name,
		Value: value,
	}), nil
}

// ResolveArgumentValue resolves an accepted argument value against its argument maturity.
func ResolveArgumentValue(command *cobra.Command, argumentName, value string) (Resolution, error) {
	argumentName, err := requiredName(argumentName, "argument name")
	if err != nil {
		return Resolution{}, err
	}
	value, err = requiredName(value, "argument value")
	if err != nil {
		return Resolution{}, err
	}
	parent, err := ResolveArgument(command, argumentName)
	if err != nil {
		return Resolution{}, err
	}
	annotations, err := readCommandAnnotations(command)
	if err != nil {
		return Resolution{}, err
	}
	var declared *Metadata
	if metadata, ok := annotations.ArgumentValues[argumentName][value]; ok {
		declared = new(metadata)
	}
	return ResolveDeclaration(parent, declared, Source{
		Kind:  KindArgumentValue,
		Path:  command.CommandPath(),
		Name:  argumentName,
		Value: value,
	}), nil
}

func flagOwner(command *cobra.Command, flag *pflag.Flag) *cobra.Command {
	for current := command; current != nil; current = current.Parent() {
		if current.LocalFlags().Lookup(flag.Name) == flag {
			return current
		}
	}
	return command
}

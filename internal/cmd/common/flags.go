package common

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// SetCommandTreeFlagValue updates every unchanged flag with the given name in a
// command tree.
func SetCommandTreeFlagValue(command *cobra.Command, name, value string) error {
	if command == nil {
		return nil
	}
	for _, flags := range []*pflag.FlagSet{
		command.Flags(),
		command.PersistentFlags(),
		command.LocalNonPersistentFlags(),
		command.InheritedFlags(),
	} {
		if flags == nil {
			continue
		}
		if flag := flags.Lookup(name); flag != nil && !flag.Changed {
			if err := flag.Value.Set(value); err != nil {
				return fmt.Errorf("set --%s default: %w", name, err)
			}
			flag.DefValue = value
		}
	}
	for _, child := range command.Commands() {
		if err := SetCommandTreeFlagValue(child, name, value); err != nil {
			return err
		}
	}
	return nil
}

// CommandTreeFlagChanged reports whether any flag with the given name changed
// in a command tree.
func CommandTreeFlagChanged(command *cobra.Command, name string) bool {
	return CommandTreeChangedFlag(command, name) != nil
}

// CommandTreeChangedFlagString returns the changed value for a flag with the
// given name in a command tree.
func CommandTreeChangedFlagString(command *cobra.Command, name string) (string, bool) {
	flag := CommandTreeChangedFlag(command, name)
	if flag == nil {
		return "", false
	}
	return flag.Value.String(), true
}

// CommandTreeChangedFlag returns the changed flag with the given name in a
// command tree.
func CommandTreeChangedFlag(command *cobra.Command, name string) *pflag.Flag {
	if command == nil {
		return nil
	}
	for _, flags := range []*pflag.FlagSet{
		command.Flags(),
		command.PersistentFlags(),
		command.LocalNonPersistentFlags(),
		command.InheritedFlags(),
	} {
		if flags == nil {
			continue
		}
		if flag := flags.Lookup(name); flag != nil && flag.Changed {
			return flag
		}
	}
	for _, child := range command.Commands() {
		if flag := CommandTreeChangedFlag(child, name); flag != nil {
			return flag
		}
	}
	return nil
}

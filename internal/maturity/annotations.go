package maturity

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const annotationKey = "kongctl/maturity"

type commandAnnotations struct {
	Command        *Metadata                      `json:"command,omitempty"`
	Arguments      map[string]Metadata            `json:"arguments,omitempty"`
	ArgumentValues map[string]map[string]Metadata `json:"argument_values,omitempty"`
}

type flagAnnotations struct {
	Flag   *Metadata           `json:"flag,omitempty"`
	Values map[string]Metadata `json:"values,omitempty"`
}

// AnnotateCommand attaches maturity metadata to a command.
func AnnotateCommand(command *cobra.Command, metadata Metadata) error {
	if command == nil {
		return fmt.Errorf("command is required")
	}
	if err := Validate(metadata); err != nil {
		return err
	}
	annotations, err := readCommandAnnotations(command)
	if err != nil {
		return err
	}
	annotations.Command = new(metadata)
	return writeCommandAnnotations(command, annotations)
}

// AnnotateFlag attaches maturity metadata to a flag declared by command.
func AnnotateFlag(command *cobra.Command, flagName string, metadata Metadata) error {
	flag, err := localFlag(command, flagName)
	if err != nil {
		return err
	}
	if err := Validate(metadata); err != nil {
		return err
	}
	annotations, err := readFlagAnnotations(flag)
	if err != nil {
		return err
	}
	annotations.Flag = new(metadata)
	return writeFlagAnnotations(flag, annotations)
}

// AnnotateArgument attaches maturity metadata to a named positional argument.
func AnnotateArgument(command *cobra.Command, argumentName string, metadata Metadata) error {
	if command == nil {
		return fmt.Errorf("command is required")
	}
	argumentName, err := requiredName(argumentName, "argument name")
	if err != nil {
		return err
	}
	if err := Validate(metadata); err != nil {
		return err
	}
	annotations, err := readCommandAnnotations(command)
	if err != nil {
		return err
	}
	if annotations.Arguments == nil {
		annotations.Arguments = make(map[string]Metadata)
	}
	annotations.Arguments[argumentName] = metadata
	return writeCommandAnnotations(command, annotations)
}

// AnnotateFlagValue attaches maturity metadata to an accepted flag value.
func AnnotateFlagValue(command *cobra.Command, flagName, value string, metadata Metadata) error {
	flag, err := localFlag(command, flagName)
	if err != nil {
		return err
	}
	value, err = requiredName(value, "flag value")
	if err != nil {
		return err
	}
	if err := Validate(metadata); err != nil {
		return err
	}
	annotations, err := readFlagAnnotations(flag)
	if err != nil {
		return err
	}
	if annotations.Values == nil {
		annotations.Values = make(map[string]Metadata)
	}
	annotations.Values[value] = metadata
	return writeFlagAnnotations(flag, annotations)
}

// AnnotateArgumentValue attaches maturity metadata to an accepted argument value.
func AnnotateArgumentValue(command *cobra.Command, argumentName, value string, metadata Metadata) error {
	if command == nil {
		return fmt.Errorf("command is required")
	}
	argumentName, err := requiredName(argumentName, "argument name")
	if err != nil {
		return err
	}
	value, err = requiredName(value, "argument value")
	if err != nil {
		return err
	}
	if err := Validate(metadata); err != nil {
		return err
	}
	annotations, err := readCommandAnnotations(command)
	if err != nil {
		return err
	}
	if annotations.ArgumentValues == nil {
		annotations.ArgumentValues = make(map[string]map[string]Metadata)
	}
	if annotations.ArgumentValues[argumentName] == nil {
		annotations.ArgumentValues[argumentName] = make(map[string]Metadata)
	}
	annotations.ArgumentValues[argumentName][value] = metadata
	return writeCommandAnnotations(command, annotations)
}

// DeclaredArguments returns positional argument maturity declared on command.
func DeclaredArguments(command *cobra.Command) (map[string]Metadata, error) {
	annotations, err := readCommandAnnotations(command)
	if err != nil {
		return nil, err
	}
	return cloneMetadataMap(annotations.Arguments), nil
}

// DeclaredArgumentValues returns accepted argument-value maturity declared on command.
func DeclaredArgumentValues(command *cobra.Command) (map[string]map[string]Metadata, error) {
	annotations, err := readCommandAnnotations(command)
	if err != nil {
		return nil, err
	}
	return cloneNestedMetadataMap(annotations.ArgumentValues), nil
}

// DeclaredFlagValues returns accepted value maturity declared on flag.
func DeclaredFlagValues(flag *pflag.Flag) (map[string]Metadata, error) {
	annotations, err := readFlagAnnotations(flag)
	if err != nil {
		return nil, err
	}
	return cloneMetadataMap(annotations.Values), nil
}

func localFlag(command *cobra.Command, flagName string) (*pflag.Flag, error) {
	if command == nil {
		return nil, fmt.Errorf("command is required")
	}
	flagName, err := requiredName(flagName, "flag name")
	if err != nil {
		return nil, err
	}
	flag := command.LocalFlags().Lookup(flagName)
	if flag == nil {
		return nil, fmt.Errorf("flag --%s is not declared by command %q", flagName, command.CommandPath())
	}
	return flag, nil
}

func requiredName(value, description string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", description)
	}
	return value, nil
}

func readCommandAnnotations(command *cobra.Command) (commandAnnotations, error) {
	var annotations commandAnnotations
	if command == nil {
		return annotations, fmt.Errorf("command is required")
	}
	encoded := command.Annotations[annotationKey]
	if encoded == "" {
		return annotations, nil
	}
	if err := json.Unmarshal([]byte(encoded), &annotations); err != nil {
		return annotations, fmt.Errorf("decode maturity annotation for command %q: %w", command.CommandPath(), err)
	}
	if err := validateCommandAnnotations(annotations); err != nil {
		return annotations, fmt.Errorf("validate maturity annotation for command %q: %w", command.CommandPath(), err)
	}
	return annotations, nil
}

func writeCommandAnnotations(command *cobra.Command, annotations commandAnnotations) error {
	encoded, err := json.Marshal(annotations)
	if err != nil {
		return fmt.Errorf("encode maturity annotation for command %q: %w", command.CommandPath(), err)
	}
	if command.Annotations == nil {
		command.Annotations = make(map[string]string)
	}
	command.Annotations[annotationKey] = string(encoded)
	return nil
}

func readFlagAnnotations(flag *pflag.Flag) (flagAnnotations, error) {
	var annotations flagAnnotations
	if flag == nil {
		return annotations, fmt.Errorf("flag is required")
	}
	values := flag.Annotations[annotationKey]
	if len(values) == 0 {
		return annotations, nil
	}
	if len(values) != 1 {
		return annotations, fmt.Errorf("maturity annotation for flag --%s must contain one value", flag.Name)
	}
	if err := json.Unmarshal([]byte(values[0]), &annotations); err != nil {
		return annotations, fmt.Errorf("decode maturity annotation for flag --%s: %w", flag.Name, err)
	}
	if err := validateFlagAnnotations(annotations); err != nil {
		return annotations, fmt.Errorf("validate maturity annotation for flag --%s: %w", flag.Name, err)
	}
	return annotations, nil
}

func writeFlagAnnotations(flag *pflag.Flag, annotations flagAnnotations) error {
	encoded, err := json.Marshal(annotations)
	if err != nil {
		return fmt.Errorf("encode maturity annotation for flag --%s: %w", flag.Name, err)
	}
	if flag.Annotations == nil {
		flag.Annotations = make(map[string][]string)
	}
	flag.Annotations[annotationKey] = []string{string(encoded)}
	return nil
}

func cloneMetadataMap(input map[string]Metadata) map[string]Metadata {
	if input == nil {
		return nil
	}
	result := make(map[string]Metadata, len(input))
	maps.Copy(result, input)
	return result
}

func cloneNestedMetadataMap(input map[string]map[string]Metadata) map[string]map[string]Metadata {
	if input == nil {
		return nil
	}
	result := make(map[string]map[string]Metadata, len(input))
	for key, values := range input {
		result[key] = cloneMetadataMap(values)
	}
	return result
}

func validateCommandAnnotations(annotations commandAnnotations) error {
	if annotations.Command != nil {
		if err := Validate(*annotations.Command); err != nil {
			return err
		}
	}
	for name, metadata := range annotations.Arguments {
		if err := Validate(metadata); err != nil {
			return fmt.Errorf("argument %q: %w", name, err)
		}
	}
	for name, values := range annotations.ArgumentValues {
		for value, metadata := range values {
			if err := Validate(metadata); err != nil {
				return fmt.Errorf("argument %q value %q: %w", name, value, err)
			}
		}
	}
	return nil
}

func validateFlagAnnotations(annotations flagAnnotations) error {
	if annotations.Flag != nil {
		if err := Validate(*annotations.Flag); err != nil {
			return err
		}
	}
	for value, metadata := range annotations.Values {
		if err := Validate(metadata); err != nil {
			return fmt.Errorf("value %q: %w", value, err)
		}
	}
	return nil
}

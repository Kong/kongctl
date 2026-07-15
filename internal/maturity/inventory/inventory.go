// Package inventory collects maturity metadata from command and resource registries.
package inventory

import (
	"cmp"
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Record describes one inventoried capability and its resolved maturity.
type Record struct {
	Kind      maturity.Kind      `json:"kind"`
	Path      string             `json:"path"`
	Name      string             `json:"name,omitempty"`
	Value     string             `json:"value,omitempty"`
	Declared  *maturity.Metadata `json:"declared,omitempty"`
	Effective maturity.Metadata  `json:"effective"`
	Source    maturity.Source    `json:"source"`
}

// Collect inventories the complete command tree and declarative resource registry.
func Collect(root *cobra.Command) ([]Record, error) {
	var records []Record
	if root != nil {
		if err := collectCommand(root, &records); err != nil {
			return nil, err
		}
	}
	if err := collectResources(&records); err != nil {
		return nil, err
	}
	slices.SortFunc(records, compareRecords)
	return records, nil
}

func collectCommand(command *cobra.Command, records *[]Record) error {
	resolved, err := maturity.ResolveCommand(command)
	if err != nil {
		return err
	}
	*records = append(*records, recordFromResolution(maturity.KindCommand, command.CommandPath(), "", "", resolved))

	var flagErr error
	command.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		if flagErr != nil {
			return
		}
		flagMaturity, err := maturity.ResolveFlag(command, flag.Name)
		if err != nil {
			flagErr = err
			return
		}
		*records = append(*records, recordFromResolution(
			maturity.KindFlag, command.CommandPath(), flag.Name, "", flagMaturity,
		))
		values, err := maturity.DeclaredFlagValues(flag)
		if err != nil {
			flagErr = err
			return
		}
		for value := range values {
			valueMaturity, err := maturity.ResolveFlagValue(command, flag.Name, value)
			if err != nil {
				flagErr = err
				return
			}
			*records = append(*records, recordFromResolution(
				maturity.KindFlagValue, command.CommandPath(), flag.Name, value, valueMaturity,
			))
		}
	})
	if flagErr != nil {
		return flagErr
	}

	arguments, err := maturity.DeclaredArguments(command)
	if err != nil {
		return err
	}
	argumentValues, err := maturity.DeclaredArgumentValues(command)
	if err != nil {
		return err
	}
	argumentNames := make(map[string]struct{}, len(arguments)+len(argumentValues))
	for name := range arguments {
		argumentNames[name] = struct{}{}
	}
	for name := range argumentValues {
		argumentNames[name] = struct{}{}
	}
	for name := range argumentNames {
		argumentMaturity, err := maturity.ResolveArgument(command, name)
		if err != nil {
			return err
		}
		*records = append(*records, recordFromResolution(
			maturity.KindArgument, command.CommandPath(), name, "", argumentMaturity,
		))
		for value := range argumentValues[name] {
			valueMaturity, err := maturity.ResolveArgumentValue(command, name, value)
			if err != nil {
				return err
			}
			*records = append(*records, recordFromResolution(
				maturity.KindArgumentValue, command.CommandPath(), name, value, valueMaturity,
			))
		}
	}

	for _, child := range command.Commands() {
		if err := collectCommand(child, records); err != nil {
			return err
		}
	}
	return nil
}

func collectResources(records *[]Record) error {
	resourceTypes := resources.RegisteredTypes()
	slices.SortFunc(resourceTypes, func(a, b resources.ResourceType) int {
		return cmp.Compare(string(a), string(b))
	})
	for _, resourceType := range resourceTypes {
		resolved, err := resources.MaturityFor(resourceType)
		if err != nil {
			return err
		}
		path := string(resourceType)
		*records = append(*records, recordFromResolution(maturity.KindResource, path, "", "", resolved))
		for _, operation := range resources.Operations() {
			operationMaturity, err := resources.MaturityFor(resourceType, operation)
			if err != nil {
				return err
			}
			*records = append(*records, recordFromResolution(
				maturity.KindOperation, path, string(operation), "", operationMaturity,
			))
		}
	}
	return nil
}

func recordFromResolution(
	kind maturity.Kind,
	path, name, value string,
	resolved maturity.Resolution,
) Record {
	return Record{
		Kind:      kind,
		Path:      path,
		Name:      name,
		Value:     value,
		Declared:  resolved.Declared,
		Effective: resolved.Effective,
		Source:    resolved.Source,
	}
}

func compareRecords(a, b Record) int {
	if result := cmp.Compare(string(a.Kind), string(b.Kind)); result != 0 {
		return result
	}
	if result := cmp.Compare(a.Path, b.Path); result != 0 {
		return result
	}
	if result := cmp.Compare(a.Name, b.Name); result != 0 {
		return result
	}
	return cmp.Compare(a.Value, b.Value)
}

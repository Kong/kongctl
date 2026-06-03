package common

import (
	"context"
	"fmt"
	"maps"
	"strconv"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/spf13/cobra"
)

const (
	NamespaceFlagName          = "namespace"
	OverwriteNamespaceFlagName = "overwrite-namespace"
)

type AdoptResult struct {
	ResourceType string `json:"resource_type"  yaml:"resource_type"`
	ID           string `json:"id"             yaml:"id"`
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace    string `json:"namespace"      yaml:"namespace"`
}

type AdoptFlags struct {
	Namespace          string
	OverwriteNamespace bool
}

func AddAdoptFlags(cmd *cobra.Command) error {
	cmd.PersistentFlags().String(NamespaceFlagName, "", "Namespace label to apply to the resource (required)")
	cmd.PersistentFlags().Bool(
		OverwriteNamespaceFlagName,
		false,
		"Overwrite an existing namespace label on the resource",
	)

	return nil
}

func ReadAdoptFlags(cmd *cobra.Command) (AdoptFlags, error) {
	var flags AdoptFlags

	namespaceFlag := cmd.Flag(NamespaceFlagName)
	if namespaceFlag == nil {
		return flags, fmt.Errorf("missing --%s flag", NamespaceFlagName)
	}
	flags.Namespace = namespaceFlag.Value.String()

	nsValidator := validator.NewNamespaceValidator()
	if err := nsValidator.ValidateNamespace(flags.Namespace); err != nil {
		return flags, &cmdpkg.ConfigurationError{Err: err}
	}

	overwriteFlag := cmd.Flag(OverwriteNamespaceFlagName)
	if overwriteFlag == nil {
		return flags, nil
	}

	overwrite, err := strconv.ParseBool(overwriteFlag.Value.String())
	if err != nil {
		return flags, fmt.Errorf("invalid --%s value: %w", OverwriteNamespaceFlagName, err)
	}
	flags.OverwriteNamespace = overwrite

	return flags, nil
}

func PointerLabelMap(existing map[string]string, namespace string) map[string]*string {
	cloned := make(map[string]string, len(existing))
	maps.Copy(cloned, existing)
	cloned[labels.NamespaceKey] = namespace

	result := make(map[string]*string, len(cloned))
	for k, v := range cloned {
		val := v
		result[k] = &val
	}

	return result
}

func StringLabelMap(existing map[string]string, namespace string) map[string]string {
	cloned := make(map[string]string, len(existing))
	maps.Copy(cloned, existing)
	cloned[labels.NamespaceKey] = namespace
	return cloned
}

func EnsureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

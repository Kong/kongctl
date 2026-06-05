package common

import (
	"context"
	"fmt"
	"maps"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/segmentio/cli"
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

type AdoptRunSetup struct {
	Helper     cmdpkg.Helper
	AdoptFlags AdoptFlags
	OutType    cmdCommon.OutputFormat
	Cfg        config.Hook
	SDK        helpers.SDKAPI
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

	overwrite, err := cmd.Flags().GetBool(OverwriteNamespaceFlagName)
	if err != nil {
		return flags, nil
	}
	flags.OverwriteNamespace = overwrite

	return flags, nil
}

func PointerLabelMap(existing map[string]string, namespace string) map[string]*string {
	result := make(map[string]*string, len(existing)+1)
	for k, v := range existing {
		val := v
		result[k] = &val
	}
	ns := namespace
	result[labels.NamespaceKey] = &ns

	return result
}

func SetupAdoptRun(cobraCmd *cobra.Command, args []string) (AdoptRunSetup, error) {
	setup := AdoptRunSetup{
		Helper: cmdpkg.BuildHelper(cobraCmd, args),
	}

	adoptFlags, err := ReadAdoptFlags(cobraCmd)
	if err != nil {
		return setup, err
	}
	setup.AdoptFlags = adoptFlags

	outType, err := setup.Helper.GetOutputFormat()
	if err != nil {
		return setup, err
	}
	setup.OutType = outType

	cfg, err := setup.Helper.GetConfig()
	if err != nil {
		return setup, err
	}
	setup.Cfg = cfg

	logger, err := setup.Helper.GetLogger()
	if err != nil {
		return setup, err
	}

	sdk, err := setup.Helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return setup, err
	}
	setup.SDK = sdk

	return setup, nil
}

func PrintAdoptResult(
	helper cmdpkg.Helper,
	outType cmdCommon.OutputFormat,
	result *AdoptResult,
	resourceDisplayName string,
) error {
	streams := helper.GetStreams()
	if outType == cmdCommon.TEXT {
		name := result.Name
		if name == "" {
			name = result.ID
		}
		fmt.Fprintf(streams.Out, "Adopted %s %q (%s) into namespace %q\n",
			resourceDisplayName, name, result.ID, result.Namespace)
		return nil
	}

	printer, err := cli.Format(outType.String(), streams.Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	printer.Print(result)
	return nil
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

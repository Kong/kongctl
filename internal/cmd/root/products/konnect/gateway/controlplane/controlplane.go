package controlplane

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "control-plane"
)

var (
	controlPlanesUse   = CommandName
	controlPlanesShort = i18n.T("root.products.konnect.gateway.controlplane.controlPlaneShort",
		"Manage Konnect Kong Gateway control planes")
	controlPlanesLong = normalizers.LongDesc(i18n.T("root.products.konnect.gateway.controlplane.controlPlaneLong",
		`The controlplane command allows you to work with Konnect Kong Gateway control plane resources.`))
	controlPlanesExample = normalizers.Examples(
		i18n.T("root.products.konnect.gateway.gateway.controlplane.controlPlaneExamples",
			fmt.Sprintf(`
	# List all the Konnect control planes for the organization
	%[1]s get konnect gateway control-planes
	# Get a specific Konnect control plane
	%[1]s get konnect gateway control-plane <id|name>
	# Use declarative workflows to provision new resources
	%[1]s apply -f <config-file>
	`, meta.CLIName)))
)

func NewControlPlaneCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     controlPlanesUse,
		Short:   controlPlanesShort,
		Long:    controlPlanesLong,
		Example: controlPlanesExample,
		Aliases: []string{"control-planes", "controlplane", "controlplanes", "cp", "cps", "CP", "CPS"},
	}

	switch verb {
	case verbs.Get:
		return newGetControlPlaneCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.List:
		return newGetControlPlaneCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.Create:
		return newCreateControlPlaneCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.Delete:
		return newDeleteControlPlaneCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	case verbs.Add, verbs.Apply, verbs.Dump, verbs.Update, verbs.Help, verbs.Login,
		verbs.Plan, verbs.Sync, verbs.Diff, verbs.Export, verbs.Adopt, verbs.API, verbs.Kai, verbs.View, verbs.Logout,
		verbs.Patch:
		return &baseCmd, nil
	}

	return &baseCmd, nil
}

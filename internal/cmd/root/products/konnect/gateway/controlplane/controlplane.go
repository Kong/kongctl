package controlplane

import (
	"fmt"

	"github.com/kong/kong-cli/internal/cmd/root/verbs"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
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
	# Create a new Konnect control plane
	%[1]s create konnect gateway control-plane <name>
	`, meta.CLIName)))
)

func NewControlPlaneCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     controlPlanesUse,
		Short:   controlPlanesShort,
		Long:    controlPlanesLong,
		Example: controlPlanesExample,
		Aliases: []string{"control-planes", "controlplane", "controlplanes", "cp", "cps", "CP", "CPS"},
	}

	// If Verb == Get or List
	if verb == verbs.Get || verb == verbs.List {
		return newGetControlPlaneCmd(&baseCmd).Command, nil
	} else if verb == verbs.Create {
		return newCreateControlPlaneCmd(&baseCmd).Command, nil
	} else if verb == verbs.Delete {
		return newDeleteControlPlaneCmd(&baseCmd).Command, nil
	}

	return nil, fmt.Errorf("unsupported verb %s", verb)
}

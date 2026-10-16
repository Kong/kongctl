package inspect

import (
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/spf13/cobra"
)

const Verb = verbs.Inspect

func NewInspectCmd() (*cobra.Command, error) {
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}
	cmd := &cobra.Command{
		Use:   Verb.String(),
		Short: "Inspect computed configuration and runtime state",
		Long: "Inspect resource relationships, computed configuration, and runtime state. " +
			"Currently supports Kong Mesh.",
		Example:           fmt.Sprintf("  %s inspect mesh dataplanes", meta.CLIName),
		PersistentPreRunE: verbs.KonnectFirstPreRunE(Verb, konnectCmd),
		RunE:              cmdpkg.RequireSubcommand,
	}
	cmdpkg.MarkRequiresSubcommand(cmd)
	bindings := []struct{ flag, path, description string }{
		{common.BaseURLFlagName, common.BaseURLConfigPath, "Base URL for Konnect API requests."},
		{common.RegionFlagName, common.RegionConfigPath, "Konnect region identifier (for example eu)."},
		{common.PATFlagName, common.PATConfigPath, "Konnect Personal Access Token (PAT)."},
	}
	for _, binding := range bindings {
		cmd.PersistentFlags().String(binding.flag, "",
			fmt.Sprintf("%s\n- Config path: [ %s ]", binding.description, binding.path))
	}
	meshCmd, err := mesh.NewMeshCmd(Verb, nil, konnectCmd.PersistentPreRunE)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(konnectCmd, meshCmd)
	return cmd, nil
}

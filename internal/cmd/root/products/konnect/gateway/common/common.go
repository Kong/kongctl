package common

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/spf13/cobra"
)

const (
	ControlPlaneNameFlagName   = "control-plane-name"
	ControlPlaneIDFlagName     = "control-plane-id"
	ControlPlaneNameConfigPath = "konnect.gateway.control-plane.name"
	ControlPlaneIDConfigPath   = "konnect.gateway.control-plane.id"
)

func BindControlPlaneFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}
	f := c.Flags().Lookup(ControlPlaneIDFlagName)
	e = cfg.BindFlag(ControlPlaneIDConfigPath, f)
	if e != nil {
		return e
	}
	f = c.Flags().Lookup(ControlPlaneNameFlagName)
	e = cfg.BindFlag(ControlPlaneNameConfigPath, f)
	if e != nil {
		return e
	}
	return nil
}

func AddControlPlaneFlags(c *cobra.Command) {
	// ---- CP Identifiers, mutually exclusive
	c.Flags().String(ControlPlaneIDFlagName, "",
		fmt.Sprintf(`The ID of the control plane to use for a gateway service command.
- Config path: [ %s ]`, ControlPlaneIDConfigPath))

	c.Flags().String(ControlPlaneNameFlagName, "",
		fmt.Sprintf(`The name of the control plane to use for a gateway service command.
- Config path: [ %s ]`, ControlPlaneNameConfigPath))
	c.MarkFlagsMutuallyExclusive(ControlPlaneIDFlagName, ControlPlaneNameFlagName)
}

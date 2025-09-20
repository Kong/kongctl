package portal

import (
	"errors"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

const (
	portalIDFlagName   = "portal-id"
	portalNameFlagName = "portal-name"

	portalIDConfigPath   = "konnect.portal.id"
	portalNameConfigPath = "konnect.portal.name"
	valueNA              = "n/a"
)

func addPortalChildFlags(cmd *cobra.Command) {
	cmd.Flags().String(portalIDFlagName, "",
		fmt.Sprintf(`The ID of the portal that owns the resource.
- Config path: [ %s ]`, portalIDConfigPath))
	cmd.Flags().String(portalNameFlagName, "",
		fmt.Sprintf(`The name of the portal that owns the resource.
- Config path: [ %s ]`, portalNameConfigPath))
	cmd.MarkFlagsMutuallyExclusive(portalIDFlagName, portalNameFlagName)
}

func bindPortalChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(portalIDFlagName); flag != nil {
		if err := cfg.BindFlag(portalIDConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(portalNameFlagName); flag != nil {
		if err := cfg.BindFlag(portalNameConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func getPortalIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(portalIDConfigPath), cfg.GetString(portalNameConfigPath)
}

func resolvePortalIDByName(
	name string,
	portalClient helpers.PortalAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (string, error) {
	portal, err := runListByName(name, portalClient, helper, cfg)
	if err != nil {
		var execErr *cmd.ExecutionError
		if errors.As(err, &execErr) {
			return "", err
		}
		return "", &cmd.ConfigurationError{Err: err}
	}

	if portal == nil {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("portal %q not found", name),
		}
	}

	return portal.ID, nil
}

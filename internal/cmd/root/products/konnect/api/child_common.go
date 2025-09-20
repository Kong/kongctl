package api

import (
	"errors"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

const (
	apiIDFlagName   = "api-id"
	apiNameFlagName = "api-name"

	apiIDConfigPath   = "konnect.api.id"
	apiNameConfigPath = "konnect.api.name"
	valueNA           = "n/a"
)

func addAPIChildFlags(cmd *cobra.Command) {
	cmd.Flags().String(apiIDFlagName, "",
		fmt.Sprintf(`The ID of the API that owns the resource.
- Config path: [ %s ]`, apiIDConfigPath))
	cmd.Flags().String(apiNameFlagName, "",
		fmt.Sprintf(`The name of the API that owns the resource.
- Config path: [ %s ]`, apiNameConfigPath))
	cmd.MarkFlagsMutuallyExclusive(apiIDFlagName, apiNameFlagName)
}

func bindAPIChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(apiIDFlagName); flag != nil {
		if err := cfg.BindFlag(apiIDConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(apiNameFlagName); flag != nil {
		if err := cfg.BindFlag(apiNameConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func getAPIIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(apiIDConfigPath), cfg.GetString(apiNameConfigPath)
}

func resolveAPIIDByName(
	name string,
	apiClient helpers.APIAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (string, error) {
	api, err := runListByName(name, apiClient, helper, cfg)
	if err != nil {
		var execErr *cmd.ExecutionError
		if errors.As(err, &execErr) {
			return "", err
		}
		return "", &cmd.ConfigurationError{Err: err}
	}
	if api == nil {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("api %q not found", name),
		}
	}
	return api.ID, nil
}

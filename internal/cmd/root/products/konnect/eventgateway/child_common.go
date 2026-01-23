package eventgateway

import (
	"errors"
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

const (
	gatewayIDFlagName   = "gateway-id"
	gatewayNameFlagName = "gateway-name"

	gatewayIDConfigPath   = "konnect.event-gateway.id"
	gatewayNameConfigPath = "konnect.event-gateway.name"

	backendClusterIDFlagName   = "backend-cluster-id"
	backendClusterNameFlagName = "backend-cluster-name"

	backendClusterIDConfigPath   = "konnect.event-gateway.backend-cluster.id"
	backendClusterNameConfigPath = "konnect.event-gateway.backend-cluster.name"

	valueNA = "n/a"
)

func addEventGatewayChildFlags(cmd *cobra.Command) {
	cmd.Flags().String(gatewayIDFlagName, "",
		fmt.Sprintf(`The ID of the event gateway that owns the resource.
- Config path: [ %s ]`, gatewayIDConfigPath))
	cmd.Flags().String(gatewayNameFlagName, "",
		fmt.Sprintf(`The name of the event gateway that owns the resource.
- Config path: [ %s ]`, gatewayNameConfigPath))
	cmd.MarkFlagsMutuallyExclusive(gatewayIDFlagName, gatewayNameFlagName)
}

func bindEventGatewayChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(gatewayIDFlagName); flag != nil {
		if err := cfg.BindFlag(gatewayIDConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(gatewayNameFlagName); flag != nil {
		if err := cfg.BindFlag(gatewayNameConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func getEventGatewayIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(gatewayIDConfigPath), cfg.GetString(gatewayNameConfigPath)
}

func addBackendClusterChildFlags(cmd *cobra.Command) {
	cmd.Flags().String(backendClusterIDFlagName, "",
		fmt.Sprintf(`The ID of the backend cluster to retrieve.
- Config path: [ %s ]`, backendClusterIDConfigPath))
	cmd.Flags().String(backendClusterNameFlagName, "",
		fmt.Sprintf(`The name of the backend cluster to retrieve.
- Config path: [ %s ]`, backendClusterNameConfigPath))
	cmd.MarkFlagsMutuallyExclusive(backendClusterIDFlagName, backendClusterNameFlagName)
}

func bindBackendClusterChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(backendClusterIDFlagName); flag != nil {
		if err := cfg.BindFlag(backendClusterIDConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(backendClusterNameFlagName); flag != nil {
		if err := cfg.BindFlag(backendClusterNameConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func getBackendClusterIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(backendClusterIDConfigPath), cfg.GetString(backendClusterNameConfigPath)
}

func resolveEventGatewayIDByName(
	name string,
	gatewayClient helpers.EGWControlPlaneAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (string, error) {
	gateway, err := runListByName(name, gatewayClient, helper, cfg)
	if err != nil {
		var execErr *cmd.ExecutionError
		if errors.As(err, &execErr) {
			return "", err
		}
		return "", &cmd.ConfigurationError{Err: err}
	}

	if gateway == nil {
		return "", &cmd.ConfigurationError{
			Err: fmt.Errorf("event gateway %q not found", name),
		}
	}

	return gateway.ID, nil
}

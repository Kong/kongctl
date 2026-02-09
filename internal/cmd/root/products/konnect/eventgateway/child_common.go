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

	virtualClusterIDFlagName   = "virtual-cluster-id"
	virtualClusterNameFlagName = "virtual-cluster-name"

	virtualClusterIDConfigPath   = "konnect.event-gateway.virtual-cluster.id"
	virtualClusterNameConfigPath = "konnect.event-gateway.virtual-cluster.name"

	listenerIDFlagName   = "listener-id"
	listenerNameFlagName = "listener-name"

	listenerIDConfigPath   = "konnect.event-gateway.listener.id"
	listenerNameConfigPath = "konnect.event-gateway.listener.name"

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

func addVirtualClusterChildFlags(cmd *cobra.Command) {
	cmd.Flags().String(virtualClusterIDFlagName, "",
		fmt.Sprintf(`The ID of the virtual cluster to retrieve.
- Config path: [ %s ]`, virtualClusterIDConfigPath))
	cmd.Flags().String(virtualClusterNameFlagName, "",
		fmt.Sprintf(`The name of the virtual cluster to retrieve.
- Config path: [ %s ]`, virtualClusterNameConfigPath))
	cmd.MarkFlagsMutuallyExclusive(virtualClusterIDFlagName, virtualClusterNameFlagName)
}

func bindVirtualClusterChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(virtualClusterIDFlagName); flag != nil {
		if err := cfg.BindFlag(virtualClusterIDConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(virtualClusterNameFlagName); flag != nil {
		if err := cfg.BindFlag(virtualClusterNameConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func getVirtualClusterIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(virtualClusterIDConfigPath), cfg.GetString(virtualClusterNameConfigPath)
}

func addListenerChildFlags(cmd *cobra.Command) {
	cmd.Flags().String(listenerIDFlagName, "",
		fmt.Sprintf(`The ID of the listener to retrieve.
- Config path: [ %s ]`, listenerIDConfigPath))
	cmd.Flags().String(listenerNameFlagName, "",
		fmt.Sprintf(`The name of the listener to retrieve.
- Config path: [ %s ]`, listenerNameConfigPath))
	cmd.MarkFlagsMutuallyExclusive(listenerIDFlagName, listenerNameFlagName)
}

func bindListenerChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(listenerIDFlagName); flag != nil {
		if err := cfg.BindFlag(listenerIDConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := c.Flags().Lookup(listenerNameFlagName); flag != nil {
		if err := cfg.BindFlag(listenerNameConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func getListenerIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(listenerIDConfigPath), cfg.GetString(listenerNameConfigPath)
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

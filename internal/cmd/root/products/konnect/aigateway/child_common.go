package aigateway

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/cobra"
)

const (
	aiGatewayIDFlagName   = "gateway-id"
	aiGatewayNameFlagName = "gateway-name"

	aiGatewayIDConfigPath   = "konnect.ai-gateway.id"
	aiGatewayNameConfigPath = "konnect.ai-gateway.name"

	aiGatewayProviderIDFlagName   = "provider-id"
	aiGatewayProviderNameFlagName = "provider-name"

	aiGatewayProviderIDConfigPath   = "konnect.ai-gateway.provider.id"
	aiGatewayProviderNameConfigPath = "konnect.ai-gateway.provider.name"

	aiGatewayMissingValue = "n/a"
)

func addAIGatewayChildFlags(c *cobra.Command) {
	c.Flags().String(aiGatewayIDFlagName, "",
		fmt.Sprintf(`The ID of the AI Gateway that owns the resource.
- Config path: [ %s ]`, aiGatewayIDConfigPath))
	c.Flags().String(aiGatewayNameFlagName, "",
		fmt.Sprintf(`The display name of the AI Gateway that owns the resource.
- Config path: [ %s ]`, aiGatewayNameConfigPath))
	c.MarkFlagsMutuallyExclusive(aiGatewayIDFlagName, aiGatewayNameFlagName)
}

func bindAIGatewayChildFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(aiGatewayIDFlagName); flag != nil {
		if err := cfg.BindFlag(aiGatewayIDConfigPath, flag); err != nil {
			return err
		}
	}
	if flag := c.Flags().Lookup(aiGatewayNameFlagName); flag != nil {
		if err := cfg.BindFlag(aiGatewayNameConfigPath, flag); err != nil {
			return err
		}
	}
	return nil
}

func getAIGatewayIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(aiGatewayIDConfigPath), cfg.GetString(aiGatewayNameConfigPath)
}

func addAIGatewayProviderFlags(c *cobra.Command) {
	c.Flags().String(aiGatewayProviderIDFlagName, "",
		fmt.Sprintf(`The ID of the AI Gateway Provider to retrieve.
- Config path: [ %s ]`, aiGatewayProviderIDConfigPath))
	c.Flags().String(aiGatewayProviderNameFlagName, "",
		fmt.Sprintf(`The name of the AI Gateway Provider to retrieve.
- Config path: [ %s ]`, aiGatewayProviderNameConfigPath))
	c.MarkFlagsMutuallyExclusive(aiGatewayProviderIDFlagName, aiGatewayProviderNameFlagName)
}

func bindAIGatewayProviderFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(aiGatewayProviderIDFlagName); flag != nil {
		if err := cfg.BindFlag(aiGatewayProviderIDConfigPath, flag); err != nil {
			return err
		}
	}
	if flag := c.Flags().Lookup(aiGatewayProviderNameFlagName); flag != nil {
		if err := cfg.BindFlag(aiGatewayProviderNameConfigPath, flag); err != nil {
			return err
		}
	}
	return nil
}

func getAIGatewayProviderIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(aiGatewayProviderIDConfigPath), cfg.GetString(aiGatewayProviderNameConfigPath)
}

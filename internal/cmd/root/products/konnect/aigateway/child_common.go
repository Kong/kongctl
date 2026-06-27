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

	aiGatewayPolicyIDFlagName   = "policy-id"
	aiGatewayPolicyNameFlagName = "policy-name"

	aiGatewayPolicyIDConfigPath   = "konnect.ai-gateway.policy.id"
	aiGatewayPolicyNameConfigPath = "konnect.ai-gateway.policy.name"

	aiGatewayMCPServerIDFlagName   = "mcp-server-id"
	aiGatewayMCPServerNameFlagName = "mcp-server-name"

	aiGatewayMCPServerIDConfigPath   = "konnect.ai-gateway.mcp-server.id"
	aiGatewayMCPServerNameConfigPath = "konnect.ai-gateway.mcp-server.name"

	aiGatewayVaultIDFlagName   = "vault-id"
	aiGatewayVaultNameFlagName = "vault-name"

	aiGatewayVaultIDConfigPath   = "konnect.ai-gateway.vault.id"
	aiGatewayVaultNameConfigPath = "konnect.ai-gateway.vault.name"

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

func addAIGatewayPolicyFlags(c *cobra.Command) {
	c.Flags().String(aiGatewayPolicyIDFlagName, "",
		fmt.Sprintf(`The ID of the AI Gateway Policy to retrieve.
- Config path: [ %s ]`, aiGatewayPolicyIDConfigPath))
	c.Flags().String(aiGatewayPolicyNameFlagName, "",
		fmt.Sprintf(`The name of the AI Gateway Policy to retrieve.
- Config path: [ %s ]`, aiGatewayPolicyNameConfigPath))
	c.MarkFlagsMutuallyExclusive(aiGatewayPolicyIDFlagName, aiGatewayPolicyNameFlagName)
}

func bindAIGatewayPolicyFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(aiGatewayPolicyIDFlagName); flag != nil {
		if err := cfg.BindFlag(aiGatewayPolicyIDConfigPath, flag); err != nil {
			return err
		}
	}
	if flag := c.Flags().Lookup(aiGatewayPolicyNameFlagName); flag != nil {
		if err := cfg.BindFlag(aiGatewayPolicyNameConfigPath, flag); err != nil {
			return err
		}
	}
	return nil
}

func getAIGatewayPolicyIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(aiGatewayPolicyIDConfigPath), cfg.GetString(aiGatewayPolicyNameConfigPath)
}

func addAIGatewayMCPServerFlags(c *cobra.Command) {
	c.Flags().String(aiGatewayMCPServerIDFlagName, "",
		fmt.Sprintf(`The ID of the AI Gateway MCP Server to retrieve.
- Config path: [ %s ]`, aiGatewayMCPServerIDConfigPath))
	c.Flags().String(aiGatewayMCPServerNameFlagName, "",
		fmt.Sprintf(`The name of the AI Gateway MCP Server to retrieve.
- Config path: [ %s ]`, aiGatewayMCPServerNameConfigPath))
	c.MarkFlagsMutuallyExclusive(aiGatewayMCPServerIDFlagName, aiGatewayMCPServerNameFlagName)
}

func bindAIGatewayMCPServerFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(aiGatewayMCPServerIDFlagName); flag != nil {
		if err := cfg.BindFlag(aiGatewayMCPServerIDConfigPath, flag); err != nil {
			return err
		}
	}
	if flag := c.Flags().Lookup(aiGatewayMCPServerNameFlagName); flag != nil {
		if err := cfg.BindFlag(aiGatewayMCPServerNameConfigPath, flag); err != nil {
			return err
		}
	}
	return nil
}

func getAIGatewayMCPServerIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(aiGatewayMCPServerIDConfigPath), cfg.GetString(aiGatewayMCPServerNameConfigPath)
}

func addAIGatewayVaultFlags(c *cobra.Command) {
	c.Flags().String(aiGatewayVaultIDFlagName, "",
		fmt.Sprintf(`The ID of the AI Gateway Vault to retrieve.
- Config path: [ %s ]`, aiGatewayVaultIDConfigPath))
	c.Flags().String(aiGatewayVaultNameFlagName, "",
		fmt.Sprintf(`The name of the AI Gateway Vault to retrieve.
- Config path: [ %s ]`, aiGatewayVaultNameConfigPath))
	c.MarkFlagsMutuallyExclusive(aiGatewayVaultIDFlagName, aiGatewayVaultNameFlagName)
}

func bindAIGatewayVaultFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(aiGatewayVaultIDFlagName); flag != nil {
		if err := cfg.BindFlag(aiGatewayVaultIDConfigPath, flag); err != nil {
			return err
		}
	}
	if flag := c.Flags().Lookup(aiGatewayVaultNameFlagName); flag != nil {
		if err := cfg.BindFlag(aiGatewayVaultNameConfigPath, flag); err != nil {
			return err
		}
	}
	return nil
}

func getAIGatewayVaultIdentifiers(cfg config.Hook) (id string, name string) {
	return cfg.GetString(aiGatewayVaultIDConfigPath), cfg.GetString(aiGatewayVaultNameConfigPath)
}

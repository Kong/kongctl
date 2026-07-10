package aigateway

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	aiGatewayIDFlagName   = "gateway-id"
	aiGatewayNameFlagName = "gateway-name"

	aiGatewayIDConfigPath   = "konnect.ai-gateway.id"
	aiGatewayNameConfigPath = "konnect.ai-gateway.name"

	aiGatewayProviderIDFlagName     = "model-provider-id"
	aiGatewayProviderNameFlagName   = "model-provider-name"
	aiGatewayProviderIDConfigPath   = "konnect.ai-gateway.model-provider.id"
	aiGatewayProviderNameConfigPath = "konnect.ai-gateway.model-provider.name"

	aiGatewayIdentityProviderIDFlagName   = "identity-provider-id"
	aiGatewayIdentityProviderNameFlagName = "identity-provider-name"

	aiGatewayIdentityProviderIDConfigPath   = "konnect.ai-gateway.identity-provider.id"
	aiGatewayIdentityProviderNameConfigPath = "konnect.ai-gateway.identity-provider.name"

	aiGatewayPolicyIDFlagName   = "policy-id"
	aiGatewayPolicyNameFlagName = "policy-name"

	aiGatewayPolicyIDConfigPath   = "konnect.ai-gateway.policy.id"
	aiGatewayPolicyNameConfigPath = "konnect.ai-gateway.policy.name"

	aiGatewayAgentIDFlagName   = "agent-id"
	aiGatewayAgentNameFlagName = "agent-name"

	aiGatewayAgentIDConfigPath   = "konnect.ai-gateway.agent.id"
	aiGatewayAgentNameConfigPath = "konnect.ai-gateway.agent.name"

	aiGatewayConsumerIDFlagName   = "consumer-id"
	aiGatewayConsumerNameFlagName = "consumer-name"

	aiGatewayConsumerIDConfigPath   = "konnect.ai-gateway.consumer.id"
	aiGatewayConsumerNameConfigPath = "konnect.ai-gateway.consumer.name"

	aiGatewayConsumerCredentialIDFlagName   = "credential-id"   //nolint:gosec
	aiGatewayConsumerCredentialNameFlagName = "credential-name" //nolint:gosec

	aiGatewayConsumerCredentialIDConfigPath   = "konnect.ai-gateway.consumer.credential.id"   //nolint:gosec
	aiGatewayConsumerCredentialNameConfigPath = "konnect.ai-gateway.consumer.credential.name" //nolint:gosec

	aiGatewayConsumerGroupIDFlagName   = "consumer-group-id"
	aiGatewayConsumerGroupNameFlagName = "consumer-group-name"

	aiGatewayConsumerGroupIDConfigPath   = "konnect.ai-gateway.consumer-group.id"
	aiGatewayConsumerGroupNameConfigPath = "konnect.ai-gateway.consumer-group.name"

	aiGatewayModelIDFlagName   = "model-id"
	aiGatewayModelNameFlagName = "model-name"

	aiGatewayModelIDConfigPath   = "konnect.ai-gateway.model.id"
	aiGatewayModelNameConfigPath = "konnect.ai-gateway.model.name"

	aiGatewayMCPServerIDFlagName   = "mcp-server-id"
	aiGatewayMCPServerNameFlagName = "mcp-server-name"

	aiGatewayMCPServerIDConfigPath   = "konnect.ai-gateway.mcp-server.id"
	aiGatewayMCPServerNameConfigPath = "konnect.ai-gateway.mcp-server.name"

	aiGatewayVaultIDFlagName   = "vault-id"
	aiGatewayVaultNameFlagName = "vault-name"

	aiGatewayVaultIDConfigPath   = "konnect.ai-gateway.vault.id"
	aiGatewayVaultNameConfigPath = "konnect.ai-gateway.vault.name"

	aiGatewayNodeIDFlagName   = "node-id"
	aiGatewayNodeIDConfigPath = "konnect.ai-gateway.node.id"

	aiGatewayDataPlaneCertificateIDFlagName    = "data-plane-certificate-id"
	aiGatewayDataPlaneCertificateTitleFlagName = "data-plane-certificate-title"

	aiGatewayDataPlaneCertificateIDConfigPath    = "konnect.ai-gateway.data-plane-certificate.id"
	aiGatewayDataPlaneCertificateTitleConfigPath = "konnect.ai-gateway.data-plane-certificate.title"

	aiGatewayMissingValue = "n/a"

	aiGatewayFieldCreatedAt   = "created_at"
	aiGatewayFieldUpdatedAt   = "updated_at"
	aiGatewayFieldID          = "id"
	aiGatewayFieldName        = "name"
	aiGatewayFieldType        = "type"
	aiGatewayFieldDisplayName = "display_name"
	aiGatewayFieldLabels      = "labels"
	aiGatewayFieldManagedBy   = "managed_by"
	aiGatewayFieldConfig      = "config"

	aiGatewayHeaderID          = "ID"
	aiGatewayHeaderName        = "NAME"
	aiGatewayHeaderDisplayName = "DISPLAY NAME"
	aiGatewayHeaderType        = "TYPE"
	aiGatewayHeaderTTL         = "TTL"
	aiGatewayHeaderEnabled     = "ENABLED"
	aiGatewayHeaderPolicies    = "POLICIES"
	aiGatewayHeaderUpdated     = "UPDATED"
)

type pairedAIGatewayFlags struct {
	idFlag   string
	idPath   string
	idHelp   string
	nameFlag string
	namePath string
	nameHelp string
}

type flagBinding struct {
	flag       string
	configPath string
}

func addPairedAIGatewayFlags(c *cobra.Command, flags pairedAIGatewayFlags) {
	c.Flags().String(flags.idFlag, "", fmt.Sprintf(`%s
- Config path: [ %s ]`, flags.idHelp, flags.idPath))
	c.Flags().String(flags.nameFlag, "", fmt.Sprintf(`%s
- Config path: [ %s ]`, flags.nameHelp, flags.namePath))
	c.MarkFlagsMutuallyExclusive(flags.idFlag, flags.nameFlag)
}

func bindFlag(cfg config.Hook, flags *pflag.FlagSet, flagName, configPath string) error {
	if cfg == nil || flags == nil {
		return nil
	}
	if flag := flags.Lookup(flagName); flag != nil {
		return cfg.BindFlag(configPath, flag)
	}
	return nil
}

func bindAIGatewayFlags(c *cobra.Command, args []string, bindings ...flagBinding) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	for _, binding := range bindings {
		if err := bindFlag(cfg, c.Flags(), binding.flag, binding.configPath); err != nil {
			return err
		}
	}
	return nil
}

func pairedAIGatewayBindings(flags pairedAIGatewayFlags) []flagBinding {
	return []flagBinding{
		{flag: flags.idFlag, configPath: flags.idPath},
		{flag: flags.nameFlag, configPath: flags.namePath},
	}
}

func getPairedAIGatewayIdentifiers(cfg config.Hook, idPath string, namePath string) (id string, name string) {
	return cfg.GetString(idPath), cfg.GetString(namePath)
}

var (
	aiGatewayChildFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayIDFlagName,
		idPath:   aiGatewayIDConfigPath,
		idHelp:   "The ID of the AI Gateway that owns the resource.",
		nameFlag: aiGatewayNameFlagName,
		namePath: aiGatewayNameConfigPath,
		nameHelp: "The name or display_name of the AI Gateway that owns the resource.",
	}

	aiGatewayProviderFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayProviderIDFlagName,
		idPath:   aiGatewayProviderIDConfigPath,
		idHelp:   "The ID of the AI Gateway Model Provider to retrieve.",
		nameFlag: aiGatewayProviderNameFlagName,
		namePath: aiGatewayProviderNameConfigPath,
		nameHelp: "The name of the AI Gateway Model Provider to retrieve.",
	}

	aiGatewayIdentityProviderFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayIdentityProviderIDFlagName,
		idPath:   aiGatewayIdentityProviderIDConfigPath,
		idHelp:   "The ID of the AI Gateway Identity Provider to retrieve.",
		nameFlag: aiGatewayIdentityProviderNameFlagName,
		namePath: aiGatewayIdentityProviderNameConfigPath,
		nameHelp: "The name of the AI Gateway Identity Provider to retrieve.",
	}

	aiGatewayPolicyFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayPolicyIDFlagName,
		idPath:   aiGatewayPolicyIDConfigPath,
		idHelp:   "The ID of the AI Gateway Policy to retrieve.",
		nameFlag: aiGatewayPolicyNameFlagName,
		namePath: aiGatewayPolicyNameConfigPath,
		nameHelp: "The name of the AI Gateway Policy to retrieve.",
	}

	aiGatewayAgentFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayAgentIDFlagName,
		idPath:   aiGatewayAgentIDConfigPath,
		idHelp:   "The ID of the AI Gateway Agent to retrieve.",
		nameFlag: aiGatewayAgentNameFlagName,
		namePath: aiGatewayAgentNameConfigPath,
		nameHelp: "The name of the AI Gateway Agent to retrieve.",
	}

	aiGatewayConsumerFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayConsumerIDFlagName,
		idPath:   aiGatewayConsumerIDConfigPath,
		idHelp:   "The ID of the AI Gateway Consumer to retrieve.",
		nameFlag: aiGatewayConsumerNameFlagName,
		namePath: aiGatewayConsumerNameConfigPath,
		nameHelp: "The name of the AI Gateway Consumer to retrieve.",
	}

	aiGatewayConsumerCredentialFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayConsumerCredentialIDFlagName,
		idPath:   aiGatewayConsumerCredentialIDConfigPath,
		idHelp:   "The ID of the AI Gateway Consumer Credential to retrieve.",
		nameFlag: aiGatewayConsumerCredentialNameFlagName,
		namePath: aiGatewayConsumerCredentialNameConfigPath,
		nameHelp: "The name of the AI Gateway Consumer Credential to retrieve.",
	}

	aiGatewayConsumerGroupFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayConsumerGroupIDFlagName,
		idPath:   aiGatewayConsumerGroupIDConfigPath,
		idHelp:   "The ID of the AI Gateway Consumer Group to retrieve.",
		nameFlag: aiGatewayConsumerGroupNameFlagName,
		namePath: aiGatewayConsumerGroupNameConfigPath,
		nameHelp: "The name of the AI Gateway Consumer Group to retrieve.",
	}

	aiGatewayModelFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayModelIDFlagName,
		idPath:   aiGatewayModelIDConfigPath,
		idHelp:   "The ID of the AI Gateway Model to retrieve.",
		nameFlag: aiGatewayModelNameFlagName,
		namePath: aiGatewayModelNameConfigPath,
		nameHelp: "The name of the AI Gateway Model to retrieve.",
	}

	aiGatewayMCPServerFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayMCPServerIDFlagName,
		idPath:   aiGatewayMCPServerIDConfigPath,
		idHelp:   "The ID of the AI Gateway MCP Server to retrieve.",
		nameFlag: aiGatewayMCPServerNameFlagName,
		namePath: aiGatewayMCPServerNameConfigPath,
		nameHelp: "The name of the AI Gateway MCP Server to retrieve.",
	}

	aiGatewayVaultFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayVaultIDFlagName,
		idPath:   aiGatewayVaultIDConfigPath,
		idHelp:   "The ID of the AI Gateway Vault to retrieve.",
		nameFlag: aiGatewayVaultNameFlagName,
		namePath: aiGatewayVaultNameConfigPath,
		nameHelp: "The name of the AI Gateway Vault to retrieve.",
	}

	aiGatewayDataPlaneCertificateFlags = pairedAIGatewayFlags{
		idFlag:   aiGatewayDataPlaneCertificateIDFlagName,
		idPath:   aiGatewayDataPlaneCertificateIDConfigPath,
		idHelp:   "The ID of the AI Gateway data plane certificate to retrieve.",
		nameFlag: aiGatewayDataPlaneCertificateTitleFlagName,
		namePath: aiGatewayDataPlaneCertificateTitleConfigPath,
		nameHelp: "The title of the AI Gateway data plane certificate to retrieve.",
	}
)

func addAIGatewayChildFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayChildFlags)
}

func bindAIGatewayChildFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayChildFlags)...)
}

func getAIGatewayIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(cfg, aiGatewayIDConfigPath, aiGatewayNameConfigPath)
}

func addAIGatewayProviderFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayProviderFlags)
}

func bindAIGatewayProviderFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayProviderFlags)...)
}

func getAIGatewayProviderIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(cfg, aiGatewayProviderIDConfigPath, aiGatewayProviderNameConfigPath)
}

func addAIGatewayIdentityProviderFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayIdentityProviderFlags)
}

func bindAIGatewayIdentityProviderFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayIdentityProviderFlags)...)
}

func getAIGatewayIdentityProviderIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(
		cfg,
		aiGatewayIdentityProviderIDConfigPath,
		aiGatewayIdentityProviderNameConfigPath,
	)
}

func addAIGatewayPolicyFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayPolicyFlags)
}

func bindAIGatewayPolicyFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayPolicyFlags)...)
}

func getAIGatewayPolicyIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(cfg, aiGatewayPolicyIDConfigPath, aiGatewayPolicyNameConfigPath)
}

func addAIGatewayAgentFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayAgentFlags)
}

func bindAIGatewayAgentFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayAgentFlags)...)
}

func getAIGatewayAgentIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(cfg, aiGatewayAgentIDConfigPath, aiGatewayAgentNameConfigPath)
}

func addAIGatewayConsumerFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayConsumerFlags)
}

func bindAIGatewayConsumerFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayConsumerFlags)...)
}

func getAIGatewayConsumerIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(cfg, aiGatewayConsumerIDConfigPath, aiGatewayConsumerNameConfigPath)
}

func addAIGatewayConsumerCredentialFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayConsumerCredentialFlags)
}

func bindAIGatewayConsumerCredentialFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayConsumerCredentialFlags)...)
}

func getAIGatewayConsumerCredentialIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(
		cfg,
		aiGatewayConsumerCredentialIDConfigPath,
		aiGatewayConsumerCredentialNameConfigPath,
	)
}

func addAIGatewayConsumerGroupFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayConsumerGroupFlags)
}

func bindAIGatewayConsumerGroupFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayConsumerGroupFlags)...)
}

func getAIGatewayConsumerGroupIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(
		cfg,
		aiGatewayConsumerGroupIDConfigPath,
		aiGatewayConsumerGroupNameConfigPath,
	)
}

func addAIGatewayModelFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayModelFlags)
}

func bindAIGatewayModelFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayModelFlags)...)
}

func getAIGatewayModelIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(cfg, aiGatewayModelIDConfigPath, aiGatewayModelNameConfigPath)
}

func addAIGatewayMCPServerFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayMCPServerFlags)
}

func bindAIGatewayMCPServerFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayMCPServerFlags)...)
}

func getAIGatewayMCPServerIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(cfg, aiGatewayMCPServerIDConfigPath, aiGatewayMCPServerNameConfigPath)
}

func addAIGatewayVaultFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayVaultFlags)
}

func bindAIGatewayVaultFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayVaultFlags)...)
}

func getAIGatewayVaultIdentifiers(cfg config.Hook) (id string, name string) {
	return getPairedAIGatewayIdentifiers(cfg, aiGatewayVaultIDConfigPath, aiGatewayVaultNameConfigPath)
}

func addAIGatewayNodeFlags(c *cobra.Command) {
	c.Flags().String(aiGatewayNodeIDFlagName, "",
		fmt.Sprintf(`The ID of the AI Gateway Node to retrieve.
- Config path: [ %s ]`, aiGatewayNodeIDConfigPath))
}

func bindAIGatewayNodeFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, flagBinding{
		flag:       aiGatewayNodeIDFlagName,
		configPath: aiGatewayNodeIDConfigPath,
	})
}

func getAIGatewayNodeIdentifier(cfg config.Hook) string {
	return cfg.GetString(aiGatewayNodeIDConfigPath)
}

func addAIGatewayDataPlaneCertificateFlags(c *cobra.Command) {
	addPairedAIGatewayFlags(c, aiGatewayDataPlaneCertificateFlags)
}

func bindAIGatewayDataPlaneCertificateFlags(c *cobra.Command, args []string) error {
	return bindAIGatewayFlags(c, args, pairedAIGatewayBindings(aiGatewayDataPlaneCertificateFlags)...)
}

func getAIGatewayDataPlaneCertificateIdentifiers(cfg config.Hook) (id string, title string) {
	return getPairedAIGatewayIdentifiers(
		cfg,
		aiGatewayDataPlaneCertificateIDConfigPath,
		aiGatewayDataPlaneCertificateTitleConfigPath,
	)
}

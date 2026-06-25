package aigateway

import (
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	aiGatewayUse   = "ai-gateway"
	aiGatewayShort = i18n.T("root.konnect.ai-gateway.gatewayShort", "Manage Konnect AI Gateway resources")
	aiGatewayLong  = normalizers.LongDesc(i18n.T("root.konnect.ai-gateway.gatewayLong",
		`The ai-gateway command allows you to manage Konnect AI Gateway resources.`))
)

// NewAIGatewayCmd creates the AI Gateway command for supported verbs.
func NewAIGatewayCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     aiGatewayUse,
		Short:   aiGatewayShort,
		Long:    aiGatewayLong,
		Aliases: []string{"ai-gateways", "aigw", "AIGW"},
	}

	if verb == verbs.Get || verb == verbs.List {
		root := newGetAIGatewayCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command
		root.AddCommand(newGetAIGatewayProvidersCmd(verb, addParentFlags, parentPreRun))
		return root, nil
	}

	return &baseCmd, nil
}

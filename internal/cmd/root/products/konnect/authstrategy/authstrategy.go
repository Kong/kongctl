package authstrategy

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "auth-strategy"
)

var (
	authStrategyUse   = CommandName
	authStrategyShort = i18n.T("root.products.konnect.authstrategy.authStrategyShort",
		"Manage Konnect authentication strategy resources")
	authStrategyLong = normalizers.LongDesc(i18n.T("root.products.konnect.authstrategy.authStrategyLong",
		`The auth-strategy command allows you to work with Konnect authentication strategy resources.`))
	authStrategyExample = normalizers.Examples(
		i18n.T("root.products.konnect.authstrategy.authStrategyExamples",
			fmt.Sprintf(`
	# List all the Konnect auth strategies for the organization
	%[1]s get auth-strategies
	# Get a specific Konnect auth strategy
	%[1]s get auth-strategy <id|name>
	# List auth strategies of a specific type
	%[1]s get auth-strategies --type key_auth
	# List auth strategies using explicit konnect product
	%[1]s get konnect auth-strategies
	`, meta.CLIName)))
)

func NewAuthStrategyCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     authStrategyUse,
		Short:   authStrategyShort,
		Long:    authStrategyLong,
		Example: authStrategyExample,
		Aliases: []string{"auth-strategies", "auth-strategy", "as", "AS"},
	}

	// Handle supported verbs
	if verb == verbs.Get || verb == verbs.List {
		return newGetAuthStrategyCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}

	// Return base command for unsupported verbs
	return &baseCmd, nil
}

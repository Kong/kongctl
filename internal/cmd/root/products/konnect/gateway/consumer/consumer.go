package consumer

import (
	"fmt"

	"github.com/kong/kong-cli/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kong-cli/internal/cmd/root/verbs"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	routeUse   = "consumer"
	routeShort = i18n.T("root.products.konnect.gateway.consumer.consumerShort",
		"Manage Konnect Kong Gateway Consumers")
	routeLong = normalizers.LongDesc(i18n.T("root.products.konnect.gateway.consumer.consumerLong",
		`The consumer command allows you to work with Konect Kong Gateway Consumer resources.`))
	routeExamples = normalizers.Examples(i18n.T("root.products.konnect.gateway.consumer.consumerExamples",
		fmt.Sprintf(`
	# List the Konnect consumers
	%[1]s get konnect gateway consumers --control-plane-id <id>
	# Get a specific Konnect route
	%[1]s get konnect gateway route --control-plane-id <id> <id|name>
	`, meta.CLIName)))
)

func NewConsumerCmd(verb verbs.VerbValue) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     routeUse,
		Short:   routeShort,
		Long:    routeLong,
		Example: routeExamples,
		Aliases: []string{"consumer", "consumers"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return common.BindControlPlaneFlags(cmd, args)
		},
	}

	common.AddControlPlaneFlags(&baseCmd)

	if verb == verbs.Get || verb == verbs.List {
		return newGetConsumerCmd(&baseCmd).Command, nil
	}

	return &baseCmd, nil
}
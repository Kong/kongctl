package get

import (
	"context"

	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

// NewDirectGatewayCmd creates a gateway command that works at the root level (Konnect-first)
func NewDirectGatewayCmd() (*cobra.Command, error) {
	addFlags := func(verb verbs.VerbValue, cmd *cobra.Command) {
		gateway.AddGatewayFlags(verb, cmd)
	}

	// Define the preRunE function to set up Konnect context
	preRunE := func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, products.Product, konnect.Product)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
		c.SetContext(ctx)

		return gateway.BindGatewayFlags(c, args)
	}

	// Create the gateway command using the existing gateway package
	gatewayCmd, err := gateway.NewGatewayCmd(Verb, addFlags, preRunE)
	if err != nil {
		return nil, err
	}

	// Override the example to show direct usage without "konnect"
	gatewayCmd.Example = `  # List all control planes
  kongctl get gateway control-planes
  # Get a specific control plane
  kongctl get gateway control-plane <id|name>
  # List services in a control plane
  kongctl get gateway control-plane services --control-plane-name <name>
  # Get a specific service
  kongctl get gateway control-plane service <id|name> --control-plane-name <name>`

	return gatewayCmd, nil
}

package update

import (
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/meteringbilling"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
)

func NewUpdateCmd() (*cobra.Command, error) {
	root := &cobra.Command{Use: "update", Short: "Update Konnect resources"}
	cmd.ConfigureRequiresSubcommand(root)
	root.AddCommand(meteringbilling.NewCommand(verbs.Update))
	product := &cobra.Command{Use: konnect.Product.String(), Short: "Update Konnect resources"}
	product.AddCommand(meteringbilling.NewCommand(verbs.Update))
	root.AddCommand(product)
	return root, nil
}

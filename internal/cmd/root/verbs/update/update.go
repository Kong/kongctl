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
	family, err := meteringbilling.NewCommand(verbs.Update)
	if err != nil {
		return nil, err
	}
	root.AddCommand(family)
	product := &cobra.Command{Use: konnect.Product.String(), Short: "Update Konnect resources"}
	family, err = meteringbilling.NewCommand(verbs.Update)
	if err != nil {
		return nil, err
	}
	product.AddCommand(family)
	root.AddCommand(product)
	return root, nil
}

package catalog

import (
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/catalog/service"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	catalogUse   = "catalog"
	catalogShort = i18n.T("root.products.konnect.catalog.catalogShort", "Manage Konnect catalog resources")
	catalogLong  = normalizers.LongDesc(i18n.T("root.products.konnect.catalog.catalogLong",
		`The catalog command allows you to manage Konnect Service Catalog resources.`))
)

// NewCatalogCmd creates the catalog command group.
func NewCatalogCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     catalogUse,
		Short:   catalogShort,
		Long:    catalogLong,
		Aliases: []string{"catalogs"},
	}

	svcCmd, err := service.NewServiceCmd(verb, addParentFlags, parentPreRun)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(svcCmd)

	return cmd, nil
}

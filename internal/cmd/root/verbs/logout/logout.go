package logout

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Logout
)

var (
	logoutUse = Verb.String()

	logoutShort = i18n.T("root.verbs.logout.logoutShort", "Logout from Kong Konnect")

	logoutLong = normalizers.LongDesc(i18n.T("root.verbs.logout.logoutLong",
		`Logout from Kong Konnect by removing stored authentication tokens.`))

	logoutExamples = normalizers.Examples(i18n.T("root.verbs.logout.logoutExamples",
		fmt.Sprintf(`
	# Logout from Kong Konnect (default)
	%[1]s logout

	# Logout from Kong Konnect (explicit)
	%[1]s logout konnect
	`, meta.CLIName)))
)

func NewLogoutCmd() (*cobra.Command, error) {
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:     logoutUse,
		Short:   logoutShort,
		Long:    logoutLong,
		Example: logoutExamples,
		RunE:    konnectCmd.RunE,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = context.WithValue(ctx, verbs.Verb, Verb)
			ctx = context.WithValue(ctx, products.Product, konnect.Product)
			ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
			cmd.SetContext(ctx)

			if konnectCmd.PersistentPreRunE != nil {
				return konnectCmd.PersistentPreRunE(cmd, args)
			}
			return nil
		},
	}

	cmd.Flags().AddFlagSet(konnectCmd.Flags())

	cmd.AddCommand(konnectCmd)

	return cmd, nil
}

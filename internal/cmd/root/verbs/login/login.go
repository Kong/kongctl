package login

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
	Verb = verbs.Login
)

var (
	loginUse = Verb.String()

	loginShort = i18n.T("root.verbs.login.loginShort", "Login to Kong Konnect")

	loginLong = normalizers.LongDesc(i18n.T("root.verbs.login.loginLong",
		`Login to Kong Konnect using browser-based device authorization flow.

The login command authenticates your CLI to Kong Konnect, storing authentication
tokens for subsequent commands. By default, this command connects to Konnect.`))

	loginExamples = normalizers.Examples(i18n.T("root.verbs.login.loginExamples",
		fmt.Sprintf(`
		# Login to Kong Konnect (default)
		%[1]s login
		
		# Login to Kong Konnect (explicit)
		%[1]s login konnect
		`, meta.CLIName)))
)

func NewLoginCmd() (*cobra.Command, error) {
	// Create the konnect subcommand first to get its implementation
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:     loginUse,
		Short:   loginShort,
		Long:    loginLong,
		Example: loginExamples,
		// Use the konnect command's RunE directly for Konnect-first pattern
		RunE: konnectCmd.RunE,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = context.WithValue(ctx, verbs.Verb, Verb)
			ctx = context.WithValue(ctx, products.Product, konnect.Product)
			ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(common.KonnectSDKFactory))
			cmd.SetContext(ctx)

			// Also call the konnect command's PersistentPreRunE to set up binding
			if konnectCmd.PersistentPreRunE != nil {
				return konnectCmd.PersistentPreRunE(cmd, args)
			}
			return nil
		},
	}

	// Copy flags from konnect command to parent
	cmd.Flags().AddFlagSet(konnectCmd.Flags())

	// Also add konnect as a subcommand for explicit usage
	cmd.AddCommand(konnectCmd)

	return cmd, nil
}

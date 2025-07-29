package apply

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
	Verb = verbs.Apply
)

var (
	applyUse = Verb.String()

	applyShort = i18n.T("root.verbs.apply.applyShort", "Apply configuration changes (create/update only)")

	applyLong = normalizers.LongDesc(i18n.T("root.verbs.apply.applyLong",
		`Apply configuration changes to Kong Konnect. Creates new resources and updates existing ones.`))

	applyExamples = normalizers.Examples(i18n.T("root.verbs.apply.applyExamples",
		fmt.Sprintf(`  %[1]s apply -f api.yaml
  %[1]s apply -f ./configs/ --recursive
  %[1]s apply --plan plan.json

Use "%[1]s help apply" for detailed documentation`, meta.CLIName)))
)

func NewApplyCmd() (*cobra.Command, error) {
	// Create the konnect subcommand first to get its implementation
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:     applyUse,
		Short:   applyShort,
		Long:    applyLong,
		Example: applyExamples,
		Aliases: []string{"a", "A"},
		// Use the konnect command's RunE directly for Konnect-first pattern
		RunE: konnectCmd.RunE,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = context.WithValue(ctx, verbs.Verb, Verb)
			ctx = context.WithValue(ctx, products.Product, konnect.Product)
			ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactory())
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

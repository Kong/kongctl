package sync

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
	Verb = verbs.Sync
)

var (
	syncUse = Verb.String()

	syncShort = i18n.T("root.verbs.sync.syncShort",
		"Synchronize declarative configuration to target environment")

	syncLong = normalizers.LongDesc(i18n.T("root.verbs.sync.syncLong",
		`Synchronize declarative configuration files to the target environment.

Sync analyzes the current state, compares it with the desired state defined
in the configuration files, and applies the necessary changes to achieve
the desired state.`))

	syncExamples = normalizers.Examples(i18n.T("root.verbs.sync.syncExamples",
		fmt.Sprintf(`
		# Sync configuration from files
		%[1]s sync -f portal.yaml -f auth.yaml
		
		# Sync configuration from comma-separated files
		%[1]s sync -f portal.yaml,auth.yaml,api.yaml
		
		# Sync configuration from directory
		%[1]s sync -f ./config
		
		# Sync configuration from directory recursively
		%[1]s sync -f ./config -R
		
		# Sync configuration with dry-run to preview changes
		%[1]s sync -f ./config --dry-run
		
		# Sync configuration from stdin
		cat portal.yaml | %[1]s sync -f -
		`, meta.CLIName)))
)

func NewSyncCmd() (*cobra.Command, error) {
	// Create the konnect subcommand first to get its implementation
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:     syncUse,
		Short:   syncShort,
		Long:    syncLong,
		Example: syncExamples,
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
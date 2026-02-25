package sync

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
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
		"Full state synchronization (create/update/delete)")

	syncLong = normalizers.LongDesc(i18n.T("root.verbs.sync.syncLong",
		`Synchronize configuration with Kong Konnect. Creates, updates, and DELETES resources.`))

	syncExamples = normalizers.Examples(i18n.T("root.verbs.sync.syncExamples",
		fmt.Sprintf(`  %[1]s sync -f api.yaml
  %[1]s sync -f ./configs/ --dry-run
  %[1]s sync --plan plan.json --auto-approve

Use "%[1]s help sync" for detailed documentation`, meta.CLIName)))
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
		Args: verbs.NoPositionalArgs,
		// Use the konnect command's RunE directly for Konnect-first pattern
		RunE: konnectCmd.RunE,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
			// Also run the konnect command's PersistentPreRunE to set up SDKAPIFactory
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

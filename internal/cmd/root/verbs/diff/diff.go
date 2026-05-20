package diff

import (
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Diff
)

var (
	diffUse = Verb.String()

	diffShort = i18n.T("root.verbs.diff.diffShort",
		"Show declarative configuration differences")

	diffLong = normalizers.LongDesc(i18n.T("root.verbs.diff.diffLong",
		`Display differences between current and desired state.`))
)

func NewDiffCmd() (*cobra.Command, error) {
	// Create the konnect subcommand first to get its implementation
	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}

	cmd := &cobra.Command{
		Use:     diffUse,
		Short:   diffShort,
		Long:    diffLong,
		Example: konnectCmd.Example,
		Args:    verbs.NoPositionalArgs,
		// Use the konnect command's RunE directly for Konnect-first pattern
		RunE:              konnectCmd.RunE,
		PersistentPreRunE: verbs.KonnectFirstPreRunE(Verb, konnectCmd),
	}

	// Copy flags from konnect command to parent
	cmd.Flags().AddFlagSet(konnectCmd.Flags())

	// Also add konnect as a subcommand for explicit usage
	cmd.AddCommand(konnectCmd)

	return cmd, nil
}

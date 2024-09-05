package apply

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
	Verb = verbs.Apply
)

var (
	applyUse = Verb.String()

	applyShort = i18n.T("root.verbs.apply.applyShort", "Apply configurations")

	applyLong = normalizers.LongDesc(i18n.T("root.verbs.apply.applyLong",
		`Use apply to apply a configuration to a system.

Further sub-commands are required to determine which remote system is contacted. 
The command will apply a given configuration and report a result depending on further arguments.
Output can be formatted in multiple ways to aid in further processing.`))

	applyExamples = normalizers.Examples(i18n.T("root.verbs.apply.applyExamples",
		fmt.Sprintf(`
		# Apply a configuration to a Konnect Kong Gateway control plane
		%[1]s apply konnect gateway --control-plane <cp-name> <path-to-config.yaml>
		`, meta.CLIName)))
)

func NewApplyCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     applyUse,
		Short:   applyShort,
		Long:    applyLong,
		Example: applyExamples,
		Aliases: []string{"a", "A"},
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		},
	}

	c, e := konnect.NewKonnectCmd(Verb)
	if e != nil {
		return nil, e
	}

	cmd.AddCommand(c)
	return cmd, nil
}

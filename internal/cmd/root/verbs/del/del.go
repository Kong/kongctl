package del

import (
	"context"
	"fmt"

	"github.com/kong/kong-cli/internal/cmd/root/products/konnect"
	"github.com/kong/kong-cli/internal/cmd/root/verbs"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Delete
)

var (
	deleteuse = Verb.String()

	deleteShort = i18n.T("root.verbs.delete.deleteShort", "Delete objects")

	deleteLong = normalizers.LongDesc(i18n.T("root.verbs.delete.deleteLong",
		`Use delete to delete a new object.

Further sub-commands are required to determine which remote system is contacted (if necessary). 
The command will delete an object and report a result depending on further arguments.
Output can be formatted in multiple ways to aid in further processing.`))

	deleteExamples = normalizers.Examples(i18n.T("root.verbs.delete.deleteExamples",
		fmt.Sprintf(`
		# Delete a Konnect Kong Gateway control plane
		%[1]s delete konnect gateway controlplane <id>
		`, meta.CLIName)))
)

func NewDeleteCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     deleteuse,
		Short:   deleteShort,
		Long:    deleteLong,
		Example: deleteExamples,
		Aliases: []string{"d", "D", "del", "rm", "DEL", "RM"},
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

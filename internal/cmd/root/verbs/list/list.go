package list

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
	Verb = verbs.List
)

var (
	listUse = Verb.String()

	listShort = i18n.T("root.verbs.list.listShort", "Retrieve object lists")

	listLong = normalizers.LongDesc(i18n.T("root.verbs.list.listLong",
		`Use list to retrieve a list of objects.

Further sub-commands are required to determine which remote system is contacted (if necessary). 
The command will return a list depending on further arguments.
Output can be formatted in multiple ways to aid in further processing.`))

	listExamples = normalizers.Examples(i18n.T("root.verbs.list.listExamples",
		fmt.Sprintf(`
		# Retrieve Konnect control planes
		%[1]s list konnect gateway controlplanes
		`, meta.CLIName)))
)

func NewListCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     listUse,
		Short:   listShort,
		Long:    listLong,
		Example: listExamples,
		Aliases: []string{"ls", "l"},
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

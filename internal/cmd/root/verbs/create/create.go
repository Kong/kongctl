package create

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
	Verb = verbs.Create
)

var (
	createUse = Verb.String()

	createShort = i18n.T("root.verbs.create.createShort", "Create objects")

	createLong = normalizers.LongDesc(i18n.T("root.verbs.create.createLong",
		`Use create to create a new object.

Further sub-commands are required to determine which remote system is contacted (if necessary). 
The command will create an object and report a result depending on further arguments.
Output can be formatted in multiple ways to aid in further processing.`))

	createExamples = normalizers.Examples(i18n.T("root.verbs.create.createExamples",
		fmt.Sprintf(`
		# Create a new Konnect Kong Gateway control planes
		%[1]s create konnect gateway controlplane <name>
		`, meta.CLIName)))
)

func NewCreateCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     createUse,
		Short:   createShort,
		Long:    createLong,
		Example: createExamples,
		Aliases: []string{"c", "C"},
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		},
	}

	// TODO: Determine if creating profiles for the command make sense and how to implement
	// cmd.AddCommand(profileCmd.NewProfileCmd())
	c, e := konnect.NewKonnectCmd(Verb)
	if e != nil {
		return nil, e
	}

	cmd.AddCommand(c)
	return cmd, nil
}

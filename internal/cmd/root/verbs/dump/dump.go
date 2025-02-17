package dump

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
	Verb = verbs.Dump
)

var (
	dumpUse = Verb.String()

	dumpShort = i18n.T("root.verbs.dump.dumpShort", "Dump configuration")

	dumpLong = normalizers.LongDesc(i18n.T("root.verbs.dump.dumpLong",
		`Export configuration from a running system`))

	dumpExamples = normalizers.Examples(i18n.T("root.verbs.dump.dumpExamples",
		fmt.Sprintf(`
		# Dump a configuration to a Konnect Kong Gateway control plane
		%[1]s dump konnect portal
		`, meta.CLIName)))
)

func NewDumpCommand() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     dumpUse,
		Short:   dumpShort,
		Long:    dumpLong,
		Example: dumpExamples,
		Aliases: []string{"d", "D"},
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

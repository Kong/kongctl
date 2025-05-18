package dump

import (
	"context"
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
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

	dumpShort = i18n.T("root.verbs.dump.dumpShort", "Dump objects")

	dumpLong = normalizers.LongDesc(i18n.T("root.verbs.dump.dumpLong",
		`Use dump to export an object or list of objects.`))

	dumpExamples = normalizers.Examples(i18n.T("root.verbs.dump.dumpExamples",
		fmt.Sprintf(`
		%[1]s dump -o tf-imports --resources=portal 
		`, meta.CLIName)))

	resources             string
	includeChildResources bool
	dumpFormat            = cmd.NewEnum([]string{"tf-imports"}, "tf-imports")
)

func NewDumpCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     dumpUse,
		Short:   dumpShort,
		Long:    dumpLong,
		Example: dumpExamples,
		Aliases: []string{"d", "D"},
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}

	cmd.Flags().StringVarP(&resources, "resources",
		"r",
		"",
		"Comma separated list of resource types to dump.")
	if err := cmd.MarkFlagRequired("resources"); err != nil {
		return nil, err
	}

	cmd.Flags().BoolVar(&includeChildResources, "include-child-resources",
		false,
		"Include child resources in the dump.")

	// This shadows the global output flag
	cmd.Flags().VarP(dumpFormat, common.OutputFlagName, common.OutputFlagShort,
		fmt.Sprintf(`Configures the format of data written to STDOUT.
- Allowed: [ %s ]`, strings.Join(dumpFormat.Allowed, "|")))

	return cmd, nil
}

package patch

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
)

const (
	Verb = verbs.Patch
)

var (
	patchUse = Verb.String()

	patchShort = i18n.T("root.verbs.patch.patchShort", "Apply patches to files")

	patchLong = normalizers.LongDesc(i18n.T("root.verbs.patch.patchLong",
		`Apply JSONPath-based patches to YAML or JSON files. Patches can set values,
remove keys, or append to arrays using JSONPath selectors to target specific
nodes in the document tree.`))

	patchExamples = normalizers.Examples(i18n.T("root.verbs.patch.patchExamples",
		fmt.Sprintf(`
        # Set a value on all services using inline flags
        %[1]s patch file input.yaml -s '$..services[*]' -v 'read_timeout:30000'

        # Apply a patch file
        %[1]s patch file input.yaml patches.yaml

        # Read from stdin, write to a file
        cat input.yaml | %[1]s patch file - -s '$' -v 'version:"2.0"' -o output.yaml
        `, meta.CLIName)))
)

func NewPatchCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     patchUse,
		Short:   patchShort,
		Long:    patchLong,
		Example: patchExamples,
		Aliases: []string{"p"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		},
	}

	cmd.AddCommand(newFileCmd())

	return cmd, nil
}

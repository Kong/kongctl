package install

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	extensioncmd "github.com/kong/kongctl/internal/cmd/root/verbs/extensions"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Install
)

var (
	installUse = Verb.String()

	installShort = i18n.T("root.verbs.install.short", "Install kongctl features")

	installLong = normalizers.LongDesc(i18n.T("root.verbs.install.long",
		`Locally install extensions, skills or other plugin type functionality.`))

	installExamples = normalizers.Examples(i18n.T("root.verbs.install.examples",
		fmt.Sprintf(`
	# Install a kongctl extension
	%[1]s install extension

  # Install kongctl skills into the current repository
  %[1]s install skills

  # Show what would be written without changing files
  %[1]s install skills --dry-run
`, meta.CLIName)))
)

// NewInstallCmd builds the install verb.
func NewInstallCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     installUse,
		Short:   installShort,
		Long:    installLong,
		Example: installExamples,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPreRun: func(c *cobra.Command, _ []string) {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
		},
	}

	cmd.AddCommand(extensioncmd.NewInstallExtensionCmd())
	cmd.AddCommand(newInstallSkillsCmd())

	return cmd, nil
}

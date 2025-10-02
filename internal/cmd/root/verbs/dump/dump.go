package dump

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
)

const (
	Verb              = verbs.Dump
	formatTFImport    = "tf-import"
	formatDeclarative = "declarative"
)

var (
	dumpUse = Verb.String()

	dumpShort = i18n.T("root.verbs.dump.dumpShort", "Dump objects")

	dumpLong = normalizers.LongDesc(i18n.T("root.verbs.dump.dumpLong",
		`Use dump to export an object or list of objects.`))

	dumpExamples = normalizers.Examples(i18n.T("root.verbs.dump.dumpExamples",
		fmt.Sprintf(`
        # Export all portals as Terraform import blocks to stdout
        %[1]s dump tf-import --resources=portal

        # Export all portals and their child resources (documents, specifications, pages, settings)
        %[1]s dump tf-import --resources=portal --include-child-resources

        # Export all portals as Terraform import blocks to a file
        %[1]s dump tf-import --resources=portal --output-file=portals.tf

        # Export all APIs with their child resources and include debug logging
        %[1]s dump tf-import --resources=api --include-child-resources --log-level=debug

        # Export declarative configuration with a default namespace
        %[1]s dump declarative --resources=portal,api --default-namespace=team-alpha
        `, meta.CLIName)))
)

func NewDumpCmd() (*cobra.Command, error) {
	dumpCommand := &cobra.Command{
		Use:     dumpUse,
		Short:   dumpShort,
		Long:    dumpLong,
		Example: dumpExamples,
		Aliases: []string{"d", "D"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	dumpCommand.PersistentPreRun = func(cmd *cobra.Command, _ []string) {
		cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		cmd.SetContext(context.WithValue(cmd.Context(),
			helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(konnectCommon.KonnectSDKFactory)))
	}

	dumpCommand.AddCommand(newTFImportCmd())
	dumpCommand.AddCommand(newDeclarativeCmd())

	return dumpCommand, nil
}

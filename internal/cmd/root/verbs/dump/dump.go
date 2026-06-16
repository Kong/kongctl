package dump

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/spf13/cobra"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
)

const (
	Verb                     = verbs.Dump
	formatTFImport           = "tf-import"
	formatDeclarative        = "declarative"
	outputFlagUnsupportedMsg = "flags -o/--" + cmdcommon.OutputFlagName +
		" are not supported for the dump command; use --output-file to save dump output to a file"
)

var (
	dumpUse = Verb.String()

	errOutputFlagUnsupported = errors.New(outputFlagUnsupportedMsg)

	// Use explicit delimiter anchors rather than strings.Contains to avoid falsely
	// matching --output-file error messages; the dump subcommands define that flag.
	outputFlagParseErrorPattern = regexp.MustCompile(`(^|[\s"',:])(?:--` +
		cmdcommon.OutputFlagName + `|-` + cmdcommon.OutputFlagShort + `)($|[\s"',:=])`)

	dumpShort = i18n.T("root.verbs.dump.dumpShort", "Dump existing resources into local declarative configuration")

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

        # Export adopted dashboards as declarative configuration
        %[1]s dump declarative --resources=analytics.dashboards --default-namespace=analytics

        # Export all organization teams
        %[1]s dump declarative --resources=organization.teams

        # Filter by name (exact match)
        %[1]s dump declarative --resources=portal --filter-name=my-dev-portal

        # Filter by name (substring match using wildcards)
        %[1]s dump declarative --resources=portal --filter-name='*dev*'

        # Filter by ID
        %[1]s dump declarative --resources=portal --filter-id=abc12345-def6-7890-abcd-ef1234567890
        `, meta.CLIName)))
)

func NewDumpCmd() (*cobra.Command, error) {
	dumpCommand := &cobra.Command{
		Use:     dumpUse,
		Short:   dumpShort,
		Long:    dumpLong,
		Example: dumpExamples,
		Aliases: []string{"d", "D"},
	}
	cmdpkg.ConfigureRequiresSubcommand(dumpCommand)

	dumpCommand.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		if isOutputFlagParseError(err) {
			return &cmdpkg.UsageError{Err: errOutputFlagUnsupported}
		}
		return err
	})

	cmdcommon.SkipOutputFormatValidation(dumpCommand)

	dumpCommand.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if outputFlag := cmd.Flag(cmdcommon.OutputFlagName); outputFlag != nil && outputFlag.Changed {
			return &cmdpkg.UsageError{Err: errOutputFlagUnsupported}
		}

		ctx := context.WithValue(cmd.Context(), verbs.Verb, Verb)
		cmd.SetContext(context.WithValue(ctx,
			helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(konnectCommon.KonnectSDKFactory)))
		return nil
	}

	dumpCommand.AddCommand(newTFImportCmd())
	dumpCommand.AddCommand(newDeclarativeCmd())

	return dumpCommand, nil
}

func isOutputFlagParseError(err error) bool {
	if err == nil {
		return false
	}
	return outputFlagParseErrorPattern.MatchString(err.Error())
}

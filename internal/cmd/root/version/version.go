package version

import (
	"fmt"
	"io"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	ShowFullFlag       = "full"
	ShowFullConfigPath = "version." + ShowFullFlag
)

var (
	versionUse   = "version"
	versionShort = i18n.T("root.version.versionShort",
		fmt.Sprintf("Print the %s version", meta.CLIName))
	versionLong = normalizers.LongDesc(i18n.T("root.version.versionLong",
		`The version command prints the version and other optional information`))
	versionExample = normalizers.Examples(i18n.T("root.version.versionExamples",
		fmt.Sprintf(`
		# Print the simple version
		%[1]s version
		# Print the full version info with commit and build date
		%[1]s version --full
		`, meta.CLIName)))
)

// Build a new instance of the version command
func NewVersionCmd() *cobra.Command {
	rv := &cobra.Command{
		Use:     versionUse,
		Short:   versionShort,
		Long:    versionLong,
		Example: versionExample,
		PreRun: func(c *cobra.Command, args []string) {
			bindFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(c, args)

			err := validate(helper)
			if err != nil {
				return err
			}
			err = run(helper)
			if err != nil {
				return err
			}
			return nil
		},
	}

	rv.Flags().Bool(ShowFullFlag, false,
		i18n.T(fmt.Sprintf("root.%s", ShowFullConfigPath),
			fmt.Sprintf("True to show the full version information.\n (config path = '%s')", ShowFullConfigPath)))

	return rv
}

func bindFlags(c *cobra.Command, args []string) {
	helper := cmd.BuildHelper(c, args)
	cfg, e := helper.GetConfig()
	util.CheckError(e)
	f := c.Flags().Lookup(ShowFullFlag)
	err := cfg.BindFlag(ShowFullConfigPath, f)
	util.CheckError(err)
}

// Validate ensures the configured command is valid
func validate(_ cmd.Helper) error {
	return nil
}

// Run performs the actual version command logic
func run(helper cmd.Helper) error {
	bi, err := helper.GetBuildInfo()
	if err != nil {
		return err
	}

	// Printer functions take objects to print
	result := map[string]any{
		"version": bi.Version,
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	full := cfg.GetBool(ShowFullConfigPath)
	if full {
		result["commit"] = bi.Commit
		result["date"] = bi.Date
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	if outType == common.TEXT {
		return printText(result, helper.GetStreams().Out, full)
	}

	p, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer p.Flush()
	p.Print(result)

	return nil
}

// printText is a custom print function for the version command. Not really necessary
// but it shows how you can override the default printers per command.
func printText(data map[string]any, out io.Writer, full bool) error {
	if ver, ok := data["version"]; ok {
		_, e := fmt.Fprintf(out, "%s", ver)
		if e != nil {
			return e
		}
	}

	if full {
		commit := data["commit"]
		date := data["date"]
		fmt.Fprintf(out, " (%s : %s)", commit.(string), date.(string))
	}

	_, err := fmt.Fprintf(out, "\n")
	return err
}

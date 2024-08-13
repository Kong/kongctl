package version

import (
	"fmt"
	"io"

	"github.com/kong/kong-cli/internal/cmd"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	ShowCommitFlagName   = "show-commit"
	ShowCommitConfigPath = "version." + ShowCommitFlagName
)

var (
	// VERSION may be overridden by the linker. See .goreleaser.yml
	VERSION = "dev"
	// COMMIT may be overridden by the linker. See .goreleaser.yml
	COMMIT = "unknown"

	versionUse   = "version"
	versionShort = i18n.T("root.version.versionShort",
		fmt.Sprintf("Print the %s version", meta.CLIName))
	versionLong = normalizers.LongDesc(i18n.T("root.version.versionLong",
		`The version command prints the version and other optional information`))
	versionExample = normalizers.Examples(i18n.T("root.version.versionExamples",
		fmt.Sprintf(`
		# Print the simple version
		%[1]s version
		# Print the version and the git commit hash
		%[1]s version --show-commit
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

	rv.Flags().Bool(ShowCommitFlagName, false,
		i18n.T(fmt.Sprintf("root.%s", ShowCommitConfigPath),
			fmt.Sprintf("True to show the git commit hash when built.\n (config path = '%s')", ShowCommitConfigPath)))

	return rv
}

func bindFlags(c *cobra.Command, args []string) {
	helper := cmd.BuildHelper(c, args)
	cfg, e := helper.GetConfig()
	util.CheckError(e)
	f := c.Flags().Lookup(ShowCommitFlagName)
	err := cfg.BindFlag(ShowCommitConfigPath, f)
	util.CheckError(err)
}

// Validate ensures the configured command is valid
func validate(_ cmd.Helper) error {
	return nil
}

// Run performs the actual version command logic
func run(helper cmd.Helper) error {
	// Printer functions take objects to print
	result := map[string]interface{}{
		"version": VERSION,
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if cfg.GetBool(ShowCommitConfigPath) {
		result["commit"] = COMMIT
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	if outType == "text" {
		return printText(result, helper.GetStreams().Out)
	}

	p, err := cli.Format(outType, helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer p.Flush()
	p.Print(result)

	return nil
}

// printText is a custom print function for the version command. Not really necessary
// but it shows how you can override the default printers per command.
func printText(data map[string]interface{}, out io.Writer) error {
	if ver, ok := data["version"]; ok {
		_, e := fmt.Fprintf(out, "%s", ver)
		if e != nil {
			return e
		}
	}
	if commit, ok := data["commit"]; ok {
		_, e := fmt.Fprintf(out, " (%s)", commit)
		if e != nil {
			return e
		}
	}
	_, err := fmt.Fprintf(out, "\n")
	return err
}

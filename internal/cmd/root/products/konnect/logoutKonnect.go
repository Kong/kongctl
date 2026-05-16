package konnect

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	logoutKonnectShort = i18n.T("root.products.konnect.logoutKonnectShort", "Logout from Konnect")
	logoutKonnectLong  = i18n.T("root.products.konnect.logoutKonnectLong",
		"Remove persisted Konnect authentication tokens for the active profile.")
	logoutKonnectExample = normalizers.Examples(
		i18n.T("root.products.konnect.logoutKonnectExample",
			fmt.Sprintf(`
# Logout from Konnect
%[1]s logout konnect`, meta.CLIName)))
)

type logoutKonnectCmd struct {
	*cobra.Command
}

func (c *logoutKonnectCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("the logout command does not accept arguments"),
		}
	}
	return nil
}

func (c *logoutKonnectCmd) run(helper cmd.Helper) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	streams := helper.GetStreams()
	profileName := cfg.GetProfile()

	removed, err := auth.DeleteAccessToken(cfg)
	if err != nil {
		return cmd.PrepareExecutionErrorWithHelper(helper,
			"failed to remove Konnect authentication tokens", err)
	}

	if removed {
		fmt.Fprintf(streams.Out, "Removed stored Konnect credentials for profile %q\n", profileName)
	} else {
		fmt.Fprintf(streams.Out, "No stored Konnect credentials found for profile %q\n", profileName)
	}

	return nil
}

func (c *logoutKonnectCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if err := c.validate(helper); err != nil {
		return err
	}

	return c.run(helper)
}

func newLogoutKonnectCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *logoutKonnectCmd {
	rv := logoutKonnectCmd{
		Command: baseCmd,
	}

	rv.Short = logoutKonnectShort
	rv.Long = logoutKonnectLong
	rv.Example = logoutKonnectExample

	addParentFlags(verb, rv.Command)

	rv.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		return parentPreRun(c, args)
	}

	rv.RunE = rv.runE

	return &rv
}

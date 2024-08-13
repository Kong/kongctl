package login

import (
	"context"
	"fmt"

	"github.com/kong/kong-cli/internal/cmd/root/products/konnect"
	"github.com/kong/kong-cli/internal/cmd/root/verbs"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Login
)

var (
	loginUse = Verb.String()

	loginShort = i18n.T("root.verbs.login.loginShort", "Login to system")

	loginLong = normalizers.LongDesc(i18n.T("root.verbs.login.loginLong",
		`Use login to authenticate to a remote system.

Further sub-commands are required to determine which remote system is contacted.`))

	loginExamples = normalizers.Examples(i18n.T("root.verbs.login.loginExamples",
		fmt.Sprintf(`
		# Login to Kong Konnect
		%[1]s login konnect
		`, meta.CLIName)))
)

func NewLoginCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     loginUse,
		Short:   loginShort,
		Long:    loginLong,
		Example: loginExamples,
		Aliases: []string{"g", "G"},
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

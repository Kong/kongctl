package listen

import (
	"context"
	"fmt"
	"strings"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	konnectAuditLogs "github.com/kong/kongctl/internal/cmd/root/products/konnect/auditlogs"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb         = verbs.Listen
	tailFlagName = "tail"
)

var (
	listenUse = Verb.String()

	listenShort = i18n.T("root.verbs.listen.short", "Listen for incoming events")
	listenLong  = normalizers.LongDesc(i18n.T("root.verbs.listen.long",
		`Use listen to create and run local receivers for remote event streams.`))
	listenExamples = normalizers.Examples(i18n.T("root.verbs.listen.examples",
		fmt.Sprintf(`
	# Konnect-first shorthand
	%[1]s listen --public-url https://example.ngrok.app
	# Resource form
	%[1]s listen audit-logs --public-url https://example.ngrok.app
	# Explicit product form
	%[1]s listen konnect audit-logs --public-url https://example.ngrok.app
	`, meta.CLIName)))

	tailUse = "tail"

	tailShort = i18n.T("root.verbs.tail.short", "Listen and stream events")
	tailLong  = normalizers.LongDesc(i18n.T("root.verbs.tail.long",
		`Use tail to create and run local receivers and stream incoming events to stdout.`))
	tailExamples = normalizers.Examples(i18n.T("root.verbs.tail.examples",
		fmt.Sprintf(`
	# Konnect-first shorthand
	%[1]s tail --public-url https://example.ngrok.app
	# Resource form
	%[1]s tail audit-logs --public-url https://example.ngrok.app
	# Explicit product form
	%[1]s tail konnect audit-logs --public-url https://example.ngrok.app
	`, meta.CLIName)))
)

type listenCommandSpec struct {
	use              string
	short            string
	long             string
	example          string
	aliases          []string
	forceTailDefault bool
}

// NewListenCmd builds the listen verb.
func NewListenCmd() (*cobra.Command, error) {
	return newListenCommand(listenCommandSpec{
		use:              listenUse,
		short:            listenShort,
		long:             listenLong,
		example:          listenExamples,
		aliases:          []string{"lsn"},
		forceTailDefault: false,
	})
}

// NewTailCmd builds the tail alias command.
func NewTailCmd() (*cobra.Command, error) {
	return newListenCommand(listenCommandSpec{
		use:              tailUse,
		short:            tailShort,
		long:             tailLong,
		example:          tailExamples,
		aliases:          nil,
		forceTailDefault: true,
	})
}

func newListenCommand(spec listenCommandSpec) (*cobra.Command, error) {
	options := konnectAuditLogs.DefaultListenAuditLogsOptions()

	cmd := &cobra.Command{
		Use:     spec.use,
		Short:   spec.short,
		Long:    spec.long,
		Example: spec.example,
		Aliases: spec.aliases,
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			if spec.forceTailDefault {
				if err := setTailFlagDefaultIfUnset(c); err != nil {
					return err
				}
			}
			return bindKonnectFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			return konnectAuditLogs.ExecuteListenAuditLogs(c, args, options)
		},
	}

	cmd.PersistentFlags().String(common.BaseURLFlagName, "",
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
			common.BaseURLConfigPath, common.BaseURLDefault))

	cmd.PersistentFlags().String(common.RegionFlagName, "",
		fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
			common.BaseURLFlagName, common.RegionConfigPath),
	)

	cmd.PersistentFlags().String(common.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			common.PATConfigPath))

	konnectAuditLogs.AddListenAuditLogsFlags(cmd, &options)

	konnectCmd, err := konnect.NewKonnectCmd(Verb)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(konnectCmd)

	auditLogsCmd, err := NewDirectAuditLogsCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(auditLogsCmd)

	if spec.forceTailDefault {
		rewriteListenExamplesToTail(cmd)
	}

	return cmd, nil
}

func setTailFlagDefaultIfUnset(c *cobra.Command) error {
	if c == nil {
		return nil
	}

	f := c.Flags().Lookup(tailFlagName)
	if f == nil || f.Changed {
		return nil
	}

	return c.Flags().Set(tailFlagName, "true")
}

func rewriteListenExamplesToTail(cmd *cobra.Command) {
	if cmd == nil {
		return
	}

	if cmd.Example != "" {
		cmd.Example = strings.ReplaceAll(cmd.Example, "kongctl listen", "kongctl tail")
	}

	for _, sub := range cmd.Commands() {
		rewriteListenExamplesToTail(sub)
	}
}

func bindKonnectFlags(c *cobra.Command, args []string) error {
	helper := cmdpkg.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if f := c.Flags().Lookup(common.BaseURLFlagName); f != nil {
		if err := cfg.BindFlag(common.BaseURLConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.RegionFlagName); f != nil {
		if err := cfg.BindFlag(common.RegionConfigPath, f); err != nil {
			return err
		}
	}

	if f := c.Flags().Lookup(common.PATFlagName); f != nil {
		if err := cfg.BindFlag(common.PATConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}

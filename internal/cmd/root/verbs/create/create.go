package create

import (
	"context"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Create
)

var (
	createUse = Verb.String()

	createShort = i18n.T("root.verbs.create.createShort", "Create objects")

	createLong = normalizers.LongDesc(i18n.T("root.verbs.create.createLong",
		`Use create to create a new object.

Further sub-commands are required to determine which remote system is contacted (if necessary).
The command will create an object and report a result depending on further arguments.
Output can be formatted in multiple ways to aid in further processing.`))

	createExamples = normalizers.Examples(i18n.T("root.verbs.create.createExamples",
		fmt.Sprintf(`
		# Create a new Konnect Kong Gateway control plane (Konnect-first)
		%[1]s create gateway control-plane <name>
		# Create a new Konnect Kong Gateway control plane (explicit)
		%[1]s create konnect gateway control-plane <name>
		`, meta.CLIName)))
)

func NewCreateCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     createUse,
		Short:   createShort,
		Long:    createLong,
		Example: createExamples,
		Aliases: []string{"c", "C"},
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			return bindKonnectFlags(c, args)
		},
	}

	// Add Konnect-specific flags as persistent flags so they appear in help
	cmd.PersistentFlags().String(common.BaseURLFlagName, common.BaseURLDefault,
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`,
			common.BaseURLConfigPath))

	cmd.PersistentFlags().String(common.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			common.PATConfigPath))

	// TODO: Determine if creating profiles for the command make sense and how to implement
	// cmd.AddCommand(profileCmd.NewProfileCmd())
	c, e := konnect.NewKonnectCmd(Verb)
	if e != nil {
		return nil, e
	}

	cmd.AddCommand(c)

	// Add gateway command directly for Konnect-first pattern
	gatewayCmd, err := NewDirectGatewayCmd()
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(gatewayCmd)

	return cmd, nil
}

// bindKonnectFlags binds Konnect-specific flags to configuration
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

	if f := c.Flags().Lookup(common.PATFlagName); f != nil {
		if err := cfg.BindFlag(common.PATConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}

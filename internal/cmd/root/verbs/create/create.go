package create

import (
	"context"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/organization"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/token"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
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
		`Use create to create Konnect access tokens.

Further sub-commands are required to determine which remote system is contacted (if necessary).
The command will create a token and report a result depending on further arguments.
Output can be formatted in multiple ways to aid in further processing.`))

	createExamples = normalizers.Examples(i18n.T("root.verbs.create.createExamples",
		fmt.Sprintf(`
		# Create a Konnect personal access token and print only the token value
		%[1]s create pat --name ci --expires-in 30d -o token
		# Create a Konnect personal access token and extract the token with jq
		%[1]s create pat --name ci --expires-in 7d --jq -r '.token'
		# Create a Konnect system account access token as an environment export
		%[1]s create spat --system-account-name ci-bot --name ci --expires-in 30d -o env
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
			ctx := c.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = context.WithValue(ctx, verbs.Verb, Verb)
			ctx = context.WithValue(ctx, products.Product, konnect.Product)
			ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactoryForVerb(Verb))
			c.SetContext(ctx)
			return bindKonnectFlags(c, args)
		},
	}

	// Add Konnect-specific flags as persistent flags so they appear in help
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

	jq.AddFlags(cmd.PersistentFlags())

	cmd.RunE = func(c *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(c, args)
		if _, err := helper.GetOutputFormat(); err != nil {
			return err
		}
		return cmdpkg.RequireSubcommand(c, args)
	}
	cmdpkg.MarkRequiresSubcommand(cmd)

	c, e := konnect.NewKonnectCmd(Verb)
	if e != nil {
		return nil, e
	}

	cmd.AddCommand(c)

	patCmd, err := token.NewPATCmd(Verb, nil, nil)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(patCmd)

	spatCmd, err := token.NewSPATCmd(Verb, nil, nil)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(spatCmd)

	orgCmd, err := organization.NewOrganizationCmd(Verb, nil, nil)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(orgCmd)

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

	if err := jq.BindFlags(cfg, c.Flags()); err != nil {
		return err
	}

	return nil
}

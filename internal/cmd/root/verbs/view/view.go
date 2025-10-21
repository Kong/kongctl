package view

import (
	"context"
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/navigator"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.View
)

var (
	viewUse = Verb.String()

	viewShort = i18n.T("root.verbs.view.viewShort", "Launch the Konnect resource viewer")

	viewLong = normalizers.LongDesc(i18n.T("root.verbs.view.viewLong",
		`Open an interactive view into Konnect resources.`))

	viewExamples = normalizers.Examples(i18n.T("root.verbs.view.viewExamples",
		fmt.Sprintf(`
		# Launch the Konnect interactive viewer
		%[1]s view
		`, meta.CLIName)))
)

// NewViewCmd creates the view command which launches the Konnect navigator.
func NewViewCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     viewUse,
		Short:   viewShort,
		Long:    viewLong,
		Example: viewExamples,
		Aliases: []string{"v", "V"},
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		},
		RunE: func(c *cobra.Command, args []string) error {
			helper := cmdpkg.BuildHelper(c, args)
			if _, err := helper.GetOutputFormat(); err != nil {
				return err
			}
			return navigator.Run(helper, navigator.Options{})
		},
	}

	addKonnectFlags(cmd)

	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		return bindKonnectFlags(c, args)
	}

	return cmd, nil
}

func addKonnectFlags(cmd *cobra.Command) {
	cmd.Flags().String(konnectcommon.BaseURLFlagName, konnectcommon.BaseURLDefault,
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`,
			konnectcommon.BaseURLConfigPath))

	cmd.Flags().String(konnectcommon.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI. 
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			konnectcommon.PATConfigPath))

	cmd.Flags().Int(konnectcommon.RequestPageSizeFlagName, konnectcommon.DefaultRequestPageSize,
		fmt.Sprintf(`Max number of results to include per response page.
- Config path: [ %s ]`,
			konnectcommon.RequestPageSizeConfigPath))
}

func bindKonnectFlags(cmd *cobra.Command, args []string) error {
	helper := cmdpkg.BuildHelper(cmd, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := cmd.Flags().Lookup(konnectcommon.BaseURLFlagName); flag != nil {
		if err := cfg.BindFlag(konnectcommon.BaseURLConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := cmd.Flags().Lookup(konnectcommon.PATFlagName); flag != nil {
		if err := cfg.BindFlag(konnectcommon.PATConfigPath, flag); err != nil {
			return err
		}
	}

	if flag := cmd.Flags().Lookup(konnectcommon.RequestPageSizeFlagName); flag != nil {
		if err := cfg.BindFlag(konnectcommon.RequestPageSizeConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

package adopt

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	AnalyticsCommandName = "analytics"
)

var (
	adoptAnalyticsShort = i18n.T("root.products.konnect.adopt.analyticsShort",
		"Adopt Konnect Analytics resources into namespace management")
	adoptAnalyticsLong = normalizers.LongDesc(i18n.T("root.products.konnect.adopt.analyticsLong",
		`The analytics command adopts Konnect Analytics resources into namespace management.`))
	adoptAnalyticsExample = normalizers.Examples(
		i18n.T("root.products.konnect.adopt.analyticsExamples",
			fmt.Sprintf(`
	# Adopt a dashboard by ID
	%[1]s adopt analytics dashboard 22cd8a0b-72e7-4212-9099-0764f8e9c5ac --namespace analytics
	# Adopt a dashboard using explicit konnect product
	%[1]s adopt konnect analytics dashboard 22cd8a0b-72e7-4212-9099-0764f8e9c5ac --namespace analytics
	`, meta.CLIName)))
)

func NewAnalyticsCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = AnalyticsCommandName
	cmd.Short = adoptAnalyticsShort
	cmd.Long = adoptAnalyticsLong
	cmd.Example = adoptAnalyticsExample
	cmd.Aliases = []string{"analytic"}
	cmd.RunE = func(c *cobra.Command, _ []string) error {
		return c.Help()
	}

	dashboardCmd, err := NewDashboardCmd(verb, &cobra.Command{}, addParentFlags, parentPreRun)
	if err != nil {
		return nil, err
	}
	cmd.AddCommand(dashboardCmd)

	return cmd, nil
}

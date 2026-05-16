package analytics

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "analytics"
)

var (
	analyticsUse   = CommandName
	analyticsShort = i18n.T("root.products.konnect.analytics.analyticsShort",
		"Manage Konnect Analytics resources")
	analyticsLong = normalizers.LongDesc(i18n.T("root.products.konnect.analytics.analyticsLong",
		`The analytics command allows you to work with Konnect Analytics resources.`))
	analyticsExample = normalizers.Examples(
		i18n.T("root.products.konnect.analytics.analyticsExamples",
			fmt.Sprintf(`
	# List all analytics dashboards
	%[1]s get analytics dashboards
	# Get a specific analytics dashboard by name
	%[1]s get analytics dashboard "API Summary"
	# List analytics dashboards using explicit konnect product
	%[1]s get konnect analytics dashboards
	`, meta.CLIName)))
)

func NewAnalyticsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     analyticsUse,
		Short:   analyticsShort,
		Long:    analyticsLong,
		Example: analyticsExample,
		Aliases: []string{"analytic"},
	}

	if verb == verbs.Get || verb == verbs.List {
		baseCmd.AddCommand(newGetAnalyticsDashboardsCmd(verb, addParentFlags, parentPreRun).Command)
	}

	return &baseCmd, nil
}

package telemetry

import (
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagAllowlist is the closed set of cobra flag names that may be reported in
// Event.FlagsSet.Values are NEVER recorded; only names.
var flagAllowlist = map[string]struct{}{
	"plan":    {},
	"dry-run": {},
}

// VisitedFlagNames returns the sorted, deduplicated names of allowlisted
// flags that were explicitly set on cmd. Defaults are not reported because
// pflag.FlagSet.Visit only iterates flags actually changed by the user.
func VisitedFlagNames(cmd *cobra.Command) []string {
	if cmd == nil {
		return nil
	}
	var out []string
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if _, ok := flagAllowlist[f.Name]; ok {
			out = append(out, f.Name)
		}
	})
	sort.Strings(out)
	return out
}

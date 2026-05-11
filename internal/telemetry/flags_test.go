package telemetry

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

func TestVisitedFlagNames_NilCommand(t *testing.T) {
	if got := VisitedFlagNames(nil); got != nil {
		t.Errorf("VisitedFlagNames(nil) = %v, want nil", got)
	}
}

func TestVisitedFlagNames(t *testing.T) {
	cases := []struct {
		name        string
		defineFlags func(*cobra.Command)
		args        []string
		want        []string
	}{
		{
			name: "no flags set returns nil",
			defineFlags: func(c *cobra.Command) {
				c.Flags().Bool("plan", false, "")
				c.Flags().Bool("dry-run", false, "")
			},
			args: []string{},
			want: nil,
		},
		{
			name: "declared but unset flag is not reported",
			defineFlags: func(c *cobra.Command) {
				c.Flags().Bool("plan", false, "")
				c.Flags().Bool("dry-run", false, "")
			},
			// Only --plan is explicitly set; pflag.Visit must skip --dry-run.
			args: []string{"--plan"},
			want: []string{"plan"},
		},
		{
			name: "non-allowlisted flag is filtered out",
			defineFlags: func(c *cobra.Command) {
				c.Flags().String("output", "text", "")
				c.Flags().Bool("force", false, "")
			},
			args: []string{"--output=json", "--force"},
			want: nil,
		},
		{
			name: "mixed allowlisted and non-allowlisted returns only allowlisted",
			defineFlags: func(c *cobra.Command) {
				c.Flags().Bool("plan", false, "")
				c.Flags().Bool("dry-run", false, "")
				c.Flags().String("output", "text", "")
				c.Flags().Bool("force", false, "")
			},
			args: []string{"--plan", "--output=json", "--force"},
			want: []string{"plan"},
		},
		{
			name: "result is sorted regardless of input order",
			defineFlags: func(c *cobra.Command) {
				c.Flags().Bool("plan", false, "")
				c.Flags().Bool("dry-run", false, "")
			},
			args: []string{"--plan", "--dry-run"},
			want: []string{"dry-run", "plan"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			tc.defineFlags(cmd)
			if err := cmd.ParseFlags(tc.args); err != nil {
				t.Fatalf("ParseFlags(%v): %v", tc.args, err)
			}
			got := VisitedFlagNames(cmd)
			if !slices.Equal(got, tc.want) {
				t.Errorf("VisitedFlagNames = %v, want %v", got, tc.want)
			}
		})
	}
}

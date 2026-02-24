package auditlogs

import (
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
)

func newCreateAuditLogsCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	if parentPreRun != nil {
		baseCmd.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, baseCmd)
	}

	baseCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	}

	baseCmd.AddCommand(newCreateListenerCmd(verb, addParentFlags, parentPreRun))
	baseCmd.AddCommand(newCreateDestinationCmd(verb, addParentFlags, parentPreRun))

	return baseCmd
}

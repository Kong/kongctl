package auditlogs

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "audit-logs"
)

var (
	auditLogsUse   = CommandName
	auditLogsShort = i18n.T("root.products.konnect.auditlogs.short",
		"Manage Konnect audit-log integrations")
	auditLogsLong = normalizers.LongDesc(i18n.T("root.products.konnect.auditlogs.long",
		`The audit-logs command provides developer-focused helpers for
receiving and configuring Konnect audit-log webhooks.`))
	auditLogsExample = normalizers.Examples(i18n.T("root.products.konnect.auditlogs.examples",
		fmt.Sprintf(`
# Start a local listener and create destination in one command
%[1]s listen audit-logs --public-url https://example.ngrok.app

# Explicit product form
%[1]s listen konnect audit-logs --public-url https://example.ngrok.app
`, meta.CLIName)))
)

// NewAuditLogsCmd builds the audit-logs command tree.
func NewAuditLogsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := &cobra.Command{
		Use:     auditLogsUse,
		Short:   auditLogsShort,
		Long:    auditLogsLong,
		Example: auditLogsExample,
		Aliases: []string{"audit-log", "al", "AL"},
	}

	switch verb {
	case verbs.Listen:
		return newListenAuditLogsCmd(verb, baseCmd, addParentFlags, parentPreRun), nil
	case verbs.Create:
		return newCreateAuditLogsCmd(verb, baseCmd, addParentFlags, parentPreRun), nil
	case verbs.Get, verbs.List, verbs.Delete, verbs.Add, verbs.Apply, verbs.Dump, verbs.Update, verbs.Help,
		verbs.Login, verbs.Plan, verbs.Sync, verbs.Diff, verbs.Export, verbs.Adopt, verbs.API, verbs.Kai,
		verbs.View, verbs.Logout:
		return baseCmd, nil
	default:
		return baseCmd, nil
	}
}

package me

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "me"
)

var (
	meUse   = CommandName
	meShort = i18n.T("root.products.konnect.me.meShort",
		"Get current user information")
	meLong = normalizers.LongDesc(i18n.T("root.products.konnect.me.meLong",
		`The me command retrieves information about the currently authenticated user.`))
	meExample = normalizers.Examples(
		i18n.T("root.products.konnect.me.meExamples",
			fmt.Sprintf(`
	# Get current user information
	%[1]s get me
	`, meta.CLIName)))
)

func NewMeCmd(verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	baseCmd := cobra.Command{
		Use:     meUse,
		Short:   meShort,
		Long:    meLong,
		Example: meExample,
	}

	// Handle supported verbs
	if verb == verbs.Get {
		return newGetMeCmd(verb, &baseCmd, addParentFlags, parentPreRun).Command, nil
	}

	// Return base command for unsupported verbs
	return &baseCmd, nil
}

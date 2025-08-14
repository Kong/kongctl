package profile

import (
	"fmt"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/profile"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	profileUse   = "profile"
	profileShort = i18n.T("root.profile.profileShort", "Manage CLI profiles")
	profileLong  = normalizers.LongDesc(i18n.T("root.profile.profileLong",
		`The profile command allows you to get, create, and delete profiles for the CLI.`))

	profileManager profile.Manager
)

func NewProfileCmd() *cobra.Command {
	rv := &cobra.Command{
		Use:     profileUse,
		Short:   profileShort,
		Long:    profileLong,
		Aliases: []string{"profiles"},
		RunE: func(c *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(c, args)

			profileManager = c.Context().Value(profile.ProfileManagerKey).(profile.Manager)

			err := validate(helper)
			if err != nil {
				return err
			}
			err = run(helper)
			if err != nil {
				return err
			}
			return nil
		},
	}
	return rv
}

func validate(_ cmd.Helper) error {
	// TODO: Validate the command if arguments are given
	return nil
}

func run(helper cmd.Helper) error {
	v, err := helper.GetVerb()
	if err != nil {
		return err
	}

	if v == verbs.Get {
		return runGet(helper)
	}

	return fmt.Errorf("command %s does not support %s", profileUse, v)
}

func runGet(helper cmd.Helper) error {
	// Algorithm for kongctl get profile
	//
	// * If an argument is provided, the user is looking for information on specific profile
	// * If no argument is provided, the user is looking for information on all profiles
	// * Use the profileManager to get all or one of the profiles and display it

	// TODO: Parse arguments to determine if user is looking for all profiles or a specific profile

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return &cmd.ExecutionError{
			Err: err,
		}
	}
	p, err := cli.Format(outType.String(),
		helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer p.Flush()

	p.Print(profileManager.GetProfiles())

	return nil
}

//func runCreate(_ *cmd.RunBucket) error {
//	// Algorithm for kongctl create profile
//	//
//	// * Use the profileManager to create a new profile
//	// * Display the new profile
//	//result := map[string]any{}
//	//printer, err := rb.GetPrinter()
//	//if err != nil {
//	//	return err
//	//}
//	//return printer(result, rb.Streams.Out)
//	return nil
//}
//
//func runDelete(_ *cmd.RunBucket) error {
//	return nil
//}

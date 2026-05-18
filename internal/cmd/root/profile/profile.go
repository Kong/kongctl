package profile

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/profile"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	profileUse   = "profile [profile-name]"
	profileShort = i18n.T("root.profile.profileShort", "Manage kongctl profiles")
	profileLong  = normalizers.LongDesc(i18n.T("root.profile.profileLong",
		`The profile command allows you to list kongctl profiles and inspect profile configuration.`))

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

			if err := validate(helper); err != nil {
				return err
			}
			return run(helper)
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

	if v == verbs.Get || v == verbs.List {
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

	args := helper.GetArgs()
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Getting profiles requires 0 or 1 arguments (profile name)"),
		}
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return &cmd.ExecutionError{
			Err: err,
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	jqSettings, err := jq.ResolveSettings(helper.GetCmd(), cfg)
	if err != nil {
		return err
	}
	if err := jq.ValidateOutputFormat(outType, jqSettings); err != nil {
		return err
	}

	payload, err := profilePayload(args)
	if err != nil {
		return err
	}
	if jq.HasFilter(jqSettings) {
		filteredPayload, handled, err := jq.ApplyToRaw(payload, outType, jqSettings, helper.GetStreams().Out)
		if err != nil {
			return cmd.PrepareExecutionErrorWithHelper(helper, "jq filter failed", err)
		}
		if handled {
			return nil
		}
		payload = filteredPayload
	}

	p, err := cli.Format(outType.String(),
		helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer p.Flush()

	p.Print(payload)

	return nil
}

func profilePayload(args []string) (any, error) {
	if len(args) == 0 {
		profiles := slices.Clone(profileManager.GetProfiles())
		slices.Sort(profiles)
		return profiles, nil
	}

	name := strings.TrimSpace(args[0])
	if name == "" {
		return nil, &cmd.ConfigurationError{
			Err: fmt.Errorf("profile name cannot be empty"),
		}
	}

	profiles := profileManager.GetProfiles()
	if !slices.Contains(profiles, name) {
		return nil, fmt.Errorf("profile %q not found", name)
	}

	return profileManager.GetProfile(name)
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

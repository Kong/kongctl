package me

import (
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	getMeShort = i18n.T("root.products.konnect.me.getMeShort",
		"Get current user information")
	getMeLong = i18n.T("root.products.konnect.me.getMeLong",
		`Use the get verb with the me command to retrieve information about the currently authenticated user.`)
	getMeExample = normalizers.Examples(
		i18n.T("root.products.konnect.me.getMeExamples",
			fmt.Sprintf(`
	# Get current user information
	%[1]s get me
	`, meta.CLIName)))
)

// Represents a text display record for current user
type textDisplayRecord struct {
	ID               string
	Email            string
	FullName         string
	PreferredName    string
	Active           string
	InferredRegion   string
	LocalCreatedTime string
	LocalUpdatedTime string
}

func userToDisplayRecord(u *kkComps.User) textDisplayRecord {
	missing := "n/a"

	var id, email, fullName, preferredName, active, inferredRegion string

	if u.ID != nil && *u.ID != "" {
		id = *u.ID
	} else {
		id = missing
	}

	if u.Email != nil && *u.Email != "" {
		email = *u.Email
	} else {
		email = missing
	}

	if u.FullName != nil && *u.FullName != "" {
		fullName = *u.FullName
	} else {
		fullName = missing
	}

	if u.PreferredName != nil && *u.PreferredName != "" {
		preferredName = *u.PreferredName
	} else {
		preferredName = missing
	}

	if u.Active != nil {
		if *u.Active {
			active = "true"
		} else {
			active = "false"
		}
	} else {
		active = missing
	}

	if u.InferredRegion != nil && *u.InferredRegion != "" {
		inferredRegion = *u.InferredRegion
	} else {
		inferredRegion = missing
	}

	var createdAt, updatedAt string
	if u.CreatedAt != nil {
		createdAt = u.CreatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	} else {
		createdAt = missing
	}

	if u.UpdatedAt != nil {
		updatedAt = u.UpdatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	} else {
		updatedAt = missing
	}

	return textDisplayRecord{
		ID:               id,
		Email:            email,
		FullName:         fullName,
		PreferredName:    preferredName,
		Active:           active,
		InferredRegion:   inferredRegion,
		LocalCreatedTime: createdAt,
		LocalUpdatedTime: updatedAt,
	}
}

type getMeCmd struct {
	*cobra.Command
}

func runGetMe(kkClient helpers.MeAPI, helper cmd.Helper) (*kkComps.User, error) {
	res, err := kkClient.GetUsersMe(helper.GetContext())
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get current user", err, helper.GetCmd(), attrs...)
	}

	return res.GetUser(), nil
}

func (c *getMeCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("the me command does not accept arguments"),
		}
	}
	return nil
}

func (c *getMeCmd) runE(cobraCmd *cobra.Command, args []string) error {
	var e error
	helper := cmd.BuildHelper(cobraCmd, args)
	if e = c.validate(helper); e != nil {
		return e
	}

	logger, e := helper.GetLogger()
	if e != nil {
		return e
	}

	outType, e := helper.GetOutputFormat()
	if e != nil {
		return e
	}

	printer, e := cli.Format(outType.String(), helper.GetStreams().Out)
	if e != nil {
		return e
	}

	defer printer.Flush()

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	sdk, e := helper.GetKonnectSDK(cfg, logger)
	if e != nil {
		return e
	}

	user, e := runGetMe(sdk.GetMeAPI(), helper)
	if e != nil {
		return e
	}

	if outType == cmdCommon.TEXT {
		printer.Print(userToDisplayRecord(user))
	} else {
		printer.Print(user)
	}

	return nil
}

func newGetMeCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getMeCmd {
	rv := getMeCmd{
		Command: baseCmd,
	}

	rv.Short = getMeShort
	rv.Long = getMeLong
	rv.Example = getMeExample
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return &rv
}
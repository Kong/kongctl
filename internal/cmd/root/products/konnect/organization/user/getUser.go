package user

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type getUserCmd struct {
	*cobra.Command
}

type organizationUserRecord struct {
	ID             string `json:"id"              yaml:"id"`
	Email          string `json:"email"           yaml:"email"`
	FullName       string `json:"full_name"       yaml:"full_name"`
	PreferredName  string `json:"preferred_name"  yaml:"preferred_name"`
	Active         string `json:"active"          yaml:"active"`
	InferredRegion string `json:"inferred_region" yaml:"inferred_region"`
	CreatedAt      string `json:"created_at"      yaml:"created_at"`
	UpdatedAt      string `json:"updated_at"      yaml:"updated_at"`
}

func newGetUserCmd(
	verb verbs.VerbValue,
	base *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getUserCmd {
	cmd := &getUserCmd{
		Command: &cobra.Command{
			Use:     base.Use,
			Short:   "List or get Konnect organization users",
			Long:    "Use the get verb with the user command to query Konnect organization users.",
			Aliases: base.Aliases,
			PreRunE: parentPreRun,
		},
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd.Command)
	}

	cmd.RunE = cmd.runE
	return cmd
}

func (u *getUserCmd) runE(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		users, err := runListUsers(sdk.GetOrganizationUsersAPI(), helper, cfg)
		if err != nil {
			return err
		}
		return renderUsersList(helper, helper.GetCmd().Name(), outType, printer, users)
	}

	if len(args) == 1 {
		identifier := helper.GetArgs()[0]
		var orgUser *kkComps.User
		if util.IsValidUUID(identifier) {
			orgUser, err = runGetUser(identifier, sdk.GetOrganizationUsersAPI(), helper)
		} else {
			orgUser, err = resolveOrganizationUserByEmail(identifier, sdk.GetOrganizationUsersAPI(), helper, cfg)
		}
		if err != nil {
			return err
		}

		return tableview.RenderForFormat(
			helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			userToDisplayRecord(orgUser),
			userToDisplayRecord(orgUser),
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
			tableview.WithDetailContext(common.ViewParentOrganizationUser, func(int) any {
				return orgUser
			}),
			tableview.WithDetailHelper(helper),
		)
	}

	return fmt.Errorf("too many arguments")
}

func runGetUser(userID string, userAPI helpers.OrganizationUsersAPI, helper cmd.Helper) (*kkComps.User, error) {
	if userAPI == nil {
		return nil, fmt.Errorf("organization users client is not available")
	}
	res, err := userAPI.GetUser(helper.GetContext(), userID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get organization user", err, helper.GetCmd(), attrs...)
	}
	if res == nil {
		return nil, fmt.Errorf("organization user response was empty")
	}
	return res.User, nil
}

func runListUsers(
	userAPI helpers.OrganizationUsersAPI,
	helper cmd.Helper,
	cfg config.Hook,
) ([]kkComps.User, error) {
	if userAPI == nil {
		return nil, fmt.Errorf("organization users client is not available")
	}

	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.User
	var totalFetched int

	for {
		req := kkOps.ListUsersRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
		}

		res, err := userAPI.ListUsers(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list organization users", err, helper.GetCmd(), attrs...)
		}
		if res == nil || res.UserCollection == nil {
			return allData, nil
		}

		data := res.UserCollection.Data
		totalFetched += len(data)
		allData = append(allData, data...)

		totalItems := res.UserCollection.Meta.Page.Total
		if totalFetched >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func resolveOrganizationUserByEmail(
	email string,
	userAPI helpers.OrganizationUsersAPI,
	helper cmd.Helper,
	cfg config.Hook,
) (*kkComps.User, error) {
	users, err := runListUsers(userAPI, helper, cfg)
	if err != nil {
		return nil, err
	}

	var matches []kkComps.User
	for _, orgUser := range users {
		if orgUser.Email != nil && strings.EqualFold(*orgUser.Email, email) {
			matches = append(matches, orgUser)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("organization user with email %q not found", email)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("organization user email %q matched %d users; use user ID", email, len(matches))
	}
	return &matches[0], nil
}

func renderUsersList(
	helper cmd.Helper,
	rootLabel string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	users []kkComps.User,
) error {
	records := make([]organizationUserRecord, 0, len(users))
	for i := range users {
		records = append(records, userToDisplayRecord(&users[i]))
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		records,
		"",
		tableview.WithRootLabel(rootLabel),
		tableview.WithDetailHelper(helper),
	)
}

func userToDisplayRecord(orgUser *kkComps.User) organizationUserRecord {
	const missing = "n/a"

	record := organizationUserRecord{
		ID:             missing,
		Email:          missing,
		FullName:       missing,
		PreferredName:  missing,
		Active:         missing,
		InferredRegion: missing,
		CreatedAt:      missing,
		UpdatedAt:      missing,
	}
	if orgUser == nil {
		return record
	}

	if value := orgUser.GetID(); value != nil && *value != "" {
		record.ID = *value
	}
	if value := orgUser.GetEmail(); value != nil && *value != "" {
		record.Email = *value
	}
	if value := orgUser.GetFullName(); value != nil && *value != "" {
		record.FullName = *value
	}
	if value := orgUser.GetPreferredName(); value != nil && *value != "" {
		record.PreferredName = *value
	}
	if value := orgUser.GetActive(); value != nil {
		record.Active = fmt.Sprintf("%t", *value)
	}
	if value := orgUser.GetInferredRegion(); value != nil && *value != "" {
		record.InferredRegion = *value
	}
	if value := orgUser.GetCreatedAt(); value != nil {
		record.CreatedAt = value.In(time.Local).Format("2006-01-02 15:04:05")
	}
	if value := orgUser.GetUpdatedAt(); value != nil {
		record.UpdatedAt = value.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

func buildUserChildView(users []kkComps.User) tableview.ChildView {
	rows := make([]table.Row, 0, len(users))
	for i := range users {
		record := userToDisplayRecord(&users[i])
		rows = append(rows, table.Row{util.AbbreviateUUID(record.ID), record.Email, record.FullName})
	}

	return tableview.ChildView{
		Headers: []string{"ID", "EMAIL", "FULL NAME"},
		Rows:    rows,
		DetailRenderer: func(index int) string {
			if index < 0 || index >= len(users) {
				return ""
			}
			return userDetailView(&users[index])
		},
		Title:      "Users",
		ParentType: common.ViewParentOrganizationUser,
		DetailContext: func(index int) any {
			if index < 0 || index >= len(users) {
				return nil
			}
			return &users[index]
		},
	}
}

func userDetailView(orgUser *kkComps.User) string {
	record := userToDisplayRecord(orgUser)
	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", record.ID)
	fmt.Fprintf(&b, "email: %s\n", record.Email)
	fmt.Fprintf(&b, "full_name: %s\n", record.FullName)
	fmt.Fprintf(&b, "preferred_name: %s\n", record.PreferredName)
	fmt.Fprintf(&b, "active: %s\n", record.Active)
	fmt.Fprintf(&b, "inferred_region: %s\n", record.InferredRegion)
	fmt.Fprintf(&b, "created_at: %s\n", record.CreatedAt)
	fmt.Fprintf(&b, "updated_at: %s\n", record.UpdatedAt)
	return strings.TrimRight(b.String(), "\n")
}

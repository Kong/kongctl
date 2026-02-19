package team

import (
	"fmt"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/charmbracelet/bubbles/table"
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

type getTeamCmd struct {
	*cobra.Command
}

func newGetTeamCmd(
	verb verbs.VerbValue,
	base *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getTeamCmd {
	cmd := &getTeamCmd{
		Command: &cobra.Command{
			Use:     base.Use,
			Short:   "List or get Konnect teams",
			Long:    `Use the get verb with the team command to query Konnect teams.`,
			Aliases: base.Aliases,
			PreRunE: parentPreRun,
		},
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd.Command)
	}

	// Add skip-system-teams flag for list/get operations
	cmd.Flags().Bool(skipSystemTeamsFlagName, false, "Skip system teams in the output")

	cmd.RunE = cmd.runE

	return cmd
}

type textDisplayRecord struct {
	ID               string
	Name             string
	Description      string
	LocalCreatedTime string
	LocalUpdatedTime string
	IsSystemTeam     string
}

func teamToDisplayRecord(s *kkComps.Team) textDisplayRecord {
	const missing = "n/a"

	record := textDisplayRecord{
		ID:               missing,
		Name:             missing,
		Description:      missing,
		IsSystemTeam:     missing,
		LocalCreatedTime: missing,
		LocalUpdatedTime: missing,
	}

	if s == nil {
		return record
	}

	if id := s.GetID(); id != nil && *id != "" {
		record.ID = util.AbbreviateUUID(*id)
	}

	if name := s.GetName(); name != nil && *name != "" {
		record.Name = *name
	}

	if description := s.GetDescription(); description != nil && *description != "" {
		record.Description = *description
	}

	if isSystemTeam := s.GetSystemTeam(); isSystemTeam != nil {
		record.IsSystemTeam = fmt.Sprintf("%t", *isSystemTeam)
	}

	if createdAt := s.GetCreatedAt(); createdAt != nil {
		record.LocalCreatedTime = createdAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	if updatedAt := s.GetUpdatedAt(); updatedAt != nil {
		record.LocalUpdatedTime = updatedAt.In(time.Local).Format("2006-01-02 15:04:05")
	}

	return record
}

func renderTeamsList(
	helper cmd.Helper,
	rootLabel string,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	teams []kkComps.Team,
) error {
	displayRecords := make([]textDisplayRecord, 0, len(teams))
	for i := range teams {
		displayRecords = append(displayRecords, teamToDisplayRecord(&teams[i]))
	}

	options := []tableview.Option{
		tableview.WithRootLabel(rootLabel),
		tableview.WithDetailHelper(helper),
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		displayRecords,
		teams,
		"",
		options...,
	)
}

func (t *getTeamCmd) runE(c *cobra.Command, args []string) error {
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

	// Read skip-system-teams flag
	skipSystemTeams, _ := c.Flags().GetBool(skipSystemTeamsFlagName)

	// No args: list all
	if len(args) == 0 {
		teams, err := runList(sdk.GetOrganizationTeamAPI(), helper, cfg, skipSystemTeams)
		if err != nil {
			return err
		}

		return renderTeamsList(helper, helper.GetCmd().Name(), outType, printer, teams)
	}

	// One arg: get specific team(s) by ID or name
	if len(args) == 1 {
		id := helper.GetArgs()[0]
		isUUID := util.IsValidUUID(id)

		var team *kkComps.Team

		if !isUUID {
			// multiple teams can have the same name, so we list by name
			teams, err := runListByName(id, sdk.GetOrganizationTeamAPI(), helper, cfg, skipSystemTeams)
			if err != nil {
				return err
			}
			return renderTeamsList(helper, helper.GetCmd().Name(), outType, printer, teams)
		}

		team, err = runGet(id, sdk.GetOrganizationTeamAPI(), helper)
		if err != nil {
			return err
		}

		return tableview.RenderForFormat(helper,
			false,
			outType,
			printer,
			helper.GetStreams(),
			teamToDisplayRecord(team),
			team,
			"",
			tableview.WithRootLabel(helper.GetCmd().Name()),
			tableview.WithDetailContext("team", func(int) any {
				return team
			}),
			tableview.WithDetailHelper(helper),
		)
	}

	return fmt.Errorf("too many arguments")
}

func runGet(id string, kkClient helpers.OrganizationTeamAPI, helper cmd.Helper) (*kkComps.Team, error) {
	res, err := kkClient.GetOrganizationTeam(helper.GetContext(), id)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		return nil, cmd.PrepareExecutionError("Failed to get Team", err, helper.GetCmd(), attrs...)
	}

	return res.GetTeam(), nil
}

func runList(kkClient helpers.OrganizationTeamAPI, helper cmd.Helper,
	cfg config.Hook, skipSystemTeams bool,
) ([]kkComps.Team, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.Team
	var totalFetched int

	for {
		req := kkOps.ListTeamsRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := kkClient.ListOrganizationTeams(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list Teams", err, helper.GetCmd(), attrs...)
		}

		data := res.GetTeamCollection().Data
		totalFetched += len(data)

		// Filter out system teams if flag is set
		for _, team := range data {
			if skipSystemTeams && team.SystemTeam != nil && *team.SystemTeam {
				continue
			}
			allData = append(allData, team)
		}

		totalItems := res.GetTeamCollection().Meta.Page.Total
		if totalFetched >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return allData, nil
}

func runListByName(name string, kkClient helpers.OrganizationTeamAPI, helper cmd.Helper,
	cfg config.Hook, skipSystemTeams bool,
) ([]kkComps.Team, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	var allData []kkComps.Team
	var totalFetched int

	for {
		req := kkOps.ListTeamsRequest{
			PageSize:   kk.Int64(requestPageSize),
			PageNumber: kk.Int64(pageNumber),
			Filter: &kkOps.ListTeamsQueryParamFilter{
				Name: &kkComps.LegacyStringFieldFilter{
					Eq: kk.String(name),
				},
			},
		}

		res, err := kkClient.ListOrganizationTeams(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list OrganizationTeams", err, helper.GetCmd(), attrs...)
		}

		data := res.GetTeamCollection().Data
		totalFetched += len(data)

		// Filter out system teams if flag is set
		for _, team := range data {
			if skipSystemTeams && team.SystemTeam != nil && *team.SystemTeam {
				continue
			}
			allData = append(allData, team)
		}

		totalItems := res.GetTeamCollection().Meta.Page.Total

		if totalFetched >= int(totalItems) {
			break
		}

		pageNumber++
	}

	if len(allData) > 0 {
		return allData, nil
	}

	return nil, fmt.Errorf("organization_team with name %s not found", name)
}

func buildTeamChildView(teams []kkComps.Team) tableview.ChildView {
	rows := make([]table.Row, 0, len(teams))
	for i := range teams {
		record := teamToDisplayRecord(&teams[i])
		rows = append(rows, table.Row{record.ID, record.Name})
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(teams) {
			return ""
		}
		return teamDetailView(&teams[index])
	}

	return tableview.ChildView{
		Headers:        []string{"ID", "NAME"},
		Rows:           rows,
		DetailRenderer: detailFn,
		Title:          "Teams",
		ParentType:     "team",
		DetailContext: func(index int) any {
			if index < 0 || index >= len(teams) {
				return nil
			}
			return &teams[index]
		},
	}
}

func teamDetailView(team *kkComps.Team) string {
	if team == nil {
		return ""
	}

	const missing = "n/a"

	id := missing
	if value := team.GetID(); value != nil && *value != "" {
		id = util.AbbreviateUUID(*value)
	}

	name := missing
	if value := team.GetName(); value != nil && *value != "" {
		name = *value
	}

	description := missing
	if value := team.GetDescription(); value != nil && *value != "" {
		description = *value
	}

	isSystem := missing
	if value := team.GetSystemTeam(); value != nil {
		isSystem = fmt.Sprintf("%t", *value)
	}

	createdAt := missing
	if value := team.GetCreatedAt(); value != nil {
		createdAt = value.In(time.Local).Format("2006-01-02 15:04:05")
	}

	updatedAt := missing
	if value := team.GetUpdatedAt(); value != nil {
		updatedAt = value.In(time.Local).Format("2006-01-02 15:04:05")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\n", id)
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", description)
	fmt.Fprintf(&b, "system_team: %s\n", isSystem)
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	fmt.Fprintf(&b, "updated_at: %s\n", updatedAt)

	return strings.TrimRight(b.String(), "\n")
}

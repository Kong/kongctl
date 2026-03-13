package members

import (
	"context"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	CommandName = "member"

	// TeamIDFlagName is the flag for providing a team identifier by UUID.
	TeamIDFlagName = "team-id"
	// TeamNameFlagName is the flag for providing a team identifier by name.
	TeamNameFlagName = "team-name"
)

var (
	membersUse   = CommandName
	membersShort = i18n.T("root.products.konnect.organization.team.membersShort",
		"List members of a Konnect organization team")
	membersLong = normalizers.LongDesc(i18n.T("root.products.konnect.organization.team.membersLong",
		`Use the member command to list users and system accounts that
belong to a specific Konnect organization team.`))
	membersExample = normalizers.Examples(i18n.T("root.products.konnect.organization.team.membersExamples",
		fmt.Sprintf(`
# List all members of a team by team ID
%[1]s get org team member --team-id <team-id>
# List all members of a team by team name
%[1]s get org team member --team-name my-team
# List only users in a team
%[1]s get org team member users --team-name my-team
# List only system accounts in a team
%[1]s get org team member system-accounts --team-name my-team
`, meta.CLIName)))
)

// AddTeamFlags adds --team-id and --team-name flags to cmd.
func AddTeamFlags(cmd *cobra.Command) {
	cmd.Flags().String(TeamIDFlagName, "", "Team ID (UUID) to list members for")
	cmd.Flags().String(TeamNameFlagName, "", "Team name to list members for")
}

// GetTeamIdentifiers reads the --team-id and --team-name flag values from cmd.
func GetTeamIdentifiers(cmd *cobra.Command) (teamID, teamName string) {
	if f := cmd.Flags().Lookup(TeamIDFlagName); f != nil {
		teamID = strings.TrimSpace(f.Value.String())
	}
	if f := cmd.Flags().Lookup(TeamNameFlagName); f != nil {
		teamName = strings.TrimSpace(f.Value.String())
	}
	return teamID, teamName
}

// ResolveOrgTeamID resolves the team UUID from either the teamID or teamName
// arguments. Exactly one must be non-empty. Returns the resolved UUID and the
// canonical team name (which equals teamID if only the ID was provided).
func ResolveOrgTeamID(
	ctx context.Context,
	teamIDFlag string,
	teamNameFlag string,
	orgTeamAPI helpers.OrganizationTeamAPI,
	cfg config.Hook,
) (resolvedID string, resolvedName string, err error) {
	if teamIDFlag != "" && teamNameFlag != "" {
		return "", "", fmt.Errorf("only one of --%s or --%s can be provided", TeamIDFlagName, TeamNameFlagName)
	}
	if teamIDFlag == "" && teamNameFlag == "" {
		return "", "", fmt.Errorf(
			"a team identifier is required. Provide --%s or --%s",
			TeamIDFlagName, TeamNameFlagName,
		)
	}
	if teamIDFlag != "" {
		return teamIDFlag, teamIDFlag, nil
	}
	// Resolve by name
	team, err := lookupTeamByName(ctx, teamNameFlag, orgTeamAPI, cfg)
	if err != nil {
		return "", "", err
	}
	id := ""
	if team.GetID() != nil {
		id = *team.GetID()
	}
	name := teamNameFlag
	if team.GetName() != nil && *team.GetName() != "" {
		name = *team.GetName()
	}
	return id, name, nil
}

// lookupTeamByName fetches all teams matching the given name (eq filter) and
// returns the first match. Returns an error when no match is found.
func lookupTeamByName(
	ctx context.Context,
	name string,
	orgTeamAPI helpers.OrganizationTeamAPI,
	cfg config.Hook,
) (kkComps.Team, error) {
	var pageNumber int64 = 1
	requestPageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if requestPageSize < 1 {
		requestPageSize = int64(common.DefaultRequestPageSize)
	}

	for {
		req := kkOps.ListTeamsRequest{
			PageSize:   &requestPageSize,
			PageNumber: &pageNumber,
			Filter: &kkOps.ListTeamsQueryParamFilter{
				Name: &kkComps.LegacyStringFieldFilter{
					Eq: &name,
				},
			},
		}
		res, err := orgTeamAPI.ListOrganizationTeams(ctx, req)
		if err != nil {
			return kkComps.Team{}, fmt.Errorf("failed to list teams: %w", err)
		}

		col := res.GetTeamCollection()
		for _, t := range col.Data {
			if t.GetName() != nil && *t.GetName() == name {
				return t, nil
			}
		}

		totalFetched := int(res.GetTeamCollection().Meta.Page.Total)
		if int(pageNumber)*int(requestPageSize) >= totalFetched {
			break
		}
		pageNumber++
	}

	return kkComps.Team{}, fmt.Errorf("team with name %q not found", name)
}

// NewMembersCmd creates the members sub-command for the given verb, wiring in
// the users and system-accounts sub-sub-commands.
func NewMembersCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	membersCmd := &cobra.Command{
		Use:     membersUse,
		Short:   membersShort,
		Long:    membersLong,
		Example: membersExample,
		Aliases: []string{"members", "Member", "Members"},
		PreRunE: parentPreRun,
	}

	AddTeamFlags(membersCmd)

	if addParentFlags != nil {
		addParentFlags(verb, membersCmd)
	}

	membersCmd.RunE = func(c *cobra.Command, args []string) error {
		h := newGetMembersHandler(c)
		return h.run(args)
	}

	return membersCmd
}

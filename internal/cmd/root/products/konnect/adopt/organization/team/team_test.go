package team

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/assert"
)

var (
	testTeamID   = "22cd8a0b-72e7-4212-9099-0764f8e9c5ac"
	testTeamName = "platform-team"
)

type teamAPIStub struct {
	t          *testing.T
	team       *kkComps.Team
	lastUpdate *kkComps.UpdateTeam
	updateCall int
}

func (c *teamAPIStub) ListOrganizationTeams(
	context.Context,
	kkOps.ListTeamsRequest,
) (*kkOps.ListTeamsResponse, error) {
	c.t.Fatalf("unexpected ListOrganizationTeams call")
	return nil, nil
}

func (c *teamAPIStub) GetOrganizationTeam(
	_ context.Context,
	id string,
) (*kkOps.GetTeamResponse, error) {
	if id != *c.team.ID {
		c.t.Fatalf("unexpected team id: %s", id)
	}
	return &kkOps.GetTeamResponse{Team: c.team}, nil
}

func (c *teamAPIStub) CreateOrganizationTeam(
	context.Context,
	*kkComps.CreateTeam,
) (*kkOps.CreateTeamResponse, error) {
	c.t.Fatalf("unexpected CreateOrganizationTeam call")
	return nil, nil
}

func (c *teamAPIStub) UpdateOrganizationTeam(
	_ context.Context,
	id string,
	team *kkComps.UpdateTeam,
) (*kkOps.UpdateTeamResponse, error) {
	if id != *c.team.ID {
		c.t.Fatalf("unexpected team id: %s", id)
	}
	c.updateCall++
	c.lastUpdate = team

	resp := &kkOps.UpdateTeamResponse{
		Team: &kkComps.Team{
			ID:     c.team.ID,
			Name:   c.team.Name,
			Labels: labels.NormalizeLabels(team.Labels),
		},
	}
	return resp, nil
}

func (c *teamAPIStub) DeleteOrganizationTeam(
	context.Context,
	string,
) (*kkOps.DeleteTeamResponse, error) {
	c.t.Fatalf("unexpected DeleteOrganizationTeam call")
	return nil, nil
}

func TestAdoptTeamAssignsNamespaceLabel(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	stub := &teamAPIStub{
		t: t,
		team: &kkComps.Team{
			ID:     &testTeamID,
			Name:   &testTeamName,
			Labels: map[string]string{"env": "prod"},
		},
	}

	result, err := adoptTeam(helper, stub, nil, "team-alpha", testTeamID)
	assert.NoError(t, err)
	assert.Equal(t, "team", result.ResourceType)
	assert.Equal(t, testTeamID, result.ID)
	assert.Equal(t, testTeamName, result.Name)
	assert.Equal(t, "team-alpha", result.Namespace)

	assert.Equal(t, 1, stub.updateCall)
	assert.NotNil(t, stub.lastUpdate.Labels)
	assert.Equal(t, "prod", *stub.lastUpdate.Labels["env"])
	assert.Equal(t, "team-alpha", *stub.lastUpdate.Labels[labels.NamespaceKey])

	helper.AssertExpectations(t)
}

func TestAdoptTeamRejectsExistingNamespace(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	stub := &teamAPIStub{
		t: t,
		team: &kkComps.Team{
			ID:     &testTeamID,
			Name:   &testTeamName,
			Labels: map[string]string{labels.NamespaceKey: "existing"},
		},
	}

	_, err := adoptTeam(helper, stub, nil, "team-alpha", testTeamID)
	assert.Error(t, err)
	var cfgErr *cmd.ConfigurationError
	assert.ErrorAs(t, err, &cfgErr)
	assert.Equal(t, 0, stub.updateCall)

	helper.AssertExpectations(t)
}

func TestAdoptTeamWithExistingLabels(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	stub := &teamAPIStub{
		t: t,
		team: &kkComps.Team{
			ID:   &testTeamID,
			Name: &testTeamName,
			Labels: map[string]string{
				"env":       "prod",
				"region":    "us-west",
				"component": "api",
			},
		},
	}

	result, err := adoptTeam(helper, stub, nil, "team-alpha", testTeamID)
	assert.NoError(t, err)
	assert.Equal(t, "team", result.ResourceType)
	assert.Equal(t, testTeamID, result.ID)
	assert.Equal(t, "team-alpha", result.Namespace)

	assert.Equal(t, 1, stub.updateCall)
	assert.NotNil(t, stub.lastUpdate.Labels)
	// Existing labels should be preserved
	assert.Equal(t, "prod", *stub.lastUpdate.Labels["env"])
	assert.Equal(t, "us-west", *stub.lastUpdate.Labels["region"])
	assert.Equal(t, "api", *stub.lastUpdate.Labels["component"])
	// Namespace label should be added
	assert.Equal(t, "team-alpha", *stub.lastUpdate.Labels[labels.NamespaceKey])

	helper.AssertExpectations(t)
}

func TestResolveTeamRejectsInvalidUUID(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	stub := &teamAPIStub{t: t}

	_, err := resolveTeam(helper, stub, nil, "not-a-uuid")
	assert.Error(t, err)
	var cfgErr *cmd.ConfigurationError
	assert.ErrorAs(t, err, &cfgErr)

	helper.AssertExpectations(t)
}

func TestResolveTeamValidUUID(t *testing.T) {
	helper := new(cmd.MockHelper)
	helper.EXPECT().GetContext().Return(context.Background())

	stub := &teamAPIStub{
		t: t,
		team: &kkComps.Team{
			ID:     &testTeamID,
			Name:   &testTeamName,
			Labels: map[string]string{},
		},
	}

	team, err := resolveTeam(helper, stub, nil, testTeamID)
	assert.NoError(t, err)
	assert.NotNil(t, team)
	assert.Equal(t, testTeamID, *team.ID)
	assert.Equal(t, testTeamName, *team.Name)

	helper.AssertExpectations(t)
}

package identity

import (
	"bytes"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestPrincipalGroupCommandsRequireExplicitPrincipalAndDirectory(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		root, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		command, _, err := root.Find([]string{"principal-groups"})
		require.NoError(t, err)
		require.Equal(t, "principal-group", command.Name())
		require.ErrorContains(t, command.Args(command, nil), "directory")
		require.NoError(t, command.Flags().Set("directory-name", "default"))
		require.ErrorContains(t, command.Args(command, nil), "principal-id")
		require.NoError(t, command.Flags().Set("principal-id", "731559d3-39bd-48c7-b33c-eb192847eb21"))
		require.NoError(t, command.Args(command, nil))
		require.NoError(t, command.Args(command, []string{"Operators"}))
		require.Error(t, command.Args(command, []string{"one", "two"}))
		require.NoError(t, command.Flags().Set("directory-id", "731559d3-39bd-48c7-b33c-eb192847eb20"))
		require.ErrorContains(t, command.Args(command, nil), "exactly one")
	}
}

func TestPrincipalGroupTextPreservesInheritedStatusAndNullableEntry(t *testing.T) {
	value := helpers.IdentityPrincipalGroup{
		ID: "group-id", Name: "Engineering", Type: new(kkComps.KongGroupTypeTeam), Direct: new(false),
		ProvisionedBy: new(kkComps.ProvisionedBy("imported")),
		TeamManager:   &kkComps.KongTeamManager{ID: "manager-id", DisplayName: "Manager", Email: new("manager@example.test")},
	}
	var table bytes.Buffer
	require.NoError(t, printPrincipalGroupRows(&table, []helpers.IdentityPrincipalGroup{value}, false))
	for _, expected := range []string{"GROUP_ID", "DIRECT", "MEMBERSHIP_ID", "false", "imported", "manager-id"} {
		require.Contains(t, table.String(), expected)
	}
	var detail bytes.Buffer
	require.NoError(t, printPrincipalGroupRows(&detail, []helpers.IdentityPrincipalGroup{value}, true))
	detailFields := []string{"id:", "direct:", "false", "membership_id:", "team_manager.email:", "manager@example.test"}
	for _, expected := range detailFields {
		require.Contains(t, detail.String(), expected)
	}
	require.NotContains(t, detail.String(), "{")
	require.Error(t, printPrincipalGroupRows(&detail, nil, true))
}

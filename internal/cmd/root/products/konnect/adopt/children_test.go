package adopt

import (
	"fmt"
	"testing"

	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestChildSelectionIsExplicitAndParentScoped(t *testing.T) {
	values := []childCandidate{{"id-a", "team-a"}, {"id-b", "team-b"}, {"id-c", "outside"}}
	for _, tc := range []struct {
		selection childSelection
		expected  []childCandidate
		error     string
	}{
		{selection: childSelection{all: true}, expected: values},
		{selection: childSelection{pattern: "team-*"}, expected: values[:2]},
		{selection: childSelection{pattern: "team-?"}, expected: values[:2]},
		{selection: childSelection{pattern: "team-[ab]"}, expected: values[:2]},
		{selection: childSelection{identifier: "team-a"}, expected: values[:1]},
		{selection: childSelection{identifier: "id-a", idOnly: true}, expected: values[:1]},
		{selection: childSelection{identifier: "foreign-id"}, error: "no children match"},
		{selection: childSelection{pattern: "["}, error: "invalid name pattern"},
	} {
		actual, err := selectChildren(values, tc.selection)
		if tc.error != "" {
			require.ErrorContains(t, err, tc.error)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.expected, actual)
		}
	}
	_, err := selectChildren(append(values, childCandidate{"id-d", "team-a"}), childSelection{identifier: "team-a"})
	require.ErrorContains(t, err, "ambiguous")
	_, err = selectChildren(append(values, values[0]), childSelection{all: true})
	require.ErrorContains(t, err, "duplicate ID")
}

func TestChildBatchPreflightAndPartialFailure(t *testing.T) {
	values := []childCandidate{{"id-a", "first"}, {"id-b", "second"}}
	for _, preflightFailure := range []bool{true, false} {
		setup, out := childTestSetup(t, false, cmdCommon.JSON)
		writes := 0
		err := runChildAdoptions(setup, values, func(value childCandidate) (preparedChildAdoption, error) {
			if preflightFailure && value.id == "id-b" {
				return nil, fmt.Errorf("ownership conflict")
			}
			return func() (*adoptCommon.AdoptResult, error) {
				if value.id == "id-b" {
					return nil, fmt.Errorf("API unavailable")
				}
				writes++
				return &adoptCommon.AdoptResult{ID: value.id, Name: value.name, Namespace: "team-a"}, nil
			}, nil
		})
		if preflightFailure {
			require.ErrorContains(t, err, "ownership conflict")
			require.Zero(t, writes)
			require.Empty(t, out.String())
		} else {
			require.ErrorContains(t, err, "after 1 of 2 children completed")
			require.Equal(t, 1, writes)
			require.Contains(t, out.String(), "id-a")
		}
	}
}

func TestChildDryRunTextJSONAndYAML(t *testing.T) {
	for _, format := range []cmdCommon.OutputFormat{cmdCommon.TEXT, cmdCommon.JSON, cmdCommon.YAML} {
		setup, output := childTestSetup(t, true, format)
		result,
			dryRun := adoptCommon.PreviewAdoption(setup.Helper.GetContext(),
			string(resources.ResourceTypeAIGatewayModel),
			adoptionChildID,
			"model",
			"team-a",
			map[string]string{"gateway_id": adoptionGatewayID},
			"managed_by",
			map[string]string{
				resources.ChildProtectedKey: "true",
				"outside":                   "remove",
			},
			resources.BuildChildManagedBy("team-a",
				true,
				"dev"))
		require.True(t, dryRun)
		require.NoError(t, adoptCommon.PrintAdoptResult(setup.Helper, format, result, "model"))
		if format == cmdCommon.TEXT {
			require.Contains(t, output.String(), "Would adopt")
			require.Contains(t, output.String(), `kongctl-namespace: <absent> -> "team-a"`)
			require.Contains(t, output.String(), `outside: "remove" -> <removed>`)
		} else {
			var preview adoptCommon.AdoptionPreview
			require.NoError(t, yaml.Unmarshal([]byte(output.String()), &preview))
			require.True(t, preview.DryRun)
			require.Len(t, preview.Changes, 1)
			require.Equal(t, adoptionChildID, preview.Changes[0].ID)
			require.Equal(t, "true", preview.Changes[0].After[resources.ChildProtectedKey])
		}
	}
}

func TestLabelDryRunPreservesUnchangedReservedLabels(t *testing.T) {
	setup, output := childTestSetup(t, true, cmdCommon.JSON)
	before := map[string]string{"KONGCTL-legacy": "keep", "KONGCTL-protected": "true", "public": "exclude"}
	result, dryRun := adoptCommon.PreviewAdoption(setup.Helper.GetContext(), string(resources.ResourceTypePortal),
		adoptionChildID, "portal", "team-a", nil, "labels", before, map[string]string{"KONGCTL-namespace": "team-a"})
	require.True(t, dryRun)
	require.NoError(t, adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "portal"))
	var preview adoptCommon.AdoptionPreview
	require.NoError(t, yaml.Unmarshal([]byte(output.String()), &preview))
	require.Equal(t, "keep", preview.Changes[0].After["KONGCTL-legacy"])
	require.Equal(t, "true", preview.Changes[0].After["KONGCTL-protected"])
	require.NotContains(t, output.String(), "exclude")
	require.NotContains(t, before, "KONGCTL-namespace")
}

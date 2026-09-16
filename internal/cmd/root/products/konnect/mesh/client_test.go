package mesh

import (
	"errors"
	"testing"

	"github.com/kong/kongctl/internal/cmd"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// selectorCmd builds a command carrying the control plane selection flags, with
// the named ones marked as given on the command line.
func selectorCmd(t *testing.T, given ...string) *cobra.Command {
	t.Helper()

	cmdObj := &cobra.Command{Use: "mesh-selector-test"}
	meshcommon.AddControlPlaneFlags(cmdObj.Flags())
	for _, flag := range given {
		require.NoError(t, cmdObj.Flags().Set(flag, "value-for-"+flag))
	}
	return cmdObj
}

func TestExplicitControlPlaneSelector(t *testing.T) {
	cases := []struct {
		name  string
		given []string
		want  string
	}{
		{name: "nothing given", given: nil, want: ""},
		{name: "id", given: []string{meshcommon.ControlPlaneIDFlagName}, want: meshcommon.ControlPlaneIDFlagName},
		{
			name:  "name",
			given: []string{meshcommon.ControlPlaneNameFlagName},
			want:  meshcommon.ControlPlaneNameFlagName,
		},
		{name: "url", given: []string{meshcommon.ControlPlaneURLFlagName}, want: meshcommon.ControlPlaneURLFlagName},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			helper := &cmd.MockHelper{}
			helper.EXPECT().GetCmd().Return(selectorCmd(t, tc.given...))

			got, err := explicitControlPlaneSelector(helper)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// Two selectors name two different control planes, and this resolver serves
// writes and deletes, so the conflict is reported rather than resolved.
func TestExplicitControlPlaneSelectorRejectsConflicts(t *testing.T) {
	cases := [][]string{
		{meshcommon.ControlPlaneIDFlagName, meshcommon.ControlPlaneNameFlagName},
		{meshcommon.ControlPlaneURLFlagName, meshcommon.ControlPlaneIDFlagName},
		{meshcommon.ControlPlaneURLFlagName, meshcommon.ControlPlaneNameFlagName},
		{
			meshcommon.ControlPlaneURLFlagName,
			meshcommon.ControlPlaneIDFlagName,
			meshcommon.ControlPlaneNameFlagName,
		},
	}

	for _, given := range cases {
		helper := &cmd.MockHelper{}
		helper.EXPECT().GetCmd().Return(selectorCmd(t, given...))

		_, err := explicitControlPlaneSelector(helper)
		require.Error(t, err, "expected %v to conflict", given)
		require.Contains(t, err.Error(), "provide only one")
		for _, flag := range given {
			require.Contains(t, err.Error(), "--"+flag)
		}
	}
}

// A nil helper or command must not panic: some call paths build the helper
// before a command is attached.
func TestExplicitControlPlaneSelectorWithoutCommand(t *testing.T) {
	got, err := explicitControlPlaneSelector(nil)
	require.NoError(t, err)
	require.Equal(t, "", got)

	helper := &cmd.MockHelper{}
	helper.EXPECT().GetCmd().Return(nil)
	got, err = explicitControlPlaneSelector(helper)
	require.NoError(t, err)
	require.Equal(t, "", got)
}

// page builds one collection page.
func page(total int, names ...string) listEnvelope {
	items := make([]map[string]any, 0, len(names))
	for _, name := range names {
		items = append(items, map[string]any{"name": name})
	}
	return listEnvelope{Total: total, Items: items}
}

func namesOf(t *testing.T, items []map[string]any) []string {
	t.Helper()

	if items == nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name, ok := item["name"].(string)
		require.True(t, ok, "item %v has no name", item)
		names = append(names, name)
	}
	return names
}

func TestPaginateCollectsEveryPage(t *testing.T) {
	cases := []struct {
		name        string
		pages       []listEnvelope
		want        []string
		wantOffsets []int
	}{
		{
			name:        "a single full page ends the walk",
			pages:       []listEnvelope{page(2, "a", "b")},
			want:        []string{"a", "b"},
			wantOffsets: []int{0},
		},
		{
			// The case the review reproduced: one item with a total of two was
			// reported as the whole collection.
			name:        "a short first page is followed",
			pages:       []listEnvelope{page(2, "a"), page(2, "b")},
			want:        []string{"a", "b"},
			wantOffsets: []int{0, 1},
		},
		{
			name:        "several pages are concatenated in order",
			pages:       []listEnvelope{page(5, "a", "b"), page(5, "c", "d"), page(5, "e")},
			want:        []string{"a", "b", "c", "d", "e"},
			wantOffsets: []int{0, 2, 4},
		},
		{
			name:        "an empty collection fetches once",
			pages:       []listEnvelope{page(0)},
			want:        nil,
			wantOffsets: []int{0},
		},
		{
			// A control plane that keeps reporting more than it returns must
			// not spin: an empty page ends the walk whatever the total says.
			name:        "an empty page ends a total that overreports",
			pages:       []listEnvelope{page(99, "a"), page(99)},
			want:        []string{"a"},
			wantOffsets: []int{0, 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var offsets []int
			items, err := paginate(func(offset int) (listEnvelope, error) {
				offsets = append(offsets, offset)
				return tc.pages[len(offsets)-1], nil
			})

			require.NoError(t, err)
			require.Equal(t, tc.want, namesOf(t, items))
			// Each request asks for what has not been collected yet, so a
			// page is never fetched twice or skipped.
			require.Equal(t, tc.wantOffsets, offsets)
		})
	}
}

func TestPaginatePropagatesErrors(t *testing.T) {
	wantErr := errors.New("collection request failed")

	items, err := paginate(func(int) (listEnvelope, error) {
		return listEnvelope{}, wantErr
	})

	require.ErrorIs(t, err, wantErr)
	// A partial collection must not be returned as if it were complete.
	require.Nil(t, items)
}

func TestListPayloadReportsWhatWasCollected(t *testing.T) {
	payload := listPayload([]map[string]any{{"name": "a"}, {"name": "b"}})
	require.Equal(t, 2, payload["total"])

	// An empty collection renders as an empty list rather than null, matching
	// the control plane's own envelope.
	empty := listPayload(nil)
	require.Equal(t, 0, empty["total"])
	require.Equal(t, []map[string]any{}, empty["items"])
}

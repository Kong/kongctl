package columns

import (
	"bytes"
	"strings"
	"testing"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestParseAndProject(t *testing.T) {
	selected, err := Parse([]string{`NAME=.name,TEAM=.labels["team,]primary"]`, `FIRST=.items[0]`})
	require.NoError(t, err)

	headers, rows, err := Project([]map[string]any{
		{
			"name":   "payments",
			"labels": map[string]any{"team,]primary": "platform"},
			"items":  []any{"one", "two"},
		},
		{"name": "billing"},
	}, selected)
	require.NoError(t, err)
	require.Equal(t, []string{"NAME", "TEAM", "FIRST"}, headers)
	require.Equal(t, [][]string{{"payments", "platform", "one"}, {"billing", "", ""}}, rows)
}

func TestProjectCollectionsAsCompactJSON(t *testing.T) {
	selected, err := Parse([]string{`CONFIG=.config`})
	require.NoError(t, err)

	_, rows, err := Project(map[string]any{"config": map[string]any{"enabled": true, "type": "dedicated"}}, selected)
	require.NoError(t, err)
	require.Equal(t, `{"enabled":true,"type":"dedicated"}`, rows[0][0])
}

func TestParseRejectsInvalidColumns(t *testing.T) {
	tests := []string{
		`NAME`,
		`=.name`,
		`NAME=name`,
		`NAME=.labels[team]`,
		`NAME=.name,name=.other`,
		`NAME=.name,`,
	}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			_, err := Parse([]string{value})
			require.Error(t, err)
		})
	}
}

func TestResolveRequiresTextOutput(t *testing.T) {
	cmd := &cobra.Command{}
	AddFlags(cmd.Flags())
	require.NoError(t, cmd.Flags().Set(FlagName, "NAME=.name"))

	_, err := Resolve(cmd, cmdcommon.JSON)
	require.EqualError(t, err, "--columns is only supported with --output text")
}

func TestRenderCapsAndFitsColumns(t *testing.T) {
	var out bytes.Buffer
	long := strings.Repeat("界", 50)
	err := Render(&out, []string{"NAME", "DESCRIPTION"}, [][]string{{long, strings.Repeat("x", 80)}}, 30)
	require.NoError(t, err)

	for line := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
		require.LessOrEqual(t, runewidth.StringWidth(line), 30)
	}
	require.Contains(t, out.String(), "…")
}

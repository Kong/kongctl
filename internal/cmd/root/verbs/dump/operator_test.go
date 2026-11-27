package dump

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/operatorexport"
)

func TestDumpRegistersOperatorSubcommand(t *testing.T) {
	dumpCmd, err := NewDumpCmd()
	require.NoError(t, err)

	operatorCmd, _, err := dumpCmd.Find([]string{formatOperator})
	require.NoError(t, err)
	assert.Equal(t, formatOperator, operatorCmd.Name())
}

func TestOperatorDumpFlagDefaults(t *testing.T) {
	cmd := newOperatorCmd()
	defaults := map[string]string{
		operatorNamespaceFlagName:      "default",
		operatorAuthRefFlagName:        "konnect-api-auth",
		operatorAdoptModeFlagName:      "auto",
		operatorIncludeSecretsFlagName: "false",
		"output-file":                  "",
		"control-plane-id":             "",
		"control-plane-name":           "",
	}
	for name, want := range defaults {
		flag := cmd.Flags().Lookup(name)
		require.NotNil(t, flag, "missing --%s", name)
		assert.Equal(t, want, flag.DefValue, "--%s default", name)
	}
}

func TestOperatorDumpRejectsInvalidOptionsBeforeCallingKonnect(t *testing.T) {
	tests := map[string][]string{
		"unknown adopt mode": {"--" + operatorAdoptModeFlagName, "merge"},
		"invalid namespace":  {"--" + operatorNamespaceFlagName, "Not_Valid"},
		"empty auth ref":     {"--" + operatorAuthRefFlagName, ""},
	}
	for name, args := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := newOperatorCmd()
			require.NoError(t, cmd.Flags().Parse(args))

			err := cmd.RunE(cmd, nil)
			var configurationError *cmdpkg.ConfigurationError
			require.ErrorAs(t, err, &configurationError)
		})
	}
}

func TestCollectOffsetPagesFollowsOffsets(t *testing.T) {
	pages := map[string][]int{"": {1, 2}, "a": {3}, "b": {4}}
	next := map[string]string{"": "a", "a": "b", "b": " "}
	var seen []string

	all, err := collectOffsetPages(func(offset *string) ([]int, *string, error) {
		key := ""
		if offset != nil {
			key = *offset
		}
		seen = append(seen, key)
		following := next[key]
		return pages[key], &following, nil
	})

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3, 4}, all)
	assert.Equal(t, []string{"", "a", "b"}, seen)
}

func TestCollectOffsetPagesReturnsErrors(t *testing.T) {
	boom := errors.New("boom")
	_, err := collectOffsetPages(func(*string) ([]int, *string, error) {
		return nil, nil, boom
	})
	require.ErrorIs(t, err, boom)
}

func TestWriteOperatorWarnings(t *testing.T) {
	var out bytes.Buffer
	result := operatorexport.Result{
		Manifests: []operatorexport.Manifest{{Kind: operatorexport.KindService, Name: "svc"}},
		Warnings: []operatorexport.Warning{{
			Kind: operatorexport.KindRoute, Name: "r", ID: "id-1", Message: "skipped: expression route",
		}},
	}

	require.NoError(t, writeOperatorWarnings(&out, []string{"short read"}, result))
	assert.Equal(t,
		"Warning: short read\n"+
			"Warning: KongRoute r (id-1): skipped: expression route\n"+
			"Exported 1 resources with 2 warnings.\n",
		out.String())
}

package identity

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/require"
)

func TestPrincipalDisplayNameIsLocalListFilter(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		root, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		principal, _, err := root.Find([]string{"principals"})
		require.NoError(t, err)
		require.NotNil(t, principal.Flags().Lookup("display-name"))
		require.Nil(t, principal.PersistentFlags().Lookup("display-name"))
		require.Nil(t, root.Flags().Lookup("display-name"))
		for _, child := range principal.Commands() {
			require.Nil(t, child.Flag("display-name"))
		}
		require.NoError(t, principal.Flags().Set("display-name", "On Call"))
		require.NoError(t, principal.Args(principal, nil))
		require.ErrorContains(t, principal.Args(principal, []string{"731559d3-39bd-48c7-b33c-eb192847eb20"}), "not both")
		require.NoError(t, principal.Flags().Set("display-name", ""))
		require.ErrorContains(t, principal.Args(principal, nil), "must not be empty")
	}
}

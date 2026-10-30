package aicostmanagement_test

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/products/konnect/aicostmanagement"
	"github.com/kong/kongctl/internal/cmd/root/verbs/get"
	listcmd "github.com/kong/kongctl/internal/cmd/root/verbs/list"
	"github.com/spf13/cobra"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/require"
)

func TestTagCaptureCommands(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		root := aicostmanagement.NewCommand(verb, nil, nil)
		child, _, err := root.Find([]string{"tag-capture"})
		require.NoError(t, err)
		require.Equal(t, "tag-capture", child.Name())
		require.Error(t, child.Args(child, []string{"gateway"}), "this is not a gateway child")
	}
}

func TestTagCaptureKonnectFirstAndExplicitPaths(t *testing.T) {
	for _, build := range []func() (*cobra.Command, error){get.NewGetCmd, listcmd.NewListCmd} {
		root, err := build()
		require.NoError(t, err)
		for _, path := range [][]string{
			{"ai-cost-management", "tag-capture"},
			{"konnect", "ai-cost-management", "tag-capture"},
		} {
			child, _, err := root.Find(path)
			require.NoError(t, err)
			require.Equal(t, "tag-capture", child.Name())
		}
	}
}

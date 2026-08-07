package declarative

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestSecretWriteFlagsAreAvailableOnAuthoringCommands(t *testing.T) {
	commands := map[string]*cobra.Command{
		"plan":  newDeclarativePlanCmd(),
		"diff":  newDeclarativeDiffCmd(),
		"apply": newDeclarativeApplyCmd(),
		"sync":  newDeclarativeSyncCmd(),
	}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			require.NotNil(t, command.Flags().Lookup(writeSecretFlagName))
			require.NotNil(t, command.Flags().Lookup(writeSecretsFlagName))
		})
	}
	require.Nil(t, newDeclarativeDeleteCmd().Flags().Lookup(writeSecretFlagName))
}

func TestRejectPlanSecretWriteFlags(t *testing.T) {
	command := newDeclarativeApplyCmd()
	require.NoError(t, command.Flags().Set(writeSecretFlagName, "portal-idp#config.client_secret"))
	require.ErrorContains(t, rejectPlanSecretWriteFlags(command, "plan.json"), "already recorded")
	require.NoError(t, rejectPlanSecretWriteFlags(command, ""))
}

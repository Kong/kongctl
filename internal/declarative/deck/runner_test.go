package deck

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildArgsGatewayInjectsKonnect(t *testing.T) {
	opts := RunOptions{
		Args:                    []string{"gateway", "apply", "kong.yaml"},
		Mode:                    "apply",
		KonnectToken:            "token-123",
		KonnectControlPlaneName: "cp-name",
		KonnectAddress:          "https://api.konghq.com",
	}

	args, err := buildArgs(opts)
	require.NoError(t, err)
	require.Equal(t, []string{
		"gateway",
		"apply",
		"--konnect-token",
		"token-123",
		"--konnect-control-plane-name",
		"cp-name",
		"--konnect-addr",
		"https://api.konghq.com",
		"kong.yaml",
	}, args)
}

func TestBuildArgsNonGatewayLeavesArgsAlone(t *testing.T) {
	opts := RunOptions{
		Args: []string{"file", "openapi2kong", "input.yaml"},
		Mode: "sync",
	}

	args, err := buildArgs(opts)
	require.NoError(t, err)
	require.Equal(t, []string{"file", "openapi2kong", "input.yaml"}, args)
}

func TestBuildArgsRejectsConflictingFlags(t *testing.T) {
	opts := RunOptions{
		Args:                    []string{"gateway", "sync", "--konnect-token", "user-token"},
		Mode:                    "sync",
		KonnectToken:            "token-123",
		KonnectControlPlaneName: "cp-name",
		KonnectAddress:          "https://api.konghq.com",
	}

	_, err := buildArgs(opts)
	var conflict ErrConflictingFlag
	require.ErrorAs(t, err, &conflict)
}

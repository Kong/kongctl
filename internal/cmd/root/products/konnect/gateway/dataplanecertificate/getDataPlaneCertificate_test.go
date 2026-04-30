package dataplanecertificate

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDataPlaneCertificateValidate(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "list",
		},
		{
			name: "get by UUID",
			args: []string{"6f91e1fb-f846-4127-9571-e8d5d6026fa5"},
		},
		{
			name:    "rejects non UUID",
			args:    []string{"not-a-uuid"},
			wantErr: "data plane certificate ID must be a UUID",
		},
		{
			name:    "rejects too many args",
			args:    []string{"6f91e1fb-f846-4127-9571-e8d5d6026fa5", "extra"},
			wantErr: "too many arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := &getDataPlaneCertificateCmd{}
			err := command.validate(&cmd.CommandHelper{Args: tt.args})
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNewDataPlaneCertificateCmd(t *testing.T) {
	command, err := NewDataPlaneCertificateCmd(verbs.Get, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, command)

	assert.Equal(t, "data-plane-certificates", command.Use)
	assert.Contains(t, command.Aliases, "data-plane-certificate")
	assert.Contains(t, command.Aliases, "data-plane-certs")
	assert.Contains(t, command.Aliases, "dp-cert")
	assert.NotNil(t, command.RunE)
}

func TestDataPlaneCertificateControlPlaneIDFromParent(t *testing.T) {
	controlPlane := kkComps.ControlPlane{ID: "cp-id"}

	id, err := dataPlaneCertificateControlPlaneIDFromParent(&controlPlane)
	require.NoError(t, err)
	assert.Equal(t, "cp-id", id)

	_, err = dataPlaneCertificateControlPlaneIDFromParent(kkComps.ControlPlane{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "control plane identifier is missing")
}

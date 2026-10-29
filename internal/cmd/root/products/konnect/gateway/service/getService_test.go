package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestGetServiceByIDUsesDefaultWorkspace(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	sdk := kk.New(kk.WithServerURL(server.URL), kk.WithClient(server.Client()))
	command := &cobra.Command{Use: "service"}
	command.SetContext(t.Context())
	helper := &cmd.CommandHelper{Cmd: command}

	err := (&getServiceCmd{}).runGet("cp-id", "entity-id", sdk, helper, nil, cmdCommon.JSON)

	require.Error(t, err)
	require.Equal(t, "/v2/control-planes/cp-id/core-entities/default/services/entity-id", requestPath)
}

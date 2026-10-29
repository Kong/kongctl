package consumer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestGetConsumerByIDUsesDefaultWorkspace(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	sdk := kk.New(kk.WithServerURL(server.URL), kk.WithClient(server.Client()))
	command := &cobra.Command{Use: "consumer"}
	command.SetContext(t.Context())
	helper := &cmd.CommandHelper{Cmd: command}

	_, err := fetchConsumerByID(helper, sdk, "cp-id", "entity-id")

	require.Error(t, err)
	require.Equal(t, "/v2/control-planes/cp-id/core-entities/default/consumers/entity-id", requestPath)
}

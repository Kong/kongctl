package helpers_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestOrganizationTeamRolePagesUseGlobalAuthenticatedClient(t *testing.T) {
	called := false
	client := secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
		called = true
		require.Equal(t, "global.api.konghq.com", request.URL.Host)
		require.Equal(t, "/v3/teams/team-id/assigned-roles", request.URL.Path)
		require.Equal(t, "100", request.URL.Query().Get("page[size]"))
		require.Equal(t, "2", request.URL.Query().Get("page[number]"))
		require.Equal(t, "Bearer test-token", request.Header.Get("Authorization"))
		return &http.Response{
			StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"data":[],"meta":{"page":{"total":0}}}`)),
		}, nil
	})
	sdk := &helpers.KonnectSDK{
		SDK: kkSDK.New(), BaseURL: "https://eu.api.konghq.com", HTTPClient: client,
		TokenSource: apiutil.NewStaticTokenSource("test-token"),
	}
	_, err := sdk.GetOrganizationTeamRolesAPI().ListTeamRolesPage(t.Context(), "team-id", 100, 2)
	require.NoError(t, err)
	require.True(t, called)
}

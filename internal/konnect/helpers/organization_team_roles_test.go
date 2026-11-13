package helpers_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/auth"
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

func TestOrganizationTeamRolePagesHonorEnvironmentAndCustomEndpoints(t *testing.T) {
	for _, tc := range []struct{ base, want string }{
		{"https://eu.api.konghq.tech", "https://global.api.konghq.tech/v3/teams/team-id/assigned-roles"},
		{"https://staging.internal/api", "https://staging.internal/api/v3/teams/team-id/assigned-roles"},
		{"http://localhost:8000/prefix", "http://localhost:8000/prefix/v3/teams/team-id/assigned-roles"},
	} {
		t.Run(tc.base, func(t *testing.T) {
			client := secretHTTPFunc(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, tc.want, r.URL.Scheme+"://"+r.URL.Host+r.URL.Path)
				require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				return &http.Response{
					StatusCode: 200,
					Header:     make(http.Header),
					Body: io.NopCloser(
						strings.NewReader(`{"data":[],"meta":{"page":{"total":0}}}`),
					),
				}, nil
			})
			sdk := &helpers.KonnectSDK{
				SDK:        kkSDK.New(),
				BaseURL:    tc.base,
				Token:      "test-token",
				HTTPClient: auth.NewKonnectHostRewriteClient(client, tc.base),
			}
			_, err := sdk.GetOrganizationTeamRolesAPI().
				ListTeamRolesPage(t.Context(), "team-id", 10, 1)
			require.NoError(t, err)
		})
	}
}

func TestOrganizationTeamRoleErrorsAreBounded(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"   ", "status 503"},
		{`{"detail":"upstream unavailable"}`, "upstream unavailable"},
		{`{"message":"try later"}`, "try later"},
		{strings.Repeat("<html>error</html>", 1000), "..."},
	} {
		t.Run(tc.want, func(t *testing.T) {
			api := &helpers.OrganizationTeamRolesAPIImpl{
				BaseURL: "https://example.test",
				HTTPClient: secretHTTPFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 503,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(tc.body)),
					}, nil
				}),
			}
			_, err := api.ListTeamRolesPage(t.Context(), "team", 100, 1)
			require.ErrorContains(t, err, tc.want)
			require.Less(t, len(err.Error()), 600)
		})
	}
}

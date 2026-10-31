package helpers_test

import (
	"net/http"
	"strings"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/stretchr/testify/require"
)

func TestMembershipCreateDoesNotReplayUncertainResponse(t *testing.T) {
	posts := 0
	inner := secretHTTPFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPost {
			posts++
			return secretResponse(request, http.StatusServiceUnavailable, `{"message":"uncertain"}`), nil
		}
		items := []any{}
		if strings.HasSuffix(request.URL.Path, "/principals") {
			items = append(items, map[string]any{"id": identityPrincipal})
		}
		return groupJSONResponse(t, request, groupCollection(items, nil)), nil
	})
	transport := httpclient.NewRetryingHTTPClient(inner, httpclient.RetryConfig{
		Strategy: httpclient.RetryStrategyBackoff, MaxAttempts: 3,
		InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := &helpers.KonnectSDK{SDK: kk.New(kk.WithServerURL("https://identity.test"), kk.WithClient(transport),
		kk.WithRetryConfig(retry.Config{Strategy: "backoff", Backoff: &retry.BackoffStrategy{
			InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
		}}))}
	client := state.NewClient(
		state.ClientConfig{
			IdentityGroupsAPI:     sdk.GetIdentityGroupsAPI(),
			IdentityPrincipalsAPI: sdk.GetIdentityPrincipalsAPI(),
		},
	)
	_, err := client.CreateIdentityGroupMembership(
		t.Context(),
		identityDirectory,
		identityGroupOne,
		kkComps.CreateGroupMembershipEntryBody{
			Member: kkComps.KongGroupMemberInput{Type: kkComps.KongGroupMembershipTypePrincipal, ID: identityPrincipal},
		},
	)
	require.ErrorContains(t, err, "inspect the collection before retrying")
	require.Equal(t, 1, posts, "disable both retry layers without rollback")
}

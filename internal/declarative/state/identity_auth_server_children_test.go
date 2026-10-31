package state

import (
	"io"
	"net/http"
	"strings"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/stretchr/testify/require"
)

type claimCreateFailure struct{ calls int }

func (c *claimCreateFailure) Do(request *http.Request) (*http.Response, error) {
	c.calls++
	return &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"message":"uncertain outcome"}`)), Request: request,
	}, nil
}

func TestClaimCreateDoesNotReplayThroughSDKOrTransport(t *testing.T) {
	inner := &claimCreateFailure{}
	transport := httpclient.NewRetryingHTTPClient(inner, httpclient.RetryConfig{
		Strategy: httpclient.RetryStrategyBackoff, MaxAttempts: 3,
		InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := kk.New(kk.WithClient(transport), kk.WithServerURL("https://probe.invalid"),
		kk.WithRetryConfig(retry.Config{Strategy: "backoff", Backoff: &retry.BackoffStrategy{
			InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
		}}))
	api := &helpers.KonnectSDK{SDK: sdk}
	client := NewClient(ClientConfig{IdentityAuthServerClaimsAPI: api.GetIdentityAuthServerClaimsAPI()})
	_, err := client.CreateIdentityAuthServerClaim(t.Context(), "parent",
		kkComps.CreateClaim{Name: "claim", Value: "value"})
	require.Error(t, err)
	require.Equal(t, 1, inner.calls, "an uncertain create must not be replayed by either retry layer")
	inner.calls = 0
	client = NewClient(ClientConfig{IdentityPrincipalsAPI: api.GetIdentityPrincipalsAPI()})
	_, err = client.CreateIdentityPrincipal(t.Context(), "directory", kkComps.CreatePrincipalBody{
		ManagedBy: resources.BuildPrincipalManagedBy("team", "principal", false, "test"),
	})
	require.Error(t, err)
	require.Equal(t, 1, inner.calls, "principal creation also has no server-enforced unique match key")
}

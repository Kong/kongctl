package helpers_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

type principalTransport func(*http.Request) (*http.Response, error)

func (f principalTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func principalAPI(body func(*http.Request) string) helpers.IdentityPrincipalsAPI {
	client := sdk.New(sdk.WithServerURL("https://example.test"), sdk.WithClient(&http.Client{
		Transport: principalTransport(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
				Body: io.NopCloser(strings.NewReader(body(request))), Request: request,
			}, nil
		}),
	}))
	return client.KongPrincipal
}

func TestPrincipalReadPreservesActivePresence(t *testing.T) {
	for _, active := range []string{"", `,"active":false`, `,"active":true`} {
		t.Run(active, func(t *testing.T) {
			api := principalAPI(func(request *http.Request) string {
				require.Equal(t, "/v2/directories/directory-id/principals/principal-id", request.URL.Path)
				return `{"id":"principal-id","type":null,"description":"","metadata":{},"labels":{},"managed_by":{}` + active + `}`
			})
			value, err := helpers.GetIdentityPrincipal(t.Context(), api, "directory-id", "principal-id")
			require.NoError(t, err)
			encoded, err := json.Marshal(value)
			require.NoError(t, err)
			if active == "" {
				require.Nil(t, value.Active)
				require.NotContains(t, string(encoded), `"active"`)
			} else {
				require.NotNil(t, value.Active)
				require.Equal(t, strings.Contains(active, "true"), *value.Active)
			}
		})
	}
}

func TestPrincipalListObservesAllPagesAndRejectsIncompleteData(t *testing.T) {
	for _, fault := range []string{"", "repeat", "missing-data"} {
		t.Run(fault, func(t *testing.T) {
			calls := 0
			api := principalAPI(func(request *http.Request) string {
				calls++
				require.Empty(t, request.URL.Query().Get("filter"))
				if calls == 1 {
					return `{"data":[],"meta":{"page":{"size":100,"next":"cursor-two"}}}`
				}
				require.Equal(t, "cursor-two", request.URL.Query().Get("page[after]"), request.URL.String())
				if fault == "missing-data" {
					return `{"meta":{"page":{"size":100,"next":null}}}`
				}
				next := "null"
				if fault == "repeat" {
					next = `"cursor-two"`
				}
				return fmt.Sprintf(`{"data":[{"id":"principal-id","type":"agent",`+
					`"metadata":{},"labels":{},"managed_by":{}}],"meta":{"page":{"size":100,"next":%s}}}`, next)
			})
			values, err := helpers.ListIdentityPrincipals(t.Context(), api, "directory-id", 100)
			if fault != "" {
				require.Error(t, err)
				require.Nil(t, values)
			} else {
				require.NoError(t, err)
				require.Len(t, values, 1)
				require.Nil(t, values[0].Active)
				require.Equal(t, "agent", string(*values[0].Type))
			}
			require.Equal(t, 2, calls)
		})
	}
}

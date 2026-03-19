package httpclient

import (
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTransportOptionsFromEnvPrefersGenericVars(t *testing.T) {
	t.Setenv("KONGCTL_HTTP_TCP_USER_TIMEOUT", "60s")
	t.Setenv("KONGCTL_HTTP_DISABLE_KEEPALIVES", "true")
	t.Setenv("KONGCTL_HTTP_RECYCLE_CONNECTIONS_ON_ERROR", "1")
	t.Setenv("KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT", "15s")
	t.Setenv("KONGCTL_E2E_HTTP_DISABLE_KEEPALIVES", "false")
	t.Setenv("KONGCTL_E2E_HTTP_RECYCLE_CONNECTIONS_ON_ERROR", "0")

	got := TransportOptionsFromEnv()
	require.Equal(t, 60*time.Second, got.TCPUserTimeout)
	require.True(t, got.DisableKeepAlives)
	require.True(t, got.RecycleConnectionsOnError)
}

func TestTransportOptionsFromEnvFallsBackToE2EVars(t *testing.T) {
	t.Setenv("KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT", "45s")
	t.Setenv("KONGCTL_E2E_HTTP_DISABLE_KEEPALIVES", "true")
	t.Setenv("KONGCTL_E2E_HTTP_RECYCLE_CONNECTIONS_ON_ERROR", "1")

	got := TransportOptionsFromEnv()
	require.Equal(t, 45*time.Second, got.TCPUserTimeout)
	require.True(t, got.DisableKeepAlives)
	require.True(t, got.RecycleConnectionsOnError)
}

func TestNewHTTPClientWithConfig(t *testing.T) {
	t.Setenv("KONGCTL_HTTP_DISABLE_KEEPALIVES", "true")
	t.Setenv("KONGCTL_HTTP_RECYCLE_CONNECTIONS_ON_ERROR", "1")

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	client := NewHTTPClientWithConfig(ClientConfig{
		Timeout: 7 * time.Second,
		Jar:     jar,
	})

	require.Equal(t, 7*time.Second, client.Timeout)
	require.Same(t, jar, client.Jar)

	transport, ok := client.Transport.(*recyclingTransport)
	require.True(t, ok)
	require.True(t, transport.recycle)
	require.True(t, transport.base.DisableKeepAlives)
	require.NotNil(t, transport.base.DialContext)
}

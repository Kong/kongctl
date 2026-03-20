package httpclient

import (
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewHTTPClientWithConfig(t *testing.T) {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	client := NewHTTPClientWithConfig(ClientConfig{
		Timeout: 7 * time.Second,
		Jar:     jar,
		TransportOptions: TransportOptions{
			TCPUserTimeout:            30 * time.Second,
			DisableKeepAlives:         true,
			RecycleConnectionsOnError: true,
		},
	})

	require.Equal(t, 7*time.Second, client.Timeout)
	require.Same(t, jar, client.Jar)

	transport, ok := client.Transport.(*recyclingTransport)
	require.True(t, ok)
	require.True(t, transport.recycle)
	require.True(t, transport.base.DisableKeepAlives)
	require.NotNil(t, transport.base.DialContext)
}

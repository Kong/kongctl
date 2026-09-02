package aigateway

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayTLSSingleRenderUsesObjects(t *testing.T) {
	record := aiGatewayTLSRecord{ID: "certificate-id", Name: "runtime-ca"}
	item := kkComps.AIGatewayCACertificate{ID: "certificate-id", Name: "runtime-ca", Cert: "public-cert"}

	display, raw := aiGatewayTLSRenderValues([]aiGatewayTLSRecord{record}, []any{item}, true)

	require.Equal(t, record, display)
	require.Equal(t, item, raw)
}

func TestAIGatewayTLSListRenderUsesSlices(t *testing.T) {
	records := []aiGatewayTLSRecord{{ID: "certificate-id", Name: "runtime-ca"}}
	items := []any{kkComps.AIGatewayCACertificate{ID: "certificate-id", Name: "runtime-ca", Cert: "public-cert"}}

	display, raw := aiGatewayTLSRenderValues(records, items, false)

	require.Equal(t, records, display)
	require.Equal(t, items, raw)
}

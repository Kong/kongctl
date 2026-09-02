package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayCertificateAdapterMapsPrivateKeyOnlyAtExecution(t *testing.T) {
	fields := map[string]any{
		planner.FieldName: "runtime-cert", planner.FieldCert: "public-cert", planner.FieldKey: "private-key",
	}
	var request kkComps.CreateAIGatewayCertificateRequest
	err := (&AIGatewayCertificateAdapter{}).MapCreateFields(t.Context(), nil, fields, &request)
	require.NoError(t, err)
	require.Equal(t, "private-key", request.Key)
}

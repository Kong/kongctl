package identitytrust

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestNormalizeJITIdentifier(t *testing.T) {
	for _, identifier := range []string{"sub", "oid"} {
		value := &kkComps.TrustedIdpJit{Mapping: &kkComps.TrustedIdpJitMapping{Identifier: &identifier}}
		result := NormalizeJIT(value)
		require.NotNil(t, result)
		require.Equal(t, identifier, *result.Mapping.Identifier)
		require.NotSame(t, value.Mapping, result.Mapping)
	}
	require.Nil(t, NormalizeJIT(&kkComps.TrustedIdpJit{Mapping: &kkComps.TrustedIdpJitMapping{}}))
}

package resources

import (
	"reflect"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestExplainSDKEnumDiscriminators(t *testing.T) {
	for _, tc := range []struct {
		typ   reflect.Type
		value string
	}{
		{reflect.TypeFor[kkComps.RedisCEDatastore](), "redis-ce"},
		{reflect.TypeFor[kkComps.RedisEEDatastore](), "redis-ee"},
		{reflect.TypeFor[kkComps.TokenVaultDatastore](), "token-vault"},
		{reflect.TypeFor[kkComps.AIGatewayModelAPI](), "api"},
	} {
		name, value, ok := explainConstDiscriminator(tc.typ)
		require.True(t, ok, tc.typ.Name())
		require.Equal(t, "type", name)
		require.Equal(t, tc.value, value)
	}
}

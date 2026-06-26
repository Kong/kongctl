package resources

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventGatewayProducePolicyEncryptFieldsUnmarshal(t *testing.T) {
	data := []byte(`{
		"ref": "produce-encrypt-fields",
		"type": "encrypt_fields",
		"name": "produce-encrypt-fields",
		"parent_policy_id": "__REF__:produce-schema-validation#id",
		"config": {
			"failure_mode": "reject",
			"encrypt_fields": [
				{
					"paths": "$.customer.ssn",
					"encryption_key": {
						"type": "static",
						"key": {
							"id": "__REF__:static-key#id"
						}
					}
				}
			]
		}
	}`)

	var policy EventGatewayProducePolicyResource
	require.NoError(t, json.Unmarshal(data, &policy))
	require.NoError(t, policy.Validate())
	require.Equal(t, "produce-encrypt-fields", policy.GetMoniker())

	variant := policy.EventGatewayParsedRecordEncryptFieldsPolicyCreate
	require.NotNil(t, variant)
	require.Equal(t, "__REF__:produce-schema-validation#id", variant.ParentPolicyID)
	require.Len(t, variant.Config.EncryptFields, 1)

	staticKey := variant.Config.EncryptFields[0].EncryptionKey.EncryptionKeyStatic
	require.NotNil(t, staticKey)
	staticKeyID := staticKey.Key.EncryptionKeyStaticReferenceByID
	require.NotNil(t, staticKeyID)
	require.Equal(t, "__REF__:static-key#id", staticKeyID.ID)
}

func TestEventGatewayConsumePolicyDecryptFieldsUnmarshal(t *testing.T) {
	data := []byte(`{
		"ref": "consume-decrypt-fields",
		"type": "decrypt_fields",
		"name": "consume-decrypt-fields",
		"parent_policy_id": "__REF__:consume-schema-validation#id",
		"config": {
			"failure_mode": "error",
			"key_sources": [
				{"type": "static"}
			],
			"decrypt_fields": {
				"paths": "$.customer.ssn"
			}
		}
	}`)

	var policy EventGatewayConsumePolicyResource
	require.NoError(t, json.Unmarshal(data, &policy))
	require.NoError(t, policy.Validate())
	require.Equal(t, "consume-decrypt-fields", policy.GetMoniker())

	variant := policy.EventGatewayParsedRecordDecryptFieldsPolicyCreate
	require.NotNil(t, variant)
	require.Equal(t, "__REF__:consume-schema-validation#id", variant.ParentPolicyID)
	require.Len(t, variant.Config.KeySources, 1)
	require.NotNil(t, variant.Config.KeySources[0].EventGatewayStaticKeySource)
	require.NotNil(t, variant.Config.DecryptFields.Paths.Str)
	require.Equal(t, "$.customer.ssn", *variant.Config.DecryptFields.Paths.Str)
}

package loader

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestTrustedIDPDeclarationsManagedParentPresenceAndNullableRoundTrip(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		for _, invalid := range []string{
			"", "issuer_url", "external", "kongctl", "managed_by", "duplicate",
			"jit-null", "jit-unknown", "mapping-null", "claim-null", "boolean-null", "jwks-empty",
		} {
			t.Run(invalid+map[bool]string{true: "/grouped", false: "/nested"}[grouped], func(t *testing.T) {
				parent := map[string]any{
					"ref":                      "people",
					"name":                     "default",
					"allow_all_control_planes": true,
					"vault_enabled":            true,
				}
				value := map[string]any{
					"ref":          "workforce",
					"issuer_url":   "https://issuer.example.test/auth",
					"display_name": nil,
					"jwks_uri":     nil,
					"jit":          map[string]any{},
				}
				switch invalid {
				case "issuer_url":
					delete(value, "issuer_url")
				case "external":
					parent["_external"] = map[string]any{"id": "731559d3-39bd-48c7-b33c-eb192847eb20"}
				case "kongctl":
					value["kongctl"] = map[string]any{"namespace": "independent"}
				case "managed_by":
					value["managed_by"] = map[string]any{"tool": "kongctl"}
				case "jit-null":
					value["jit"] = nil
				case "jit-unknown":
					value["jit"] = map[string]any{"mapping": map[string]any{"unknown": "claim"}}
				case "mapping-null":
					value["jit"] = map[string]any{"mapping": nil}
				case "claim-null":
					value["jit"] = map[string]any{"mapping": map[string]any{"email": nil}}
				case "boolean-null":
					value["vault_access_enabled"] = nil
				case "jwks-empty":
					value["jwks_uri"] = ""
				}
				values := []any{value}
				if invalid == "duplicate" {
					values = append(values, map[string]any{"ref": "other", "issuer_url": value["issuer_url"]})
				}
				identity := map[string]any{"directories": []any{parent}}
				if grouped {
					for _, v := range values {
						v.(map[string]any)["directory"] = "people"
					}
					identity["trusted_idps"] = values
				} else {
					parent["trusted_idps"] = values
				}
				data, err := json.Marshal(map[string]any{"identity": identity})
				require.NoError(t, err)
				rs, err := New().LoadFile(writeLoaderTestFile(t, string(data)))
				if invalid != "" {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.Len(t, rs.Identity.TrustedIDPs, 1)
				p := rs.Identity.TrustedIDPs[0]
				require.Equal(t, "people", p.Directory)
				require.True(t, p.DisplayNameDeclared())
				require.True(t, p.JwksURIDeclared())
				require.Nil(t, p.DisplayName)
				require.Nil(t, p.JwksURI)
				require.NotNil(t, p.Jit)
				require.True(
					t,
					rs.SyncScope.ChildInScope(
						resources.ResourceTypeIdentityDirectory,
						"people",
						resources.ResourceTypeIdentityTrustedIDP,
					),
				)
				data, err = json.Marshal(p)
				require.NoError(t, err)
				require.Contains(t, string(data), `"display_name":null`)
				require.Contains(t, string(data), `"jwks_uri":null`)
			})
		}
	}
}

func TestTrustedIDPCollectionOmissionAndSharedDisplayNames(t *testing.T) {
	for _, presence := range []string{"", `,"trusted_idps":[]`} {
		input := `{"identity":{"directories":[{"ref":"people","name":"default",` +
			`"allow_all_control_planes":true` + presence + `}]}}`
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		require.Equal(
			t,
			presence != "",
			rs.SyncScope.ChildInScope(
				resources.ResourceTypeIdentityDirectory,
				"people",
				resources.ResourceTypeIdentityTrustedIDP,
			),
		)
	}
	input := `{"identity":{"directories":[{"ref":"people","name":"default",` +
		`"allow_all_control_planes":true,"trusted_idps":[` +
		`{"ref":"a","issuer_url":"https://a.example.test","display_name":"same"},` +
		`{"ref":"b","issuer_url":"https://b.example.test","display_name":"same"}]}]}}`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Len(t, rs.Identity.TrustedIDPs, 2)
	data, err := json.Marshal(rs.Identity.TrustedIDPs[0])
	require.NoError(t, err)
	require.False(t, strings.Contains(string(data), `"jwks_uri"`), "omission must survive serialization")
}

func TestTrustedIDPPublishedExampleLoads(t *testing.T) {
	t.Setenv("IDENTITY_WORKFORCE_ISSUER", "https://issuer.example.test/auth")
	rs, err := New().LoadFile("../../../docs/examples/declarative/identity/trusted-idps.yaml")
	require.NoError(t, err)
	require.Len(t, rs.Identity.TrustedIDPs, 1)
	require.True(t, rs.Identity.TrustedIDPs[0].JwksURIDeclared())
	require.Nil(t, rs.Identity.TrustedIDPs[0].JwksURI)
}

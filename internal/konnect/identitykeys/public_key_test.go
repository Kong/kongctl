package identitykeys

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The expected kids were computed independently and matched direct API reads.
func TestVaultPublicKeyIdentityAndEquivalentPEM(t *testing.T) {
	data, err := os.ReadFile("testdata/public-keys.json")
	require.NoError(t, err)
	var fixtures []struct {
		Label     string `json:"label"`
		PublicKey string `json:"public_key"`
		Alternate string `json:"alternate"`
		Kid       string `json:"kid"`
		Algorithm string `json:"algorithm"`
		Bits      int    `json:"bits"`
	}
	require.NoError(t, json.Unmarshal(data, &fixtures))
	require.Len(t, fixtures, 7)
	for _, fixture := range fixtures {
		t.Run(fixture.Label, func(t *testing.T) {
			if fixture.Bits == 1024 {
				_, err := ParsePublicKey(fixture.PublicKey)
				require.ErrorContains(t, err, "at least 2048")
				return
			}
			for _, source := range []string{
				fixture.PublicKey, strings.ReplaceAll(fixture.PublicKey, "\n", "\r\n"),
				" \n" + fixture.PublicKey + " \n",
			} {
				key, err := ParsePublicKey(source)
				require.NoError(t, err)
				require.Equal(t, fixture.Kid, key.Kid)
				require.Equal(t, fixture.Algorithm, key.Algorithm)
				require.Equal(t, fixture.Bits, key.Bits)
				require.Equal(t, fixture.PublicKey, key.PEM)
			}
			if fixture.Alternate != "" {
				_, err := ParsePublicKey(fixture.Alternate)
				require.Error(t, err, "PKCS#1 has not been accepted by the native API")
			}
		})
	}
	valid := fixtures[0].PublicKey
	for _, source := range []string{
		"", "invalid", "prefix\n" + valid, valid + "trailing", valid + valid,
		strings.Replace(valid, "BEGIN PUBLIC KEY", "BEGIN PRIVATE KEY", 1),
		strings.Replace(valid, "-----BEGIN PUBLIC KEY-----\n", "-----BEGIN PUBLIC KEY-----\nHeader: value\n\n", 1),
		"-----BEGIN PUBLIC KEY-----\ninvalid\n-----END PUBLIC KEY-----", strings.Repeat("x", 8193),
	} {
		_, err := ParsePublicKey(source)
		require.Error(t, err)
	}
	_, err = ParsePublicKey("-----BEGIN PRIVATE KEY-----\nsensitive-sentinel\n-----END PRIVATE KEY-----")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "sensitive-sentinel")
}

func TestVaultPublicKeyRejectsOtherPublicAlgorithms(t *testing.T) {
	key, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	x, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)
	for _, key := range []any{key, x.PublicKey()} {
		der, err := x509.MarshalPKIXPublicKey(key)
		require.NoError(t, err)
		_, err = ParsePublicKey(string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})))
		require.ErrorContains(t, err, "RSA or EC")
	}
}

func TestVaultKeyExplicitNameConstraints(t *testing.T) {
	for _, name := range []string{"a", "RSA_2048-key", strings.Repeat("a", 255)} {
		require.NoError(t, ValidateName(name))
	}
	for _, name := range []string{"", "has space", "has.dot", "é", strings.Repeat("a", 256)} {
		require.Error(t, ValidateName(name))
	}
}

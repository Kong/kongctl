// Package identitykeys validates Token Vault public configuration and derives
// its stable API identity without retaining or accepting private key material.
package identitykeys

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
)

type PublicKey struct {
	PEM       string
	Kid       string
	Algorithm string
	Bits      int
}

func ValidateName(name string) error {
	if len(name) == 0 || len(name) > 255 {
		return fmt.Errorf("vault key name must contain 1 to 255 ASCII letters, digits, underscores or hyphens")
	}
	for _, c := range name {
		if c != '_' && c != '-' && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return fmt.Errorf("vault key name must contain only ASCII letters, digits, underscores or hyphens")
		}
	}
	return nil
}

// ParsePublicKey accepts the verified PKIX PUBLIC KEY representation, including
// equivalent PEM whitespace. The thumbprint uses only RFC 7638 required JWK
// members, sorted lexically, with fixed-width EC coordinates.
func ParsePublicKey(value string) (PublicKey, error) {
	var result PublicKey
	if len(value) > 8192 {
		return result, fmt.Errorf("vault public_key exceeds 8192 bytes")
	}
	data := bytes.TrimSpace([]byte(value))
	if !bytes.HasPrefix(data, []byte("-----BEGIN PUBLIC KEY-----")) {
		return result, fmt.Errorf("vault public_key must be a PEM-encoded PKIX PUBLIC KEY")
	}
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" || len(block.Headers) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return result, fmt.Errorf("vault public_key must contain exactly one PKIX PUBLIC KEY without PEM headers")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return result, fmt.Errorf("invalid Vault public_key")
	}
	var jwk map[string]string
	switch key := key.(type) {
	case *rsa.PublicKey:
		if key.N.BitLen() < 2048 {
			return result, fmt.Errorf("vault RSA public keys must be at least 2048 bits")
		}
		result.Algorithm, result.Bits = "RSA", key.N.BitLen()
		jwk = map[string]string{
			"kty": "RSA", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}
	case *ecdsa.PublicKey:
		var curve string
		switch key.Curve {
		case elliptic.P256():
			curve = "P-256"
		case elliptic.P384():
			curve = "P-384"
		case elliptic.P521():
			curve = "P-521"
		default:
			return result, fmt.Errorf("vault EC public keys must use P-256, P-384 or P-521")
		}
		result.Algorithm, result.Bits = "EC", key.Curve.Params().BitSize
		width := (result.Bits + 7) / 8
		point, err := key.Bytes()
		if err != nil || len(point) != 1+2*width || point[0] != 4 {
			return result, fmt.Errorf("invalid Vault EC public key encoding")
		}
		jwk = map[string]string{
			"kty": "EC", "crv": curve,
			"x": base64.RawURLEncoding.EncodeToString(point[1 : 1+width]),
			"y": base64.RawURLEncoding.EncodeToString(point[1+width:]),
		}
	default:
		return result, fmt.Errorf("vault public_key must be an RSA or EC public key")
	}
	canonical, err := json.Marshal(jwk)
	if err != nil {
		return PublicKey{}, fmt.Errorf("cannot compute Vault public key identity")
	}
	digest := sha256.Sum256(canonical)
	result.Kid = base64.RawURLEncoding.EncodeToString(digest[:])
	result.PEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: block.Bytes}))
	return result, nil
}

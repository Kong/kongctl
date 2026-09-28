package tags

import (
	"encoding/base64"
	"strings"
)

const literalPlaceholderPrefix = "__KONGCTL_LITERAL__:"

// ProtectReferenceLiteral preserves the distinction between explicit YAML tags
// and ordinary strings that happen to resemble the internal encodings.
func ProtectReferenceLiteral(value string) string {
	if IsRefPlaceholder(value) || IsExternalPlaceholder(value) || strings.HasPrefix(value, literalPlaceholderPrefix) {
		return literalPlaceholderPrefix + base64.RawURLEncoding.EncodeToString([]byte(value))
	}
	return value
}

// ParseReferenceLiteral decodes a string protected during YAML tag processing.
func ParseReferenceLiteral(value string) (string, bool) {
	if !strings.HasPrefix(value, literalPlaceholderPrefix) {
		return "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, literalPlaceholderPrefix))
	return string(decoded), err == nil
}

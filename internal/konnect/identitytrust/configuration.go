package identitytrust

import (
	"fmt"
	"maps"
	"net/url"
	"unicode/utf8"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// NormalizeJIT follows the verified empty-mapping contract without changing
// explicit claim names. It never mutates the supplied mapping. The SDK
// default-fills an omitted identifier claim to "sub"; that default and an
// omitted identifier are treated as equivalent so request/response diffing
// is not tripped by the server filling in its documented default.
func NormalizeJIT(value *kkComps.TrustedIdpJit) *kkComps.TrustedIdpJit {
	if value == nil || value.Mapping == nil {
		return nil
	}
	mapping := *value.Mapping
	if len(mapping.Metadata) == 0 {
		mapping.Metadata = nil
	} else {
		mapping.Metadata = maps.Clone(mapping.Metadata)
	}
	if mapping.Identifier != nil && *mapping.Identifier == "sub" {
		mapping.Identifier = nil
	}
	if mapping.Metadata == nil && mapping.Email == nil && mapping.Groups == nil &&
		mapping.DisplayName == nil && mapping.Identifier == nil {
		return nil
	}
	return &kkComps.TrustedIdpJit{Mapping: &mapping}
}

func ValidatePublic(issuer string, display, jwks *string, jit *kkComps.TrustedIdpJit) error {
	parsed, err := url.Parse(issuer)
	if err != nil || !parsed.IsAbs() {
		return fmt.Errorf("trusted IdP issuer_url must be an absolute URI")
	}
	if display != nil && utf8.RuneCountInString(*display) > 255 {
		return fmt.Errorf("trusted IdP display_name must be at most 255 characters")
	}
	if jwks != nil {
		parsed, err := url.Parse(*jwks)
		if err != nil || !parsed.IsAbs() {
			return fmt.Errorf("trusted IdP jwks_uri must be an absolute URI")
		}
	}
	if jit != nil && jit.Mapping != nil && len(jit.Mapping.Metadata) > 64 {
		return fmt.Errorf("trusted IdP JIT metadata mapping allows at most 64 entries")
	}
	return nil
}

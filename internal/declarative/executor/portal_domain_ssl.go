package executor

import (
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// buildCreatePortalCustomDomainSSL converts planner/executor field maps into the SDK union type.
func buildCreatePortalCustomDomainSSL(data map[string]any) (kkComps.CreatePortalCustomDomainSSL, bool, error) {
	methodRaw, ok := data["domain_verification_method"].(string)
	if !ok || strings.TrimSpace(methodRaw) == "" {
		return kkComps.CreatePortalCustomDomainSSL{}, false, nil
	}

	method := strings.ToLower(strings.TrimSpace(methodRaw))

	switch method {
	case "http":
		ssl := kkComps.CreateCreatePortalCustomDomainSSLHTTP(kkComps.HTTP{})
		return ssl, true, nil
	case "custom_certificate":
		certValue, _ := data["custom_certificate"].(string)
		keyValue, _ := data["custom_private_key"].(string)
		if strings.TrimSpace(certValue) == "" || strings.TrimSpace(keyValue) == "" {
			return kkComps.CreatePortalCustomDomainSSL{}, false,
				fmt.Errorf("custom_certificate and custom_private_key are required when domain_verification_method is custom_certificate")
		}

		cert := kkComps.CustomCertificate{
			CustomCertificate: certValue,
			CustomPrivateKey:  keyValue,
		}
		if skip, ok := data["skip_ca_check"].(bool); ok {
			cert.SkipCaCheck = &skip
		}

		ssl := kkComps.CreateCreatePortalCustomDomainSSLCustomCertificate(cert)
		return ssl, true, nil
	default:
		return kkComps.CreatePortalCustomDomainSSL{}, false,
			fmt.Errorf("unsupported domain_verification_method: %s", methodRaw)
	}
}

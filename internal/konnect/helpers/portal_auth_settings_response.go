package helpers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
)

// HydratePortalAuthSettingsOIDCConfig restores oidc_config from the raw API
// payload when SDK unmarshalling omits it.
func HydratePortalAuthSettingsOIDCConfig(
	settings *kkComponents.PortalAuthenticationSettingsResponse,
	rawResponse *http.Response,
) error {
	if settings == nil || settings.OidcConfig != nil || rawResponse == nil || rawResponse.Body == nil {
		return nil
	}

	body, err := io.ReadAll(rawResponse.Body)
	if err != nil {
		return fmt.Errorf("failed reading portal auth settings response body: %w", err)
	}
	rawResponse.Body = io.NopCloser(bytes.NewBuffer(body))

	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}

	var raw struct {
		OidcConfig json.RawMessage `json:"oidc_config"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("failed parsing portal auth settings response body: %w", err)
	}

	if len(raw.OidcConfig) > 0 && string(raw.OidcConfig) != "null" {
		if err := json.Unmarshal(raw.OidcConfig, &settings.OidcConfig); err != nil {
			return fmt.Errorf("failed parsing portal auth settings oidc_config: %w", err)
		}
	}

	return nil
}

package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeApplicationAuthStrategy,
		func(rs *ResourceSet) *[]ApplicationAuthStrategyResource { return &rs.ApplicationAuthStrategies },
		AutoExplain[ApplicationAuthStrategyResource](),
	)
}

// ApplicationAuthStrategyResource represents an application auth strategy in declarative configuration
type ApplicationAuthStrategyResource struct {
	BaseResource
	kkComps.CreateAppAuthStrategyRequest `yaml:",inline" json:",inline"`
}

// GetType returns the resource type
func (a ApplicationAuthStrategyResource) GetType() ResourceType {
	return ResourceTypeApplicationAuthStrategy
}

// GetDependencies returns references to other resources this auth strategy depends on
func (a ApplicationAuthStrategyResource) GetDependencies() []ResourceRef {
	// Auth strategies don't depend on other resources
	return []ResourceRef{}
}

// GetLabels returns the labels for this resource
func (a ApplicationAuthStrategyResource) GetLabels() map[string]string {
	switch a.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if a.AppAuthStrategyKeyAuthRequest != nil {
			return a.AppAuthStrategyKeyAuthRequest.Labels
		}
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if a.AppAuthStrategyOpenIDConnectRequest != nil {
			return a.AppAuthStrategyOpenIDConnectRequest.Labels
		}
	}
	return nil
}

// SetLabels sets the labels for this resource
func (a *ApplicationAuthStrategyResource) SetLabels(labels map[string]string) {
	switch a.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if a.AppAuthStrategyKeyAuthRequest != nil {
			a.AppAuthStrategyKeyAuthRequest.Labels = labels
		}
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if a.AppAuthStrategyOpenIDConnectRequest != nil {
			a.AppAuthStrategyOpenIDConnectRequest.Labels = labels
		}
	}
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (a ApplicationAuthStrategyResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{} // No outbound references
}

// Validate ensures the application auth strategy resource is valid
func (a ApplicationAuthStrategyResource) Validate() error {
	if err := ValidateRef(a.Ref); err != nil {
		return fmt.Errorf("invalid application auth strategy ref: %w", err)
	}
	return nil
}

// SetDefaults applies default values to application auth strategy resource
func (a *ApplicationAuthStrategyResource) SetDefaults() {
	// No defaults to set for auth strategies
}

// GetMoniker returns the moniker (name) of the auth strategy from the union type
func (a ApplicationAuthStrategyResource) GetMoniker() string {
	switch a.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if a.AppAuthStrategyKeyAuthRequest != nil {
			return a.AppAuthStrategyKeyAuthRequest.Name
		}
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if a.AppAuthStrategyOpenIDConnectRequest != nil {
			return a.AppAuthStrategyOpenIDConnectRequest.Name
		}
	}
	return ""
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (a ApplicationAuthStrategyResource) GetKonnectMonikerFilter() string {
	return a.BaseResource.GetKonnectMonikerFilter(a.GetMoniker())
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (a *ApplicationAuthStrategyResource) TryMatchKonnectResource(konnectResource any) bool {
	return a.TryMatchByName(a.GetMoniker(), konnectResource, matchOptions{})
}

// UnmarshalJSON implements custom JSON unmarshaling to handle SDK union types
// (sigs.k8s.io/yaml uses JSON unmarshaling internally)
func (a *ApplicationAuthStrategyResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields
	var temp struct {
		Ref          string            `json:"ref"`
		Name         string            `json:"name"`
		DisplayName  string            `json:"display_name"`
		StrategyType string            `json:"strategy_type"`
		Configs      map[string]any    `json:"configs"`
		Labels       map[string]string `json:"labels,omitempty"`
		Kongctl      *KongctlMeta      `json:"kongctl,omitempty"`
	}

	// Use a decoder with DisallowUnknownFields to catch typos
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&temp); err != nil {
		return err
	}

	// Set our fields
	a.Ref = temp.Ref
	a.Kongctl = temp.Kongctl

	// Based on strategy_type, create the appropriate SDK union type
	switch temp.StrategyType {
	case "openid_connect":
		// Create OpenID Connect request
		var oidcConfig kkComps.AppAuthStrategyConfigOpenIDConnect
		if configData, ok := getConfigByKey(temp.Configs, "openid-connect", "openid_connect"); ok {
			configData = normalizeOIDCConfig(configData)
			if err := remarshalConfig(configData, &oidcConfig, "openid-connect"); err != nil {
				return err
			}
		}

		oidcRequest := kkComps.AppAuthStrategyOpenIDConnectRequest{
			Name:         temp.Name,
			DisplayName:  temp.DisplayName,
			StrategyType: kkComps.AppAuthStrategyOpenIDConnectRequestStrategyTypeOpenidConnect,
			Configs: kkComps.AppAuthStrategyOpenIDConnectRequestConfigs{
				OpenidConnect: oidcConfig,
			},
			Labels: temp.Labels,
		}

		a.CreateAppAuthStrategyRequest = kkComps.CreateCreateAppAuthStrategyRequestOpenidConnect(oidcRequest)

	case "key_auth":
		// Create Key Auth request
		var keyAuthConfig kkComps.AppAuthStrategyConfigKeyAuth
		if configData, ok := getConfigByKey(temp.Configs, "key-auth", "key_auth"); ok {
			if err := remarshalConfig(configData, &keyAuthConfig, "key-auth"); err != nil {
				return err
			}
		}

		keyAuthRequest := kkComps.AppAuthStrategyKeyAuthRequest{
			Name:         temp.Name,
			DisplayName:  temp.DisplayName,
			StrategyType: kkComps.StrategyTypeKeyAuth,
			Configs: kkComps.AppAuthStrategyKeyAuthRequestConfigs{
				KeyAuth: keyAuthConfig,
			},
			Labels: temp.Labels,
		}

		a.CreateAppAuthStrategyRequest = kkComps.CreateCreateAppAuthStrategyRequestKeyAuth(keyAuthRequest)

	default:
		return fmt.Errorf("unsupported strategy_type: %s", temp.StrategyType)
	}

	return nil
}

// remarshalConfig converts config data through a JSON round-trip to populate typed structs.
// This ensures any map[string]any config is properly converted to the target type.
func remarshalConfig(configData any, target any, configType string) error {
	configBytes, err := json.Marshal(configData)
	if err != nil {
		return fmt.Errorf("failed to marshal %s config: %w", configType, err)
	}
	if err := json.Unmarshal(configBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal %s config: %w", configType, err)
	}
	return nil
}

// MarshalJSON ensures the ref field is always included alongside the union payload
func (a ApplicationAuthStrategyResource) MarshalJSON() ([]byte, error) {
	// Marshal the union portion to capture the strategy-specific fields
	unionBytes, err := json.Marshal(a.CreateAppAuthStrategyRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal application auth strategy union: %w", err)
	}

	payload := map[string]any{}
	if err := json.Unmarshal(unionBytes, &payload); err != nil {
		return nil, fmt.Errorf("failed to build application auth strategy payload: %w", err)
	}

	payload["ref"] = a.Ref
	if a.Kongctl != nil {
		payload["kongctl"] = a.Kongctl
	}

	return json.Marshal(payload)
}

// getConfigByKey returns the first matching config entry for canonical key names.
// This allows declarative YAML to accept both underscore and hyphen variants.
func getConfigByKey(configs map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if config, ok := configs[key]; ok {
			return config, true
		}
	}
	return nil, false
}

// normalizeOIDCConfig ensures OIDC config values are compatible with SDK validation.
// The SDK currently requires credential_claim and auth_methods even when unset.
func normalizeOIDCConfig(config any) any {
	configMap, ok := config.(map[string]any)
	if !ok {
		return config
	}

	normalized := maps.Clone(configMap)
	if _, ok := normalized["credential_claim"]; !ok {
		normalized["credential_claim"] = []string(nil)
	}
	if _, ok := normalized["auth_methods"]; !ok {
		normalized["auth_methods"] = []string(nil)
	}

	return normalized
}

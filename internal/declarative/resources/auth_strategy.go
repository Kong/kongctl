package resources

import (
	"bytes"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

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
		if configData, ok := temp.Configs["openid-connect"]; ok {
			configBytes, err := json.Marshal(configData)
			if err != nil {
				return fmt.Errorf("failed to marshal openid-connect config: %w", err)
			}
			if err := json.Unmarshal(configBytes, &oidcConfig); err != nil {
				return fmt.Errorf("failed to unmarshal openid-connect config: %w", err)
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
		if configData, ok := temp.Configs["key-auth"]; ok {
			configBytes, err := json.Marshal(configData)
			if err != nil {
				return fmt.Errorf("failed to marshal key-auth config: %w", err)
			}
			if err := json.Unmarshal(configBytes, &keyAuthConfig); err != nil {
				return fmt.Errorf("failed to unmarshal key-auth config: %w", err)
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

package resources

import (
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// ApplicationAuthStrategyResource represents an application auth strategy in declarative configuration
type ApplicationAuthStrategyResource struct {
	kkComps.CreateAppAuthStrategyRequest `yaml:",inline" json:",inline"`
	Ref     string       `yaml:"ref" json:"ref"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// GetKind returns the resource kind
func (a ApplicationAuthStrategyResource) GetKind() string {
	return "application_auth_strategy"
}

// GetRef returns the reference identifier used for cross-resource references
func (a ApplicationAuthStrategyResource) GetRef() string {
	return a.Ref
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
	if a.Ref == "" {
		return fmt.Errorf("application auth strategy ref is required")
	}
	return nil
}

// SetDefaults applies default values to application auth strategy resource
func (a *ApplicationAuthStrategyResource) SetDefaults() {
	// No defaults to set for auth strategies
}

// GetName returns the name of the auth strategy from the union type
func (a ApplicationAuthStrategyResource) GetName() string {
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

// UnmarshalJSON implements custom JSON unmarshaling to handle SDK union types
// (sigs.k8s.io/yaml uses JSON unmarshaling internally)
func (a *ApplicationAuthStrategyResource) UnmarshalJSON(data []byte) error {
	// Temporary struct to capture all fields
	var temp struct {
		Ref          string                 `json:"ref"`
		Name         string                 `json:"name"`
		DisplayName  string                 `json:"display_name"`
		StrategyType string                 `json:"strategy_type"`
		Configs      map[string]interface{} `json:"configs"`
		Labels       map[string]string      `json:"labels,omitempty"`
		Kongctl      *KongctlMeta           `json:"kongctl,omitempty"`
	}
	
	if err := json.Unmarshal(data, &temp); err != nil {
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
		if configData, ok := temp.Configs["openid_connect"]; ok {
			configBytes, err := json.Marshal(configData)
			if err != nil {
				return fmt.Errorf("failed to marshal openid_connect config: %w", err)
			}
			if err := json.Unmarshal(configBytes, &oidcConfig); err != nil {
				return fmt.Errorf("failed to unmarshal openid_connect config: %w", err)
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
		if configData, ok := temp.Configs["key_auth"]; ok {
			configBytes, err := json.Marshal(configData)
			if err != nil {
				return fmt.Errorf("failed to marshal key_auth config: %w", err)
			}
			if err := json.Unmarshal(configBytes, &keyAuthConfig); err != nil {
				return fmt.Errorf("failed to unmarshal key_auth config: %w", err)
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


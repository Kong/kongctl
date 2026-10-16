package fieldcontracts

import (
	"fmt"
	"slices"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const (
	authConfigKeyAuth = "key-auth"
	authConfigOIDC    = "openid-connect"
	authConfigIssuer  = "issuer"
)

type authConfigListField[C, U any] struct {
	name          string
	create        func(*C) *[]string
	update        func(*U) *[]string
	createDefault []string
}

type authConfigListFields[C, U any] []authConfigListField[C, U]

var keyAuthConfigLists = authConfigListFields[
	kkComps.AppAuthStrategyConfigKeyAuth,
	kkComps.PartialAppAuthStrategyConfigKeyAuth,
]{
	{
		"key_names",
		func(c *kkComps.AppAuthStrategyConfigKeyAuth) *[]string { return &c.KeyNames },
		func(c *kkComps.PartialAppAuthStrategyConfigKeyAuth) *[]string { return &c.KeyNames }, nil,
	},
}

var oidcConfigLists = authConfigListFields[
	kkComps.AppAuthStrategyConfigOpenIDConnect,
	kkComps.PartialAppAuthStrategyConfigOpenIDConnect,
]{
	{
		"credential_claim",
		func(c *kkComps.AppAuthStrategyConfigOpenIDConnect) *[]string { return &c.CredentialClaim },
		func(c *kkComps.PartialAppAuthStrategyConfigOpenIDConnect) *[]string { return &c.CredentialClaim },
		[]string{"sub"},
	},
	{
		"scopes",
		func(c *kkComps.AppAuthStrategyConfigOpenIDConnect) *[]string { return &c.Scopes },
		func(c *kkComps.PartialAppAuthStrategyConfigOpenIDConnect) *[]string { return &c.Scopes }, nil,
	},
	{
		"auth_methods",
		func(c *kkComps.AppAuthStrategyConfigOpenIDConnect) *[]string { return &c.AuthMethods },
		func(c *kkComps.PartialAppAuthStrategyConfigOpenIDConnect) *[]string { return &c.AuthMethods }, nil,
	},
}

func (fields authConfigListFields[C, U]) extract(config *C) map[string]any {
	result := make(map[string]any)
	for _, field := range fields {
		if value := *field.create(config); value != nil {
			result[field.name] = value
		}
	}
	return result
}

func (fields authConfigListFields[C, U]) compare(config *C, current map[string]any) (map[string]any, map[string]any) {
	old, updated := make(map[string]any), make(map[string]any)
	for _, field := range fields {
		desired := *field.create(config)
		if desired == nil {
			continue
		}
		observed := authConfigStrings(current[field.name], []string{})
		if !slices.Equal(observed, desired) {
			old[field.name], updated[field.name] = observed, desired
		}
	}
	return old, updated
}

func (fields authConfigListFields[C, U]) mapCreate(config map[string]any, target *C) {
	for _, field := range fields {
		*field.create(target) = authConfigStrings(config[field.name], slices.Clone(field.createDefault))
	}
}

func (fields authConfigListFields[C, U]) mapUpdate(config map[string]any, target *U) {
	for _, field := range fields {
		*field.update(target) = authConfigStrings(config[field.name], nil)
	}
}

// Typed slices preserve presence. Decoded arrays filter nonstrings; if no string
// remains, each stage supplies its own fallback (not a universal empty slice).
func authConfigStrings(raw any, fallback []string) []string {
	switch value := raw.(type) {
	case []string:
		return value
	case []any:
		result := make([]string, 0, len(value))
		for _, item := range value {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return fallback
}

// ExtractAuthStrategyConfigs preserves declaration presence and the nested plan
// shape. Identity, labels, strategy type and DCR references belong to callers.
func ExtractAuthStrategyConfigs(desired kkComps.CreateAppAuthStrategyRequest) map[string]any {
	switch desired.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if req := desired.AppAuthStrategyKeyAuthRequest; req != nil {
			return map[string]any{authConfigKeyAuth: keyAuthConfigLists.extract(&req.Configs.KeyAuth)}
		}
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if req := desired.AppAuthStrategyOpenIDConnectRequest; req != nil {
			config := oidcConfigLists.extract(&req.Configs.OpenidConnect)
			if issuer := req.Configs.OpenidConnect.Issuer; issuer != "" {
				config[authConfigIssuer] = issuer
			}
			return map[string]any{authConfigOIDC: config}
		}
	}
	return nil
}

// CompareAuthStrategyConfigs returns only changed config fields in the existing
// nested diagnostic shape. Nil results mean no change; list order matters.
func CompareAuthStrategyConfigs(
	desired kkComps.CreateAppAuthStrategyRequest, current map[string]any,
) (old, updated map[string]any) {
	var key string
	switch desired.Type {
	case kkComps.CreateAppAuthStrategyRequestTypeKeyAuth:
		if req := desired.AppAuthStrategyKeyAuthRequest; req != nil {
			key = authConfigKeyAuth
			observed, _ := current[key].(map[string]any)
			old, updated = keyAuthConfigLists.compare(&req.Configs.KeyAuth, observed)
		}
	case kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect:
		if req := desired.AppAuthStrategyOpenIDConnectRequest; req != nil {
			key = authConfigOIDC
			observed, _ := current[key].(map[string]any)
			old, updated = oidcConfigLists.compare(&req.Configs.OpenidConnect, observed)
			issuer, _ := observed[authConfigIssuer].(string)
			if value := req.Configs.OpenidConnect.Issuer; value != "" && value != issuer {
				old[authConfigIssuer], updated[authConfigIssuer] = issuer, value
			}
		}
	}
	if len(updated) == 0 {
		return nil, nil
	}
	return map[string]any{key: old}, map[string]any{key: updated}
}

func authCreateConfig(raw any, key, strategyType string) (map[string]any, error) {
	configs, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configs is required for %s strategy", strategyType)
	}
	config, ok := configs[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configs.%s is required for %s strategy", key, strategyType)
	}
	return config, nil
}

// MapKeyAuthConfigCreate retains the saved-plan config shape checks and errors.
func MapKeyAuthConfigCreate(raw any) (kkComps.AppAuthStrategyConfigKeyAuth, error) {
	var result kkComps.AppAuthStrategyConfigKeyAuth
	config, err := authCreateConfig(raw, authConfigKeyAuth, "key_auth")
	if err != nil {
		return result, err
	}
	keyAuthConfigLists.mapCreate(config, &result)
	return result, nil
}

// MapOIDCConfigCreate retains issuer validation and the create-only claim fallback.
func MapOIDCConfigCreate(raw any) (kkComps.AppAuthStrategyConfigOpenIDConnect, error) {
	var result kkComps.AppAuthStrategyConfigOpenIDConnect
	config, err := authCreateConfig(raw, authConfigOIDC, "openid_connect")
	if err != nil {
		return result, err
	}
	issuer, ok := config[authConfigIssuer].(string)
	if !ok {
		return result, fmt.Errorf("issuer is required for openid_connect strategy")
	}
	result.Issuer = issuer
	oidcConfigLists.mapCreate(config, &result)
	return result, nil
}

// MapAuthStrategyConfigUpdate constructs the exact partial SDK union, without
// applying create defaults or broadening the accepted saved-plan shapes.
func MapAuthStrategyConfigUpdate(strategyType string, configs map[string]any) (*kkComps.Configs, error) {
	switch strategyType {
	case "key_auth":
		config, ok := configs[authConfigKeyAuth].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("key-auth config missing for key_auth strategy")
		}
		var keyAuth kkComps.PartialAppAuthStrategyConfigKeyAuth
		keyAuthConfigLists.mapUpdate(config, &keyAuth)
		result := kkComps.CreateConfigsUpdateAppAuthStrategyRequestKeyAuth(
			kkComps.UpdateAppAuthStrategyRequestKeyAuth{KeyAuth: keyAuth},
		)
		return &result, nil
	case "openid_connect":
		config, ok := configs[authConfigOIDC].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("openid-connect config missing for openid_connect strategy")
		}
		var oidc kkComps.PartialAppAuthStrategyConfigOpenIDConnect
		if issuer, ok := config[authConfigIssuer].(string); ok {
			oidc.Issuer = &issuer
		}
		oidcConfigLists.mapUpdate(config, &oidc)
		result := kkComps.CreateConfigsUpdateAppAuthStrategyRequestOpenIDConnect(
			kkComps.UpdateAppAuthStrategyRequestOpenIDConnect{OpenidConnect: oidc},
		)
		return &result, nil
	default:
		return nil, fmt.Errorf("unsupported strategy type: %s", strategyType)
	}
}

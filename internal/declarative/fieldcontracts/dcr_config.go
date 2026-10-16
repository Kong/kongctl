package fieldcontracts

import (
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// DCR config policy is provider-specific: response observability is narrower
// than the sparse update shape. Create decoding and raw export stay separate.
type dcrConfigPolicy interface {
	validate(map[string]any) error
	changed(current, desired map[string]any) bool
	update(map[string]any) *kkComps.DcrConfig
}

type dcrConfigField[T any] struct {
	name       string
	observable bool
	boolean    bool
	assign     func(*T, any)
}

type dcrConfigFields[T any] struct {
	fields []dcrConfigField[T]
	wrap   func(T) kkComps.DcrConfig
}

func dcrStringField[T any](name string, observable bool, target func(*T) **string) dcrConfigField[T] {
	return dcrConfigField[T]{
		name: name, observable: observable,
		assign: func(req *T, raw any) {
			if value, ok := raw.(string); ok {
				*target(req) = &value
			}
		},
	}
}

// These observable booleans accept an omitted response as false only for
// comparison. Validation and sparse mapping never inject a default.
func dcrBoolField[T any](name string, target func(*T) **bool) dcrConfigField[T] {
	return dcrConfigField[T]{
		name: name, observable: true, boolean: true,
		assign: func(req *T, raw any) {
			if value, ok := raw.(bool); ok {
				*target(req) = &value
			}
		},
	}
}

var dcrConfigPolicies = map[string]dcrConfigPolicy{
	"auth0": dcrConfigFields[kkComps.UpdateDcrConfigAuth0InRequest]{
		fields: []dcrConfigField[kkComps.UpdateDcrConfigAuth0InRequest]{
			dcrStringField("initial_client_id", true,
				func(r *kkComps.UpdateDcrConfigAuth0InRequest) **string { return &r.InitialClientID }),
			dcrStringField("initial_client_secret", false,
				func(r *kkComps.UpdateDcrConfigAuth0InRequest) **string { return &r.InitialClientSecret }),
			dcrStringField("initial_client_audience", true,
				func(r *kkComps.UpdateDcrConfigAuth0InRequest) **string { return &r.InitialClientAudience }),
			dcrBoolField("use_developer_managed_scopes",
				func(r *kkComps.UpdateDcrConfigAuth0InRequest) **bool { return &r.UseDeveloperManagedScopes }),
		},
		wrap: kkComps.CreateDcrConfigUpdateDcrConfigAuth0InRequest,
	},
	"azureAd": dcrConfigFields[kkComps.UpdateDcrConfigAzureAdInRequest]{
		fields: []dcrConfigField[kkComps.UpdateDcrConfigAzureAdInRequest]{
			dcrStringField("initial_client_id", true,
				func(r *kkComps.UpdateDcrConfigAzureAdInRequest) **string { return &r.InitialClientID }),
			dcrStringField("initial_client_secret", false,
				func(r *kkComps.UpdateDcrConfigAzureAdInRequest) **string { return &r.InitialClientSecret }),
		},
		wrap: kkComps.CreateDcrConfigUpdateDcrConfigAzureAdInRequest,
	},
	"curity": dcrConfigFields[kkComps.UpdateDcrConfigCurityInRequest]{
		fields: []dcrConfigField[kkComps.UpdateDcrConfigCurityInRequest]{
			dcrStringField("initial_client_id", true,
				func(r *kkComps.UpdateDcrConfigCurityInRequest) **string { return &r.InitialClientID }),
			dcrStringField("initial_client_secret", false,
				func(r *kkComps.UpdateDcrConfigCurityInRequest) **string { return &r.InitialClientSecret }),
		},
		wrap: kkComps.CreateDcrConfigUpdateDcrConfigCurityInRequest,
	},
	"okta": dcrConfigFields[kkComps.UpdateDcrConfigOktaInRequest]{
		fields: []dcrConfigField[kkComps.UpdateDcrConfigOktaInRequest]{
			dcrStringField("dcr_token", false,
				func(r *kkComps.UpdateDcrConfigOktaInRequest) **string { return &r.DcrToken }),
		},
		wrap: kkComps.CreateDcrConfigUpdateDcrConfigOktaInRequest,
	},
	"http": dcrConfigFields[kkComps.UpdateDcrConfigHTTPInRequest]{
		fields: []dcrConfigField[kkComps.UpdateDcrConfigHTTPInRequest]{
			dcrStringField("dcr_base_url", true,
				func(r *kkComps.UpdateDcrConfigHTTPInRequest) **string { return &r.DcrBaseURL }),
			dcrStringField("api_key", false,
				func(r *kkComps.UpdateDcrConfigHTTPInRequest) **string { return &r.APIKey }),
			dcrBoolField("disable_event_hooks",
				func(r *kkComps.UpdateDcrConfigHTTPInRequest) **bool { return &r.DisableEventHooks }),
			dcrBoolField("disable_refresh_secret",
				func(r *kkComps.UpdateDcrConfigHTTPInRequest) **bool { return &r.DisableRefreshSecret }),
			dcrBoolField("allow_multiple_credentials",
				func(r *kkComps.UpdateDcrConfigHTTPInRequest) **bool { return &r.AllowMultipleCredentials }),
		},
		wrap: kkComps.CreateDcrConfigUpdateDcrConfigHTTPInRequest,
	},
}

// ValidateDCRConfig checks declared boolean values in deterministic field order.
// Unknown providers and other fields retain their existing validation elsewhere.
func ValidateDCRConfig(providerType string, config map[string]any) error {
	if policy, ok := dcrConfigPolicies[providerType]; ok {
		return policy.validate(config)
	}
	return nil
}

func (p dcrConfigFields[T]) validate(config map[string]any) error {
	for _, field := range p.fields {
		if !field.boolean {
			continue
		}
		value := config[field.name]
		if value == nil {
			continue
		}
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("dcr_config.%s must be a boolean", field.name)
		}
	}
	return nil
}

// DCRConfigChanged compares only declared observable fields. Secrets are selected
// by the existing write-only intent path, never by observed value comparison.
func DCRConfigChanged(providerType string, current, desired map[string]any) bool {
	if policy, ok := dcrConfigPolicies[providerType]; ok {
		return policy.changed(current, desired)
	}
	return false
}

func (p dcrConfigFields[T]) changed(current, desired map[string]any) bool {
	for _, field := range p.fields {
		if !field.observable {
			continue
		}
		value, declared := desired[field.name]
		if !declared {
			continue
		}
		old, observed := current[field.name]
		if !observed {
			if boolean, ok := value.(bool); field.boolean && ok && !boolean {
				continue
			}
			return true
		}
		if !reflect.DeepEqual(old, value) {
			return true
		}
	}
	return false
}

// MapDCRConfigUpdate preserves type filtering, PATCH sparsity, union selection
// and the existing object-before-provider error precedence for saved plans.
func MapDCRConfigUpdate(providerType string, raw any) (*kkComps.DcrConfig, error) {
	config, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("DCR config must be an object")
	}
	policy, ok := dcrConfigPolicies[providerType]
	if !ok {
		return nil, fmt.Errorf("unsupported DCR provider type: %s", providerType)
	}
	return policy.update(config), nil
}

func (p dcrConfigFields[T]) update(config map[string]any) *kkComps.DcrConfig {
	var request T
	for _, field := range p.fields {
		field.assign(&request, config[field.name])
	}
	result := p.wrap(request)
	return &result
}

package resources

import (
	"encoding/json"
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeDCRProvider,
		func(rs *ResourceSet) *[]DCRProviderResource { return &rs.DCRProviders },
		AutoExplain[DCRProviderResource](),
	)
}

// DCRProviderResource represents a DCR provider in declarative configuration.
type DCRProviderResource struct {
	BaseResource
	Name         string            `yaml:"name,omitempty"          json:"name,omitempty"`
	DisplayName  string            `yaml:"display_name,omitempty"  json:"display_name,omitempty"`
	ProviderType string            `yaml:"provider_type,omitempty" json:"provider_type,omitempty"`
	Issuer       string            `yaml:"issuer,omitempty"        json:"issuer,omitempty"`
	DCRConfig    map[string]any    `yaml:"dcr_config,omitempty"    json:"dcr_config,omitempty"`
	Labels       map[string]string `yaml:"labels,omitempty"     json:"labels,omitempty"`
}

func NormalizeDCRProviderIssuer(issuer string) string {
	issuer = strings.TrimSpace(issuer)
	return strings.TrimSuffix(issuer, "/")
}

func (d DCRProviderResource) GetType() ResourceType {
	return ResourceTypeDCRProvider
}

func (d DCRProviderResource) GetMoniker() string {
	return d.Name
}

func (d DCRProviderResource) GetDependencies() []ResourceRef {
	return []ResourceRef{}
}

func (d DCRProviderResource) GetLabels() map[string]string {
	return d.Labels
}

func (d *DCRProviderResource) SetLabels(labels map[string]string) {
	d.Labels = labels
}

func (d DCRProviderResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{}
}

func (d DCRProviderResource) Validate() error {
	if err := ValidateRef(d.Ref); err != nil {
		return fmt.Errorf("invalid dcr provider ref: %w", err)
	}
	if d.ProviderType == "" {
		return fmt.Errorf("provider_type is required")
	}
	if NormalizeDCRProviderIssuer(d.Issuer) == "" {
		return fmt.Errorf("issuer is required")
	}
	if d.DCRConfig == nil {
		return fmt.Errorf("dcr_config is required")
	}
	return nil
}

func (d *DCRProviderResource) SetDefaults() {
	if d.Name == "" {
		d.Name = d.Ref
	}
}

func (d DCRProviderResource) GetKonnectMonikerFilter() string {
	return d.BaseResource.GetKonnectMonikerFilter(d.Name)
}

func (d *DCRProviderResource) TryMatchKonnectResource(konnectResource any) bool {
	return d.TryMatchByName(d.Name, konnectResource, matchOptions{sdkType: "DcrProviderResponse"})
}

func (d DCRProviderResource) ToCreatePayload() map[string]any {
	payload := map[string]any{
		"name":          d.Name,
		"provider_type": d.ProviderType,
		"issuer":        NormalizeDCRProviderIssuer(d.Issuer),
		"dcr_config":    d.DCRConfig,
	}
	if d.DisplayName != "" {
		payload["display_name"] = d.DisplayName
	}
	if len(d.Labels) > 0 {
		payload["labels"] = d.Labels
	}
	return payload
}

func (d DCRProviderResource) ToUpdatePayload() map[string]any {
	payload := map[string]any{}
	if d.DisplayName != "" {
		payload["display_name"] = d.DisplayName
	}
	if d.Issuer != "" {
		payload["issuer"] = NormalizeDCRProviderIssuer(d.Issuer)
	}
	if d.DCRConfig != nil {
		payload["dcr_config"] = d.DCRConfig
	}
	if d.Labels != nil {
		payload["labels"] = d.Labels
	}
	return payload
}

func (d DCRProviderResource) ToSDKRequest() (kkComps.CreateDcrProviderRequest, error) {
	var req kkComps.CreateDcrProviderRequest
	payload := d.ToCreatePayload()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return req, err
	}
	if err := json.Unmarshal(payloadBytes, &req); err != nil {
		return req, fmt.Errorf("failed to build DCR provider request: %w", err)
	}
	return req, nil
}

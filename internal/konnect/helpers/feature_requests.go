package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"unicode/utf8"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/apiutil"
)

// FeatureUnitCost contains only writable configuration. Resolved LLM pricing
// belongs to responses and must never be reconciled back into a request.
type FeatureUnitCost struct {
	Type              string  `json:"type"                          yaml:"type"`
	Amount            *string `json:"amount,omitempty"              yaml:"amount,omitempty"`
	Provider          *string `json:"provider,omitempty"            yaml:"provider,omitempty"`
	ProviderProperty  *string `json:"provider_property,omitempty"   yaml:"provider_property,omitempty"`
	Model             *string `json:"model,omitempty"               yaml:"model,omitempty"`
	ModelProperty     *string `json:"model_property,omitempty"      yaml:"model_property,omitempty"`
	TokenType         *string `json:"token_type,omitempty"          yaml:"token_type,omitempty"`
	TokenTypeProperty *string `json:"token_type_property,omitempty" yaml:"token_type_property,omitempty"`
}

var featureAmountPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

func (c *FeatureUnitCost) Validate() error {
	if c == nil {
		return nil
	}
	switch c.Type {
	case "manual":
		if c.Amount == nil || !featureAmountPattern.MatchString(*c.Amount) {
			return fmt.Errorf("manual unit_cost requires a decimal string amount in USD")
		}
		if c.Provider != nil || c.ProviderProperty != nil || c.Model != nil ||
			c.ModelProperty != nil || c.TokenType != nil || c.TokenTypeProperty != nil {
			return fmt.Errorf("manual unit_cost accepts only type and amount")
		}
	case "llm":
		if c.Amount != nil {
			return fmt.Errorf("llm unit_cost does not accept amount")
		}
		if c.Provider != nil && c.ProviderProperty != nil || c.Model != nil && c.ModelProperty != nil ||
			c.TokenType != nil && c.TokenTypeProperty != nil {
			return fmt.Errorf("llm unit_cost static values and their property selectors are mutually exclusive")
		}
		if c.TokenType != nil && !slices.Contains([]string{
			"input", "output", "cache_read", "cache_write", "reasoning", "request", "response",
		}, *c.TokenType) {
			return fmt.Errorf("invalid llm unit_cost token_type")
		}
	default:
		return fmt.Errorf("unit_cost type must be manual or llm")
	}
	return nil
}

// FeatureUpdateRequest preserves the PATCH distinction between omission and null.
type FeatureUpdateRequest struct {
	UnitCost    *FeatureUnitCost `json:"unit_cost,omitempty"`
	UnitCostSet bool             `json:"-"`
}

func (r *FeatureUpdateRequest) UnmarshalJSON(data []byte) error {
	type wire struct {
		UnitCost *FeatureUnitCost `json:"unit_cost"`
	}
	var value wire
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("feature PATCH accepts only unit_cost: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	_, present := fields["unit_cost"]
	*r = FeatureUpdateRequest{UnitCost: value.UnitCost, UnitCostSet: present}
	return nil
}

func (r FeatureUpdateRequest) MarshalJSON() ([]byte, error) {
	fields := map[string]any{}
	if r.UnitCostSet || r.UnitCost != nil {
		fields["unit_cost"] = r.UnitCost
	}
	return json.Marshal(fields)
}

func (r FeatureUpdateRequest) Validate() error { return r.UnitCost.Validate() }

func ValidateFeatureCreate(r kkComps.CreateFeatureRequest) error {
	if r.Meter != nil && !IsMeterULID(r.Meter.ID) {
		return fmt.Errorf("feature meter.id must be an uppercase ULID")
	}
	return ValidateFeatureFields(r)
}

// ValidateFeatureFields validates writable fields independently of meter ID
// resolution, which declarative references defer until execution.
func ValidateFeatureFields(r kkComps.CreateFeatureRequest) error {
	if r.Name == "" {
		return fmt.Errorf("feature name is required")
	}
	if r.Key == "" {
		return fmt.Errorf("feature key is required")
	}
	if len(r.Key) > 64 || !meterKeyPattern.MatchString(r.Key) {
		return fmt.Errorf("feature key must be 1-64 lowercase alphanumeric characters separated by single underscores")
	}
	if utf8.RuneCountInString(r.Name) > 256 {
		return fmt.Errorf("feature name must be 1-256 characters")
	}
	if r.Description != nil && utf8.RuneCountInString(*r.Description) > 1024 {
		return fmt.Errorf("feature description must be at most 1024 characters")
	}
	if r.Meter != nil {
		if r.Meter.ID == "" {
			return fmt.Errorf("feature meter.id is required")
		}
		if len(r.Meter.Filters) > 10 {
			return fmt.Errorf("feature meter.filters must contain at most 10 dimensions")
		}
	}
	if err := ValidateMeterLabels(r.Labels); err != nil {
		return fmt.Errorf("feature labels: %w", err)
	}
	if r.UnitCost != nil {
		data, err := json.Marshal(r.UnitCost)
		if err != nil {
			return err
		}
		var cost FeatureUnitCost
		if err := json.Unmarshal(data, &cost); err != nil {
			return err
		}
		return cost.Validate()
	}
	return nil
}

// FeaturesAPIImpl keeps SDK authentication, retries and error decoding while
// repairing the generated SDK's loss of explicit unit_cost: null.
type FeaturesAPIImpl struct {
	kkSDK.OpenMeterFeaturesSDK
	BaseURL     string
	Token       string
	TokenSource apiutil.TokenSource
	HTTPClient  kkSDK.HTTPClient
}

func (a *FeaturesAPIImpl) UpdateFeature(
	ctx context.Context,
	id string,
	req FeatureUpdateRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateFeatureResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var sdkRequest kkComps.UpdateFeatureRequest
	if err := json.Unmarshal(data, &sdkRequest); err != nil {
		return nil, err
	}
	if !req.UnitCostSet || req.UnitCost != nil {
		return a.OpenMeterFeaturesSDK.UpdateFeature(ctx, id, sdkRequest, opts...)
	}
	if a.HTTPClient == nil || a.BaseURL == "" {
		return nil, fmt.Errorf("clearing feature unit_cost requires a configured HTTP client and base URL")
	}
	options := []kkSDK.SDKOption{
		kkSDK.WithServerURL(a.BaseURL),
		kkSDK.WithClient(
			&meteringBillingRequestFieldsClient{client: a.HTTPClient, fields: map[string]any{"unit_cost": nil}},
		),
	}
	if a.TokenSource != nil {
		options = append(options, kkSDK.WithSecuritySource(func(ctx context.Context) (kkComps.Security, error) {
			token, err := a.TokenSource.Token(ctx)
			return kkComps.Security{PersonalAccessToken: &token}, err
		}))
	} else {
		options = append(options, kkSDK.WithSecurity(kkComps.Security{PersonalAccessToken: &a.Token}))
	}
	return kkSDK.New(options...).OpenMeterFeatures.UpdateFeature(ctx, id, sdkRequest, opts...)
}

// FeatureCreateRequest strictly decodes the writable request branches before
// converting to the SDK unions, whose unmarshallers discard unknown fields.
type FeatureCreateRequest struct {
	Name        string                                      `json:"name"`
	Key         string                                      `json:"key"`
	Description *string                                     `json:"description,omitempty"`
	Labels      map[string]string                           `json:"labels,omitempty"`
	Meter       *kkComps.CreateFeatureRequestMeterReference `json:"meter,omitempty"`
	UnitCost    *FeatureUnitCost                            `json:"unit_cost,omitempty"`
}

func (r FeatureCreateRequest) SDKRequest() (kkComps.CreateFeatureRequest, error) {
	if err := r.UnitCost.Validate(); err != nil {
		return kkComps.CreateFeatureRequest{}, err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return kkComps.CreateFeatureRequest{}, err
	}
	var request kkComps.CreateFeatureRequest
	err = json.Unmarshal(data, &request)
	return request, err
}

// FeatureCostConfiguration excludes resolved, read-only LLM pricing.
func FeatureCostConfiguration(cost *kkComps.UnitCost) *FeatureUnitCost {
	if cost == nil {
		return nil
	}
	if manual := cost.BillingFeatureManualUnitCost; manual != nil {
		return &FeatureUnitCost{Type: "manual", Amount: &manual.Amount}
	}
	if llm := cost.BillingFeatureLLMUnitCost; llm != nil {
		result := &FeatureUnitCost{
			Type: "llm", Provider: llm.Provider, ProviderProperty: llm.ProviderProperty,
			Model: llm.Model, ModelProperty: llm.ModelProperty, TokenTypeProperty: llm.TokenTypeProperty,
		}
		if llm.TokenType != nil {
			result.TokenType = new(string(*llm.TokenType))
		}
		return result
	}
	return &FeatureUnitCost{Type: string(cost.Type)}
}

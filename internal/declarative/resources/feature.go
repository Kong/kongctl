package resources

import (
	"bytes"
	"encoding/json"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func init() {
	registerResourceType(
		ResourceTypeFeature,
		func(rs *ResourceSet) *[]FeatureResource { return &rs.Features },
		AutoExplain[FeatureResource](
			WithExplainAliases("features"),
			WithExplainFieldHint("key", ExplainFieldHint{Literal: "requests"}),
			WithExplainFieldHint("meter.id", ExplainFieldHint{
				Description: "Remote meter ULID or local meter ref; immutable after creation.",
				RefKind:     string(ResourceTypeMeter),
			}),
			WithExplainFieldHint("unit_cost.type", ExplainFieldHint{Literal: "manual", Enum: []any{"manual", "llm"}}),
			WithExplainFieldHint("unit_cost.amount", ExplainFieldHint{Literal: "'0.001'", Recommended: new(true)}),

			WithExplainRecommendedFields("ref", "name", "key"),
			WithExplainFieldHint("unit_cost", ExplainFieldHint{
				Description: "Internal cost in USD; omitted leaves unchanged, null clears. Distinct from plan prices.",
			}),
		),
		WithNamespace(140, func(r *FeatureResource) NamespaceParticipant {
			return NamespaceParticipant{Ref: r.Ref, Label: "feature", SupportsProtected: true, Meta: &r.Kongctl}
		}, func(rs *ResourceSet) []FeatureResource {
			if rs.MeteringBilling == nil {
				return nil
			}
			return rs.MeteringBilling.Features
		}),
		WithRootSyncScope(),
		withCollectionValidation(200, featureCollectionValidation),
	)
}

// FeatureResource keeps local references, API keys and remote ULIDs distinct.
type FeatureResource struct {
	BaseResource
	ID          string                                      `yaml:"id,omitempty"          json:"id,omitempty"`
	Name        string                                      `yaml:"name"                  json:"name"`
	Key         string                                      `yaml:"key"                   json:"key"`
	Description *string                                     `yaml:"description,omitempty" json:"description,omitempty"`
	Labels      map[string]string                           `yaml:"labels,omitempty"      json:"labels,omitempty"`
	Meter       *kkComps.CreateFeatureRequestMeterReference `yaml:"meter,omitempty"       json:"meter,omitempty"`
	UnitCost    *helpers.FeatureUnitCost                    `yaml:"unit_cost,omitempty"   json:"unit_cost,omitempty"`
	UnitCostSet bool                                        `yaml:"-"                     json:"-"`
}

func (m FeatureResource) GetType() ResourceType { return ResourceTypeFeature }
func (m FeatureResource) GetMoniker() string    { return m.Key }
func (m FeatureResource) GetDependencies() []ResourceRef {
	if m.Meter != nil && !helpers.IsMeterULID(m.Meter.ID) && !tags.IsRefPlaceholder(m.Meter.ID) {
		return []ResourceRef{{Kind: ResourceTypeMeter, Ref: m.Meter.ID}}
	}
	return nil
}
func (m FeatureResource) GetLabels() map[string]string       { return m.Labels }
func (m *FeatureResource) SetLabels(value map[string]string) { m.Labels = value }
func (m *FeatureResource) SetDefaults()                      {}
func (m FeatureResource) GetKonnectMonikerFilter() string    { return m.Key }
func (m *FeatureResource) TryMatchKonnectResource(value any) bool {
	if m.ID != "" {
		if id := tryMatchByField(value, "ID", m.ID); id != "" {
			m.SetKonnectID(id)
			return true
		}
		return false
	}
	if id := tryMatchByField(value, "Key", m.Key); id != "" {
		m.SetKonnectID(id)
		return true
	}
	return false
}

// UnmarshalJSON tracks null independently of the optional cost value.
func (m *FeatureResource) UnmarshalJSON(data []byte) error {
	type plain FeatureResource
	var decoded plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	_, decoded.UnitCostSet = fields["unit_cost"]
	*m = FeatureResource(decoded)
	return nil
}

func (m FeatureResource) MarshalJSON() ([]byte, error) {
	type plain FeatureResource
	data, err := json.Marshal(plain(m))
	if err != nil {
		return nil, err
	}
	if !m.UnitCostSet || m.UnitCost != nil {
		return data, nil
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	fields["unit_cost"] = nil
	return json.Marshal(fields)
}

func (m FeatureResource) CreateRequest() (kkComps.CreateFeatureRequest, error) {
	return (helpers.FeatureCreateRequest{
		Name: m.Name, Key: m.Key, Description: m.Description,
		Labels: m.Labels, Meter: m.Meter, UnitCost: m.UnitCost,
	}).SDKRequest()
}

func (m FeatureResource) Validate() error {
	if err := ValidateRef(m.Ref); err != nil {
		return fmt.Errorf("invalid feature ref: %w", err)
	}
	if m.ID != "" && !helpers.IsMeterULID(m.ID) {
		return fmt.Errorf("feature id must be a valid uppercase ULID")
	}
	if err := m.UnitCost.Validate(); err != nil {
		return err
	}
	req, err := m.CreateRequest()
	if err != nil {
		return err
	}
	if err := helpers.ValidateFeatureFields(req); err != nil {
		return fmt.Errorf("feature %q: %w", m.Ref, err)
	}

	for key := range m.Labels {
		if labels.IsKongctlLabel(key) {
			return fmt.Errorf(
				"feature %q: management label %q belongs in the kongctl block; labels accept user labels only",
				m.Ref, key,
			)
		}
	}
	namespace := GetNamespace(m.Kongctl)
	return helpers.ValidateMeterLabels(labels.BuildCreateLabels(m.Labels, namespace, true))
}

func featureCollectionValidation(rs *ResourceSet) func(*FeatureResource) error {
	refs, keys, ids := map[string]bool{}, map[string]bool{}, map[string]bool{}
	return func(m *FeatureResource) error {
		if err := validateCollectionResource(rs, m); err != nil {
			return err
		}
		if m.Meter != nil && !helpers.IsMeterULID(m.Meter.ID) && !tags.IsRefPlaceholder(m.Meter.ID) {
			target, ok := rs.GetResourceByRef(m.Meter.ID)
			if !ok || target.GetType() != ResourceTypeMeter {
				return fmt.Errorf(
					"feature %q meter.id must reference a declared meter or a remote uppercase ULID",
					m.Ref,
				)
			}
		}
		if refs[m.Ref] {
			return fmt.Errorf("duplicate feature ref %q", m.Ref)
		}
		if keys[m.Key] {
			return fmt.Errorf("duplicate feature key %q; feature keys are unique across namespaces", m.Key)
		}
		if m.ID != "" && ids[m.ID] {
			return fmt.Errorf("duplicate feature id %q", m.ID)
		}
		refs[m.Ref], keys[m.Key], ids[m.ID] = true, true, true
		return nil
	}
}

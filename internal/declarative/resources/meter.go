package resources

import (
	"fmt"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

func init() {
	registerResourceType(
		ResourceTypeMeter,
		func(rs *ResourceSet) *[]MeterResource { return &rs.Meters },
		AutoExplain[MeterResource](
			WithExplainAliases("meters"),
			WithExplainRecommendedFields("ref", "name", "key", "aggregation", "event_type"),
			WithExplainFieldHint(
				"aggregation",
				ExplainFieldHint{
					Literal: "count",
					Enum:    []any{"sum", "count", "unique_count", "avg", "min", "max", "latest"},
				},
			),
			WithExplainFieldHint("event_type", ExplainFieldHint{Literal: "request"}),
			WithExplainFieldHint(
				"key",
				ExplainFieldHint{
					Literal:     "request_count",
					Description: "Globally unique API key; distinct from the local ref and display name.",
				},
			),
			WithExplainFieldHint(
				"id",
				ExplainFieldHint{Description: "Optional explicit remote ULID. Never derived from ref."},
			),
			WithExplainFieldHint(
				"events_from",
				ExplainFieldHint{Description: "RFC3339 start time; immutable after creation."},
			),
		),
		WithNamespace(130, func(r *MeterResource) NamespaceParticipant {
			return NamespaceParticipant{Ref: r.Ref, Label: "meter", SupportsProtected: true, Meta: &r.Kongctl}
		}),
		WithRootSyncScope(),
		withCollectionValidation(190, meterCollectionValidation),
	)
}

// MeterResource keeps local references, API keys and remote ULIDs distinct.
type MeterResource struct {
	BaseResource
	ID            string              `yaml:"id,omitempty"             json:"id,omitempty"`
	Name          string              `yaml:"name"                     json:"name"`
	Key           string              `yaml:"key"                      json:"key"`
	Aggregation   kkComps.Aggregation `yaml:"aggregation"              json:"aggregation"`
	EventType     string              `yaml:"event_type"               json:"event_type"`
	Description   *string             `yaml:"description,omitempty"    json:"description,omitempty"`
	Labels        map[string]string   `yaml:"labels,omitempty"         json:"labels,omitempty"`
	Dimensions    map[string]string   `yaml:"dimensions,omitempty"     json:"dimensions,omitempty"`
	ValueProperty *string             `yaml:"value_property,omitempty" json:"value_property,omitempty"`
	EventsFrom    *string             `yaml:"events_from,omitempty"    json:"events_from,omitempty"`
}

func (m MeterResource) GetType() ResourceType              { return ResourceTypeMeter }
func (m MeterResource) GetMoniker() string                 { return m.Key }
func (m MeterResource) GetDependencies() []ResourceRef     { return nil }
func (m MeterResource) GetLabels() map[string]string       { return m.Labels }
func (m *MeterResource) SetLabels(value map[string]string) { m.Labels = value }
func (m *MeterResource) SetDefaults()                      {}
func (m MeterResource) GetKonnectMonikerFilter() string    { return m.Key }
func (m *MeterResource) TryMatchKonnectResource(value any) bool {
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

func (m MeterResource) CreateRequest() kkComps.CreateMeterRequest {
	return kkComps.CreateMeterRequest{
		Name: m.Name, Key: m.Key, Aggregation: m.Aggregation, EventType: m.EventType,
		Description: m.Description, Labels: m.Labels, Dimensions: m.Dimensions,
		ValueProperty: m.ValueProperty, EventsFrom: m.eventsFromTime(),
	}
}

func (m MeterResource) eventsFromTime() *time.Time {
	if m.EventsFrom == nil {
		return nil
	}
	value, err := time.Parse(time.RFC3339Nano, *m.EventsFrom)
	if err != nil {
		return nil
	}
	return &value
}

func (m MeterResource) Validate() error {
	if err := ValidateRef(m.Ref); err != nil {
		return fmt.Errorf("invalid meter ref: %w", err)
	}
	if m.ID != "" && !helpers.IsMeterULID(m.ID) {
		return fmt.Errorf("meter id must be a valid uppercase ULID")
	}
	if m.EventsFrom != nil && m.eventsFromTime() == nil {
		return fmt.Errorf("meter events_from must be an RFC3339 timestamp")
	}
	if err := helpers.ValidateMeterCreate(m.CreateRequest()); err != nil {
		return fmt.Errorf("meter %q: %w", m.Ref, err)
	}
	for key := range m.Labels {
		if labels.IsKongctlLabel(key) {
			return fmt.Errorf(
				"meter %q: management label %q belongs in the kongctl block; labels accept user labels only",
				m.Ref, key,
			)
		}
	}
	namespace := GetNamespace(m.Kongctl)
	return helpers.ValidateMeterLabels(labels.BuildCreateLabels(m.Labels, namespace, true))
}

func meterCollectionValidation(rs *ResourceSet) func(*MeterResource) error {
	refs, keys, ids := map[string]bool{}, map[string]bool{}, map[string]bool{}
	return func(m *MeterResource) error {
		if err := validateCollectionResource(rs, m); err != nil {
			return err
		}
		if refs[m.Ref] {
			return fmt.Errorf("duplicate meter ref %q", m.Ref)
		}
		if keys[m.Key] {
			return fmt.Errorf("duplicate meter key %q; meter keys are unique across namespaces", m.Key)
		}
		if m.ID != "" && ids[m.ID] {
			return fmt.Errorf("duplicate meter id %q", m.ID)
		}
		refs[m.Ref], keys[m.Key], ids[m.ID] = true, true, true
		return nil
	}
}

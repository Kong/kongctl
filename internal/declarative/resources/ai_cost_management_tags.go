package resources

import (
	"fmt"
	"regexp"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeAICostManagementTags,
		func(rs *ResourceSet) *[]AICostManagementTagsResource { return &rs.AICostManagementTags },
		AutoExplain[AICostManagementTagsResource](
			WithExplainRecommendedFields("ref", "headers", "prefixes"),
		),
		WithNamespace(55, func(r *AICostManagementTagsResource) NamespaceParticipant {
			return NamespaceParticipant{
				Ref:   r.Ref,
				Label: "AI cost management tags",
				Meta:  &r.Kongctl,
			}
		}),
		WithRootSyncScope(),
		withCollectionValidation(
			85,
			func(rs *ResourceSet) func(*AICostManagementTagsResource) error {
				return func(r *AICostManagementTagsResource) error {
					if len(rs.AICostManagementTags) > 1 {
						return fmt.Errorf(
							"ai_cost_management_tags supports one configuration per region and organization",
						)
					}
					return validateCollectionResource(rs, r)
				}
			},
		),
	)
}

// AICostManagementTagsResource manages the regional organization-wide capture configuration.
type AICostManagementTagsResource struct {
	BaseResource
	Headers   []kkComps.CostTagHeader `json:"headers" yaml:"headers"`
	Prefixes  []kkComps.CostTagPrefix `json:"prefixes" yaml:"prefixes"`
	Labels    map[string]string       `json:"labels,omitempty" yaml:"labels,omitempty"`
	ManagedBy map[string]string       `json:"managed_by,omitempty" yaml:"managed_by,omitempty"`
}

func (r AICostManagementTagsResource) GetType() ResourceType             { return ResourceTypeAICostManagementTags }
func (r AICostManagementTagsResource) GetMoniker() string                { return r.Ref }
func (r AICostManagementTagsResource) GetDependencies() []ResourceRef    { return nil }
func (r AICostManagementTagsResource) GetKonnectMonikerFilter() string   { return "" }
func (r *AICostManagementTagsResource) TryMatchKonnectResource(any) bool { return false }
func (r *AICostManagementTagsResource) SetDefaults()                     {}

var (
	costHeaderName = regexp.MustCompile(`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`)
	costTagKey     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

func (r AICostManagementTagsResource) Validate() error {
	if err := ValidateRef(r.Ref); err != nil {
		return err
	}
	if r.Headers == nil || r.Prefixes == nil {
		return fmt.Errorf("headers and prefixes must be explicit arrays; use [] to reset capture")
	}
	if len(r.Headers)+len(r.Prefixes) > 32 {
		return fmt.Errorf("headers and prefixes support at most 32 entries combined")
	}
	headers, prefixes := map[string]bool{}, map[string]bool{}
	for _, h := range r.Headers {
		name := strings.ToLower(h.Name)
		if !costHeaderName.MatchString(h.Name) || !costTagKey.MatchString(h.TagKey) || len(h.TagKey) > 128 {
			return fmt.Errorf("invalid cost tag header name or tag_key")
		}
		if headers[name] {
			return fmt.Errorf("duplicate cost tag header %q", h.Name)
		}
		headers[name] = true
	}
	for _, p := range r.Prefixes {
		name := strings.ToLower(p.Name)
		if !costHeaderName.MatchString(p.Name) || !strings.Contains(p.Name, "-") {
			return fmt.Errorf("cost tag prefixes require at least two dash-separated segments")
		}
		if prefixes[name] {
			return fmt.Errorf("duplicate cost tag prefix %q", p.Name)
		}
		prefixes[name] = true
	}
	return nil
}

func (r AICostManagementTagsResource) UpdateRequest() kkComps.UpdateCostManagementTags {
	return kkComps.UpdateCostManagementTags{
		Headers:   r.Headers,
		Prefixes:  r.Prefixes,
		Labels:    r.Labels,
		ManagedBy: r.ManagedBy,
	}
}

func (r AICostManagementTagsResource) PayloadMap() (map[string]any, error) {
	return marshalObjectToMap(r.UpdateRequest(), "AI cost management tags")
}

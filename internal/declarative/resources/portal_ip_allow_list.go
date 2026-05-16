package resources

import (
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"strings"
)

func init() {
	registerResourceType(
		ResourceTypePortalIPAllowList,
		func(rs *ResourceSet) *[]PortalIPAllowListResource { return &rs.PortalIPAllowLists },
		AutoExplain[PortalIPAllowListResource](),
	)
}

// PortalIPAllowListResource represents a portal IP allow-list entry.
type PortalIPAllowListResource struct {
	Ref        string   `yaml:"ref"              json:"ref"`
	Portal     string   `yaml:"portal,omitempty" json:"portal,omitempty"`
	AllowedIPs []string `yaml:"allowed_ips"      json:"allowed_ips"`

	konnectID string `yaml:"-" json:"-"`
}

func (l PortalIPAllowListResource) GetRef() string {
	return l.Ref
}

func (l PortalIPAllowListResource) Validate() error {
	if err := ValidateRef(l.Ref); err != nil {
		return fmt.Errorf("invalid portal IP allow list ref: %w", err)
	}

	if len(l.AllowedIPs) == 0 {
		return fmt.Errorf("allowed_ips must contain at least one IP address or CIDR block")
	}

	seen := make(map[string]struct{}, len(l.AllowedIPs))
	for i, value := range l.AllowedIPs {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			return fmt.Errorf("allowed_ips[%d] cannot be empty", i)
		}
		if !isValidIPAddressOrCIDR(normalized) {
			return fmt.Errorf("allowed_ips[%d] must be an IP address or CIDR block: %q", i, value)
		}
		if _, ok := seen[normalized]; ok {
			return fmt.Errorf("allowed_ips[%d] duplicates %q", i, normalized)
		}
		seen[normalized] = struct{}{}
	}

	return nil
}

func (l *PortalIPAllowListResource) SetDefaults() {}

func (l PortalIPAllowListResource) GetType() ResourceType {
	return ResourceTypePortalIPAllowList
}

func (l PortalIPAllowListResource) GetMoniker() string {
	return strings.Join(normalizedAllowedIPs(l.AllowedIPs), ",")
}

func (l PortalIPAllowListResource) GetDependencies() []ResourceRef {
	if l.Portal == "" {
		return []ResourceRef{}
	}
	return []ResourceRef{{Kind: ResourceTypePortal, Ref: l.Portal}}
}

func (l PortalIPAllowListResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{
		"portal": "portal",
	}
}

func (l PortalIPAllowListResource) GetKonnectID() string {
	return l.konnectID
}

func (l PortalIPAllowListResource) GetKonnectMonikerFilter() string {
	return ""
}

func (l *PortalIPAllowListResource) TryMatchKonnectResource(_ any) bool {
	// Matched via parent portal state; entries do not expose a stable declarative moniker.
	return false
}

// GetParentRef implements ResourceWithParent for inheritance of namespace and protection.
func (l PortalIPAllowListResource) GetParentRef() *ResourceRef {
	if l.Portal == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypePortal, Ref: l.Portal}
}

// UnmarshalJSON rejects kongctl metadata on child resources.
func (l *PortalIPAllowListResource) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	allowedKeys := map[string]struct{}{
		"ref":         {},
		"portal":      {},
		"allowed_ips": {},
		"kongctl":     {},
	}
	for key := range raw {
		if _, ok := allowedKeys[key]; !ok {
			return fmt.Errorf("json: unknown field %q", key)
		}
	}

	var temp struct {
		Ref        string   `json:"ref"`
		Portal     string   `json:"portal,omitempty"`
		AllowedIPs []string `json:"allowed_ips"`
		Kongctl    any      `json:"kongctl,omitempty"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	if temp.Kongctl != nil {
		return fmt.Errorf("kongctl metadata not supported on portal IP allow lists")
	}

	l.Ref = temp.Ref
	l.Portal = temp.Portal
	l.AllowedIPs = temp.AllowedIPs
	return nil
}

func normalizedAllowedIPs(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	slices.Sort(normalized)
	return normalized
}

func isValidIPAddressOrCIDR(value string) bool {
	if ip := net.ParseIP(value); ip != nil {
		return true
	}
	_, _, err := net.ParseCIDR(value)
	return err == nil
}

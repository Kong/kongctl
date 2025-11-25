package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
)

const (
	// MaxNamespaceLength is the maximum allowed length for a namespace
	MaxNamespaceLength = 63

	// NamespacePattern defines the valid pattern for namespace names
	// Must start and end with alphanumeric, can contain hyphens in the middle
	NamespacePattern = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
)

var (
	namespaceRegex = regexp.MustCompile(NamespacePattern)

	// ReservedNamespaces contains namespace names that cannot be used
	// Currently empty but structured to allow future additions
	ReservedNamespaces = map[string]bool{}
)

// NamespaceValidator validates namespace values
type NamespaceValidator struct{}

// NewNamespaceValidator creates a new namespace validator
func NewNamespaceValidator() *NamespaceValidator {
	return &NamespaceValidator{}
}

// NamespaceRequirementMode represents how namespace enforcement should behave
type NamespaceRequirementMode int

const (
	// NamespaceRequirementNone means no namespace requirement is active
	NamespaceRequirementNone NamespaceRequirementMode = iota
	// NamespaceRequirementAny requires every resource to declare a namespace (explicitly or via _defaults)
	NamespaceRequirementAny
	// NamespaceRequirementSpecific requires every resource to use a specific namespace
	NamespaceRequirementSpecific
)

// NamespaceRequirement captures the parsed namespace enforcement settings
type NamespaceRequirement struct {
	Mode              NamespaceRequirementMode
	AllowedNamespaces []string // Empty means "any namespace", populated means "only these namespaces"
}

// ParseNamespaceRequirementSlice interprets a slice of namespace values into a requirement structure
func (v *NamespaceValidator) ParseNamespaceRequirementSlice(namespaces []string) (NamespaceRequirement, error) {
	// If empty slice, means flag was provided without values - require any namespace
	if len(namespaces) == 0 {
		return NamespaceRequirement{
			Mode:              NamespaceRequirementAny,
			AllowedNamespaces: []string{},
		}, nil
	}

	// Validate and collect unique namespaces
	seen := make(map[string]bool)
	unique := make([]string, 0, len(namespaces))

	for _, ns := range namespaces {
		ns = strings.TrimSpace(ns)
		if ns == "" {
			continue // Skip empty strings
		}

		// Check if this looks like a flag that was accidentally consumed
		if strings.HasPrefix(ns, "-") {
			return NamespaceRequirement{}, fmt.Errorf(
				"'%s' looks like a flag but was interpreted as a namespace value.\n"+
					"If you meant to require any namespace, use --require-any-namespace instead.\n"+
					"If you meant to specify a namespace, use --require-namespace=<namespace> or "+
					"place --require-namespace values before other flags",
				ns)
		}

		// Validate the namespace
		if err := v.ValidateNamespace(ns); err != nil {
			return NamespaceRequirement{}, fmt.Errorf("invalid namespace '%s': %w", ns, err)
		}

		// Add to unique list
		if !seen[ns] {
			seen[ns] = true
			unique = append(unique, ns)
		}
	}

	// If all entries were empty/invalid, treat as "any"
	if len(unique) == 0 {
		return NamespaceRequirement{
			Mode:              NamespaceRequirementAny,
			AllowedNamespaces: []string{},
		}, nil
	}

	// Return with specific allowed namespaces
	return NamespaceRequirement{
		Mode:              NamespaceRequirementSpecific,
		AllowedNamespaces: unique,
	}, nil
}

// ParseNamespaceRequirement interprets a raw flag/config value into a requirement structure
// Deprecated: Use ParseNamespaceRequirementSlice for new code
func (v *NamespaceValidator) ParseNamespaceRequirement(raw string) (NamespaceRequirement, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return NamespaceRequirement{Mode: NamespaceRequirementNone}, nil
	}

	switch strings.ToLower(trimmed) {
	case "false", "off", "no":
		return NamespaceRequirement{Mode: NamespaceRequirementNone}, nil
	case "true", "any":
		return NamespaceRequirement{Mode: NamespaceRequirementAny}, nil
	default:
		if err := v.ValidateNamespace(trimmed); err != nil {
			return NamespaceRequirement{}, err
		}
		return NamespaceRequirement{
			Mode:              NamespaceRequirementSpecific,
			AllowedNamespaces: []string{trimmed},
		}, nil
	}
}

// ValidateNamespace validates a single namespace value
func (v *NamespaceValidator) ValidateNamespace(namespace string) error {
	// Empty namespace is invalid (though this should be caught earlier)
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}

	// Check length
	if len(namespace) > MaxNamespaceLength {
		return fmt.Errorf("namespace '%s' exceeds maximum length of %d characters",
			namespace, MaxNamespaceLength)
	}

	// Check pattern
	if !namespaceRegex.MatchString(namespace) {
		return fmt.Errorf("namespace '%s' is invalid: must consist of lowercase alphanumeric "+
			"characters or '-', and must start and end with an alphanumeric character",
			namespace)
	}

	// Check reserved namespaces
	if ReservedNamespaces[namespace] {
		return fmt.Errorf("namespace '%s' is reserved and cannot be used", namespace)
	}

	// Additional validation: no double hyphens
	if strings.Contains(namespace, "--") {
		return fmt.Errorf("namespace '%s' is invalid: cannot contain consecutive hyphens", namespace)
	}

	return nil
}

// ValidateNamespaces validates a collection of namespaces
func (v *NamespaceValidator) ValidateNamespaces(namespaces []string) error {
	seen := make(map[string]bool)

	for _, ns := range namespaces {
		// Validate individual namespace
		if err := v.ValidateNamespace(ns); err != nil {
			return err
		}

		// Check for duplicates (shouldn't happen but good to verify)
		if seen[ns] {
			// This is not an error, just means the same namespace appears multiple times
			// which is fine for our use case
			continue
		}
		seen[ns] = true
	}

	return nil
}

// ValidateNamespaceRequirement enforces namespace requirements against a resource set
func (v *NamespaceValidator) ValidateNamespaceRequirement(
	rs *resources.ResourceSet,
	req NamespaceRequirement,
) error {
	if req.Mode == NamespaceRequirementNone || rs == nil {
		return nil
	}
	defaultNamespaces := rs.DefaultNamespaces
	if len(defaultNamespaces) == 0 && rs.DefaultNamespace != "" {
		defaultNamespaces = []string{rs.DefaultNamespace}
	}

	managedPortals := make([]resources.PortalResource, 0, len(rs.Portals))
	for _, portal := range rs.Portals {
		if portal.IsExternal() {
			continue
		}
		managedPortals = append(managedPortals, portal)
	}

	managedControlPlanes := make([]resources.ControlPlaneResource, 0, len(rs.ControlPlanes))
	for _, cp := range rs.ControlPlanes {
		if cp.IsExternal() {
			continue
		}
		managedControlPlanes = append(managedControlPlanes, cp)
	}

	totalParents := len(managedPortals) +
		len(rs.ApplicationAuthStrategies) +
		len(managedControlPlanes) +
		len(rs.APIs)

	if totalParents == 0 {
		switch req.Mode {
		case NamespaceRequirementNone:
			return nil
		case NamespaceRequirementAny:
			if len(defaultNamespaces) == 0 {
				return fmt.Errorf("namespace enforcement requires resources or _defaults.kongctl.namespace to be set")
			}
		case NamespaceRequirementSpecific:
			if len(defaultNamespaces) == 0 {
				namespaceList := strings.Join(req.AllowedNamespaces, ", ")
				return fmt.Errorf(
					"namespace enforcement requires one of [%s] but no resources or _defaults.kongctl.namespace were provided",
					namespaceList,
				)
			}
			// Check if default namespace is in allowed list
			allowed := false
			for _, ns := range req.AllowedNamespaces {
				for _, def := range defaultNamespaces {
					if def == ns {
						allowed = true
						break
					}
				}
				if allowed {
					break
				}
			}
			if !allowed {
				namespaceList := strings.Join(req.AllowedNamespaces, ", ")
				return fmt.Errorf(
					"namespace enforcement requires one of [%s] but _defaults.kongctl.namespace is '%s'",
					namespaceList, strings.Join(defaultNamespaces, ", "),
				)
			}
		}
		return nil
	}

	violations := make([]string, 0)

	check := func(resourceType, ref string, meta *resources.KongctlMeta) {
		origin := resources.NamespaceOriginUnset
		if meta != nil {
			origin = meta.NamespaceOrigin
		}

		switch req.Mode {
		case NamespaceRequirementNone:
			return
		case NamespaceRequirementAny:
			if meta == nil || meta.Namespace == nil ||
				origin == resources.NamespaceOriginImplicitDefault || origin == resources.NamespaceOriginUnset {
				reason := "missing explicit namespace; add kongctl.namespace or set _defaults.kongctl.namespace"
				violations = append(violations, fmt.Sprintf("%s '%s': %s", resourceType, ref, reason))
			}
		case NamespaceRequirementSpecific:
			namespace := resources.GetNamespace(meta)
			if meta == nil || meta.Namespace == nil ||
				origin == resources.NamespaceOriginImplicitDefault || origin == resources.NamespaceOriginUnset {
				namespaceList := strings.Join(req.AllowedNamespaces, ", ")
				reason := fmt.Sprintf(
					"missing explicit namespace; expected one of [%s] via kongctl.namespace or _defaults.kongctl.namespace",
					namespaceList,
				)
				violations = append(violations, fmt.Sprintf("%s '%s': %s", resourceType, ref, reason))
				return
			}
			// Check if namespace is in allowed list
			allowed := false
			for _, ns := range req.AllowedNamespaces {
				if namespace == ns {
					allowed = true
					break
				}
			}
			if !allowed {
				namespaceList := strings.Join(req.AllowedNamespaces, ", ")
				reason := fmt.Sprintf("uses namespace '%s' (expected one of [%s])", namespace, namespaceList)
				violations = append(violations, fmt.Sprintf("%s '%s': %s", resourceType, ref, reason))
			}
		}
	}

	for i := range managedPortals {
		check(string(resources.ResourceTypePortal), managedPortals[i].Ref, managedPortals[i].Kongctl)
	}
	for i := range rs.APIs {
		check(string(resources.ResourceTypeAPI), rs.APIs[i].Ref, rs.APIs[i].Kongctl)
	}
	for i := range rs.ApplicationAuthStrategies {
		check(string(resources.ResourceTypeApplicationAuthStrategy),
			rs.ApplicationAuthStrategies[i].Ref,
			rs.ApplicationAuthStrategies[i].Kongctl,
		)
	}
	for i := range managedControlPlanes {
		check(string(resources.ResourceTypeControlPlane), managedControlPlanes[i].Ref, managedControlPlanes[i].Kongctl)
	}

	if len(violations) == 0 {
		return nil
	}

	return fmt.Errorf("namespace enforcement failed:\n  - %s", strings.Join(violations, "\n  - "))
}

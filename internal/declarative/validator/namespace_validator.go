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
	Mode      NamespaceRequirementMode
	Namespace string
}

// ParseNamespaceRequirement interprets a raw flag/config value into a requirement structure
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
			Mode:      NamespaceRequirementSpecific,
			Namespace: trimmed,
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

	totalParents := len(rs.Portals) + len(rs.ApplicationAuthStrategies) + len(rs.ControlPlanes) + len(rs.APIs)

	if totalParents == 0 {
		switch req.Mode {
		case NamespaceRequirementAny:
			if rs.DefaultNamespace == "" {
				return fmt.Errorf("namespace enforcement requires resources or _defaults.kongctl.namespace to be set")
			}
		case NamespaceRequirementSpecific:
			if rs.DefaultNamespace == "" {
				return fmt.Errorf(
					"namespace enforcement requires namespace '%s' but no resources or _defaults.kongctl.namespace were provided",
					req.Namespace,
				)
			}
			if rs.DefaultNamespace != req.Namespace {
				return fmt.Errorf(
					"namespace enforcement requires namespace '%s' but _defaults.kongctl.namespace is '%s'",
					req.Namespace, rs.DefaultNamespace,
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
		case NamespaceRequirementAny:
			if meta == nil || meta.Namespace == nil || origin == resources.NamespaceOriginImplicitDefault || origin == resources.NamespaceOriginUnset {
				reason := "missing explicit namespace; add kongctl.namespace or set _defaults.kongctl.namespace"
				violations = append(violations, fmt.Sprintf("%s '%s': %s", resourceType, ref, reason))
			}
		case NamespaceRequirementSpecific:
			namespace := resources.GetNamespace(meta)
			if meta == nil || meta.Namespace == nil || origin == resources.NamespaceOriginImplicitDefault || origin == resources.NamespaceOriginUnset {
				reason := fmt.Sprintf(
					"missing explicit namespace; expected '%s' via kongctl.namespace or _defaults.kongctl.namespace",
					req.Namespace,
				)
				violations = append(violations, fmt.Sprintf("%s '%s': %s", resourceType, ref, reason))
				return
			}
			if namespace != req.Namespace {
				reason := fmt.Sprintf("uses namespace '%s' (expected '%s')", namespace, req.Namespace)
				violations = append(violations, fmt.Sprintf("%s '%s': %s", resourceType, ref, reason))
			}
		}
	}

	for i := range rs.Portals {
		check(string(resources.ResourceTypePortal), rs.Portals[i].Ref, rs.Portals[i].Kongctl)
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
	for i := range rs.ControlPlanes {
		check(string(resources.ResourceTypeControlPlane), rs.ControlPlanes[i].Ref, rs.ControlPlanes[i].Kongctl)
	}

	if len(violations) == 0 {
		return nil
	}

	return fmt.Errorf("namespace enforcement failed:\n  - %s", strings.Join(violations, "\n  - "))
}

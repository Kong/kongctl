package validator

import (
	"fmt"
	"regexp"
	"strings"
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

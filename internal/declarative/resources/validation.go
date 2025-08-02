package resources

import (
	"fmt"
	"regexp"
	"strings"
)

// refPattern defines the allowed pattern for resource refs
// Allows alphanumeric characters, hyphens, and underscores
// Must start with a letter or number, and can contain hyphens and underscores
var refPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

const (
	// MaxRefLength is the maximum allowed length for a ref
	MaxRefLength = 63
	// MinRefLength is the minimum allowed length for a ref
	MinRefLength = 1
)

// ValidateRef validates that a resource ref follows naming conventions
func ValidateRef(ref string) error {
	if ref == "" {
		return fmt.Errorf("ref cannot be empty")
	}

	if len(ref) < MinRefLength || len(ref) > MaxRefLength {
		return fmt.Errorf("ref must be between %d and %d characters long", MinRefLength, MaxRefLength)
	}

	if strings.Contains(ref, ":") {
		return fmt.Errorf("ref cannot contain colons (:)")
	}

	if strings.Contains(ref, " ") {
		return fmt.Errorf("ref cannot contain spaces")
	}

	if !refPattern.MatchString(ref) {
		return fmt.Errorf("ref must start with a letter or number and " +
			"contain only alphanumeric characters, hyphens, and underscores")
	}

	return nil
}
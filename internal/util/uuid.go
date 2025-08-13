package util

import "regexp"

// uuidRegex is a compiled regular expression for validating UUID format.
// Uses case-insensitive pattern to handle all UUID formats consistently.
var uuidRegex = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// IsValidUUID checks if a string is a valid UUID format.
// It validates the standard 8-4-4-4-12 hexadecimal pattern with dashes.
func IsValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}
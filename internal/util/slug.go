package util

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// GenerateSlug converts a title to a URL-friendly slug
// This matches the Konnect server-side slugify implementation
// Example: "My API Document" -> "my-api-document"
func GenerateSlug(title string) string {
	// Normalize using NFKD to decompose characters
	slug := norm.NFKD.String(title)

	// Remove all combining diacritical marks (accents)
	// This handles é→e, ñ→n, etc.
	reg := regexp.MustCompile(`[\x{0300}-\x{036F}]`)
	slug = reg.ReplaceAllString(slug, "")

	// Add space around uppercase characters in middle of word
	// Handles: XMLParser -> XML Parser, HTMLElement -> HTML Element
	reg = regexp.MustCompile(`([A-Z]*)([A-Z]{1})([a-z]+)`)
	slug = reg.ReplaceAllString(slug, " $1 $2$3")

	// Add space before uppercase characters at the end of word
	// Handles: parseXML -> parse XML
	reg = regexp.MustCompile(`([A-Z]+)$`)
	slug = reg.ReplaceAllString(slug, " $1")

	// Convert to lowercase
	slug = strings.ToLower(slug)

	// Trim whitespace from both sides
	slug = strings.TrimSpace(slug)

	// Replace spaces with hyphens
	reg = regexp.MustCompile(`\s+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Remove all non-word chars (keeping only letters, numbers, hyphens, underscores)
	// Note: In JavaScript, \w includes [a-zA-Z0-9_], so [^\w-]+ removes everything except those and hyphens
	reg = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	slug = reg.ReplaceAllString(slug, "")

	// Replace underscores with hyphens
	slug = strings.ReplaceAll(slug, "_", "-")

	// Replace multiple hyphens with single hyphen
	reg = regexp.MustCompile(`--+`)
	slug = reg.ReplaceAllString(slug, "-")

	// Remove trailing hyphen
	slug = strings.TrimSuffix(slug, "-")

	// Remove leading hyphen
	slug = strings.TrimPrefix(slug, "-")

	return slug
}

package util

import "testing"

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "simple title",
			title:    "Main",
			expected: "main",
		},
		{
			name:     "title with spaces",
			title:    "My API Document",
			expected: "my-api-document",
		},
		{
			name:     "title with special characters",
			title:    "API v2.0 (Beta)",
			expected: "api-v20-beta", // Dots are removed, not converted to hyphens
		},
		{
			name:     "title with multiple spaces",
			title:    "This   Has    Multiple     Spaces",
			expected: "this-has-multiple-spaces",
		},
		{
			name:     "title with leading/trailing spaces",
			title:    "  Trimmed Title  ",
			expected: "trimmed-title",
		},
		{
			name:     "title with underscores",
			title:    "API_Document_Name",
			expected: "api-document-name", // Underscores become hyphens
		},
		{
			name:     "mixed case title",
			title:    "MixedCaseTitle",
			expected: "mixed-case-title", // CamelCase is split
		},
		{
			name:     "title with numbers",
			title:    "Document 123",
			expected: "document-123",
		},
		{
			name:     "title with dots and slashes",
			title:    "api/v1.0/docs",
			expected: "apiv10docs", // Slashes and dots are removed
		},
		{
			name:     "empty title",
			title:    "",
			expected: "",
		},
		// Additional test cases for server compatibility
		{
			name:     "XMLParser camelCase",
			title:    "XMLParser",
			expected: "xml-parser",
		},
		{
			name:     "HTMLElement camelCase",
			title:    "HTMLElement",
			expected: "html-element",
		},
		{
			name:     "parseXML end uppercase",
			title:    "parseXML",
			expected: "parse-xml",
		},
		{
			name:     "APIv2 consecutive uppercase",
			title:    "APIv2",
			expected: "ap-iv2",
		},
		{
			name:     "unicode normalization - café",
			title:    "café",
			expected: "cafe",
		},
		{
			name:     "unicode normalization - piñata",
			title:    "piñata",
			expected: "pinata",
		},
		{
			name:     "unicode normalization - naïve",
			title:    "naïve",
			expected: "naive",
		},
		{
			name:     "complex example",
			title:    "My APIv2 café_Document (BETA)",
			expected: "my-ap-iv2-cafe-document-beta",
		},
		{
			name:     "multiple underscores and spaces",
			title:    "test___multiple___underscores   and   spaces",
			expected: "test-multiple-underscores-and-spaces",
		},
		{
			name:     "leading and trailing special chars",
			title:    "!!!Hello World!!!",
			expected: "hello-world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateSlug(tt.title)
			if got != tt.expected {
				t.Errorf("GenerateSlug(%q) = %q, want %q", tt.title, got, tt.expected)
			}
		})
	}
}

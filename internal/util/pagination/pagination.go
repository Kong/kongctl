package pagination

import (
	"net/url"
	"strings"
)

// ExtractPageAfterCursor returns the cursor from a pagination token, next-page
// URL, or query parameter snippet. Bare tokens are opaque and are not decoded.
func ExtractPageAfterCursor(next *string) string {
	if next == nil {
		return ""
	}

	value := strings.TrimSpace(*next)
	if value == "" {
		return ""
	}

	if parsed, err := url.Parse(value); err == nil {
		if cursor := parsed.Query().Get("page[after]"); cursor != "" {
			return cursor
		}
	}

	if cursor, ok := extractPageAfterCursorSnippet(value); ok {
		return cursor
	}

	if decoded, err := url.QueryUnescape(value); err == nil {
		if cursor, ok := extractPageAfterCursorSnippet(decoded); ok {
			return cursor
		}
	}

	if strings.Contains(value, "://") || strings.Contains(value, "?") {
		return ""
	}
	return value
}

func extractPageAfterCursorSnippet(value string) (string, bool) {
	if _, after, ok := strings.Cut(value, "page[after]="); ok {
		cursor := after
		if end := strings.Index(cursor, "&"); end >= 0 {
			cursor = cursor[:end]
		}

		decoded, err := url.QueryUnescape(cursor)
		if err == nil {
			return decoded, true
		}

		return cursor, true
	}

	return "", false
}

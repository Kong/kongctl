package pagination

import (
	"net/url"
	"strings"
)

// ExtractPageAfterCursor returns the cursor value from a paginated "next" link.
// It tolerates raw URLs as well as plain query parameter snippets.
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

	if idx := strings.Index(value, "page[after]="); idx >= 0 {
		cursor := value[idx+len("page[after]="):]
		if end := strings.Index(cursor, "&"); end >= 0 {
			cursor = cursor[:end]
		}
		return cursor
	}

	return ""
}

package pagination

import (
	"errors"
	"net/url"
	"strings"
)

// CursorTracker detects repeated cursors within one list operation, including
// cycles involving multiple pages. Its zero value is ready to use.
type CursorTracker struct {
	seen map[string]struct{}
}

// Next extracts a next-page cursor and rejects previously seen cursors.
func (t *CursorTracker) Next(next *string) (string, error) {
	cursor := ExtractPageAfterCursor(next)
	if cursor == "" {
		return "", nil
	}
	if _, exists := t.seen[cursor]; exists {
		return "", errors.New("pagination returned a previously seen cursor")
	}
	if t.seen == nil {
		t.seen = make(map[string]struct{})
	}
	t.seen[cursor] = struct{}{}
	return cursor, nil
}

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

package helpers

import "strings"

func nextOffsetToken(offset *string) (string, bool) {
	if offset == nil {
		return "", false
	}

	next := strings.TrimSpace(*offset)
	if next == "" {
		return "", false
	}

	return next, true
}

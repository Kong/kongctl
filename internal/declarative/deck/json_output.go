package deck

import "strings"

const maskedDeckEnvValue = "[masked]"

// NormalizeMaskedJSONOutput quotes bare [masked] values emitted by deck for
// masked DECK_* numeric values. This is a temporary compatibility workaround
// for Kong/deck#2047.
func NormalizeMaskedJSONOutput(stdout string) (string, bool) {
	if !strings.Contains(stdout, maskedDeckEnvValue) {
		return stdout, false
	}

	var b strings.Builder
	b.Grow(len(stdout))

	changed := false
	inString := false
	escaped := false
	for i := 0; i < len(stdout); i++ {
		ch := stdout[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			b.WriteByte(ch)
			continue
		}

		if strings.HasPrefix(stdout[i:], maskedDeckEnvValue) {
			b.WriteString(`"` + maskedDeckEnvValue + `"`)
			i += len(maskedDeckEnvValue) - 1
			changed = true
			continue
		}

		b.WriteByte(ch)
	}

	if !changed {
		return stdout, false
	}
	return b.String(), true
}

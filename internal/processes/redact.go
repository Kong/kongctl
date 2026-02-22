package processes

import "strings"

var sensitiveFlags = map[string]struct{}{
	"--authorization": {},
	"--pat":           {},
	"--token":         {},
	"--access-token":  {},
	"--api-key":       {},
	"--apikey":        {},
	"--password":      {},
	"--secret":        {},
}

// RedactArgs returns a copy of args with sensitive flag values redacted.
func RedactArgs(args []string) []string {
	redacted := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]

		flagName, ok := splitFlagName(arg)
		if ok && isSensitiveFlag(flagName) {
			redacted = append(redacted, flagName+"=<redacted>")
			continue
		}

		if isSensitiveFlag(arg) {
			redacted = append(redacted, arg)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				redacted = append(redacted, "<redacted>")
				i++
			}
			continue
		}

		redacted = append(redacted, arg)
	}
	return redacted
}

func splitFlagName(arg string) (string, bool) {
	if !strings.HasPrefix(arg, "-") {
		return "", false
	}

	idx := strings.IndexByte(arg, '=')
	if idx <= 0 {
		return "", false
	}

	return arg[:idx], true
}

func isSensitiveFlag(flag string) bool {
	normalized := strings.ToLower(strings.TrimSpace(flag))
	_, ok := sensitiveFlags[normalized]
	return ok
}

package processes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactArgs(t *testing.T) {
	t.Parallel()

	args := []string{
		"listen",
		"--authorization",
		"super-secret",
		"--pat=token-value",
		"--endpoint",
		"https://example.test/audit-logs",
	}

	got := RedactArgs(args)
	require.Equal(t,
		[]string{
			"listen",
			"--authorization",
			"<redacted>",
			"--pat=<redacted>",
			"--endpoint",
			"https://example.test/audit-logs",
		},
		got,
	)
}

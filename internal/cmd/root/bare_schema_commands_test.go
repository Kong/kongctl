package root

import (
	"strings"
	"testing"
)

func TestExplainAndScaffoldWithNoArgsListResourcePaths(t *testing.T) {
	for _, verb := range []string{"explain", "scaffold"} {
		t.Run(verb, func(t *testing.T) {
			result := executeRootForTest(t, verb)
			if result.exitCode != 0 {
				t.Fatalf("expected %s with no args to succeed, got exit %d\nstderr:\n%s",
					verb, result.exitCode, result.stderr)
			}
			for _, want := range []string{
				"Available resource paths:",
				"  api\n",
				"  api.versions\n",
				"  portal\n",
				"  portal.pages\n",
				"  portal.snippets\n",
				verb + " <resource-path>",
			} {
				if !strings.Contains(result.stdout, want) {
					t.Fatalf("expected %s output to contain %q\nstdout:\n%s", verb, want, result.stdout)
				}
			}
		})
	}
}

func TestExplainWithTooManyArgsStillFails(t *testing.T) {
	result := executeRootForTest(t, "explain", "api", "portal")
	if result.exitCode == 0 {
		t.Fatalf("expected explain with two args to fail\nstdout:\n%s", result.stdout)
	}
	if !strings.Contains(result.stderr, "accepts at most 1 arg") {
		t.Fatalf("expected arg-count error, got stderr:\n%s", result.stderr)
	}
}

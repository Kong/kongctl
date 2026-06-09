package iostreams

import "testing"

func TestHasTrueColorEnvTrimsWhitespace(t *testing.T) {
	t.Setenv("COLORTERM", " truecolor \n")
	if !HasTrueColorEnv() {
		t.Fatal("expected trimmed truecolor value to be detected")
	}
}

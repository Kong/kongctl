package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// events renders a sequence of test2json-shaped events as newline-delimited
// JSON, the same shape `go test -json` emits on stdout.
func events(t *testing.T, evs ...testEvent) string {
	t.Helper()
	var b strings.Builder
	for _, ev := range evs {
		line, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.String()
}

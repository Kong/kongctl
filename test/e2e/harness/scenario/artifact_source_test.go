//go:build e2e

package scenario

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/test/e2e/harness"
)

func TestLoadArtifactAssertionSourceGlobReturnsCountAndMatches(t *testing.T) {
	baseDir := t.TempDir()
	dumpDir := filepath.Join(baseDir, "http-dumps")
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		t.Fatalf("mkdir http-dumps: %v", err)
	}
	for _, name := range []string{"request-002.txt", "request-001.txt"} {
		path := filepath.Join(dumpDir, name)
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	got, err := loadArtifactAssertionSource(baseDir, AssertionArtifactSource{
		Glob: "http-dumps/request-*.txt",
	}, nil)
	if err != nil {
		t.Fatalf("loadArtifactAssertionSource() error = %v", err)
	}

	data, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("source = %T, want map[string]any", got)
	}
	if count, ok := data["count"].(int); !ok || count != 2 {
		t.Fatalf("count = %#v, want 2", data["count"])
	}

	matches, ok := data["matches"].([]any)
	if !ok {
		t.Fatalf("matches = %T, want []any", data["matches"])
	}
	want := []string{
		"http-dumps/request-001.txt",
		"http-dumps/request-002.txt",
	}
	if len(matches) != len(want) {
		t.Fatalf("len(matches) = %d, want %d", len(matches), len(want))
	}
	for i, match := range matches {
		if match != want[i] {
			t.Fatalf("matches[%d] = %v, want %q", i, match, want[i])
		}
	}
}

func TestLoadArtifactAssertionSourcePathAutoParsesJSON(t *testing.T) {
	baseDir := t.TempDir()
	path := filepath.Join(baseDir, "meta.json")
	if err := os.WriteFile(path, []byte(`{"count":2,"items":["a","b"]}`), 0o644); err != nil {
		t.Fatalf("write meta.json: %v", err)
	}

	got, err := loadArtifactAssertionSource(baseDir, AssertionArtifactSource{
		Path: "meta.json",
	}, nil)
	if err != nil {
		t.Fatalf("loadArtifactAssertionSource() error = %v", err)
	}

	data, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("source = %T, want map[string]any", got)
	}
	if count := data["count"]; count != float64(2) {
		t.Fatalf("count = %#v, want 2", count)
	}
}

func TestLoadArtifactAssertionSourcePathParsesText(t *testing.T) {
	baseDir := t.TempDir()
	path := filepath.Join(baseDir, "stderr.txt")
	if err := os.WriteFile(path, []byte("trace line 1\ntrace line 2\n"), 0o644); err != nil {
		t.Fatalf("write stderr.txt: %v", err)
	}

	got, err := loadArtifactAssertionSource(baseDir, AssertionArtifactSource{
		Path: "stderr.txt",
	}, nil)
	if err != nil {
		t.Fatalf("loadArtifactAssertionSource() error = %v", err)
	}

	data, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("source = %T, want map[string]any", got)
	}
	if text := data["text"]; text != "trace line 1\ntrace line 2\n" {
		t.Fatalf("text = %#v, want trace contents", text)
	}
}

func TestLoadArtifactAssertionSourceRejectsInvalidConfig(t *testing.T) {
	baseDir := t.TempDir()

	_, err := loadArtifactAssertionSource(baseDir, AssertionArtifactSource{
		Path: "stderr.txt",
		Glob: "http-dumps/request-*.txt",
	}, nil)
	if err == nil {
		t.Fatal("expected error for path+glob")
	}

	_, err = loadArtifactAssertionSource(baseDir, AssertionArtifactSource{}, nil)
	if err == nil {
		t.Fatal("expected error for empty artifact source")
	}
}

func TestResolveAssertionSourceRejectsMultipleSourceModes(t *testing.T) {
	_, err := resolveAssertionSource(
		&harness.CLI{},
		Assertion{
			Source: AssertionSrc{
				Get: "apis",
				Artifact: &AssertionArtifactSource{
					Glob: "http-dumps/request-*.txt",
				},
			},
		},
		nil,
		"assert-000",
		0,
		"",
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected error for get+artifact")
	}
}

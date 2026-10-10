//go:build e2e

package scenario

import (
	"testing"

	jmespath "github.com/jmespath/go-jmespath"
)

func TestYAMLStreamAssertionsVisitAllDocuments(t *testing.T) {
	output := "---\ntype: Mesh\nname: default\n---\n---\ntype: MeshTimeout\nname: slow\n" +
		"labels:\n  kuma.io/origin: global\nspec:\n  count: 2\n"
	data, err := parseCommandOutput("yaml-stream", output)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Array) != 2 {
		t.Fatalf("expected two nonempty documents, got %v", data.Array)
	}
	for _, query := range []string{
		"length([?type=='MeshTimeout' && name=='slow'])",
		"length([?labels.\"kuma.io/origin\"=='global'])",
		"length([?spec.count==`2`])",
	} {
		value, err := jmespath.Search(query, data.Value())
		if err != nil || value != float64(1) {
			t.Fatalf("query %s: value=%v error=%v", query, value, err)
		}
	}
}

func TestYAMLStreamRejectsMalformedLaterDocument(t *testing.T) {
	if _, err := parseCommandOutput("yaml-stream", "type: Mesh\nname: default\n---\nlabels: [\n"); err == nil {
		t.Fatal("accepted a malformed document after the first valid resource")
	}
}

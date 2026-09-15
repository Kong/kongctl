package mesh

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/iostreams"
)

func TestDecodeResourcesMultiDocument(t *testing.T) {
	input := `type: MeshTimeout
name: slow
mesh: prod
spec:
  targetRef:
    kind: Mesh
---
type: MeshRetry
name: retries
spec: {}
`
	resources, err := decodeResources(strings.NewReader(input), "policies.yaml", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(resources))
	}

	if resources[0].Type != "MeshTimeout" || resources[0].Name != "slow" || resources[0].Mesh != "prod" {
		t.Errorf("unexpected first resource: %+v", resources[0])
	}
	// A document that omits mesh inherits the resolved default.
	if resources[1].Mesh != "default" {
		t.Errorf("expected the default mesh, got %q", resources[1].Mesh)
	}
	// The second document is reported by position so an error can be located.
	if !strings.Contains(resources[1].Origin, "document 2") {
		t.Errorf("expected the origin to name the document, got %q", resources[1].Origin)
	}

	// The whole document is forwarded, not just the addressing fields.
	var body map[string]any
	if err := json.Unmarshal(resources[0].Body, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["spec"]; !ok {
		t.Error("spec was dropped from the forwarded body")
	}
}

// JSON is valid YAML, so a JSON document needs no separate path.
func TestDecodeResourcesAcceptsJSON(t *testing.T) {
	resources, err := decodeResources(
		strings.NewReader(`{"type":"MeshTimeout","name":"slow","spec":{}}`), "policy.json", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 || resources[0].Type != "MeshTimeout" {
		t.Fatalf("unexpected resources: %+v", resources)
	}
}

// A stream of separators, or trailing separators, yields nothing rather than
// empty resources that would be sent to the control plane.
func TestDecodeResourcesSkipsEmptyDocuments(t *testing.T) {
	resources, err := decodeResources(strings.NewReader("---\n---\n"), "empty.yaml", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected no resources, got %d", len(resources))
	}
}

func TestDecodeResourcesRequiresAddressableFields(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"missing type", "name: orphan\n", "'type'"},
		{"missing name", "type: MeshTimeout\n", "'name'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeResources(strings.NewReader(tc.input), "stdin", "default")
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error should name the missing field %s, got %q", tc.want, err)
			}
		})
	}
}

func TestDecodeResourcesReportsMalformedInput(t *testing.T) {
	_, err := decodeResources(strings.NewReader("type: [unclosed\n"), "broken.yaml", "default")
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if !strings.Contains(err.Error(), "broken.yaml") {
		t.Errorf("error should name the source, got %q", err)
	}
}

func TestDescribeApplyOutcome(t *testing.T) {
	tests := []struct {
		name   string
		result applyResult
		want   string
	}{
		{"created", applyResult{Created: true}, "created"},
		{"updated", applyResult{Created: false}, "updated"},
		{"failed", applyResult{Err: errors.New("boom")}, "failed: boom"},
		// A failure is reported as such even if the status suggested a create.
		{"failure wins over created", applyResult{Created: true, Err: errors.New("boom")}, "failed: boom"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := describeApplyOutcome(tc.result); got != tc.want {
				t.Errorf("outcome = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDescribeDocument(t *testing.T) {
	if got := describeDocument("policies.yaml", 0); got != "policies.yaml" {
		t.Errorf("single document should not be numbered, got %q", got)
	}
	if got := describeDocument("policies.yaml", 2); got != "policies.yaml (document 3)" {
		t.Errorf("unexpected description: %q", got)
	}
}

func TestParseWarnings(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "warnings are returned in order",
			body: `{"warnings":["first notice","second notice"]}`,
			want: []string{"first notice", "second notice"},
		},
		{
			name: "blank entries are dropped",
			body: `{"warnings":["  ","kept","\t"]}`,
			want: []string{"kept"},
		},
		{
			name: "surrounding space is trimmed",
			body: `{"warnings":["  padded  "]}`,
			want: []string{"padded"},
		},
		// A write the control plane accepted must not be reported as failed
		// just because its body carried nothing useful.
		{name: "an empty body yields nothing", body: "", want: nil},
		{name: "an empty object yields nothing", body: `{}`, want: nil},
		{name: "an empty list yields nothing", body: `{"warnings":[]}`, want: nil},
		{name: "a body that is not JSON yields nothing", body: "not json at all", want: nil},
		{name: "a body of the wrong shape yields nothing", body: `{"warnings":"a string"}`, want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseWarnings([]byte(tc.body))
			if len(got) != len(tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("entry %d: expected %q, got %q", i, tc.want[i], got[i])
				}
			}
		})
	}
}

func TestReportApplyWarningsNamesTheResource(t *testing.T) {
	errOut := &strings.Builder{}
	streams := &iostreams.IOStreams{Out: &strings.Builder{}, ErrOut: errOut}
	helper := &cmd.MockHelper{}
	helper.EXPECT().GetStreams().Return(streams)

	resource := meshResource{Type: "MeshRateLimit", Name: "warn-probe", Mesh: "default"}
	reportApplyWarnings(helper, resource, []string{"status must be 400 or higher", "second notice"})

	got := errOut.String()
	// Naming the resource matters because one command can apply many.
	for _, want := range []string{
		"warning: MeshRateLimit warn-probe: status must be 400 or higher\n",
		"warning: MeshRateLimit warn-probe: second notice\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected stderr to contain %q, got %q", want, got)
		}
	}
}

func TestReportApplyWarningsSilentWithoutWarnings(t *testing.T) {
	errOut := &strings.Builder{}
	streams := &iostreams.IOStreams{Out: &strings.Builder{}, ErrOut: errOut}
	helper := &cmd.MockHelper{}
	helper.EXPECT().GetStreams().Return(streams).Maybe()

	reportApplyWarnings(helper, meshResource{Type: "MeshTimeout", Name: "slow"}, nil)

	// The common case must stay quiet: a clean apply prints no stderr at all.
	if got := errOut.String(); got != "" {
		t.Errorf("expected no output, got %q", got)
	}
}

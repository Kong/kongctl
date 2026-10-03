package mesh

import (
	"encoding/json"
	"testing"
	"time"
)

func TestHeadersFor(t *testing.T) {
	tests := []struct {
		name       string
		descriptor ResourceDescriptor
		want       []string
	}{
		{
			"Dataplane has its own printer",
			ResourceDescriptor{Name: "Dataplane", Path: "dataplanes", Scope: ScopeMesh},
			[]string{"MESH", "NAME", "TAGS", "ADDRESS", "AGE"},
		},
		{
			"any other mesh scoped type",
			ResourceDescriptor{Name: "MeshTimeout", Path: "meshtimeouts", Scope: ScopeMesh},
			[]string{"MESH", "NAME", "AGE"},
		},
		{
			// Mesh.mtls was removed from the API in Kong Mesh 3, so Mesh falls
			// through to the global printer rather than carrying an mTLS column.
			"Mesh falls through to the global printer",
			ResourceDescriptor{Name: "Mesh", Path: "meshes", Scope: ScopeGlobal},
			[]string{"NAME", "AGE"},
		},
		{
			"any other global type",
			ResourceDescriptor{Name: "Zone", Path: "zones", Scope: ScopeGlobal},
			[]string{"NAME", "AGE"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := headersFor(tc.descriptor)
			if len(got) != len(tc.want) {
				t.Fatalf("headers = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("headers = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// No printer may emit an mTLS column: the field does not exist on Kong Mesh 3,
// so any value shown would be fabricated.
func TestNoPrinterEmitsMTLSColumn(t *testing.T) {
	for _, d := range []ResourceDescriptor{
		{Name: "Mesh", Path: "meshes", Scope: ScopeGlobal},
		{Name: "Dataplane", Path: "dataplanes", Scope: ScopeMesh},
		{Name: "MeshTimeout", Path: "meshtimeouts", Scope: ScopeMesh},
	} {
		for _, h := range headersFor(d) {
			if h == "mTLS" || h == "MTLS" {
				t.Errorf("%s printer emits an mTLS column", d.Name)
			}
		}
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m"},
		{2 * time.Hour, "2h"},
		{50 * time.Hour, "2d"},
		{24 * 400 * time.Hour, "1y"},
		{-5 * time.Second, "never"},
	}
	for _, tc := range tests {
		if got := duration(tc.d); got != tc.want {
			t.Errorf("duration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// The TAGS column on Kong Mesh 3 is the resource's labels merged with its
// gateway tags, with labels winning — not the inbound tags older Kuma showed.
func TestDisplayTags(t *testing.T) {
	var item map[string]any
	body := `{
	  "labels": {"kuma.io/zone": "east", "app": "backend"},
	  "networking": {"address": "10.0.0.1", "gateway": {"tags": {"role": "edge", "app": "ignored"}}}
	}`
	if err := json.Unmarshal([]byte(body), &item); err != nil {
		t.Fatal(err)
	}

	want := "app=backend kuma.io/zone=east role=edge"
	if got := displayTags(item); got != want {
		t.Errorf("displayTags = %q, want %q", got, want)
	}
	if got := dataplaneAddress(item); got != "10.0.0.1" {
		t.Errorf("address = %q, want 10.0.0.1", got)
	}
}

func TestDisplayTagsEmpty(t *testing.T) {
	if got := displayTags(map[string]any{}); got != "" {
		t.Errorf("expected no tags, got %q", got)
	}
	if got := dataplaneAddress(map[string]any{}); got != "" {
		t.Errorf("expected no address, got %q", got)
	}
}

func TestAgePrefersModificationTime(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	item := map[string]any{
		"creationTime":     "2026-09-01T12:00:00Z",
		"modificationTime": "2026-09-08T10:00:00Z",
	}
	if got := age(item, now); got != "2h" {
		t.Errorf("age = %q, want 2h from modificationTime", got)
	}

	// Falls back to creationTime, then to a placeholder.
	if got := age(map[string]any{"creationTime": "2026-09-08T11:00:00Z"}, now); got != "1h" {
		t.Errorf("age = %q, want 1h from creationTime", got)
	}
	if got := age(map[string]any{}, now); got != "-" {
		t.Errorf("age = %q, want -", got)
	}
	if got := age(map[string]any{"modificationTime": "not-a-time"}, now); got != "-" {
		t.Errorf("age = %q, want - for an unparseable time", got)
	}
}

func TestItemsFrom(t *testing.T) {
	// A list arrives wrapped in an envelope.
	items, err := itemsFrom([]byte(`{"total":2,"items":[{"name":"a"},{"name":"b"}],"next":null}`), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0]["name"] != "a" {
		t.Errorf("unexpected items: %v", items)
	}

	// A single named resource arrives bare.
	items, err = itemsFrom([]byte(`{"name":"default","type":"Mesh"}`), "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["name"] != "default" {
		t.Errorf("unexpected item: %v", items)
	}
}

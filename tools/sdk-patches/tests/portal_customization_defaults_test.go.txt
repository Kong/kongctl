package components

import (
	"encoding/json"
	"testing"
)

func TestPortalCustomizationThemeNamePreservesOmission(t *testing.T) {
	for _, name := range []*string{nil, new("ocean"), new("glacier")} {
		request := PortalCustomization{Theme: &Theme{Name: name}}
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			Theme map[string]any `json:"theme"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatal(err)
		}
		actual, present := payload.Theme["name"]
		if name == nil {
			if present {
				t.Fatalf("omitted theme name serialized: %s", data)
			}
		} else if !present || actual != *name {
			t.Fatalf("theme name = %v, want %s", actual, *name)
		}
	}
	var request PortalCustomization
	if err := json.Unmarshal([]byte(`{"theme":{"mode":"light"}}`), &request); err != nil {
		t.Fatal(err)
	}
	if request.Theme == nil || request.Theme.Name != nil {
		t.Fatalf("decoding introduced a theme name: %+v", request.Theme)
	}
}

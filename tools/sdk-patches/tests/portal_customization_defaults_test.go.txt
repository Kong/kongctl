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

func TestPortalCustomizationTryItUIAudiencePreservesOmission(t *testing.T) {
	for _, tc := range []struct {
		name     string
		audience *PortalCustomizationTryItUIAudience
	}{
		{"omitted", nil},
		{"all", PortalCustomizationTryItUIAudienceAll.ToPointer()},
		{"authenticated", PortalCustomizationTryItUIAudienceAuthenticated.ToPointer()},
		{"registered", PortalCustomizationTryItUIAudienceRegistered.ToPointer()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := PortalCustomization{SpecRenderer: &PortalCustomizationSpecRenderer{
				TryItUI: new(false), TryItUIAudience: tc.audience,
			}}
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				SpecRenderer map[string]any `json:"spec_renderer"`
			}
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}
			actual, present := payload.SpecRenderer["try_it_ui_audience"]
			if tc.audience == nil {
				if present {
					t.Fatalf("omitted audience serialized: %s", data)
				}
			} else if !present || actual != string(*tc.audience) {
				t.Fatalf("audience = %v, want %s", actual, *tc.audience)
			}
			if payload.SpecRenderer["try_it_ui"] != false {
				t.Fatalf("explicit try_it_ui=false lost: %s", data)
			}
			var decoded PortalCustomization
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.SpecRenderer == nil {
				t.Fatal("spec_renderer lost during decoding")
			}
			got := decoded.SpecRenderer.TryItUIAudience
			if (got == nil) != (tc.audience == nil) || (got != nil && *got != *tc.audience) {
				t.Fatalf("decoded audience = %v, want %v", got, tc.audience)
			}
		})
	}
}

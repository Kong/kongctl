package telemetry

import "testing"

func TestAreaFor(t *testing.T) {
	cases := map[string]string{
		"":                     AreaOther,
		"kongctl":              AreaOther,
		"kongctl apply":        AreaDeclarative,
		"kongctl sync":         AreaDeclarative,
		"kongctl diff":         AreaDeclarative,
		"kongctl plan":         AreaDeclarative,
		"kongctl adopt":        AreaDeclarative,
		"kongctl dump":         AreaDeclarative,
		"kongctl get apis":     AreaKonnectImperative,
		"kongctl list portals": AreaKonnectImperative,
		"kongctl create api":   AreaKonnectImperative,
		"kongctl delete api":   AreaKonnectImperative,
		"kongctl view":         AreaKonnectImperative,
		"kongctl ps":           AreaKonnectImperative,
		"kongctl login":        AreaAuth,
		"kongctl logout":       AreaAuth,
		"kongctl config show":  AreaConfig,
		"kongctl version":      AreaOther,
		"kongctl explain":      AreaOther,
	}
	for path, want := range cases {
		if got := AreaFor(path); got != want {
			t.Errorf("AreaFor(%q) = %q, want %q", path, got, want)
		}
	}
}

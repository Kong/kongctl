package mesh

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestSplitTagValues(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]string
		want map[string][]string
	}{
		{"no tags yields nothing to send", nil, nil},
		{"empty map yields nothing to send", map[string]string{}, nil},
		{
			"a single value",
			map[string]string{"kuma.io/service": "web"},
			map[string][]string{"kuma.io/service": {"web"}},
		},
		{
			// kumactl splits on commas so one flag can carry several values.
			"commas separate multiple values",
			map[string]string{"kuma.io/service": "web,web-api"},
			map[string][]string{"kuma.io/service": {"web", "web-api"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitTagValues(tc.raw)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if !maps.EqualFunc(got, tc.want, slices.Equal) {
				t.Errorf("tags = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRequireValidFor(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
		wantErr  bool
	}{
		{"a day", 24 * time.Hour, "24h0m0s", false},
		{"a minute", time.Minute, "1m0s", false},
		// A token with no expiry would be accepted by the control plane, so it
		// is refused here rather than sent.
		{"zero is refused", 0, "", true},
		{"negative is refused", -time.Hour, "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmdObj := newDataplaneTokenCmd(nil)
			if err := cmdObj.Flags().Set(tokenValidForFlagName, tc.duration.String()); err != nil {
				t.Fatal(err)
			}

			got, err := requireValidFor(cmdObj)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				if !strings.Contains(err.Error(), tokenValidForFlagName) {
					t.Errorf("error should name the flag, got %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("validFor = %q, want %q", got, tc.want)
			}
		})
	}
}

// A zone token must carry a scope by default. Omitting it makes the control
// plane answer 500 instead of falling back to the distribution's full scope,
// and kumactl defaults the same way. Do not remove this default without
// confirming the control plane handles an absent scope.
func TestZoneTokenDefaultsToControlPlaneScope(t *testing.T) {
	cmdObj := newZoneTokenCmd(nil)

	scope, err := cmdObj.Flags().GetStringSlice(tokenScopeFlagName)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(scope, []string{controlPlaneZoneScope}) {
		t.Errorf("default scope = %v, want [%s]", scope, controlPlaneZoneScope)
	}
}

// Empty fields are omitted so the control plane applies its own defaults,
// while the fields it requires are always present.
func TestDataplaneTokenRequestOmitsEmptyFields(t *testing.T) {
	body, err := json.Marshal(dataplaneTokenRequest{Mesh: "default", ValidFor: "24h0m0s"})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}

	for _, required := range []string{"mesh", "validFor"} {
		if _, ok := got[required]; !ok {
			t.Errorf("%s must always be sent, got %s", required, body)
		}
	}
	for _, omitted := range []string{"name", "tags", "type", "workload"} {
		if _, ok := got[omitted]; ok {
			t.Errorf("%s should be omitted when empty, got %s", omitted, body)
		}
	}
}

func TestZoneTokenRequestShape(t *testing.T) {
	body, err := json.Marshal(zoneTokenRequest{
		Zone: "zone-1", Scope: []string{controlPlaneZoneScope}, ValidFor: "24h0m0s",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"zone":"zone-1","scope":["cp"],"validFor":"24h0m0s"}`
	if string(body) != want {
		t.Errorf("body = %s, want %s", body, want)
	}
}

// Both commands take no positional arguments; everything is a flag.
func TestTokenCommandsRejectPositionalArgs(t *testing.T) {
	if err := newDataplaneTokenCmd(nil).Args(newDataplaneTokenCmd(nil), []string{"stray"}); err == nil {
		t.Error("dataplane-token should reject positional arguments")
	}
	if err := newZoneTokenCmd(nil).Args(newZoneTokenCmd(nil), []string{"stray"}); err == nil {
		t.Error("zone-token should reject positional arguments")
	}
}

package mesh

import (
	"encoding/json"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
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
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "a day", raw: (24 * time.Hour).String(), want: "24h0m0s"},
		{name: "a minute", raw: time.Minute.String(), want: "1m0s"},
		// A token with no expiry would be accepted by the control plane, so it
		// is refused here rather than sent.
		{name: "zero is refused", raw: "0s", wantErr: true},
		{name: "negative is refused", raw: (-time.Hour).String(), wantErr: true},
		// Nothing configured and no flag given is the same refusal: the
		// requirement is checked after resolution, not by MarkFlagRequired.
		{name: "absent is refused", raw: "", wantErr: true},
		{name: "unparseable is refused", raw: "soon", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := meshTestConfig(t, map[string]any{
				meshcommon.TokenValidForConfigPath: tc.raw,
			})

			got, err := requireValidFor(cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				// The message has to name a way to supply the value, since
				// either the flag or configuration will do.
				if !strings.Contains(err.Error(), tokenValidForFlagName) &&
					!strings.Contains(err.Error(), "token lifetime") {
					t.Errorf("error should name the flag or the value, got %q", err)
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

// The configurable token options must resolve with the flag winning over an
// environment variable, which wins over the configuration file. The control
// plane selection flags already behaved this way; these did not exist as
// configuration at all.
// Mesh options resolve through configuration, so the file, the environment
// and the flag must layer in that order.
//
// This claimed environment coverage with an envValue field that no case set
// and nothing read, so the environment rung was never exercised.
func TestMeshOptionPrecedence(t *testing.T) {
	cases := []struct {
		name       string
		configPath string
		flagName   string
		envVar     string
		fileValue  string
		envValue   string
		flagValue  string
		want       string
	}{
		{
			name:       "token lifetime from the file",
			configPath: meshcommon.TokenValidForConfigPath,
			flagName:   meshcommon.TokenValidForFlagName,
			fileValue:  "1h0m0s",
			want:       "1h0m0s",
		},
		{
			name:       "the flag wins over the file",
			configPath: meshcommon.TokenValidForConfigPath,
			flagName:   meshcommon.TokenValidForFlagName,
			fileValue:  "1h0m0s",
			flagValue:  "5m0s",
			want:       "5m0s",
		},
		{
			name:       "token lifetime from the environment",
			configPath: meshcommon.TokenValidForConfigPath,
			flagName:   meshcommon.TokenValidForFlagName,
			envVar:     "KONGCTL_DEFAULT_KONNECT_MESH_TOKEN_VALID_FOR",
			envValue:   "30m0s",
			want:       "30m0s",
		},
		{
			name:       "the environment wins over the file",
			configPath: meshcommon.TokenValidForConfigPath,
			flagName:   meshcommon.TokenValidForFlagName,
			envVar:     "KONGCTL_DEFAULT_KONNECT_MESH_TOKEN_VALID_FOR",
			fileValue:  "1h0m0s",
			envValue:   "30m0s",
			want:       "30m0s",
		},
		{
			name:       "the flag wins over the environment",
			configPath: meshcommon.TokenValidForConfigPath,
			flagName:   meshcommon.TokenValidForFlagName,
			envVar:     "KONGCTL_DEFAULT_KONNECT_MESH_TOKEN_VALID_FOR",
			fileValue:  "1h0m0s",
			envValue:   "30m0s",
			flagValue:  "5m0s",
			want:       "5m0s",
		},
		{
			name:       "control plane id from the environment",
			configPath: meshcommon.ControlPlaneIDConfigPath,
			flagName:   meshcommon.ControlPlaneIDFlagName,
			envVar:     "KONGCTL_DEFAULT_KONNECT_MESH_CONTROL_PLANE_ID",
			envValue:   "from-the-environment",
			want:       "from-the-environment",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envVar != "" {
				t.Setenv(tc.envVar, tc.envValue)
			}

			settings := map[string]any{}
			if tc.fileValue != "" {
				settings[tc.configPath] = tc.fileValue
			}
			cfg := meshTestConfigWithEnv(t, settings)

			flags := pflag.NewFlagSet("precedence", pflag.ContinueOnError)
			flags.String(tc.flagName, "", "")
			if tc.flagValue != "" {
				require.NoError(t, flags.Set(tc.flagName, tc.flagValue))
			}
			require.NoError(t, cfg.BindFlag(tc.configPath, flags.Lookup(tc.flagName)))

			require.Equal(t, tc.want, cfg.GetString(tc.configPath))
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

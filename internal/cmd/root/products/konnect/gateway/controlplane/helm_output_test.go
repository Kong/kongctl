package controlplane

import (
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func TestParseEndpoint(t *testing.T) {
	cases := []struct {
		in       string
		wantHost string
		wantPort string
	}{
		{"https://f47a4e996f.us.cp.konghq.com", "f47a4e996f.us.cp.konghq.com", ""},
		{"https://f47a4e996f.us.cp.konghq.com/", "f47a4e996f.us.cp.konghq.com", ""},
		{"f47a4e996f.us.cp.konghq.com", "f47a4e996f.us.cp.konghq.com", ""},
		{"f47a4e996f.us.tp.konghq.com:443", "f47a4e996f.us.tp.konghq.com", "443"},
		{"https://[2001:db8::1]:8443", "2001:db8::1", "8443"},
		{"[2001:db8::1]:443", "2001:db8::1", "443"},
		{"[2001:db8::1]", "2001:db8::1", ""},
	}
	for _, c := range cases {
		host, port, err := parseEndpoint(c.in)
		if err != nil {
			t.Errorf("parseEndpoint(%q) error: %v", c.in, err)
			continue
		}
		if host != c.wantHost || port != c.wantPort {
			t.Errorf("parseEndpoint(%q) = (%q, %q), want (%q, %q)",
				c.in, host, port, c.wantHost, c.wantPort)
		}
	}
}

func TestParseEndpoint_Empty(t *testing.T) {
	if _, _, err := parseEndpoint(""); err == nil {
		t.Fatal("expected error for empty endpoint")
	}
}

func TestRenderHelmValues(t *testing.T) {
	cp := &kkComps.ControlPlane{
		Config: kkComps.ControlPlaneConfig{
			ControlPlaneEndpoint: "https://f47a4e996f.us.cp.konghq.com",
			TelemetryEndpoint:    "https://f47a4e996f.us.tp.konghq.com",
		},
	}
	out, err := renderHelmValues(cp)
	if err != nil {
		t.Fatalf("renderHelmValues: %v", err)
	}
	mustContain := []string{
		`cluster_control_plane: "f47a4e996f.us.cp.konghq.com"`,
		`cluster_telemetry_endpoint: "f47a4e996f.us.tp.konghq.com:443"`,
		`cluster_telemetry_server_name: "f47a4e996f.us.tp.konghq.com"`,
		"role: data_plane",
		"konnect_mode: \"on\"",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("rendered output missing %q\n---\n%s", s, out)
		}
	}
}

func TestRenderHelmValues_TelemetryWithExplicitPort(t *testing.T) {
	cp := &kkComps.ControlPlane{
		Config: kkComps.ControlPlaneConfig{
			ControlPlaneEndpoint: "https://example.us.cp.konghq.com",
			TelemetryEndpoint:    "https://example.us.tp.konghq.com:8443",
		},
	}
	out, err := renderHelmValues(cp)
	if err != nil {
		t.Fatalf("renderHelmValues: %v", err)
	}
	if !strings.Contains(out, `cluster_telemetry_endpoint: "example.us.tp.konghq.com:8443"`) {
		t.Errorf("expected explicit port preserved, got:\n%s", out)
	}
	if !strings.Contains(out, `cluster_telemetry_server_name: "example.us.tp.konghq.com"`) {
		t.Errorf("expected host-only server name, got:\n%s", out)
	}
}

func TestRenderHelmValues_IPv6Telemetry(t *testing.T) {
	cp := &kkComps.ControlPlane{
		Config: kkComps.ControlPlaneConfig{
			ControlPlaneEndpoint: "https://[2001:db8::1]",
			TelemetryEndpoint:    "https://[2001:db8::2]",
		},
	}
	out, err := renderHelmValues(cp)
	if err != nil {
		t.Fatalf("renderHelmValues: %v", err)
	}
	if !strings.Contains(out, `cluster_telemetry_endpoint: "[2001:db8::2]:443"`) {
		t.Errorf("expected bracketed IPv6 host:port with default port, got:\n%s", out)
	}
	if !strings.Contains(out, `cluster_telemetry_server_name: "2001:db8::2"`) {
		t.Errorf("expected unbracketed IPv6 server name, got:\n%s", out)
	}
	if !strings.Contains(out, `cluster_control_plane: "2001:db8::1"`) {
		t.Errorf("expected unbracketed IPv6 CP host, got:\n%s", out)
	}
}

func TestRenderHelmValues_MissingEndpoints(t *testing.T) {
	cp := &kkComps.ControlPlane{}
	if _, err := renderHelmValues(cp); err == nil {
		t.Fatal("expected error when endpoints are empty")
	}
}

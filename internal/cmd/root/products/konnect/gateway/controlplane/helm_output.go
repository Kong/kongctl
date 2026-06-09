package controlplane

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"strings"
	"text/template"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

const helmValuesTemplate = `ingressController:
  enabled: false

image:
  repository: kong/kong-gateway
  tag: "latest"

secretVolumes:
  - kong-cluster-cert

env:
  role: data_plane
  database: "off"
  konnect_mode: "on"
  vitals: "off"
  cluster_mtls: pki

  cluster_control_plane: "{{ .ControlPlaneHost }}"
  cluster_telemetry_endpoint: "{{ .TelemetryHostPort }}"
  cluster_telemetry_server_name: "{{ .TelemetryHost }}"
  cluster_cert: /etc/secrets/kong-cluster-cert/tls.crt
  cluster_cert_key: /etc/secrets/kong-cluster-cert/tls.key

  lua_ssl_trusted_certificate: system
  tls_certificate_verify: "off"
  proxy_access_log: "off"
  dns_stale_ttl: "3600"

resources:
  requests:
    cpu: 1
    memory: "2Gi"

proxy:
  enabled: true

admin:
  enabled: false

manager:
  enabled: false
`

type helmValues struct {
	ControlPlaneHost  string
	TelemetryHost     string
	TelemetryHostPort string
}

// parseEndpoint extracts (hostname, port) from a Konnect endpoint. The input
// may be a full URL ("https://host[:port]"), a bare "host[:port]", or an IPv6
// literal in bracketed form ("[::1]:443"). port is "" if not specified.
func parseEndpoint(endpoint string) (host, port string, err error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", "", fmt.Errorf("endpoint is empty")
	}
	s := endpoint
	if !strings.Contains(s, "://") {
		// url.Parse needs a scheme or "//" prefix to populate Host correctly.
		s = "//" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", "", fmt.Errorf("parse endpoint %q: %w", endpoint, err)
	}
	if u.Hostname() == "" {
		return "", "", fmt.Errorf("endpoint %q has no host", endpoint)
	}
	return u.Hostname(), u.Port(), nil
}

func renderHelmValues(cp *kkComps.ControlPlane) (string, error) {
	if cp == nil {
		return "", fmt.Errorf("control plane is nil")
	}
	cpHost, _, err := parseEndpoint(cp.Config.ControlPlaneEndpoint)
	if err != nil {
		return "", fmt.Errorf("control plane endpoint: %w", err)
	}
	tpHost, tpPort, err := parseEndpoint(cp.Config.TelemetryEndpoint)
	if err != nil {
		return "", fmt.Errorf("telemetry endpoint: %w", err)
	}
	if tpPort == "" {
		tpPort = "443"
	}

	values := helmValues{
		ControlPlaneHost:  cpHost,
		TelemetryHost:     tpHost,
		TelemetryHostPort: net.JoinHostPort(tpHost, tpPort),
	}

	tmpl, err := template.New("helm-values").Parse(helmValuesTemplate)
	if err != nil {
		return "", fmt.Errorf("parse helm template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, values); err != nil {
		return "", fmt.Errorf("render helm template: %w", err)
	}
	return buf.String(), nil
}

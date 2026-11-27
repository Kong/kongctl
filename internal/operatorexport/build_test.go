package operatorexport

import (
	"bytes"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const (
	cpID       = "6f0c1d2e-0000-4000-8000-000000000001"
	serviceID  = "2b1a3c4d-0000-4000-8000-000000000002"
	routeID    = "3c2b4d5e-0000-4000-8000-000000000003"
	pluginID   = "4d3c5e6f-0000-4000-8000-000000000004"
	consumerID = "5e4d6f70-0000-4000-8000-000000000005"
	groupID    = "6f5e7081-0000-4000-8000-000000000006"
	certID     = "70a1b2c3-0000-4000-8000-000000000007"
)

func defaultOptions() Options {
	return Options{Namespace: "kong", AuthRef: "konnect-api-auth", AdoptMode: AdoptModeMatch}
}

func baseSnapshot() Snapshot {
	return Snapshot{ControlPlane: ControlPlane{
		ID: cpID, Name: "gateway-control-plane", ClusterType: "CLUSTER_TYPE_CONTROL_PLANE",
	}}
}

func findManifest(t *testing.T, result Result, kind, name string) map[string]any {
	t.Helper()
	for _, m := range result.Manifests {
		if m.Kind == kind && m.Name == name {
			return m.Object
		}
	}
	t.Fatalf("manifest %s/%s not found; have %v", kind, name, manifestKeys(result))
	return nil
}

func manifestKeys(result Result) []string {
	keys := make([]string, 0, len(result.Manifests))
	for _, m := range result.Manifests {
		keys = append(keys, m.Kind+"/"+m.Name)
	}
	return keys
}

func hasKind(result Result, kind string) bool {
	for _, m := range result.Manifests {
		if m.Kind == kind {
			return true
		}
	}
	return false
}

func warningsContaining(result Result, substr string) []Warning {
	var matches []Warning
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, substr) {
			matches = append(matches, w)
		}
	}
	return matches
}

// roundTrip serializes and parses a manifest so assertions see what kubectl
// would receive.
func roundTrip(t *testing.T, obj map[string]any) map[string]any {
	t.Helper()
	raw, err := yaml.Marshal(obj)
	require.NoError(t, err)
	parsed := map[string]any{}
	require.NoError(t, yaml.Unmarshal(raw, &parsed))
	return parsed
}

func TestBuildControlPlaneIsMirror(t *testing.T) {
	result, err := Build(baseSnapshot(), defaultOptions())
	require.NoError(t, err)

	cp := roundTrip(t, findManifest(t, result, KindControlPlane, "gateway-control-plane"))
	assert.Equal(t, "konnect.konghq.com/v1alpha2", cp["apiVersion"])
	assert.Equal(t, map[string]any{
		"source":  "Mirror",
		"mirror":  map[string]any{"konnect": map[string]any{"id": cpID}},
		"konnect": map[string]any{"authRef": map[string]any{"name": "konnect-api-auth"}},
	}, cp["spec"])
	assert.Equal(t, "kong", cp["metadata"].(map[string]any)["namespace"])
}

func TestBuildRejectsGroupsAndKIC(t *testing.T) {
	for _, clusterType := range []string{clusterTypeGroup, clusterTypeKIC} {
		snap := baseSnapshot()
		snap.ControlPlane.ClusterType = clusterType
		_, err := Build(snap, defaultOptions())
		require.Error(t, err, clusterType)
	}
}

func TestOptionsValidate(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Options)
	}{
		{"empty namespace", func(o *Options) { o.Namespace = "" }},
		{"invalid namespace", func(o *Options) { o.Namespace = "Kong_NS" }},
		{"empty auth ref", func(o *Options) { o.AuthRef = " " }},
		{"unknown mode", func(o *Options) { o.AdoptMode = "merge" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := defaultOptions()
			tt.mut(&opts)
			require.Error(t, opts.Validate())
		})
	}
	require.NoError(t, defaultOptions().Validate())
}

func TestBuildServiceReproducesAllFields(t *testing.T) {
	snap := baseSnapshot()
	snap.Services = []kkComps.Service{{
		ID:             new(serviceID),
		Name:           new("demo-service"),
		Host:           "httpbin.konghq.com",
		Path:           new("/anything"),
		Port:           new(int64(443)),
		Protocol:       new(kkComps.ServiceProtocolHTTPS),
		ConnectTimeout: new(int64(1000)),
		ReadTimeout:    new(int64(2000)),
		WriteTimeout:   new(int64(3000)),
		Retries:        new(int64(2)),
		Enabled:        new(false),
		TLSVerify:      new(true),
		Tags:           []string{"team:a", "k8s-name:old"},
	}}

	result, err := Build(snap, defaultOptions())
	require.NoError(t, err)

	svc := roundTrip(t, findManifest(t, result, KindService, "demo-service"))
	spec := svc["spec"].(map[string]any)
	assert.Equal(t, "configuration.konghq.com/v1alpha1", svc["apiVersion"])
	assert.Equal(t, "demo-service", spec["name"])
	assert.Equal(t, "httpbin.konghq.com", spec["host"])
	assert.Equal(t, "/anything", spec["path"])
	assert.EqualValues(t, 443, spec["port"])
	assert.Equal(t, "https", spec["protocol"])
	assert.EqualValues(t, 1000, spec["connect_timeout"])
	assert.EqualValues(t, 2000, spec["read_timeout"])
	assert.EqualValues(t, 3000, spec["write_timeout"])
	assert.EqualValues(t, 2, spec["retries"])
	assert.Equal(t, false, spec["enabled"])
	assert.Equal(t, true, spec["tls_verify"])
	assert.Equal(t, []any{"team:a"}, spec["tags"])
	assert.NotContains(t, spec, "id")
	assert.Equal(t, map[string]any{
		"type":                 "konnectNamespacedRef",
		"konnectNamespacedRef": map[string]any{"name": "gateway-control-plane"},
	}, spec["controlPlaneRef"])
	assert.Equal(t, map[string]any{
		"from": "konnect", "mode": "match", "konnect": map[string]any{"id": serviceID},
	}, spec["adopt"])
	assert.Len(t, warningsContaining(result, "removed reserved tags"), 1)
}

func TestBuildServiceKeepsCertificateReferenceWhenCertificateSkipped(t *testing.T) {
	snap := baseSnapshot()
	snap.Certificates = []kkComps.Certificate{{ID: new(certID), Cert: "CERT", Key: "KEY"}}
	snap.Services = []kkComps.Service{{
		ID: new(serviceID), Name: new("svc"), Host: "example.com",
		ClientCertificate: &kkComps.ClientCertificate{ID: new(certID)},
	}}

	result, err := Build(snap, defaultOptions())
	require.NoError(t, err)

	assert.False(t, hasKind(result, KindCertificate), "certificate must be skipped without --include-secrets")
	spec := findManifest(t, result, KindService, "svc")["spec"].(map[string]any)
	assert.Equal(t, map[string]any{"name": "certificate-70a1b2c3"}, spec["clientCertificateRef"],
		"the reference must survive so the operator does not clear the client certificate")
	assert.NotEmpty(t, warningsContaining(result, "was not exported"))
}

func TestBuildSecretsAreOptIn(t *testing.T) {
	snap := baseSnapshot()
	snap.Consumers = []kkComps.Consumer{{ID: new(consumerID), Username: new("alice")}}
	consumer := &kkComps.KeyAuthConsumer{ID: new(consumerID)}
	snap.KeyAuths = []kkComps.KeyAuth{
		{ID: new("8a000000-0000-4000-8000-000000000008"), Key: new("s3cr3t"), Consumer: consumer},
	}
	snap.Certificates = []kkComps.Certificate{{ID: new(certID), Cert: "CERT", Key: "KEY"}}

	without, err := Build(snap, defaultOptions())
	require.NoError(t, err)
	assert.False(t, hasKind(without, KindCredentialAPIKey))
	assert.False(t, hasKind(without, KindCertificate))
	var out bytes.Buffer
	require.NoError(t, Write(&out, without.Manifests))
	assert.NotContains(t, out.String(), "s3cr3t")
	assert.NotContains(t, out.String(), "KEY")

	opts := defaultOptions()
	opts.IncludeSecrets = true
	with, err := Build(snap, opts)
	require.NoError(t, err)
	key := findManifest(t, with, KindCredentialAPIKey, "alice-key-auth-8a000000")["spec"].(map[string]any)
	assert.Equal(t, "s3cr3t", key["key"])
	assert.Equal(t, map[string]any{"name": "alice"}, key["consumerRef"])
	cert := findManifest(t, with, KindCertificate, "certificate-70a1b2c3")["spec"].(map[string]any)
	assert.Equal(t, "inline", cert["type"])
	assert.Equal(t, "KEY", cert["key"])
}

func TestBuildBasicAuthAlwaysSkipped(t *testing.T) {
	snap := baseSnapshot()
	snap.BasicAuths = []kkComps.BasicAuth{{ID: new("9b000000-0000-4000-8000-000000000009"), Username: "bob"}}
	opts := defaultOptions()
	opts.IncludeSecrets = true

	result, err := Build(snap, opts)
	require.NoError(t, err)
	assert.False(t, hasKind(result, KindCredentialBasic))
	assert.Len(t, warningsContaining(result, "password hash"), 1)
}

func TestBuildRoutes(t *testing.T) {
	snap := baseSnapshot()
	snap.Services = []kkComps.Service{{ID: new(serviceID), Name: new("svc"), Host: "example.com"}}
	snap.Routes = []kkComps.Route{
		kkComps.CreateRouteRouteJSON(kkComps.RouteJSON{
			ID: new(routeID), Name: new("demo-route"), Paths: []string{"/anything"},
			StripPath: new(false), Service: &kkComps.RouteJSONService{ID: new(serviceID)},
		}),
		kkComps.CreateRouteRouteJSON(kkComps.RouteJSON{
			ID: new("a1000000-0000-4000-8000-00000000000a"), Paths: []string{"/orphan"},
		}),
		kkComps.CreateRouteRouteExpression(kkComps.RouteExpression{
			ID: new("b2000000-0000-4000-8000-00000000000b"), Name: new("expr"),
		}),
	}

	result, err := Build(snap, defaultOptions())
	require.NoError(t, err)

	spec := roundTrip(t, findManifest(t, result, KindRoute, "demo-route"))["spec"].(map[string]any)
	assert.Equal(t, false, spec["strip_path"])
	assert.Equal(t, map[string]any{
		"type": "namespacedRef", "namespacedRef": map[string]any{"name": "svc"},
	}, spec["serviceRef"])
	assert.NotContains(t, spec, "controlPlaneRef")

	orphan := findManifest(t, result, KindRoute, "route-a1000000")["spec"].(map[string]any)
	assert.Contains(t, orphan, "controlPlaneRef")

	assert.Len(t, warningsContaining(result, "expression routes"), 1)
}

func TestBuildPlugins(t *testing.T) {
	snap := baseSnapshot()
	snap.Services = []kkComps.Service{{ID: new(serviceID), Name: new("svc"), Host: "example.com"}}
	snap.Plugins = []kkComps.Plugin{
		{
			ID: new(pluginID), Name: "rate-limiting", Enabled: new(false),
			Config:  map[string]any{"second": 5, "hour": 1000, "policy": "local", "redis": nil},
			Service: &kkComps.PluginService{ID: new(serviceID)},
		},
		{ID: new("c3000000-0000-4000-8000-00000000000c"), Name: "cors", InstanceName: new("global-cors")},
		{
			ID: new("d4000000-0000-4000-8000-00000000000d"), Name: "key-auth",
			Route: &kkComps.PluginRoute{ID: new("e5000000-0000-4000-8000-00000000000e")},
		},
	}

	result, err := Build(snap, defaultOptions())
	require.NoError(t, err)

	plugin := roundTrip(t, findManifest(t, result, KindPlugin, "rate-limiting"))
	assert.Equal(t, "configuration.konghq.com/v1", plugin["apiVersion"])
	assert.Equal(t, "rate-limiting", plugin["plugin"])
	assert.Equal(t, true, plugin["disabled"])
	config := plugin["config"].(map[string]any)
	assert.Contains(t, config, "redis", "explicit nulls must be kept for the operator's DeepEqual")
	assert.Nil(t, config["redis"])
	assert.NotContains(t, plugin, "spec")

	binding := findManifest(t, result, KindPluginBinding, "rate-limiting")["spec"].(map[string]any)
	assert.Equal(t, map[string]any{"name": "rate-limiting", "kind": "KongPlugin"}, binding["pluginRef"])
	assert.Equal(t, "OnlyTargets", binding["scope"])
	assert.Equal(t, map[string]any{
		"serviceRef": map[string]any{"name": "svc", "kind": "KongService", "group": "configuration.konghq.com"},
	}, binding["targets"])

	global := findManifest(t, result, KindPluginBinding, "global-cors")["spec"].(map[string]any)
	assert.Equal(t, "GlobalInControlPlane", global["scope"])
	assert.NotContains(t, global, "targets")

	assert.Len(t, warningsContaining(result, "attached to route"), 1)
	for _, m := range result.Manifests {
		assert.NotEqual(t, "key-auth", m.Name)
	}
}

func TestBuildConsumersAndGroups(t *testing.T) {
	snap := baseSnapshot()
	snap.ConsumerGroups = []kkComps.ConsumerGroup{{ID: new(groupID), Name: "gold"}}
	snap.Consumers = []kkComps.Consumer{{ID: new(consumerID), CustomID: new("cust-1")}}
	snap.ConsumerGroupMembers = map[string][]string{groupID: {consumerID}}

	result, err := Build(snap, defaultOptions())
	require.NoError(t, err)

	group := findManifest(t, result, KindConsumerGroup, "gold")
	assert.Equal(t, "configuration.konghq.com/v1beta1", group["apiVersion"])

	consumer := findManifest(t, result, KindConsumer, "cust-1")
	assert.Equal(t, "configuration.konghq.com/v1", consumer["apiVersion"])
	assert.Equal(t, "cust-1", consumer["custom_id"])
	assert.NotContains(t, consumer, "username")
	assert.Equal(t, []any{"gold"}, consumer["consumerGroups"])
}

func TestBuildDataPlaneCertificateIsAlwaysMatch(t *testing.T) {
	snap := baseSnapshot()
	snap.DataPlaneCertificates = []kkComps.DataPlaneClientCertificate{
		{ID: new("f6000000-0000-4000-8000-00000000000f"), Cert: new("CERT")},
	}
	opts := defaultOptions()
	opts.AdoptMode = AdoptModeOverride

	result, err := Build(snap, opts)
	require.NoError(t, err)
	spec := findManifest(t, result, KindDataPlaneClientCrt, "dataplane-certificate-f6000000")["spec"].(map[string]any)
	assert.Equal(t, "match", spec["adopt"].(map[string]any)["mode"])
}

func TestBuildVaultIsClusterScoped(t *testing.T) {
	snap := baseSnapshot()
	snap.Vaults = []kkComps.Vault{
		{
			ID:     new("a7000000-0000-4000-8000-000000000010"),
			Name:   "env",
			Prefix: "my-env",
			Config: map[string]any{"prefix": "KONG_"},
		},
		{
			ID:     new("b8000000-0000-4000-8000-000000000011"),
			Name:   "hcv",
			Prefix: "hcv",
			Config: map[string]any{"token": "t0k3n"},
		},
	}

	result, err := Build(snap, defaultOptions())
	require.NoError(t, err)

	vault := findManifest(t, result, KindVault, "gateway-control-plane-my-env")
	assert.NotContains(t, vault["metadata"].(map[string]any), "namespace")
	spec := vault["spec"].(map[string]any)
	assert.Equal(t, "env", spec["backend"])
	assert.Equal(t, map[string]any{"name": "gateway-control-plane", "namespace": "kong"},
		spec["controlPlaneRef"].(map[string]any)["konnectNamespacedRef"])

	assert.Len(t, warningsContaining(result, "config contains credentials"), 1)
}

func TestBuildSkipsEntitiesOwnedByKubernetes(t *testing.T) {
	snap := baseSnapshot()
	snap.Services = []kkComps.Service{{
		ID: new(serviceID), Name: new("svc"), Host: "example.com",
		Tags: []string{"k8s-uid:1234", "managed-by:kong-operator"},
	}}

	result, err := Build(snap, defaultOptions())
	require.NoError(t, err)
	assert.False(t, hasKind(result, KindService))
	assert.Len(t, warningsContaining(result, "already managed"), 1)
}

func TestBuildWarnsOnUnrepresentedFields(t *testing.T) {
	snap := baseSnapshot()
	snap.Targets = []kkComps.Target{{
		ID: new("c9000000-0000-4000-8000-000000000012"), Target: new("10.0.0.1:80"), Weight: new(int64(100)),
		Failover: new(true), Upstream: &kkComps.TargetUpstream{ID: new("d0000000-0000-4000-8000-000000000013")},
	}}

	result, err := Build(snap, defaultOptions())
	require.NoError(t, err)
	assert.Len(t, warningsContaining(result, "fields failover have no equivalent"), 1)
}

func TestNameAllocatorDisambiguatesCollisions(t *testing.T) {
	a := newNameAllocator()
	a.propose(KindService, "11111111-aaaa", "Demo Service", "service")
	a.propose(KindService, "22222222-bbbb", "demo_service", "service")
	a.propose(KindService, "33333333-cccc", "unique", "service")
	a.propose(KindService, "44444444-dddd", "!!!", "service")

	assert.Equal(t, "demo-service-11111111", a.name(KindService, "11111111-aaaa"))
	assert.Equal(t, "demo-service-22222222", a.name(KindService, "22222222-bbbb"))
	assert.Equal(t, "unique", a.name(KindService, "33333333-cccc"))
	assert.Equal(t, "service-44444444", a.name(KindService, "44444444-dddd"))
	assert.Empty(t, a.name(KindService, "unknown"))
}

func TestSanitizeName(t *testing.T) {
	assert.Equal(t, "wildcard.example.com", sanitizeName("*.example.com"))
	assert.Equal(t, "my-api-v1", sanitizeName("  My API_v1 "))
	assert.Empty(t, sanitizeName("---"))
}

func TestWriteProducesYAMLStream(t *testing.T) {
	result, err := Build(baseSnapshot(), defaultOptions())
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, Write(&out, result.Manifests))
	assert.True(t, strings.HasPrefix(out.String(), "# Kong Operator resources"))
	assert.Equal(t, len(result.Manifests), strings.Count(out.String(), "\n---\n"))
	assert.Contains(t, out.String(), "kind: KonnectGatewayControlPlane")
}

func TestBuildEmitsReferenceGrantForVaults(t *testing.T) {
	snap := baseSnapshot()
	without, err := Build(snap, defaultOptions())
	require.NoError(t, err)
	assert.False(t, hasKind(without, KindReferenceGrant), "no grant without vaults")

	snap.Vaults = []kkComps.Vault{
		{ID: new("a7000000-0000-4000-8000-000000000010"), Name: "env", Prefix: "my-env"},
	}
	result, err := Build(snap, defaultOptions())
	require.NoError(t, err)

	grant := roundTrip(t, findManifest(t, result, KindReferenceGrant, "kongvault-to-gateway-control-plane"))
	assert.Equal(t, "kong", grant["metadata"].(map[string]any)["namespace"])
	assert.Equal(t, map[string]any{
		"from": []any{map[string]any{
			"group": "configuration.konghq.com", "kind": "KongVault", "namespace": "",
		}},
		"to": []any{map[string]any{
			"group": "konnect.konghq.com", "kind": "KonnectGatewayControlPlane", "name": "gateway-control-plane",
		}},
	}, grant["spec"])
}

func TestBuildSkipsPluginsOnUnsupportedProtocols(t *testing.T) {
	snap := baseSnapshot()
	snap.Plugins = []kkComps.Plugin{{
		ID: new(pluginID), Name: "key-auth",
		Protocols: []kkComps.Protocols{
			kkComps.ProtocolsHTTP, kkComps.ProtocolsHTTPS, kkComps.ProtocolsWs, kkComps.ProtocolsWss,
		},
	}}

	result, err := Build(snap, defaultOptions())
	require.NoError(t, err)
	assert.False(t, hasKind(result, KindPlugin))
	assert.False(t, hasKind(result, KindPluginBinding))
	require.Len(t, warningsContaining(result, "protocols ws, wss"), 1)
}

func TestBuildWarnsThatPluginMatchFailsOnNullConfig(t *testing.T) {
	snap := baseSnapshot()
	snap.Plugins = []kkComps.Plugin{
		{ID: new(pluginID), Name: "cors", Config: map[string]any{"origins": nil}},
		{
			ID: new("c3000000-0000-4000-8000-00000000000c"), Name: "acl",
			Config: map[string]any{"allow": []any{"admins"}},
		},
	}

	match, err := Build(snap, defaultOptions())
	require.NoError(t, err)
	nullWarnings := warningsContaining(match, "will fail with NotMatch")
	require.Len(t, nullWarnings, 1, "only the plugin with a null config is affected")
	assert.Equal(t, "cors", nullWarnings[0].Name)

	opts := defaultOptions()
	opts.AdoptMode = AdoptModeOverride
	override, err := Build(snap, opts)
	require.NoError(t, err)
	assert.Empty(t, warningsContaining(override, "will fail with NotMatch"))
}

func adoptModeOf(t *testing.T, result Result, kind, name string) string {
	t.Helper()
	spec := findManifest(t, result, kind, name)["spec"].(map[string]any)
	return spec["adopt"].(map[string]any)["mode"].(string)
}

func TestBuildAutoModePicksModePerKind(t *testing.T) {
	snap := baseSnapshot()
	snap.Services = []kkComps.Service{{ID: new(serviceID), Name: new("svc"), Host: "example.com"}}
	snap.Upstreams = []kkComps.Upstream{{ID: new("e5000000-0000-4000-8000-000000000014"), Name: "up"}}
	snap.Plugins = []kkComps.Plugin{{ID: new(pluginID), Name: "cors", Config: map[string]any{"origins": nil}}}
	opts := defaultOptions()
	opts.AdoptMode = AdoptModeAuto

	result, err := Build(snap, opts)
	require.NoError(t, err)

	assert.Equal(t, AdoptModeMatch, adoptModeOf(t, result, KindService, "svc"))
	assert.Equal(t, AdoptModeOverride, adoptModeOf(t, result, KindUpstream, "up"))
	assert.Equal(t, AdoptModeOverride, adoptModeOf(t, result, KindPluginBinding, "cors"))
	assert.Empty(t, warningsContaining(result, "NotMatch"), "auto mode avoids the known match failures")
	assert.Empty(t, warningsContaining(result, "healthchecks"))
}

func TestBuildWarnsThatOverrideClearsServiceCertificates(t *testing.T) {
	snap := baseSnapshot()
	snap.Services = []kkComps.Service{
		{
			ID: new(serviceID), Name: new("with-certs"), Host: "example.com",
			ClientCertificate: &kkComps.ClientCertificate{ID: new(certID)},
		},
		{ID: new("f6000000-0000-4000-8000-000000000015"), Name: new("plain"), Host: "example.com"},
	}
	opts := defaultOptions()
	opts.AdoptMode = AdoptModeOverride

	result, err := Build(snap, opts)
	require.NoError(t, err)
	warnings := warningsContaining(result, "may clear the client and CA certificate references")
	require.Len(t, warnings, 1)
	assert.Equal(t, "with-certs", warnings[0].Name)

	opts.AdoptMode = AdoptModeAuto
	auto, err := Build(snap, opts)
	require.NoError(t, err)
	assert.Empty(t, warningsContaining(auto, "may clear"))
}

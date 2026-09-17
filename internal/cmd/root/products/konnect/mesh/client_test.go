package mesh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	utilviper "github.com/kong/kongctl/internal/util/viper"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// selectorCmd builds a command carrying the control plane selection flags, with
// the named ones marked as given on the command line.
func selectorCmd(t *testing.T, given ...string) *cobra.Command {
	t.Helper()

	cmdObj := &cobra.Command{Use: "mesh-selector-test"}
	meshcommon.AddControlPlaneFlags(cmdObj.Flags())
	for _, flag := range given {
		require.NoError(t, cmdObj.Flags().Set(flag, "value-for-"+flag))
	}
	return cmdObj
}

func TestExplicitControlPlaneSelector(t *testing.T) {
	cases := []struct {
		name  string
		given []string
		want  string
	}{
		{name: "nothing given", given: nil, want: ""},
		{name: "id", given: []string{meshcommon.ControlPlaneIDFlagName}, want: meshcommon.ControlPlaneIDFlagName},
		{
			name:  "name",
			given: []string{meshcommon.ControlPlaneNameFlagName},
			want:  meshcommon.ControlPlaneNameFlagName,
		},
		{name: "url", given: []string{meshcommon.ControlPlaneURLFlagName}, want: meshcommon.ControlPlaneURLFlagName},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			helper := &cmd.MockHelper{}
			helper.EXPECT().GetCmd().Return(selectorCmd(t, tc.given...))

			got, err := explicitControlPlaneSelector(helper)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// Two selectors name two different control planes, and this resolver serves
// writes and deletes, so the conflict is reported rather than resolved.
func TestExplicitControlPlaneSelectorRejectsConflicts(t *testing.T) {
	cases := [][]string{
		{meshcommon.ControlPlaneIDFlagName, meshcommon.ControlPlaneNameFlagName},
		{meshcommon.ControlPlaneURLFlagName, meshcommon.ControlPlaneIDFlagName},
		{meshcommon.ControlPlaneURLFlagName, meshcommon.ControlPlaneNameFlagName},
		{
			meshcommon.ControlPlaneURLFlagName,
			meshcommon.ControlPlaneIDFlagName,
			meshcommon.ControlPlaneNameFlagName,
		},
	}

	for _, given := range cases {
		helper := &cmd.MockHelper{}
		helper.EXPECT().GetCmd().Return(selectorCmd(t, given...))

		_, err := explicitControlPlaneSelector(helper)
		require.Error(t, err, "expected %v to conflict", given)
		require.Contains(t, err.Error(), "provide only one")
		for _, flag := range given {
			require.Contains(t, err.Error(), "--"+flag)
		}
	}
}

// A nil helper or command must not panic: some call paths build the helper
// before a command is attached.
func TestExplicitControlPlaneSelectorWithoutCommand(t *testing.T) {
	got, err := explicitControlPlaneSelector(nil)
	require.NoError(t, err)
	require.Equal(t, "", got)

	helper := &cmd.MockHelper{}
	helper.EXPECT().GetCmd().Return(nil)
	got, err = explicitControlPlaneSelector(helper)
	require.NoError(t, err)
	require.Equal(t, "", got)
}

// page builds one collection page.
func page(total int, names ...string) listEnvelope {
	items := make([]map[string]any, 0, len(names))
	for _, name := range names {
		items = append(items, map[string]any{"name": name})
	}
	return listEnvelope{Total: total, Items: items}
}

func namesOf(t *testing.T, items []map[string]any) []string {
	t.Helper()

	if items == nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name, ok := item["name"].(string)
		require.True(t, ok, "item %v has no name", item)
		names = append(names, name)
	}
	return names
}

func TestPaginateCollectsEveryPage(t *testing.T) {
	cases := []struct {
		name        string
		pages       []listEnvelope
		want        []string
		wantOffsets []int
	}{
		{
			name:        "a single full page ends the walk",
			pages:       []listEnvelope{page(2, "a", "b")},
			want:        []string{"a", "b"},
			wantOffsets: []int{0},
		},
		{
			// The case the review reproduced: one item with a total of two was
			// reported as the whole collection.
			name:        "a short first page is followed",
			pages:       []listEnvelope{page(2, "a"), page(2, "b")},
			want:        []string{"a", "b"},
			wantOffsets: []int{0, 1},
		},
		{
			name:        "several pages are concatenated in order",
			pages:       []listEnvelope{page(5, "a", "b"), page(5, "c", "d"), page(5, "e")},
			want:        []string{"a", "b", "c", "d", "e"},
			wantOffsets: []int{0, 2, 4},
		},
		{
			name:        "an empty collection fetches once",
			pages:       []listEnvelope{page(0)},
			want:        nil,
			wantOffsets: []int{0},
		},
		{
			// A control plane that keeps reporting more than it returns must
			// not spin: an empty page ends the walk whatever the total says.
			name:        "an empty page ends a total that overreports",
			pages:       []listEnvelope{page(99, "a"), page(99)},
			want:        []string{"a"},
			wantOffsets: []int{0, 1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var offsets []int
			items, err := paginate(func(offset int) (listEnvelope, error) {
				offsets = append(offsets, offset)
				return tc.pages[len(offsets)-1], nil
			})

			require.NoError(t, err)
			require.Equal(t, tc.want, namesOf(t, items))
			// Each request asks for what has not been collected yet, so a
			// page is never fetched twice or skipped.
			require.Equal(t, tc.wantOffsets, offsets)
		})
	}
}

func TestPaginatePropagatesErrors(t *testing.T) {
	wantErr := errors.New("collection request failed")

	items, err := paginate(func(int) (listEnvelope, error) {
		return listEnvelope{}, wantErr
	})

	require.ErrorIs(t, err, wantErr)
	// A partial collection must not be returned as if it were complete.
	require.Nil(t, items)
}

func TestListPayloadReportsWhatWasCollected(t *testing.T) {
	payload := listPayload([]map[string]any{{"name": "a"}, {"name": "b"}})
	require.Equal(t, 2, payload["total"])

	// An empty collection renders as an empty list rather than null, matching
	// the control plane's own envelope.
	empty := listPayload(nil)
	require.Equal(t, 0, empty["total"])
	require.Equal(t, []map[string]any{}, empty["items"])
}

// meshTestConfig builds a profiled config carrying the given settings under
// the active profile.
func meshTestConfig(t *testing.T, settings map[string]any) config.Hook {
	t.Helper()

	main := viper.New()
	main.Set("default", settings)
	return config.BuildProfiledConfig("default", "/tmp/kongctl-mesh-test-config.yaml", main)
}

// meshTestConfigWithEnv is meshTestConfig with environment variable handling
// wired up, which plain viper.New() does not do. Use it when a test needs the
// KONGCTL_<PROFILE>_... rung of the precedence chain.
func meshTestConfigWithEnv(t *testing.T, settings map[string]any) config.Hook {
	t.Helper()

	const path = "/tmp/kongctl-mesh-test-config.yaml"
	main := utilviper.NewViper(path)
	main.Set("default", settings)
	return config.BuildProfiledConfig("default", path, main)
}

// Mesh requests must use the configured HTTP behaviour rather than a default
// client, which is what constructing one inline had made them do.
func TestMeshClientConfigUsesConfiguredSettings(t *testing.T) {
	clientConfig, err := baseClientConfig(meshTestConfig(t, map[string]any{
		cmdcommon.HTTPTimeoutConfigPath:                   "7s",
		cmdcommon.HTTPDisableKeepAlivesConfigPath:         true,
		cmdcommon.HTTPRecycleConnectionsOnErrorConfigPath: true,
	}))

	require.NoError(t, err)
	require.Equal(t, 7*time.Second, clientConfig.Timeout)
	require.True(t, clientConfig.TransportOptions.DisableKeepAlives)
	require.True(t, clientConfig.TransportOptions.RecycleConnectionsOnError)
}

func TestMeshClientConfigFallsBackToTheDefaultTimeout(t *testing.T) {
	clientConfig, err := baseClientConfig(meshTestConfig(t, map[string]any{}))

	require.NoError(t, err)
	require.Equal(t, httpclient.DefaultHTTPClientTimeout, clientConfig.Timeout)
}

func TestClientBuildersBuildAClient(t *testing.T) {
	cfg := meshTestConfig(t, map[string]any{})
	logger := slog.New(slog.DiscardHandler)

	cpClient, err := newControlPlaneClient(cfg, meshTarget{baseURL: "https://cp.example"}, logger)
	require.NoError(t, err)
	require.NotNil(t, cpClient)

	konnectClient, err := newKonnectClient(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, konnectClient)

	inputClient, err := newInputClient(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, inputClient)
}

// writeTempFile puts content on disk and returns its path.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// selfSignedCAPEM generates a CA certificate, so the test does not depend on
// one existing on disk.
func selfSignedCAPEM(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "mesh-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestSelfManagedTLSConfig(t *testing.T) {
	t.Run("nothing configured leaves Go's defaults", func(t *testing.T) {
		tlsConfig, err := selfManagedTLSConfig(meshTestConfig(t, map[string]any{}))
		require.NoError(t, err)
		require.Nil(t, tlsConfig)
	})

	t.Run("skip verify is opt in", func(t *testing.T) {
		tlsConfig, err := selfManagedTLSConfig(meshTestConfig(t, map[string]any{
			meshcommon.TLSSkipVerifyConfigPath: true,
		}))
		require.NoError(t, err)
		require.NotNil(t, tlsConfig)
		require.True(t, tlsConfig.InsecureSkipVerify)
		// Even when verification is skipped, the floor on the protocol stands.
		require.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
	})

	t.Run("a CA file becomes the root pool", func(t *testing.T) {
		caFile := writeTempFile(t, "ca.pem", selfSignedCAPEM(t))

		tlsConfig, err := selfManagedTLSConfig(meshTestConfig(t, map[string]any{
			meshcommon.CACertFileConfigPath: caFile,
		}))
		require.NoError(t, err)
		require.NotNil(t, tlsConfig.RootCAs)
		require.False(t, tlsConfig.InsecureSkipVerify)
	})

	t.Run("a missing CA file is reported", func(t *testing.T) {
		_, err := selfManagedTLSConfig(meshTestConfig(t, map[string]any{
			meshcommon.CACertFileConfigPath: filepath.Join(t.TempDir(), "absent.pem"),
		}))
		require.ErrorContains(t, err, meshcommon.CACertFileFlagName)
	})

	t.Run("a CA file holding no certificate is reported", func(t *testing.T) {
		_, err := selfManagedTLSConfig(meshTestConfig(t, map[string]any{
			meshcommon.CACertFileConfigPath: writeTempFile(t, "bad.pem", "not a certificate"),
		}))
		require.ErrorContains(t, err, "no PEM certificate")
	})

	t.Run("a client certificate needs its key", func(t *testing.T) {
		_, err := selfManagedTLSConfig(meshTestConfig(t, map[string]any{
			meshcommon.ClientCertFileConfigPath: writeTempFile(t, "cert.pem", selfSignedCAPEM(t)),
		}))
		require.ErrorContains(t, err, meshcommon.ClientKeyFileFlagName)
	})

	t.Run("a client key needs its certificate", func(t *testing.T) {
		_, err := selfManagedTLSConfig(meshTestConfig(t, map[string]any{
			meshcommon.ClientKeyFileConfigPath: writeTempFile(t, "key.pem", "key material"),
		}))
		require.ErrorContains(t, err, meshcommon.ClientCertFileFlagName)
	})
}

// TLS material is meaningless for a Konnect hosted control plane, which is
// reached over Konnect's own certificates. It is also meaningless for the two
// destinations that are not the control plane at all.
func TestClientTLSAppliesOnlyToASelfManagedControlPlane(t *testing.T) {
	// Configured as though a self managed control plane were in use, so the
	// material is available to leak if a builder installs it.
	cfg := meshTestConfig(t, map[string]any{
		meshcommon.TLSSkipVerifyConfigPath:   true,
		meshcommon.ControlPlaneURLConfigPath: "https://mesh.example:5682",
	})
	selfManaged, err := controlPlaneClientConfig(
		cfg, meshTarget{baseURL: "https://mesh.example:5682", selfManaged: true})
	require.NoError(t, err)
	require.NotNil(t, selfManaged.TransportOptions.TLSClientConfig)
	require.True(t, selfManaged.TransportOptions.TLSClientConfig.InsecureSkipVerify)

	// A hosted target, even with self managed TLS sitting in configuration.
	hosted, err := controlPlaneClientConfig(
		cfg, meshTarget{baseURL: "https://global.api.konghq.com"})
	require.NoError(t, err)
	require.Nil(t, hosted.TransportOptions.TLSClientConfig)

	// Konnect's own API and a remote `-f` input are never the control plane,
	// so neither may be handed the control plane's identity or trust policy.
	shared, err := baseClientConfig(cfg)
	require.NoError(t, err)
	require.Nil(t, shared.TransportOptions.TLSClientConfig)

	// The timeout and transport settings are still shared by all of them.
	require.Equal(t, shared.Timeout, selfManaged.Timeout)
	require.Equal(t, shared.Timeout, hosted.Timeout)
}

// An explicit hosted selector must win over a self managed URL left in
// configuration, and the classification must follow it.
//
// These were decided separately: the URL resolver honoured the explicit
// selector while a second check asked only whether a URL sat in
// configuration. A profile holding a self managed URL and token therefore sent
// that token to a Konnect hosted endpoint, and applied the self managed TLS
// policy to it.
func TestResolveTargetClassifiesFromTheWinningSelector(t *testing.T) {
	const (
		selfManagedURL = "https://mesh.example:5682"
		hostedID       = "8f1c2d3e-0000-4000-8000-000000000001"
	)

	cases := []struct {
		name            string
		settings        map[string]any
		given           []string
		wantSelfManaged bool
		wantURLContains string
	}{
		{
			name: "explicit id beats a configured self managed url",
			settings: map[string]any{
				meshcommon.ControlPlaneURLConfigPath: selfManagedURL,
				meshcommon.ControlPlaneIDConfigPath:  hostedID,
			},
			given:           []string{meshcommon.ControlPlaneIDFlagName},
			wantSelfManaged: false,
			wantURLContains: hostedID,
		},
		{
			name: "explicit url is self managed",
			settings: map[string]any{
				meshcommon.ControlPlaneURLConfigPath: selfManagedURL,
			},
			given:           []string{meshcommon.ControlPlaneURLFlagName},
			wantSelfManaged: true,
			wantURLContains: "mesh.example",
		},
		{
			name: "a configured url with no explicit selector is self managed",
			settings: map[string]any{
				meshcommon.ControlPlaneURLConfigPath: selfManagedURL,
			},
			given:           nil,
			wantSelfManaged: true,
			wantURLContains: "mesh.example",
		},
		{
			name: "a configured id with no explicit selector is hosted",
			settings: map[string]any{
				meshcommon.ControlPlaneIDConfigPath: hostedID,
			},
			given:           nil,
			wantSelfManaged: false,
			wantURLContains: hostedID,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			helper := &cmd.MockHelper{}
			helper.EXPECT().GetCmd().Return(selectorCmd(t, tc.given...))

			target, err := resolveTarget(helper, meshTestConfig(t, tc.settings))
			require.NoError(t, err)
			require.Equal(t, tc.wantSelfManaged, target.selfManaged,
				"selfManaged for %q", tc.name)
			require.Contains(t, target.baseURL, tc.wantURLContains)
		})
	}
}

// The classification decides which credential is sent, so a hosted target
// resolved over a configured self managed URL must not be handed the self
// managed TLS policy either.
func TestResolveTargetHostedOverSelfManagedURLGetsNoSelfManagedTLS(t *testing.T) {
	cfg := meshTestConfig(t, map[string]any{
		meshcommon.ControlPlaneURLConfigPath: "https://mesh.example:5682",
		meshcommon.ControlPlaneIDConfigPath:  "8f1c2d3e-0000-4000-8000-000000000001",
		meshcommon.TLSSkipVerifyConfigPath:   true,
	})

	helper := &cmd.MockHelper{}
	helper.EXPECT().GetCmd().Return(selectorCmd(t, meshcommon.ControlPlaneIDFlagName))

	target, err := resolveTarget(helper, cfg)
	require.NoError(t, err)
	require.False(t, target.selfManaged)

	clientConfig, err := controlPlaneClientConfig(cfg, target)
	require.NoError(t, err)
	require.Nil(t, clientConfig.TransportOptions.TLSClientConfig,
		"a hosted target must not receive the self managed TLS policy")
}

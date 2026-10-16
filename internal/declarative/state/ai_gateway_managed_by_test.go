package state

import (
	"context"
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestManagedByProvenancePreservesSDKPayloads(t *testing.T) {
	const (
		modelProvider = `{
  "display_name": "Probe",
  "name": "provider",
  "type": "openai",
  "config": {
    "auth": {
      "type": "basic",
      "headers": [
        {
          "name": "Authorization",
          "value": "synthetic-secret"
        }
      ]
    }
  }
}`
		authStrategy = `{
  "display_name": "Probe",
  "name": "auth",
  "type": "openid-connect",
  "config": {
    "issuer": "https://example.test",
    "cache_tokens_salt": "synthetic-salt",
    "client_secret": [
      "synthetic-secret"
    ]
  }
}`
		agent = `{
  "display_name": "Probe",
  "name": "agent",
  "type": "http"
}`
		consumer = `{
  "display_name": "Probe",
  "name": "consumer"
}`
		consumerGroup = `{
  "display_name": "Probe",
  "name": "group",
  "consumers": [
    "retained-consumer"
  ]
}`
		model = `{
  "display_name": "Probe",
  "name": "model",
  "type": "model",
  "formats": [],
  "targets": [],
  "config": {
    "route": {}
  },
  "capabilities": []
}`
		policy = `{
  "display_name": "Probe",
  "name": "policy",
  "type": "cors"
}`
		customPolicy = `{
  "display_name": "Probe",
  "name": "custom",
  "type": "streaming",
  "handler": "synthetic-handler",
  "schema": "synthetic-schema"
}`
		mCPServer = `{
  "display_name": "Probe",
  "name": "mcp",
  "type": "upstream-server",
  "config": {
    "url": "https://example.test",
    "tools_cache_ttl_seconds": 300
  }
}`
		vault = `{
  "display_name": "Probe",
  "name": "vault",
  "type": "env",
  "config": {
    "prefix": "PROBE_"
  }
}`
		certificate = `{
  "display_name": "Probe",
  "name": "cert",
  "cert": "public-cert",
  "key": "synthetic-private-key"
}`
		cACertificate = `{
  "display_name": "Probe",
  "name": "ca",
  "cert": "public-cert"
}`
		sNI = `{
  "display_name": "Probe",
  "name": "sni",
  "hostname": "example.test",
  "certificate": "cert"
}`
		//nolint:gosec // Deliberately synthetic API key for payload-preservation coverage.
		consumerCredential = `{
  "display_name": "Probe",
  "name": "credential",
  "type": "api-key",
  "api_key": "synthetic-key"
}`
	)
	t.Run(
		"CreateModelProvider",
		managedByStampCase[kkComps.CreateAIGatewayModelProviderRequest](modelProvider),
	)
	t.Run(
		"UpdateModelProvider",
		managedByStampCase[kkComps.UpdateAIGatewayModelProviderRequest](modelProvider),
	)
	t.Run(
		"CreateAuthStrategy",
		managedByStampCase[kkComps.CreateAIGatewayAuthStrategyRequest](authStrategy),
	)
	t.Run(
		"UpdateAuthStrategy",
		managedByStampCase[kkComps.UpdateAIGatewayAuthStrategyRequest](authStrategy),
	)
	t.Run(
		"CreateAgent",
		managedByStampCase[kkComps.CreateAIGatewayAgentRequest](agent),
	)
	t.Run(
		"UpdateAgent",
		managedByStampCase[kkComps.UpdateAIGatewayAgentRequest](agent),
	)
	t.Run(
		"CreateConsumer",
		managedByStampCase[kkComps.CreateAIGatewayConsumerRequest](consumer),
	)
	t.Run(
		"UpdateConsumer",
		managedByStampCase[kkComps.UpdateAIGatewayConsumerRequest](consumer),
	)
	t.Run(
		"CreateConsumerGroup",
		managedByStampCase[kkComps.CreateAIGatewayConsumerGroupRequest](consumerGroup),
	)
	t.Run(
		"UpdateConsumerGroup",
		managedByStampCase[kkComps.UpdateAIGatewayConsumerGroupRequest](consumerGroup),
	)
	t.Run(
		"CreateModel",
		managedByStampCase[kkComps.CreateAIGatewayModelRequest](model),
	)
	t.Run(
		"UpdateModel",
		managedByStampCase[kkComps.UpdateAIGatewayModelRequest](model),
	)
	t.Run(
		"CreatePolicy",
		managedByStampCase[kkComps.CreateAIGatewayPolicyRequest](policy),
	)
	t.Run(
		"UpdatePolicy",
		managedByStampCase[kkComps.UpdateAIGatewayPolicyRequest](policy),
	)
	t.Run(
		"CreateCustomPolicy",
		managedByStampCase[kkComps.CreateAIGatewayCustomPolicyRequest](customPolicy),
	)
	t.Run(
		"UpdateCustomPolicy",
		managedByStampCase[kkComps.UpdateAIGatewayCustomPolicyRequest](customPolicy),
	)
	t.Run(
		"CreateMCPServer",
		managedByStampCase[kkComps.CreateAIGatewayMCPServerRequest](mCPServer),
	)
	t.Run(
		"UpdateMCPServer",
		managedByStampCase[kkComps.UpdateAIGatewayMCPServerRequest](mCPServer),
	)
	t.Run(
		"CreateVault",
		managedByStampCase[kkComps.CreateAIGatewayVaultRequest](vault),
	)
	t.Run(
		"UpdateVault",
		managedByStampCase[kkComps.UpdateAIGatewayVaultRequest](vault),
	)
	t.Run(
		"CreateCertificate",
		managedByStampCase[kkComps.CreateAIGatewayCertificateRequest](certificate),
	)
	t.Run(
		"UpdateCertificate",
		managedByStampCase[kkComps.UpdateAIGatewayCertificateRequest](certificate),
	)
	t.Run(
		"CreateCACertificate",
		managedByStampCase[kkComps.CreateAIGatewayCACertificateRequest](cACertificate),
	)
	t.Run(
		"UpdateCACertificate",
		managedByStampCase[kkComps.UpdateAIGatewayCACertificateRequest](cACertificate),
	)
	t.Run(
		"CreateSNI",
		managedByStampCase[kkComps.CreateAIGatewaySNIRequest](sNI),
	)
	t.Run(
		"UpdateSNI",
		managedByStampCase[kkComps.UpdateAIGatewaySNIRequest](sNI),
	)
	t.Run(
		"CreateConsumerCredential",
		managedByStampCase[kkComps.CreateAIGatewayConsumerCredentialRequest](consumerCredential),
	)
}

func managedByStampCase[T any](raw string) func(*testing.T) {
	return func(t *testing.T) {
		var req T
		require.NoError(t, json.Unmarshal([]byte(raw), &req))
		data, err := json.Marshal(req)
		require.NoError(t, err)
		var before map[string]any
		require.NoError(t, json.Unmarshal(data, &before))
		ctx := context.WithValue(t.Context(), build.InfoKey, &build.Info{Version: "2.3.4"})
		require.NoError(t, stampAIGatewayManagedBy(ctx, &req))
		data, err = json.Marshal(req)
		require.NoError(t, err)
		var after map[string]any
		require.NoError(t, json.Unmarshal(data, &after))
		require.Equal(t, map[string]any{"tool": "kongctl", "version": "2.3.4"}, after["managed_by"])
		delete(after, "managed_by")
		require.Equal(t, before, after,
			"provenance cannot alter credentials, memberships, or other payload fields")
	}
}

func TestManagedByExecutionOverridesSavedVersionAndEnforcesCapacity(t *testing.T) {
	req := kkComps.CreateAIGatewayConsumerRequest{Name: "consumer", ManagedBy: map[string]string{
		"tool": "old-tool", "version": "old-binary", "other": "preserved",
	}}
	ctx := context.WithValue(t.Context(), build.InfoKey, &build.Info{Version: "running-binary"})
	require.NoError(t, stampAIGatewayManagedBy(ctx, &req))
	require.Equal(t,
		map[string]string{"tool": "kongctl", "version": "running-binary", "other": "preserved"},
		req.ManagedBy)
	// The second pass must not mutate a map retained by the caller.
	previous := req.ManagedBy
	require.NoError(t, stampAIGatewayManagedBy(t.Context(), &req))
	require.Equal(t, "dev", req.ManagedBy["version"])
	require.Equal(t, "running-binary", previous["version"])
	req.ManagedBy = map[string]string{
		resources.ChildNamespaceKey: "team-a", resources.ChildProtectedKey: "false",
		"other": "keep", "extra": "keep-too",
	}
	require.ErrorContains(t, stampAIGatewayManagedBy(ctx, &req), "five-entry")
	require.NotContains(t, req.ManagedBy, "tool", "overflow must fail without mutating the request")
}

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

type lossyManagedByRequest struct {
	Mode      string            `json:"mode"`
	Type      string            `json:"type"`
	Secret    *string           `json:"secret,omitempty"`
	Count     int64             `json:"count"`
	ManagedBy map[string]string `json:"managed_by,omitempty"`
}

func (r *lossyManagedByRequest) UnmarshalJSON(data []byte) error {
	type plain lossyManagedByRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = lossyManagedByRequest(decoded)
	switch r.Mode {
	case "drop-field":
		r.Secret = nil
	case "change-variant":
		r.Type = "other"
	case "change-integer":
		r.Count++
	}
	return nil
}

func TestManagedByStampRejectsLossyPayloadRoundTrip(t *testing.T) {
	for _, mode := range []string{"drop-field", "change-variant", "change-integer"} {
		t.Run(mode, func(t *testing.T) {
			request := lossyManagedByRequest{
				Mode: mode, Type: "original", Secret: new("synthetic-sensitive-value"), Count: 9007199254740992,
			}
			before := request
			err := stampManagedByRequest(t.Context(), resources.ResourceTypeAIGatewayProvider, &request)
			require.ErrorContains(t, err, "did not preserve payload")
			require.NotContains(t, err.Error(), *request.Secret)
			require.Equal(t, before, request, "failed stamping must not mutate the caller's request")
		})
	}
}

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
		managedByStampCase[kkComps.CreateAIGatewayModelProviderRequest](
			resources.ResourceTypeAIGatewayProvider, modelProvider,
		),
	)
	t.Run(
		"UpdateModelProvider",
		managedByStampCase[kkComps.UpdateAIGatewayModelProviderRequest](
			resources.ResourceTypeAIGatewayProvider, modelProvider,
		),
	)
	t.Run(
		"CreateAuthStrategy",
		managedByStampCase[kkComps.CreateAIGatewayAuthStrategyRequest](
			resources.ResourceTypeAIGatewayAuthStrategy, authStrategy,
		),
	)
	t.Run(
		"UpdateAuthStrategy",
		managedByStampCase[kkComps.UpdateAIGatewayAuthStrategyRequest](
			resources.ResourceTypeAIGatewayAuthStrategy, authStrategy,
		),
	)
	t.Run(
		"CreateAgent",
		managedByStampCase[kkComps.CreateAIGatewayAgentRequest](
			resources.ResourceTypeAIGatewayAgent, agent,
		),
	)
	t.Run(
		"UpdateAgent",
		managedByStampCase[kkComps.UpdateAIGatewayAgentRequest](
			resources.ResourceTypeAIGatewayAgent, agent,
		),
	)
	t.Run(
		"CreateConsumer",
		managedByStampCase[kkComps.CreateAIGatewayConsumerRequest](
			resources.ResourceTypeAIGatewayConsumer, consumer,
		),
	)
	t.Run(
		"UpdateConsumer",
		managedByStampCase[kkComps.UpdateAIGatewayConsumerRequest](
			resources.ResourceTypeAIGatewayConsumer, consumer,
		),
	)
	t.Run(
		"CreateConsumerGroup",
		managedByStampCase[kkComps.CreateAIGatewayConsumerGroupRequest](
			resources.ResourceTypeAIGatewayConsumerGroup, consumerGroup,
		),
	)
	t.Run(
		"UpdateConsumerGroup",
		managedByStampCase[kkComps.UpdateAIGatewayConsumerGroupRequest](
			resources.ResourceTypeAIGatewayConsumerGroup, consumerGroup,
		),
	)
	t.Run(
		"CreateModel",
		managedByStampCase[kkComps.CreateAIGatewayModelRequest](
			resources.ResourceTypeAIGatewayModel, model,
		),
	)
	t.Run(
		"UpdateModel",
		managedByStampCase[kkComps.UpdateAIGatewayModelRequest](
			resources.ResourceTypeAIGatewayModel, model,
		),
	)
	t.Run(
		"CreatePolicy",
		managedByStampCase[kkComps.CreateAIGatewayPolicyRequest](
			resources.ResourceTypeAIGatewayPolicy, policy,
		),
	)
	t.Run(
		"UpdatePolicy",
		managedByStampCase[kkComps.UpdateAIGatewayPolicyRequest](
			resources.ResourceTypeAIGatewayPolicy, policy,
		),
	)
	t.Run(
		"CreateCustomPolicy",
		managedByStampCase[kkComps.CreateAIGatewayCustomPolicyRequest](
			resources.ResourceTypeAIGatewayCustomPolicy, customPolicy,
		),
	)
	t.Run(
		"UpdateCustomPolicy",
		managedByStampCase[kkComps.UpdateAIGatewayCustomPolicyRequest](
			resources.ResourceTypeAIGatewayCustomPolicy, customPolicy,
		),
	)
	t.Run(
		"CreateMCPServer",
		managedByStampCase[kkComps.CreateAIGatewayMCPServerRequest](
			resources.ResourceTypeAIGatewayMCPServer, mCPServer,
		),
	)
	t.Run(
		"UpdateMCPServer",
		managedByStampCase[kkComps.UpdateAIGatewayMCPServerRequest](
			resources.ResourceTypeAIGatewayMCPServer, mCPServer,
		),
	)
	t.Run(
		"CreateVault",
		managedByStampCase[kkComps.CreateAIGatewayVaultRequest](
			resources.ResourceTypeAIGatewayVault, vault,
		),
	)
	t.Run(
		"UpdateVault",
		managedByStampCase[kkComps.UpdateAIGatewayVaultRequest](
			resources.ResourceTypeAIGatewayVault, vault,
		),
	)
	t.Run(
		"CreateCertificate",
		managedByStampCase[kkComps.CreateAIGatewayCertificateRequest](
			resources.ResourceTypeAIGatewayCertificate, certificate,
		),
	)
	t.Run(
		"UpdateCertificate",
		managedByStampCase[kkComps.UpdateAIGatewayCertificateRequest](
			resources.ResourceTypeAIGatewayCertificate, certificate,
		),
	)
	t.Run(
		"CreateCACertificate",
		managedByStampCase[kkComps.CreateAIGatewayCACertificateRequest](
			resources.ResourceTypeAIGatewayCACertificate, cACertificate,
		),
	)
	t.Run(
		"UpdateCACertificate",
		managedByStampCase[kkComps.UpdateAIGatewayCACertificateRequest](
			resources.ResourceTypeAIGatewayCACertificate, cACertificate,
		),
	)
	t.Run(
		"CreateSNI",
		managedByStampCase[kkComps.CreateAIGatewaySNIRequest](
			resources.ResourceTypeAIGatewaySNI, sNI,
		),
	)
	t.Run(
		"UpdateSNI",
		managedByStampCase[kkComps.UpdateAIGatewaySNIRequest](
			resources.ResourceTypeAIGatewaySNI, sNI,
		),
	)
	t.Run(
		"CreateConsumerCredential",
		managedByStampCase[kkComps.CreateAIGatewayConsumerCredentialRequest](
			resources.ResourceTypeAIGatewayConsumerCredential, consumerCredential,
		),
	)
}

func managedByStampCase[T any](kind resources.ResourceType, raw string) func(*testing.T) {
	return func(t *testing.T) {
		var req T
		require.NoError(t, json.Unmarshal([]byte(raw), &req))
		data, err := json.Marshal(req)
		require.NoError(t, err)
		var before map[string]any
		require.NoError(t, json.Unmarshal(data, &before))
		ctx := context.WithValue(t.Context(), build.InfoKey, &build.Info{Version: "2.3.4"})
		require.NoError(t, stampManagedByRequest(ctx, kind, &req))
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

func TestManagedByExecutionOverridesSavedVersionAndForeignMetadata(t *testing.T) {
	req := kkComps.CreateAIGatewayConsumerRequest{Name: "consumer", ManagedBy: map[string]string{
		"tool": "old-tool", "version": "old-binary", "other": "preserved",
	}}
	ctx := context.WithValue(t.Context(), build.InfoKey, &build.Info{Version: "running-binary"})
	require.NoError(t, stampManagedByRequest(ctx, resources.ResourceTypeAIGatewayConsumer, &req))
	require.Equal(t,
		map[string]string{"tool": "kongctl", "version": "running-binary"},
		req.ManagedBy)
	// The second pass must not mutate a map retained by the caller.
	previous := req.ManagedBy
	require.NoError(t, stampManagedByRequest(t.Context(), resources.ResourceTypeAIGatewayConsumer, &req))
	require.Equal(t, "dev", req.ManagedBy["version"])
	require.Equal(t, "running-binary", previous["version"])
	req.ManagedBy = map[string]string{
		resources.ChildNamespaceKey: "team-a", resources.ChildProtectedKey: "false",
		"other": "keep", "extra": "keep-too",
	}
	require.NoError(t, stampManagedByRequest(ctx, resources.ResourceTypeAIGatewayConsumer, &req))
	require.Equal(t, resources.BuildChildManagedBy("team-a", false, "running-binary"), req.ManagedBy)
}

func TestManagedByRequestRequiresEnrollmentAndSDKSupport(t *testing.T) {
	type unsupportedRequest struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	}
	req := unsupportedRequest{Name: "example", Labels: map[string]string{"KONGCTL-namespace": "team-a"}}
	require.NoError(t, stampManagedByRequest(t.Context(), resources.ResourceTypePortal, &req))
	require.Equal(t, "team-a", req.Labels["KONGCTL-namespace"])
	require.ErrorContains(t, stampManagedByRequest(t.Context(), resources.ResourceTypeAIGatewayProvider, &req),
		"SDK request did not preserve managed_by")
	require.Equal(t, "example", req.Name)
}

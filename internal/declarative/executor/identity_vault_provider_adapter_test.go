package executor

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	logctx "github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestVaultProviderSavedLifecycleSecretContextAndParentAuthority(t *testing.T) {
	const namespace = "vault-provider-team"
	root := fixture.Directory("default", namespace, false)
	root.VaultEnabled = new(true)
	roots := &fixture.DirectoryAPI{Current: root, Directories: []kkComps.KongDirectory{root}, Lifecycle: true}
	providers := &fixture.VaultProvidersAPI{AttachedCredentials: make(map[string]bool)}
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI:    roots,
		IdentityVaultProvidersAPI: providers, IdentityDirectoryDependenciesAPI: providers,
	})
	file := filepath.Join(t.TempDir(), "manifest.yaml")
	generate := func(values []map[string]any, present bool, mode planner.PlanMode, write bool) (*planner.Plan, error) {
		t.Helper()
		parent := map[string]any{
			"ref": "people", "name": "default", "allow_all_control_planes": true,
			"vault_enabled": true, "ttl_secs": 900, "negative_ttl_secs": 450,
		}
		if present {
			if values == nil {
				values = []map[string]any{}
			}
			parent["vault_providers"] = values
		}
		data, err := yaml.Marshal(map[string]any{
			"_defaults": map[string]any{"kongctl": map[string]any{"namespace": namespace}},
			"identity":  map[string]any{"directories": []any{parent}},
		})
		require.NoError(t, err)
		data = []byte(
			strings.ReplaceAll(
				string(data),
				"DEFER_PROVIDER_SECRET",
				"!secret {source: !env IDENTITY_PROVIDER_TEST_SECRET}",
			),
		)
		require.NoError(t, os.WriteFile(file, data, 0o600))
		rs, err := loader.New().LoadFile(file)
		if err != nil {
			return nil, err
		}
		opts := planner.Options{Mode: mode}
		if write {
			opts.WriteSecretSelectors = []string{"oauth#client_secret"}
		}
		value, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, opts)
		if err != nil {
			return nil, err
		}
		data, err = json.Marshal(value)
		require.NoError(t, err)
		var saved planner.Plan
		require.NoError(t, json.Unmarshal(data, &saved))
		return &saved, nil
	}
	execute := func(value *planner.Plan) *ExecutionResult {
		t.Helper()
		ctx := context.WithValue(t.Context(), logctx.LoggerKey, slog.Default())
		return NewWithOptions(client, nil, false, Options{MaxConcurrency: 1}).Execute(ctx, value)
	}
	static := map[string]any{
		"ref": "static", "name": "billing-provider", "template_name": "static_secret",
		"credential_type": "shared", "enabled": false, "refresh_buffer_seconds": int64(30),
	}
	oauth := map[string]any{ // #nosec G101 -- deferred source marker and public endpoints only.
		"ref": "oauth", "name": "github-provider", "template_name": "github",
		"client_id": "upstream-client", "client_secret": "DEFER_PROVIDER_SECRET", "credential_type": "shared",
		"enabled": true, "scopes": []string{"repo"}, "pkce_enabled": true, "refresh_buffer_seconds": int64(60),
		"token_endpoint":         "https://fixture.example.test/token",
		"authorization_endpoint": "https://fixture.example.test/authorize",
	}
	dcr := map[string]any{ // #nosec G101 -- public OAuth endpoint fixture, no credentials.
		"ref": "dcr", "name": "custom-provider", "template_name": "custom-oauth", "dcr_enabled": true,
		"credential_type": "user", "scopes": []string{"read"}, "pkce_enabled": true, "enabled": false,
		"token_endpoint":         "https://fixture.example.test/token",
		"authorization_endpoint": "https://fixture.example.test/authorize",
		"registration_endpoint":  "https://fixture.example.test/register",
	}
	initial, err := generate([]map[string]any{static, oauth, dcr}, true, planner.PlanModeApply, false)
	require.NoError(t, err)
	require.Len(t, initial.Changes, 3)
	private := strings.Repeat("private-test-value", 3)
	t.Setenv("IDENTITY_PROVIDER_TEST_SECRET", private)
	encoded, err := json.Marshal(initial)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), private)
	result := execute(initial)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, providers.Providers, 3)
	require.Equal(t, 1, providers.SecretWrites)
	require.False(t, *providers.Providers[0].Enabled, "initial false requires the post-create PATCH")
	noop, err := generate([]map[string]any{static, oauth, dcr}, true, planner.PlanModeApply, false)
	require.NoError(t, err)
	require.Empty(t, noop.Changes)
	updated := maps.Clone(oauth)
	updated["refresh_buffer_seconds"] = int64(0)
	updated["enabled"] = false
	updated["scopes"] = []string{"repo", "read:org"}
	updated["pkce_enabled"] = false
	updated["token_endpoint"] = "https://other.example.test/token"
	updated["authorization_endpoint"] = "https://other.example.test/authorize"
	update, err := generate([]map[string]any{static, updated, dcr}, true, planner.PlanModeApply, false)
	require.NoError(t, err)
	require.Len(t, update.Changes, 1)
	require.Empty(t, update.Changes[0].SecretWrites)
	result = execute(update)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Equal(t, 1, providers.SecretWrites, "ordinary configuration updates omit secrets")
	noop, err = generate([]map[string]any{static, updated, dcr}, true, planner.PlanModeApply, false)
	require.NoError(t, err)
	require.Empty(t, noop.Changes)
	secret, err := generate([]map[string]any{static, updated, dcr}, true, planner.PlanModeApply, true)
	require.NoError(t, err)
	require.Len(t, secret.Changes, 1)
	change := secret.Changes[0]
	require.Equal(t, namespace, change.Namespace)
	require.Equal(t, root.ID, change.Parent.ID)
	require.NotNil(t, change.ProtectingParent)
	require.Equal(t, root.ID, change.ProtectingParent.ResourceID)
	require.Equal(t, planner.ResourceTypeIdentityDirectory, change.ProtectingParent.ResourceType)
	before := providers.Writes
	for _, invalid := range []string{"namespace", "owner", "protected", "disabled", "name"} {
		roots.Current = root
		roots.Current.ManagedBy = maps.Clone(root.ManagedBy)
		switch invalid {
		case "namespace":
			roots.Current.ManagedBy["kongctl-namespace"] = "foreign"
		case "owner":
			roots.Current.ManagedBy["tool"] = "another-tool"
		case "protected":
			roots.Current.ManagedBy["kongctl-protected"] = "true"
		case "disabled":
			roots.Current.VaultEnabled = new(false)
		case "name":
			roots.Current.Name = "changed-parent"
		}
		result = execute(secret)
		require.Positive(t, result.FailureCount, invalid)
		require.Equal(t, before, providers.Writes, invalid)
	}
	roots.Current = root
	providers.Providers[1].ClientID = new("changed-client")
	result = execute(secret)
	require.Positive(t, result.FailureCount)
	require.Equal(t, before, providers.Writes, "saved secret context cannot silently discard a changed static client")
	providers.Providers[1].ClientID = new("upstream-client")
	result = execute(secret)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Equal(t, 2, providers.SecretWrites)
	renamed := maps.Clone(static)
	renamed["name"] = "billing-replacement"
	replacement, err := generate([]map[string]any{renamed, updated, dcr}, true, planner.PlanModeSync, false)
	require.NoError(t, err)
	require.Len(t, replacement.Changes, 2)
	var creationID string
	for _, change := range replacement.Changes {
		if change.Action == planner.ActionCreate {
			creationID = change.ID
		}
	}
	require.NotEmpty(t, creationID)
	for _, change := range replacement.Changes {
		if change.Action == planner.ActionDelete {
			require.Contains(t, change.DependsOn, creationID)
		}
	}
	immutable := maps.Clone(updated)
	immutable["client_id"] = "replacement-client"
	_, err = generate([]map[string]any{static, immutable, dcr}, true, planner.PlanModeApply, false)
	require.ErrorContains(t, err, "verified PATCH")
	omitted, err := generate(nil, false, planner.PlanModeSync, false)
	require.NoError(t, err)
	require.Empty(t, omitted.Changes)
	applySubset, err := generate([]map[string]any{static}, true, planner.PlanModeApply, false)
	require.NoError(t, err)
	require.Empty(t, applySubset.Changes)
	providers.AttachedCredentials[providers.Providers[1].ID] = true
	_, err = generate([]map[string]any{static}, true, planner.PlanModeSync, false)
	require.ErrorContains(t, err, "attached")
	clear(providers.AttachedCredentials)
	prune, err := generate([]map[string]any{static}, true, planner.PlanModeSync, false)
	require.NoError(t, err)
	require.Len(t, prune.Changes, 2)
	for _, p := range providers.Providers[1:] {
		providers.AttachedCredentials[p.ID] = true
	}
	before = providers.Writes
	result = execute(prune)
	require.Positive(t, result.FailureCount)
	require.Equal(t, before, providers.Writes)
	clear(providers.AttachedCredentials)
	result = execute(prune)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, providers.Providers, 1)
	empty, err := generate(nil, true, planner.PlanModeSync, false)
	require.NoError(t, err)
	result = execute(empty)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Empty(t, providers.Providers)
	recreate, err := generate([]map[string]any{static}, true, planner.PlanModeApply, false)
	require.NoError(t, err)
	result = execute(recreate)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	deletion, err := generate([]map[string]any{static}, true, planner.PlanModeDelete, false)
	require.NoError(t, err)
	require.Len(t, deletion.Changes, 2)
	late, err := client.CreateIdentityVaultProvider(t.Context(), root.ID, kkComps.CreateVaultProviderBody{
		Name: "late", TemplateName: kkComps.ProviderTemplateNameStaticSecret, CredentialType: new("shared"),
	})
	require.NoError(t, err)
	result = execute(deletion)
	require.Positive(t, result.FailureCount)
	require.Zero(t, roots.Deletes)
	require.Len(t, providers.Providers, 1)
	require.Equal(t, late.ID, providers.Providers[0].ID)
	require.NoError(t, client.DeleteIdentityVaultProvider(t.Context(), root.ID, late.ID, late.Name))
	result = execute(deletion)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Equal(t, 1, roots.Deletes)
}

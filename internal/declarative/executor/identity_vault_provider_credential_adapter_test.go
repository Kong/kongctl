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
)

func TestVaultCredentialSavedLifecycleExplicitSecretsAndAuthority(t *testing.T) {
	const namespace = "vault-credential-team"
	root := fixture.Directory("default", namespace, false)
	root.VaultEnabled = new(true)
	roots := &fixture.DirectoryAPI{Current: root, Directories: []kkComps.KongDirectory{root}, Lifecycle: true}
	providers := &fixture.VaultProvidersAPI{}
	keys := &fixture.VaultKeysAPI{}
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: roots, IdentityVaultProvidersAPI: providers,
		IdentityStaticVaultCredentialsAPI: providers, IdentityVaultKeysAPI: keys, IdentityDirectoryDependenciesAPI: providers,
	})
	public := vaultKeyPublicFixtures(t)[0]
	file := filepath.Join(t.TempDir(), "manifest.yaml")
	generate := func(
		credentials []map[string]any, present bool, mode planner.PlanMode, selection string,
	) (*planner.Plan, error) {
		t.Helper()
		provider := map[string]any{
			"ref":             "billing",
			"name":            "billing-provider",
			"template_name":   "static_secret",
			"credential_type": "shared",
		}
		if present {
			if credentials == nil {
				credentials = []map[string]any{}
			}
			provider["shared_credentials"] = credentials
		}
		parent := map[string]any{
			"ref": "people", "name": "default", "allow_all_control_planes": true,
			"ttl_secs": 900, "negative_ttl_secs": 450,
			"vault": map[string]any{
				"enabled": true, "providers": []any{provider},
				"keys": []any{map[string]any{"ref": "encryption", "name": "encryption", "public_key": public}},
			},
		}
		data, err := json.Marshal(
			map[string]any{
				"_defaults": map[string]any{"kongctl": map[string]any{"namespace": namespace}},
				"identity":  map[string]any{"directories": []any{parent}},
			},
		)
		require.NoError(t, err)
		data = []byte(
			strings.ReplaceAll(
				string(data),
				`"DEFER_STATIC_SECRET"`,
				`!secret {source: !env IDENTITY_STATIC_TEST_SECRET}`,
			),
		)
		require.NoError(t, os.WriteFile(file, data, 0o600))
		rs, err := loader.New().LoadFile(file)
		if err != nil {
			return nil, err
		}
		opts := planner.Options{Mode: mode}
		if selection != "" {
			opts.WriteSecretSelectors = []string{selection + "#secret"}
		}
		value, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, opts)
		if err != nil {
			return nil, err
		}
		data, err = json.Marshal(value)
		require.NoError(t, err)
		require.NotContains(t, string(data), "private-static-material")
		var saved planner.Plan
		require.NoError(t, json.Unmarshal(data, &saved))
		return &saved, nil
	}
	execute := func(plan *planner.Plan) *ExecutionResult {
		ctx := context.WithValue(t.Context(), logctx.LoggerKey, slog.Default())
		return NewWithOptions(client, nil, false, Options{MaxConcurrency: 1}).Execute(ctx, plan)
	}
	defaultCredential := map[string]any{"ref": "default-credential", "secret": "DEFER_STATIC_SECRET"} // #nosec G101

	rule := map[string]any{ // #nosec G101 -- deferred source marker, not credential material.
		"ref":      "routing-rule",
		"selector": "true",
		"priority": int64(0),
		"secret":   "DEFER_STATIC_SECRET",
	}
	withoutSource := maps.Clone(defaultCredential)
	delete(withoutSource, "secret")
	_, err := generate([]map[string]any{withoutSource}, true, planner.PlanModeApply, "")
	require.ErrorContains(t, err, "deferred secret")
	initial, err := generate([]map[string]any{defaultCredential, rule}, true, planner.PlanModeApply, "")
	require.NoError(t, err)
	require.Len(t, initial.Changes, 4)
	for _, change := range initial.Changes {
		if change.ResourceType == planner.ResourceTypeIdentityVaultProviderCredential {
			require.Len(t, change.SecretWrites, 1)
			require.NotContains(t, change.Fields, planner.FieldSecret)
			require.Equal(t, root.ID, change.References[planner.FieldDirectoryID].ID)
			require.Equal(t, planner.ResourceTypeIdentityDirectory, change.ProtectingParent.ResourceType)
		}
	}
	t.Setenv("IDENTITY_STATIC_TEST_SECRET", "private-static-material")
	result := execute(initial)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, providers.Providers, 1)
	providerID := providers.Providers[0].ID
	require.Len(t, providers.Credentials[providerID], 2)
	require.Equal(t, 2, providers.CredentialWrites)
	noop, err := generate([]map[string]any{defaultCredential, rule}, true, planner.PlanModeApply, "")
	require.NoError(t, err)
	require.Empty(t, noop.Changes)
	rotation, err := generate([]map[string]any{defaultCredential, rule}, true, planner.PlanModeApply, "routing-rule")
	require.NoError(t, err)
	require.Len(t, rotation.Changes, 1)
	change := rotation.Changes[0]
	require.Equal(t, providerID, change.Parent.ID)
	require.Equal(t, namespace, change.Namespace)
	require.Equal(t, root.ID, change.References[planner.FieldDirectoryID].ID)
	require.Equal(t, root.ID, change.ProtectingParent.ResourceID)
	before := providers.CredentialWrites
	for _, reason := range []string{
		"namespace", "owner", "protected", "vault", "name", "provider-name", "provider-mode", "routing", "key",
	} {
		roots.Current = root
		roots.Current.ManagedBy = maps.Clone(root.ManagedBy)
		savedProvider := providers.Providers[0]
		savedCredential := providers.Credentials[providerID][1]
		savedKeys := keys.Keys
		switch reason {
		case "namespace":
			roots.Current.ManagedBy["kongctl-namespace"] = "foreign"
		case "owner":
			roots.Current.ManagedBy["tool"] = "another"
		case "protected":
			roots.Current.ManagedBy["kongctl-protected"] = "true"
		case "vault":
			roots.Current.VaultEnabled = new(false)
		case "name":
			roots.Current.Name = "renamed"
		case "provider-name":
			providers.Providers[0].Name = "renamed"
		case "provider-mode":
			providers.Providers[0].TemplateName = "github"
		case "routing":
			providers.Credentials[providerID][1].Selector = new("false")
		case "key":
			keys.Keys = nil
		}
		result = execute(rotation)
		require.Positive(t, result.FailureCount, reason)
		require.Equal(t, before, providers.CredentialWrites, reason)
		providers.Providers[0] = savedProvider
		providers.Credentials[providerID][1] = savedCredential
		keys.Keys = savedKeys
	}
	roots.Current = root
	result = execute(rotation)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Equal(t, before+1, providers.CredentialWrites)
	require.EqualValues(t, 0, *providers.Credentials[providerID][1].Priority)
	require.Equal(t, "true", *providers.Credentials[providerID][1].Selector)
	updated := maps.Clone(rule)
	updated["selector"] = "false"
	_, err = generate([]map[string]any{defaultCredential, updated}, true, planner.PlanModeApply, "")
	require.ErrorContains(t, err, "explicit secret-write")
	update, err := generate([]map[string]any{defaultCredential, updated}, true, planner.PlanModeApply, "routing-rule")
	require.NoError(t, err)
	result = execute(update)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Equal(t, "false", *providers.Credentials[providerID][1].Selector)
	omitted, err := generate(nil, false, planner.PlanModeSync, "")
	require.NoError(t, err)
	require.Empty(t, omitted.Changes)
	prune, err := generate([]map[string]any{defaultCredential}, true, planner.PlanModeSync, "")
	require.NoError(t, err)
	require.Len(t, prune.Changes, 1)
	result = execute(prune)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, providers.Credentials[providerID], 1)
	// A provider/root declaration alone grants no implicit credential retirement.
	_, err = generate(nil, false, planner.PlanModeDelete, "")
	require.Error(t, err)
	deletion, err := generate([]map[string]any{defaultCredential}, true, planner.PlanModeDelete, "")
	require.NoError(t, err)
	var credentialDeletion, providerDeletion *planner.PlannedChange
	for i := range deletion.Changes {
		change := &deletion.Changes[i]
		switch change.ResourceType {
		case planner.ResourceTypeIdentityVaultProviderCredential:
			credentialDeletion = change
		case planner.ResourceTypeIdentityVaultProvider:
			providerDeletion = change
		}
	}
	require.NotNil(t, credentialDeletion)
	require.NotNil(t, providerDeletion)
	require.Contains(t, providerDeletion.DependsOn, credentialDeletion.ID)
	result = execute(deletion)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Empty(t, providers.Credentials[providerID])
	require.Empty(t, providers.Providers)
	require.Empty(t, keys.Keys)
}

func TestVaultCredentialSavedPriorityRetainsIntegerPrecision(t *testing.T) {
	const priority int64 = 9007199254740993
	change := planner.PlannedChange{
		ResourceType: planner.ResourceTypeIdentityVaultProviderCredential,
		Fields: map[string]any{
			planner.FieldSelector: "true", planner.FieldPriority: priority,
			planner.FieldExpectedSelector: "true", planner.FieldExpectedPriority: priority,
		},
	}
	data, err := json.Marshal(change)
	require.NoError(t, err)
	var saved planner.PlannedChange
	require.NoError(t, json.Unmarshal(data, &saved))
	request, err := vaultCredentialPayload(saved.Fields, false)
	require.NoError(t, err)
	require.Equal(t, priority, *request.Priority)
	expected, err := expectedVaultCredentialRouting(saved.Fields)
	require.NoError(t, err)
	require.Equal(t, priority, *expected.Priority)
}

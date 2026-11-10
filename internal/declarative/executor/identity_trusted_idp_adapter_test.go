package executor

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	logctx "github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestTrustedIDPSavedLifecycleHydrationClearsAndDirectoryAuthority(t *testing.T) {
	const namespace = "trust-team"
	root := fixture.Directory("default", namespace, false)
	root.VaultEnabled = new(true)
	roots := &fixture.DirectoryAPI{Current: root, Directories: []kkComps.KongDirectory{root}, Lifecycle: true}
	trusts := &fixture.TrustedIDPsAPI{}
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: roots,
		IdentityTrustedIDPsAPI: trusts, IdentityDirectoryDependenciesAPI: trusts,
	})
	file := filepath.Join(t.TempDir(), "manifest.yaml")
	generate := func(values []map[string]any, present bool, mode planner.PlanMode) (*planner.Plan, error) {
		t.Helper()
		parent := map[string]any{
			"ref": "people", "name": "default", "allow_all_control_planes": true,
			"vault": map[string]any{"enabled": true}, "ttl_secs": 900, "negative_ttl_secs": 450,
		}
		if present {
			if values == nil {
				values = []map[string]any{}
			}
			parent["trusted_idps"] = values
		}
		data, err := yaml.Marshal(map[string]any{
			"_defaults": map[string]any{"kongctl": map[string]any{"namespace": namespace}},
			"identity":  map[string]any{"directories": []any{parent}},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(file, data, 0o600))
		rs, err := loader.New().LoadFile(file)
		if err != nil {
			return nil, err
		}
		value, err := planner.NewPlanner(client, slog.Default()).
			GeneratePlan(t.Context(), rs, planner.Options{Mode: mode})
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
		return NewWithOptions(client, nil, false, Options{MaxConcurrency: 1}).Execute(
			context.WithValue(t.Context(), logctx.LoggerKey, slog.Default()), value,
		)
	}
	full := map[string]any{
		"ref": "workforce", "issuer_url": "https://a.example.test/auth", "display_name": "Shared display",
		"jwks_uri": "https://a.example.test/keys", "jit_provisioning_enabled": false, "vault_access_enabled": true,
		"jit": map[string]any{"mapping": map[string]any{
			"email": "email", "groups": "groups", "display_name": "name",
			"metadata": map[string]string{"team": "department"},
		}},
	}
	other := map[string]any{
		"ref":          "partners",
		"issuer_url":   "https://b.example.test/auth",
		"display_name": "Shared display",
	}
	initial, err := generate([]map[string]any{full, other}, true, planner.PlanModeApply)
	require.NoError(t, err)
	require.Len(t, initial.Changes, 2)
	result := execute(initial)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, trusts.Records[root.ID], 2)
	noop, err := generate([]map[string]any{full, other}, true, planner.PlanModeApply)
	require.NoError(t, err)
	require.Empty(t, noop.Changes)
	partial := map[string]any{"ref": "workforce", "issuer_url": full["issuer_url"], "display_name": "Updated display"}
	update, err := generate([]map[string]any{partial, other}, true, planner.PlanModeApply)
	require.NoError(t, err)
	require.Len(t, update.Changes, 1)
	before := trusts.Writes
	for _, invalid := range []string{
		"namespace", "tool", "protected", "disabled", "name", "issuer", "membership", "unknown-field",
	} {
		rootCopy := root
		rootCopy.ManagedBy = maps.Clone(root.ManagedBy)
		roots.Current = rootCopy
		original := trusts.Records[root.ID][0]
		switch invalid {
		case "namespace":
			roots.Current.ManagedBy["kongctl-namespace"] = "foreign"
		case "tool":
			roots.Current.ManagedBy["tool"] = "another-tool"
		case "protected":
			roots.Current.ManagedBy["kongctl-protected"] = "true"
		case "disabled":
			roots.Current.VaultEnabled = new(false)
		case "name":
			roots.Current.Name = "changed"
		case "issuer":
			trusts.Records[root.ID][0].IssuerURL = "https://changed.example.test"
		case "membership":
			trusts.Records[root.ID] = trusts.Records[root.ID][1:]
		case "unknown-field":
			update.Changes[0].Fields["discarded_field"] = "bad"
		}
		result = execute(update)
		require.Positive(t, result.FailureCount, invalid)
		require.Equal(t, before, trusts.Writes, invalid)
		if invalid == "membership" {
			trusts.Records[root.ID] = append([]helpers.IdentityTrustedIDP{original}, trusts.Records[root.ID]...)
		} else {
			trusts.Records[root.ID][0] = original
		}
		delete(update.Changes[0].Fields, "discarded_field")
	}
	roots.Current = root
	// Fresh state after planning must supply omitted public settings, rather
	// than a stale snapshot embedded in the saved plan.
	trusts.Records[root.ID][0].JwksURI = new("https://a.example.test/rotated-keys")
	trusts.Records[root.ID][0].Jit.Mapping.Email = new("preferred_email")
	result = execute(update)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	current := trusts.Records[root.ID][0]
	require.Equal(t, "https://a.example.test/rotated-keys", *current.JwksURI)
	require.Equal(t, "preferred_email", *current.Jit.Mapping.Email)
	require.False(t, *current.JitProvisioningEnabled)
	require.True(t, *current.VaultAccessEnabled)
	cleared := maps.Clone(partial)
	cleared["display_name"] = nil
	cleared["jwks_uri"] = nil
	cleared["jit"] = map[string]any{}
	cleared["vault_access_enabled"] = false
	cleared["jit_provisioning_enabled"] = true
	clearPlan, err := generate([]map[string]any{cleared, other}, true, planner.PlanModeApply)
	require.NoError(t, err)
	result = execute(clearPlan)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	current = trusts.Records[root.ID][0]
	require.Nil(t, current.DisplayName)
	require.Nil(t, current.JwksURI)
	require.Nil(t, current.Jit)
	require.True(t, *current.JitProvisioningEnabled)
	require.False(t, *current.VaultAccessEnabled)
	noop, err = generate([]map[string]any{cleared, other}, true, planner.PlanModeApply)
	require.NoError(t, err)
	require.Empty(t, noop.Changes)
	noop, err = generate(nil, false, planner.PlanModeSync)
	require.NoError(t, err)
	require.Empty(t, noop.Changes)
	prune, err := generate(nil, true, planner.PlanModeSync)
	require.NoError(t, err)
	require.Len(t, prune.Changes, 2)
	trusts.Associations = func(string, string) []helpers.IdentityPrincipal {
		return []helpers.IdentityPrincipal{{ID: root.ID, Active: new(false)}}
	}
	_, err = generate(nil, true, planner.PlanModeSync)
	require.Error(t, err, "associations block planning without explicit detachment")
	before = trusts.Writes
	result = execute(prune)
	require.Positive(t, result.FailureCount)
	require.Equal(t, before, trusts.Writes)
	trusts.Associations = nil
	result = execute(prune)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Empty(t, trusts.Records[root.ID])
	recreate, err := generate([]map[string]any{full}, true, planner.PlanModeApply)
	require.NoError(t, err)
	result = execute(recreate)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	deletion, err := generate([]map[string]any{full}, true, planner.PlanModeDelete)
	require.NoError(t, err)
	require.Len(t, deletion.Changes, 2)
	late, err := client.CreateIdentityTrustedIDP(
		t.Context(),
		root.ID,
		kkComps.CreateTrustedIdpPayload{IssuerURL: "https://late.example.test"},
	)
	require.NoError(t, err)
	result = execute(deletion)
	require.Positive(t, result.FailureCount)
	require.Zero(t, roots.Deletes)
	require.Len(t, trusts.Records[root.ID], 1)
	require.Equal(t, late.ID, trusts.Records[root.ID][0].ID)
	require.NoError(t, client.DeleteIdentityTrustedIDP(t.Context(), root.ID, late.ID, late.IssuerURL))
	result = execute(deletion)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Equal(t, 1, roots.Deletes)
}

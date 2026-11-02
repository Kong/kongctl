package executor

import (
	"context"
	"encoding/json"
	"log/slog"
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

func vaultKeyPublicFixtures(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("../../konnect/identitykeys/testdata/public-keys.json")
	require.NoError(t, err)
	var values []struct {
		PublicKey string `json:"public_key"`
		Bits      int    `json:"bits"`
	}
	require.NoError(t, json.Unmarshal(data, &values))
	var result []string
	for _, value := range values {
		if value.Bits != 1024 {
			result = append(result, value.PublicKey)
		}
	}
	return result
}

func TestVaultKeySavedLifecycleFingerprintMatchingAndParentAuthority(t *testing.T) {
	public := vaultKeyPublicFixtures(t)
	root := fixture.Directory("default", "vault-team", false)
	root.VaultEnabled = new(true)
	roots := &fixture.DirectoryAPI{Current: root, Lifecycle: true}
	keys := &fixture.VaultKeysAPI{}
	client := state.NewClient(state.ClientConfig{
		RegistryAgentReferencesReader: &fixture.RegistryReferencesAPI{},
		IdentityDirectoriesAPI:        roots, IdentityVaultKeysAPI: keys, IdentityDirectoryDependenciesAPI: keys,
	})
	file := filepath.Join(t.TempDir(), "manifest.yaml")
	plan := func(names, materials []string, mode planner.PlanMode, present bool) (*planner.Plan, error) {
		t.Helper()
		parent := map[string]any{
			"ref":                      "people",
			"name":                     "default",
			"allow_all_control_planes": true,
			"vault":                    map[string]any{"enabled": true},
		}
		if present {
			values := make([]any, 0, len(names))
			for i, name := range names {
				values = append(
					values,
					map[string]any{"ref": "key-" + string(rune('a'+i)), "name": name, "public_key": materials[i]},
				)
			}
			parent["vault"].(map[string]any)["keys"] = values
		}
		data, err := json.Marshal(map[string]any{
			"_defaults": map[string]any{"kongctl": map[string]any{"namespace": "vault-team"}},
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
		ctx := context.WithValue(t.Context(), logctx.LoggerKey, slog.Default())
		return NewWithOptions(client, nil, false, Options{MaxConcurrency: 1}).Execute(ctx, value)
	}
	initial, err := plan([]string{"same-name", "same-name"}, public[:2], planner.PlanModeApply, true)
	require.NoError(t, err)
	require.Len(t, initial.Changes, 3)
	for _, change := range initial.Changes[1:] {
		require.Contains(t, change.DependsOn, initial.Changes[0].ID)
	}
	result := execute(initial)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, keys.Keys, 2)
	firstID, firstKid := keys.Keys[0].ID, keys.Keys[0].Kid
	crlf := []string{strings.ReplaceAll(public[0], "\n", "\r\n"), public[1]}
	noop, err := plan([]string{"same-name", "same-name"}, crlf, planner.PlanModeApply, true)
	require.NoError(t, err)
	require.Empty(t, noop.Changes)
	rename, err := plan([]string{"renamed", "same-name"}, public[:2], planner.PlanModeApply, true)
	require.NoError(t, err)
	require.Len(t, rename.Changes, 1)
	require.Equal(t, planner.ActionUpdate, rename.Changes[0].Action)
	oldWrites := keys.Writes
	for _, invalid := range []string{"namespace", "owner", "protected", "disabled", "name"} {
		roots.Current = root
		roots.Current.ManagedBy = map[string]string{
			"tool":              "kongctl",
			"version":           "dev",
			"kongctl-namespace": "vault-team",
			"kongctl-protected": "false",
		}
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
		result = execute(rename)
		require.Positive(t, result.FailureCount, invalid)
		require.Equal(t, oldWrites, keys.Writes, invalid)
	}
	roots.Current = root
	result = execute(rename)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Equal(t, firstID, keys.Keys[0].ID)
	require.Equal(t, firstKid, keys.Keys[0].Kid)
	require.Equal(t, "renamed", keys.Keys[0].Name)
	noop, err = plan(nil, nil, planner.PlanModeSync, false)
	require.NoError(t, err)
	require.Empty(t, noop.Changes, "omitted collection preserves keys")
	rotation, err := plan([]string{"same-name"}, public[2:3], planner.PlanModeSync, true)
	require.NoError(t, err)
	require.Len(t, rotation.Changes, 3)
	createID := rotation.Changes[0].ID
	require.Equal(t, planner.ActionCreate, rotation.Changes[0].Action)
	for _, change := range rotation.Changes[1:] {
		require.Contains(t, change.DependsOn, createID)
	}
	result = execute(rotation)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, keys.Keys, 1)
	capacity, err := plan([]string{"one", "two", "three", "four", "five"}, public[:5], planner.PlanModeApply, true)
	require.NoError(t, err)
	result = execute(capacity)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, keys.Keys, 5)
	oldWrites = keys.Writes
	_, err = plan([]string{"one", "two", "three", "four", "five", "six"}, public, planner.PlanModeSync, true)
	require.ErrorContains(t, err, "five Vault keys")
	require.Equal(t, oldWrites, keys.Writes, "planning must not retire keys to make space")
	prune, err := plan([]string{"replacement"}, public[2:3], planner.PlanModeSync, true)
	require.NoError(t, err)
	result = execute(prune)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, keys.Keys, 1)
	// A declared key cannot confer authority to cascade another attachment.
	late, err := client.CreateIdentityVaultKey(
		t.Context(),
		root.ID,
		kkComps.CreateVaultKeyBody{Name: "late", PublicKey: public[0]},
	)
	require.NoError(t, err)
	_, err = plan([]string{"replacement"}, public[2:3], planner.PlanModeDelete, true)
	require.ErrorContains(t, err, "without explicit planned deletion")
	require.Zero(t, roots.Deletes)
	require.NoError(t, client.DeleteIdentityVaultKey(t.Context(), root.ID, late.ID))
	deletion, err := plan([]string{"replacement"}, public[2:3], planner.PlanModeDelete, true)
	require.NoError(t, err)
	require.Len(t, deletion.Changes, 2)
	for _, change := range deletion.Changes {
		if change.ResourceType == planner.ResourceTypeIdentityDirectory {
			require.NotEmpty(t, change.DependsOn, "directory waits for key retirement")
		}
	}
	result = execute(deletion)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Empty(t, keys.Keys)
	require.Equal(t, 1, roots.Deletes)
}

func TestVaultKeyPayloadCannotChangeMaterialThroughPATCH(t *testing.T) {
	a := &identityVaultKeyAdapter{}
	fields := map[string]any{planner.FieldName: "key", planner.FieldPublicKey: vaultKeyPublicFixtures(t)[0]}
	var request kkComps.PatchVaultKeyBody
	require.NoError(t, a.MapUpdateFields(t.Context(), nil, fields, &request, nil))
	data, err := json.Marshal(request)
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"key"}`, string(data))
	require.NoError(
		t,
		validateMappedPayload(planner.ResourceTypeIdentityVaultKey, planner.ActionUpdate, fields, request),
	)
	fields[planner.FieldManagedBy] = map[string]string{"tool": "kongctl"}
	require.Error(t, a.MapUpdateFields(t.Context(), nil, fields, &request, nil))
}

func TestVaultKeySavedCreationAndDeletionRecheckCollectionIdentity(t *testing.T) {
	public := vaultKeyPublicFixtures(t)
	root := fixture.Directory("default", "vault-team", false)
	root.VaultEnabled = new(true)
	keys := &fixture.VaultKeysAPI{}
	client := state.NewClient(state.ClientConfig{
		RegistryAgentReferencesReader: &fixture.RegistryReferencesAPI{},
		IdentityDirectoriesAPI:        &fixture.DirectoryAPI{Current: root}, IdentityVaultKeysAPI: keys,
	})
	for _, material := range public[:5] {
		_, err := client.CreateIdentityVaultKey(t.Context(), root.ID,
			kkComps.CreateVaultKeyBody{Name: "existing", PublicKey: material})
		require.NoError(t, err)
	}
	a := &identityVaultKeyAdapter{client: client}
	change := planner.PlannedChange{
		ResourceType: planner.ResourceTypeIdentityVaultKey, Action: planner.ActionCreate, Namespace: "vault-team",
		Parent: &planner.ParentInfo{ID: root.ID},
		ProtectingParent: &planner.ProtectingParentInfo{
			ResourceType: planner.ResourceTypeIdentityDirectory, ResourceName: root.Name, ResourceID: root.ID,
		},
		Fields: map[string]any{planner.FieldName: "new", planner.FieldPublicKey: public[5]},
	}
	before := keys.Writes
	_, err := a.Create(t.Context(), kkComps.CreateVaultKeyBody{Name: "new", PublicKey: public[5]},
		"vault-team", NewExecutionContext(&change))
	require.ErrorContains(t, err, "five Vault keys")
	require.Equal(t, before, keys.Writes)
	change.Action = planner.ActionDelete
	change.ResourceID = keys.Keys[0].ID
	err = a.Delete(t.Context(), change.ResourceID, NewExecutionContext(&change))
	require.ErrorContains(t, err, "identity changed")
	require.Equal(t, before, keys.Writes)
	change.Action = planner.ActionCreate
	_, err = a.Create(t.Context(), kkComps.CreateVaultKeyBody{Name: "new", PublicKey: public[0]},
		"vault-team", NewExecutionContext(&change))
	require.ErrorContains(t, err, "already exists")
	require.Equal(t, before, keys.Writes)
}

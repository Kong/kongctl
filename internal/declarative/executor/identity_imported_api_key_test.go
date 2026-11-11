package executor

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const importedKeysManifest = `_defaults:
  kongctl: {namespace: keys}
identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      principals:
        - ref: worker
          type: user
          api_keys:
            - ref: worker-primary
              type: imported
              name: Worker primary
              secret: !secret {source: !env IMPORTED_KEY_TEST}
              expires_at: 2030-01-01T00:00:00Z
`

func TestImportedAPIKeySavedPlanLifecycleWithoutCapture(t *testing.T) {
	for _, sourceDeclared := range []bool{true, false} {
		name := "source_omitted"
		if sourceDeclared {
			name = "source_declared"
		}
		t.Run(name, func(t *testing.T) { testImportedAPIKeySavedPlanLifecycle(t, sourceDeclared) })
	}
}

func testImportedAPIKeySavedPlanLifecycle(t *testing.T, sourceDeclared bool) {
	client, _, _, api := generatedKeysTestState()
	t.Setenv("IMPORTED_KEY_TEST", "fixture-imported-material")
	plan := identityClientTestPlan(t, client, importedKeysManifest, planner.PlanModeApply)
	require.Len(t, plan.Changes, 1)
	encoded, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "fixture-imported-material")
	require.NotContains(t, plan.Changes[0].Fields, planner.FieldSecret)
	result := executeIdentityClientTest(t, client, plan, "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Empty(t, result.GeneratedFiles)
	require.Len(t, api.Writes, 1)
	require.Equal(t, "fixture-imported-material", api.Writes[0].Body[planner.FieldSecret])
	require.Equal(t, resources.IdentityAPIKeyImportedType,
		api.Writes[0].Body[planner.FieldManagedBy].(map[string]any)[resources.IdentityAPIKeyCreateTypeKey])
	public, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(public), "fixture-imported-material")

	// Replaying a successful create must never issue a second key.
	result = executeIdentityClientTest(t, client, plan, "")
	require.Positive(t, result.FailureCount)
	require.Len(t, api.Writes, 1)
	t.Setenv("IMPORTED_KEY_TEST", "different-fixture-material")
	require.Empty(t, identityClientTestPlan(t, client, importedKeysManifest, planner.PlanModeApply).Changes)
	metadataOnly := strings.Replace(importedKeysManifest, "name: Worker primary", "name: Renamed", 1)
	if !sourceDeclared {
		metadataOnly = strings.Replace(metadataOnly,
			"              secret: !secret {source: !env IMPORTED_KEY_TEST}\n", "", 1)
	}
	t.Setenv("IMPORTED_KEY_TEST", "")
	plan = identityClientTestPlan(t, client, metadataOnly, planner.PlanModeApply)
	require.Len(t, plan.Changes, 1)
	require.Empty(t, plan.Changes[0].SecretWrites)
	result = executeIdentityClientTest(t, client, plan, "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.NotContains(t, api.Writes[1].Body, planner.FieldSecret)
	require.Equal(t, "2030-01-01T00:00:00Z", api.Writes[1].Body[planner.FieldExpiresAt])
	require.Empty(t, identityClientTestPlan(t, client, metadataOnly, planner.PlanModeApply).Changes)

	// Saved updates pin the creation type as well as the ref and parent.
	for path, keys := range api.Keys {
		keys[0].ManagedBy[resources.IdentityAPIKeyCreateTypeKey] = resources.IdentityAPIKeyGeneratedType
		api.Keys[path] = keys
	}
	result = executeIdentityClientTest(t, client, plan, "")
	require.Positive(t, result.FailureCount)
	require.Len(t, api.Writes, 2)
	for path, keys := range api.Keys {
		keys[0].ManagedBy[resources.IdentityAPIKeyCreateTypeKey] = resources.IdentityAPIKeyImportedType
		api.Keys[path] = keys
	}
	plan = identityClientTestPlan(t, client, metadataOnly, planner.PlanModeDelete)
	result = executeIdentityClientTest(t, client, plan, "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	for _, keys := range api.Keys {
		require.Empty(t, keys)
	}
}

func TestImportedAPIKeyPreflightRejectsMaterialBeforeAnyWrites(t *testing.T) {
	for _, material := range []string{"", strings.Repeat("s", 256), string([]byte{0xff})} {
		t.Run("invalid", func(t *testing.T) {
			client, principals, _, api := generatedKeysTestState()
			principals.Directories["4078f559-063b-4f10-9098-9e6553b21d9b"] = nil
			t.Setenv("IMPORTED_KEY_TEST", material)
			plan := identityClientTestPlan(t, client, importedKeysManifest, planner.PlanModeApply)
			result := executeIdentityClientTest(t, client, plan, "")
			require.Positive(t, result.FailureCount)
			require.Empty(t, api.Writes)
			require.Empty(t, principals.Directories["4078f559-063b-4f10-9098-9e6553b21d9b"])
			if len(material) > 1 {
				data, err := json.Marshal(result)
				require.NoError(t, err)
				require.NotContains(t, string(data), material)
			}
		})
	}
}

func TestImportedAPIKeySavedFileSourceAndParts(t *testing.T) {
	client, _, _, api := generatedKeysTestState()
	dir := t.TempDir()
	file := filepath.Join(dir, "key")
	require.NoError(t, os.WriteFile(file, []byte("file-material"), 0o600))
	manifest := filepath.Join(dir, "manifest.yaml")
	input := strings.Replace(importedKeysManifest, "!secret {source: !env IMPORTED_KEY_TEST}",
		`!secret {parts: ["prefix-", !file ./key]}`, 1)
	require.NoError(t, os.WriteFile(manifest, []byte(input), 0o600))
	rs, err := loader.New().LoadFile(manifest)
	require.NoError(t, err)
	plan, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, planner.Options{
		Mode: planner.PlanModeSync,
	})
	require.NoError(t, err)
	encoded, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "file-material")
	var saved planner.Plan
	require.NoError(t, json.Unmarshal(encoded, &saved))
	require.NoError(t, os.Remove(file))
	result := executeIdentityClientTest(t, client, &saved, "")
	require.Positive(t, result.FailureCount)
	require.Empty(t, api.Writes)
	require.NoError(t, os.WriteFile(file, []byte("changed-file-material"), 0o600))
	result = executeIdentityClientTest(t, client, &saved, "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Equal(t, "prefix-changed-file-material", api.Writes[0].Body[planner.FieldSecret])
	// A saved plan cannot escape its execution boundary through a file source.
	otherClient, _, _, otherAPI := generatedKeysTestState()
	result = NewWithOptions(otherClient, nil, false, Options{PlanBaseDir: t.TempDir()}).Execute(t.Context(), &saved)
	require.Positive(t, result.FailureCount)
	require.Empty(t, otherAPI.Writes)
}

func TestImportedAPIKeySavedPlansRejectInvalidSecretContracts(t *testing.T) {
	for _, mode := range []string{"missing", "raw", "literal", "update", "group", "type"} {
		t.Run(mode, func(t *testing.T) {
			client, _, _, api := generatedKeysTestState()
			plan := identityClientTestPlan(t, client, importedKeysManifest, planner.PlanModeApply)
			change := &plan.Changes[0]
			switch mode {
			case "missing":
				change.SecretWrites = nil
			case "raw":
				change.Fields[planner.FieldSecret] = "never-print-this-material"
			case "literal":
				change.SecretWrites[0].Expression = tags.SecretExpression{
					Parts: []tags.SecretPart{{Literal: new("never-print-this-material")}},
				}
			case "update":
				change.Action = planner.ActionUpdate
			case "group":
				change.ResourceType = planner.ResourceTypeIdentityGroupAPIKey
			case "type":
				change.CreationType = resources.IdentityAPIKeyGeneratedType
			}
			result := executeIdentityClientTest(t, client, plan, "")
			require.Positive(t, result.FailureCount)
			require.Empty(t, api.Writes)
			data, err := json.Marshal(result)
			require.NoError(t, err)
			require.NotContains(t, string(data), "never-print-this-material")
		})
	}
}

func TestImportedAPIKeyExternalParentScopedSync(t *testing.T) {
	client, principals, _, api := generatedKeysTestState()
	principal := &principals.Directories["4078f559-063b-4f10-9098-9e6553b21d9b"][0]
	principal.ManagedBy = nil
	principal.ProvisionedBy = new(kkComps.ProvisionedBy("imported"))
	input := strings.Replace(importedKeysManifest, "          type: user",
		"          _external: {id: 5078f559-063b-4f10-9098-9e6553b21d9b}", 1)
	t.Setenv("IMPORTED_KEY_TEST", "external-parent-material")
	plan := identityClientTestPlan(t, client, input, planner.PlanModeSync)
	require.Nil(t, plan.Changes[0].ProtectingParent)
	result := executeIdentityClientTest(t, client, plan, "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	for path, keys := range api.Keys {
		require.Equal(t, "keys", keys[0].ManagedBy[resources.ChildNamespaceKey])
		foreign := keys[0]
		foreign.ID = "7078f559-063b-4f10-9098-9e6553b21d9b"
		foreign.ManagedBy = resources.BuildChildManagedBy("other-team", false, "old")
		foreign.ManagedBy[resources.ManagedByRefKey] = "worker-primary"
		foreign.ManagedBy[resources.IdentityAPIKeyCreateTypeKey] = resources.IdentityAPIKeyImportedType
		delete(foreign.ManagedBy, resources.ChildProtectedKey)
		api.Keys[path] = append(keys, foreign)
	}
	omitted := strings.Split(input, "          api_keys:")[0]
	require.Empty(t, identityClientTestPlan(t, client, omitted, planner.PlanModeSync).Changes)
	plan = identityClientTestPlan(t, client, omitted+"          api_keys: []\n", planner.PlanModeSync)
	require.Len(t, plan.Changes, 1)
	result = executeIdentityClientTest(t, client, plan, "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	for _, keys := range api.Keys {
		require.Len(t, keys, 1)
		require.Equal(t, "other-team", keys[0].ManagedBy[resources.ChildNamespaceKey])
	}
}

func TestImportedAPIKeyUncertainCreateDoesNotRetryOrDelete(t *testing.T) {
	client, _, _, api := generatedKeysTestState()
	t.Setenv("IMPORTED_KEY_TEST", "fixture-private-material")
	api.Failure = "post-uncertain"
	plan := identityClientTestPlan(t, client, importedKeysManifest, planner.PlanModeApply)
	result := executeIdentityClientTest(t, client, plan, "")
	require.Positive(t, result.FailureCount)
	require.Len(t, api.Writes, 1)
	require.Equal(t, http.MethodPost, api.Writes[0].Method)
	data, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(data), "fixture-private-material")
	require.Contains(t, string(data), "inspect parent metadata before retrying")
	for _, keys := range api.Keys {
		require.Len(t, keys, 1, "uncertain issuance must not automatically retire the key")
	}
}

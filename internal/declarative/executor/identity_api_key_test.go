package executor

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

const generatedKeysManifest = `_defaults:
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
              type: v1
              name: Worker primary
              expires_at: 2030-01-01T00:00:00Z
      groups:
        - ref: operators
          name: Operators
          api_keys:
            - ref: operators-primary
              type: v1
              name: Operators primary
              expires_at: 2030-01-01T00:00:00Z
`

func generatedKeysTestState() (*state.Client, *fixture.PrincipalAPI, *fixture.GroupAPI, *fixture.APIKeyHTTP) {
	const directory = "4078f559-063b-4f10-9098-9e6553b21d9b"
	principals := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directory: {{
		ID: "5078f559-063b-4f10-9098-9e6553b21d9b", Type: new(kkComps.KongPrincipalTypeUser),
		ProvisionedBy: new(kkComps.ProvisionedByKonnectUser),
		ManagedBy:     resources.BuildPrincipalManagedBy("keys", "worker", false, "old"),
	}}}}
	groups := &fixture.GroupAPI{
		Directories: map[string][]helpers.IdentityGroup{directory: {fixture.Group("Operators", "keys", false)}},
	}
	keys := &fixture.APIKeyHTTP{PageSize: 1}
	sdk := keys.SDK()
	return state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{
			Directories: []kkComps.KongDirectory{{ID: directory, Name: "default"}},
		},
		IdentityPrincipalsAPI: principals, IdentityGroupsAPI: groups,
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{}, IdentityBasicAuthAPI: &fixture.BasicAuthAPI{},
		IdentityPrincipalKeysAPI: sdk.GetIdentityPrincipalKeysAPI(), IdentityGroupKeysAPI: sdk.GetIdentityGroupKeysAPI(),
		IdentityPrincipalKeysReader: sdk.GetIdentityPrincipalKeysAPI(),
	}), principals, groups, keys
}

func TestGeneratedAPIKeysSavedPlanLifecycleAndPrivateCapture(t *testing.T) {
	client, _, _, api := generatedKeysTestState()
	plan := identityClientTestPlan(t, client, generatedKeysManifest, planner.PlanModeApply)
	require.Len(t, plan.Changes, 2)
	for _, change := range plan.Changes {
		metadata := change.Fields[planner.FieldManagedBy].(map[string]any)
		require.Len(t, metadata, 4)
		require.Equal(t, change.ResourceRef, metadata[resources.ManagedByRefKey])
		require.NotContains(t, metadata, resources.ChildNamespaceKey)
		require.NotContains(t, metadata, resources.ChildProtectedKey)
	}
	result := executeIdentityClientTest(t, client, plan, "")
	require.Positive(t, result.FailureCount)
	require.Empty(t, api.Writes)
	out := t.TempDir()
	result = executeIdentityClientTest(t, client, plan, out)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, result.GeneratedFiles, 2)
	for _, path := range result.GeneratedFiles {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var capture map[string]any
		require.NoError(t, json.Unmarshal(data, &capture))
		require.Equal(t, "captured", capture["status"])
		require.Equal(t, "fixture-private-material", capture["secret"])
		require.Equal(t, "keys", capture["namespace"])
		require.Equal(t, "4078f559-063b-4f10-9098-9e6553b21d9b", capture["directory_id"])
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
	public, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(public), "fixture-private-material")
	require.Empty(t, identityClientTestPlan(t, client, generatedKeysManifest, planner.PlanModeApply).Changes)

	// Name-only updates must hydrate expiration: native PATCH omission clears it.
	update := strings.Replace(generatedKeysManifest, "name: Worker primary", "name: Renamed worker", 1)
	update = strings.ReplaceAll(update, "              expires_at: 2030-01-01T00:00:00Z\n", "")
	plan = identityClientTestPlan(t, client, update, planner.PlanModeApply)
	require.Len(t, plan.Changes, 1)
	require.NotContains(t, plan.Changes[0].Fields, planner.FieldManagedBy)
	result = executeIdentityClientTest(t, client, plan, "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	write := api.Writes[len(api.Writes)-1]
	require.Equal(t, "2030-01-01T00:00:00Z", write.Body["expires_at"])
	require.NotContains(t, write.Body, "managed_by")
	cleared := strings.ReplaceAll(
		update,
		"              type: v1",
		"              type: v1\n              expires_at: null",
	)
	plan = identityClientTestPlan(t, client, cleared, planner.PlanModeApply)
	require.Len(t, plan.Changes, 2)
	result = executeIdentityClientTest(t, client, plan, "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Empty(t, identityClientTestPlan(t, client, cleared, planner.PlanModeApply).Changes)
	for _, values := range api.Keys {
		for _, key := range values {
			require.Nil(t, key.ExpiresAt)
			require.Len(t, key.ManagedBy, 4)
		}
	}
	plan = identityClientTestPlan(t, client, cleared, planner.PlanModeDelete)
	result = executeIdentityClientTest(t, client, plan, "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	for _, values := range api.Keys {
		require.Empty(t, values)
	}
}

func TestGeneratedAPIKeysResolveNewParentsBeforeIssuance(t *testing.T) {
	client, principals, groups, api := generatedKeysTestState()
	principals.Directories = make(map[string][]kkComps.KongPrincipal)
	groups.Directories = make(map[string][]helpers.IdentityGroup)
	plan := identityClientTestPlan(t, client, generatedKeysManifest, planner.PlanModeApply)
	require.Len(t, plan.Changes, 4)
	result := executeIdentityClientTest(t, client, plan, "")
	require.Positive(t, result.FailureCount)
	require.Zero(t, principals.Creates)
	require.Zero(t, groups.Creates)
	result = executeIdentityClientTest(t, client, plan, t.TempDir())
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Equal(t, 1, principals.Creates)
	require.Equal(t, 1, groups.Creates)
	require.Len(t, api.Writes, 2)
	for _, path := range result.GeneratedFiles {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		var capture map[string]any
		require.NoError(t, json.Unmarshal(data, &capture))
		require.Equal(t, "captured", capture["status"])
		if capture["principal_id"] != nil {
			require.Equal(
				t,
				principals.Directories["4078f559-063b-4f10-9098-9e6553b21d9b"][0].ID,
				capture["principal_id"],
			)
		} else {
			require.Equal(t, groups.Directories["4078f559-063b-4f10-9098-9e6553b21d9b"][0].ID, capture["group_id"])
		}
	}
	before := len(api.Writes)
	result = executeIdentityClientTest(t, client, plan, t.TempDir())
	require.Positive(t, result.FailureCount, "a saved create cannot silently reissue a matched key")
	require.Len(t, api.Writes, before)
}

func TestGeneratedAPIKeysRejectUnsupportedSavedPayloadBeforeWrites(t *testing.T) {
	faults := []string{"secret", "imported", "null-name", "ownership", "independent-protection", "missing-parent-context"}
	for _, fault := range faults {
		t.Run(fault, func(t *testing.T) {
			client, principals, groups, api := generatedKeysTestState()
			plan := identityClientTestPlan(t, client, generatedKeysManifest, planner.PlanModeApply)
			change := &plan.Changes[0]
			switch fault {
			case "secret":
				change.Fields["secret"] = "fixture-private-material"
			case "imported":
				change.Fields[planner.FieldType] = "imported"
			case "null-name":
				change.Fields[planner.FieldName] = nil
			case "ownership":
				change.Fields[planner.FieldManagedBy].(map[string]any)[resources.ChildNamespaceKey] = "keys"
			case "independent-protection":
				change.Protection = true
			case "missing-parent-context":
				change.ProtectingParent = nil
			}
			out := t.TempDir()
			result := executeIdentityClientTest(t, client, plan, out)
			require.Positive(t, result.FailureCount)
			require.Empty(t, api.Writes)
			require.Empty(t, principals.Writes)
			require.Empty(t, groups.Writes)
			entries, err := os.ReadDir(out)
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}
}

func TestGeneratedAPIKeySavedWritesRecheckParentAndIdentity(t *testing.T) {
	for _, group := range []bool{false, true} {
		faults := []string{
			"namespace", "unowned", "identity", "protection", "imported", "duplicate", "legacy", "changed-id", "failed-list",
		}
		for _, fault := range faults {
			t.Run(fault+"/group="+map[bool]string{true: "yes", false: "no"}[group], func(t *testing.T) {
				client, principals, groups, api := generatedKeysTestState()
				plan := identityClientTestPlan(t, client, generatedKeysManifest, planner.PlanModeApply)
				result := executeIdentityClientTest(t, client, plan, t.TempDir())
				require.Zero(t, result.FailureCount, "%+v", result.Errors)
				changed := strings.ReplaceAll(generatedKeysManifest, "2030-01-01T00:00:00Z", "2031-01-01T00:00:00Z")
				plan = identityClientTestPlan(t, client, changed, planner.PlanModeApply)
				var selected planner.PlannedChange
				for _, change := range plan.Changes {
					if (change.ResourceType == planner.ResourceTypeIdentityGroupAPIKey) == group {
						selected = change
					}
				}
				plan.Changes = []planner.PlannedChange{selected}
				plan.ExecutionOrder = []string{selected.ID}
				plan.ExecutionGroups = [][]string{{selected.ID}}
				directory := selected.References[planner.FieldDirectoryID].ID
				principal := &principals.Directories[directory][0]
				ordinary := &groups.Directories[directory][0]
				metadata := principal.ManagedBy
				if group {
					metadata = ordinary.ManagedBy
				}
				switch fault {
				case "namespace":
					metadata[resources.ChildNamespaceKey] = "other"
				case "unowned":
					delete(metadata, resources.ManagedByToolKey)
				case "identity":
					if group {
						ordinary.Name = "Different"
					} else {
						metadata[resources.ManagedByRefKey] = "different"
					}
				case "protection":
					metadata[resources.ChildProtectedKey] = "true"
				case "imported":
					if group {
						ordinary.Type = new(kkComps.KongGroupTypeTeam)
					} else {
						principal.ProvisionedBy = new(kkComps.ProvisionedBy("imported"))
					}
				case "failed-list":
					api.Failure = "list"
				default:
					for path, values := range api.Keys {
						if !strings.Contains(path, selected.Parent.ID) {
							continue
						}
						switch fault {
						case "duplicate":
							duplicate := values[0]
							duplicate.ID = "6078f559-063b-4f10-9098-9e6553b21d9b"
							api.Keys[path] = append(values, duplicate)
						case "legacy":
							delete(values[0].ManagedBy, resources.IdentityAPIKeyCreateTypeKey)
						case "changed-id":
							values[0].ID = "6078f559-063b-4f10-9098-9e6553b21d9b"
						}
					}
				}
				before := len(api.Writes)
				result = executeIdentityClientTest(t, client, plan, "")
				require.Positive(t, result.FailureCount)
				require.Len(t, api.Writes, before)
				public, err := json.Marshal(result)
				require.NoError(t, err)
				require.NotContains(t, string(public), "fixture-private-material")
				if fault == "duplicate" {
					require.Contains(t, string(public), "multiple API keys match persisted ref")
				}
			})
		}
	}
}

func TestGeneratedAPIKeyCaptureConflictsAndUncertainCreationPreserveOldKeys(t *testing.T) {
	client, _, _, api := generatedKeysTestState()
	plan := identityClientTestPlan(t, client, generatedKeysManifest, planner.PlanModeApply)
	out := t.TempDir()
	exec := NewWithOptions(client, nil, false, Options{SecretOutputDir: out})
	require.NoError(t, exec.prepareClientSecretOutputs(plan))
	result := executeIdentityClientTest(t, client, plan, out)
	require.Positive(t, result.FailureCount)
	require.Empty(t, api.Writes)
	exec.finishClientSecretOutputs(&ExecutionResult{})
	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	require.Empty(t, entries)
	result = executeIdentityClientTest(t, client, plan, out)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	replacement := strings.ReplaceAll(generatedKeysManifest, "-primary", "-replacement")
	plan = identityClientTestPlan(t, client, replacement, planner.PlanModeSync)
	require.Len(t, plan.Changes, 4)
	api.Before = func(request *http.Request) {
		if request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/api-keys/") {
			paths, err := filepath.Glob(filepath.Join(out, "replacement", "*.json"))
			require.NoError(t, err)
			captured := false
			for _, path := range paths {
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				captured = captured || strings.Contains(string(data), `"status": "captured"`)
			}
			require.True(t, captured, "issuance must be captured before public read-back")
			api.Failure = "read"
		}
	}
	before := len(api.Writes)
	result = executeIdentityClientTest(t, client, plan, filepath.Join(out, "replacement"))
	require.Positive(t, result.FailureCount)
	require.NotEmpty(t, result.GeneratedFiles)
	for _, write := range api.Writes[before:] {
		require.Equal(t, http.MethodPost, write.Method, "failed replacement cannot retire existing keys")
	}
	require.LessOrEqual(t, len(api.Writes)-before, 2, "each replacement issues at most once")
	for _, values := range api.Keys {
		found := false
		for _, value := range values {
			found = found || strings.HasSuffix(value.ManagedBy[resources.ManagedByRefKey], "-primary")
		}
		require.True(t, found)
	}
}

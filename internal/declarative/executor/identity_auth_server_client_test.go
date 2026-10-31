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
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	logctx "github.com/kong/kongctl/internal/log"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

const identityClientManifest = `_defaults:
  kongctl: {namespace: z-auth-team}
identity:
  auth_servers:
    - ref: issuer
      name: issuer
      audience: https://api.example.test
      scopes:
        - {ref: read, name: read:data}
      clients:
        - ref: worker
          name: worker
          kongctl: {namespace: client-team}
          grant_types: [client_credentials]
          response_types: []
          redirect_uris: [https://app.example.test/callback]
          login_uri: https://app.example.test/login
          access_token_duration: 900
          id_token_duration: 1200
          allow_all_scopes: false
          allow_scopes: [!ref read#id]
          token_endpoint_auth_method: client_secret_basic
          labels: {team: platform}
`

func identityClientTestPlan(t *testing.T, client *state.Client, input string, mode planner.PlanMode) *planner.Plan {
	t.Helper()
	file := filepath.Join(t.TempDir(), "manifest.yaml")
	require.NoError(t, os.WriteFile(file, []byte(input), 0o600))
	rs, err := loader.New().LoadFile(file)
	require.NoError(t, err)
	plan, err := planner.NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, planner.Options{Mode: mode})
	require.NoError(t, err)
	encoded, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "fixture-initial-secret")
	var saved planner.Plan
	require.NoError(t, json.Unmarshal(encoded, &saved))
	return &saved
}

func identityClientTestState() (*state.Client, *fixture.AuthServerAPI, *fixture.AuthServerChildrenAPI) {
	roots, children := &fixture.AuthServerAPI{}, &fixture.AuthServerChildrenAPI{}
	client := state.NewClient(state.ClientConfig{
		IdentityAuthServersAPI:      roots,
		IdentityAuthServerScopesAPI: children, IdentityAuthServerClaimsAPI: children,
		IdentityAuthServerClientAPI: children,
	})
	return client, roots, children
}

func executeIdentityClientTest(t *testing.T, client *state.Client, plan *planner.Plan, output string) *ExecutionResult {
	t.Helper()
	ctx := context.WithValue(t.Context(), logctx.LoggerKey, slog.Default())
	return NewWithOptions(client, nil, false, Options{MaxConcurrency: 1, SecretOutputDir: output}).Execute(ctx, plan)
}

func TestIdentityClientSavedPlanLifecycleAndInitialSecret(t *testing.T) {
	client, roots, children := identityClientTestState()
	plan := identityClientTestPlan(t, client, identityClientManifest, planner.PlanModeApply)
	require.Len(t, plan.Changes, 3)
	result := executeIdentityClientTest(t, client, plan, "")
	require.Positive(t, result.FailureCount)
	require.Empty(t, roots.Servers, "missing capture must fail before even the parent write")
	out := filepath.Join(t.TempDir(), "private")
	result = executeIdentityClientTest(t, client, plan, out)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, result.GeneratedFiles, 1)
	captured, err := os.ReadFile(result.GeneratedFiles[0])
	require.NoError(t, err)
	var record map[string]any
	require.NoError(t, json.Unmarshal(captured, &record))
	require.Equal(t, "captured", record["status"])
	require.Equal(t, "fixture-initial-secret", record["secret"])
	require.Equal(t, "worker", record["resource_ref"])
	require.Equal(t, "client-team", record["namespace"])
	info, err := os.Stat(result.GeneratedFiles[0])
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	parent := roots.Servers[0].ID
	require.Len(t, children.Clients[parent], 1)
	require.Equal(t, children.Scopes[parent][0].ID, children.Clients[parent][0].AllowScopes[0])
	require.Equal(t, "client-team", children.Clients[parent][0].Labels[labels.NamespaceKey])
	require.Empty(t, identityClientTestPlan(t, client, identityClientManifest, planner.PlanModeApply).Changes)
	// Independent client namespaces must not widen auth-server pruning.
	otherRoot := fixture.AuthServer("unrelated", "client-team", false)
	otherRoot.ID = "22345678-1234-4123-8123-123456789012"
	roots.Servers = append(roots.Servers, otherRoot)
	require.Empty(t, identityClientTestPlan(t, client, identityClientManifest, planner.PlanModeSync).Changes)
	roots.Servers = roots.Servers[:1]

	updated := strings.ReplaceAll(
		identityClientManifest,
		"          token_endpoint_auth_method: client_secret_basic\n",
		"",
	)
	updated = strings.ReplaceAll(updated, "redirect_uris: [https://app.example.test/callback]", "redirect_uris: []")
	updated = strings.ReplaceAll(updated, "allow_scopes: [!ref read#id]", "allow_scopes: []")
	updated = strings.ReplaceAll(updated, "labels: {team: platform}", "labels: {}")
	updated = strings.ReplaceAll(updated, "      scopes:\n        - {ref: read, name: read:data}", "      scopes: []")
	result = executeIdentityClientTest(t, client, identityClientTestPlan(t, client, updated, planner.PlanModeSync), "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	value := children.Clients[parent][0]
	require.Empty(t, value.RedirectUris)
	require.Empty(t, value.AllowScopes)
	require.Empty(t, children.Scopes[parent], "scope removal must follow the explicit client detachment")
	require.NotContains(t, value.Labels, "team")
	require.Equal(
		t,
		"client_secret_basic",
		string(*value.TokenEndpointAuthMethod),
		"omitted auth method must be preserved",
	)
	require.Nil(t, children.ClientUpdates[0].UpdateClient.ClientSecret)
	require.Empty(t, identityClientTestPlan(t, client, updated, planner.PlanModeApply).Changes)
	result = executeIdentityClientTest(
		t,
		client,
		identityClientTestPlan(t, client, updated, planner.PlanModeDelete),
		"",
	)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Empty(t, children.Clients[parent])
	require.Equal(t, 1, roots.Deletes)
}

func TestIdentityClientFreshGuardsAndProtection(t *testing.T) {
	client, roots, children := identityClientTestState()
	result := executeIdentityClientTest(
		t,
		client,
		identityClientTestPlan(t, client, identityClientManifest, planner.PlanModeApply),
		t.TempDir(),
	)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	parent := roots.Servers[0].ID
	updated := strings.Replace(identityClientManifest, "access_token_duration: 900", "access_token_duration: 1800", 1)
	plan := identityClientTestPlan(t, client, updated, planner.PlanModeApply)
	writes := children.Writes
	children.Clients[parent][0].Labels[labels.NamespaceKey] = "foreign"
	result = executeIdentityClientTest(t, client, plan, "")
	require.Positive(t, result.FailureCount)
	require.Equal(t, writes, children.Writes)
	children.Clients[parent][0].Labels[labels.NamespaceKey] = "client-team"
	roots.Servers[0].Name = "renamed"
	result = executeIdentityClientTest(t, client, plan, "")
	require.Positive(t, result.FailureCount)
	require.Equal(t, writes, children.Writes)
	roots.Servers[0].Name = "issuer"
	protected := strings.Replace(
		identityClientManifest,
		"namespace: client-team}",
		"namespace: client-team, protected: true}",
		1,
	)
	result = executeIdentityClientTest(
		t,
		client,
		identityClientTestPlan(t, client, protected, planner.PlanModeApply),
		"",
	)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.True(t, labels.IsProtectedResource(children.Clients[parent][0].Labels))
	result = executeIdentityClientTest(
		t,
		client,
		identityClientTestPlan(t, client, identityClientManifest, planner.PlanModeApply),
		"",
	)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.False(t, labels.IsProtectedResource(children.Clients[parent][0].Labels))
	result = executeIdentityClientTest(t, client,
		identityClientTestPlan(t, client, identityClientManifest, planner.PlanModeDelete), "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Empty(t, children.Clients[parent])
	require.Empty(t, children.Scopes[parent])
	require.Equal(t, 1, roots.Deletes)
}

func TestIdentityClientExternalParentAndNamespaceScopedSync(t *testing.T) {
	client, roots, children := identityClientTestState()
	roots.Servers = []kkComps.AuthServer{fixture.AuthServer("shared", "foreign-parent", true)}
	parent := roots.Servers[0].ID
	input := `identity:
  auth_servers:
    - ref: shared
      _external: {id: ` + parent + `}
  clients:
    - ref: worker
      auth_server: shared
      name: worker
      kongctl: {namespace: client-team}
      grant_types: [client_credentials]
      response_types: []
`
	result := executeIdentityClientTest(
		t,
		client,
		identityClientTestPlan(t, client, input, planner.PlanModeApply),
		t.TempDir(),
	)
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	children.Clients[parent] = append(
		children.Clients[parent],
		kkComps.AuthServerClient{
			ID:     "foreign",
			Name:   "foreign",
			Labels: map[string]string{labels.NamespaceKey: "other"},
		},
	)
	empty := `_defaults:
  kongctl: {namespace: client-team}
identity:
  auth_servers:
    - ref: shared
      _external: {id: ` + parent + `}
      clients: []
`
	plan := identityClientTestPlan(t, client, empty, planner.PlanModeSync)
	require.Len(t, plan.Changes, 1)
	result = executeIdentityClientTest(t, client, plan, "")
	require.Zero(t, result.FailureCount, "%+v", result.Errors)
	require.Len(t, children.Clients[parent], 1)
	require.Equal(t, "foreign", children.Clients[parent][0].ID)
	require.Zero(t, roots.Deletes)
}

func TestIdentityClientCapturePreflightRejectsConflictsAndDiscardsUnattempted(t *testing.T) {
	client, roots, children := identityClientTestState()
	plan := identityClientTestPlan(t, client, identityClientManifest, planner.PlanModeApply)
	out := t.TempDir()
	exec := NewWithOptions(client, nil, false, Options{SecretOutputDir: out})
	require.NoError(t, exec.prepareClientSecretOutputs(plan))
	var reserved string
	for _, value := range exec.generatedOutputs {
		reserved = value.path
	}
	require.NotEmpty(t, reserved)
	other := executeIdentityClientTest(t, client, plan, out)
	require.Positive(t, other.FailureCount)
	require.Empty(t, roots.Servers)
	require.Zero(t, children.Writes)
	require.FileExists(t, reserved)
	exec.finishClientSecretOutputs(&ExecutionResult{})
	require.NoFileExists(t, reserved)
	// A stale parent identity fails before client creation, so its reservation
	// must be removed instead of leaving a misleading recovery marker.
	parent := fixture.AuthServer("issuer", "z-auth-team", false)
	roots.Servers = []kkComps.AuthServer{parent}
	input := strings.Replace(identityClientManifest, "      scopes:\n        - {ref: read, name: read:data}\n", "", 1)
	input = strings.Replace(input, "allow_scopes: [!ref read#id]", "allow_scopes: []", 1)
	plan = identityClientTestPlan(t, client, input, planner.PlanModeApply)
	roots.Servers[0].Name = "changed"
	result := executeIdentityClientTest(t, client, plan, out)
	require.Positive(t, result.FailureCount)
	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	require.Empty(t, entries)
	require.Empty(t, result.GeneratedFiles)
}

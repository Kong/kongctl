package smoke_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestGeneratedAIGatewayFixturesEnforceNamespace(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "kongctl")
	script, err := os.ReadFile("fake_kongctl.py")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(fake, script, 0o600))
	require.NoError(t, os.Chmod(fake, 0o700))
	t.Setenv("FAKE_KONGCTL_LOG", filepath.Join(dir, "commands.log"))
	t.Setenv("FAKE_KONGCTL_STATE", filepath.Join(dir, "state.json"))
	for _, key := range []string{
		"FAKE_FAIL_ON", "FAKE_FAIL_DELETE", "FAKE_BAD_SCAFFOLD", "FAKE_FAULT", "KONGCTL_SMOKE_YES",
	} {
		t.Setenv(key, "")
	}
	cmd := exec.CommandContext(t.Context(), "bash", "../../scripts/smoke-test.sh",
		"--quick", "--resources", "ai_gateway", "--binary", fake, "--artifacts-dir", dir)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	paths, err := filepath.Glob(filepath.Join(dir, "kongctl-smoke.*", "fixtures", "ai_gateway", "*.yaml"))
	require.NoError(t, err)
	require.Len(t, paths, 4)
	t.Setenv("KONGCTL_SMOKE_NAMESPACE_AI_GATEWAY", "smoke-test")
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			rs, err := loader.New().LoadFile(path)
			require.NoError(t, err)
			if filepath.Base(path) == "pruned.yaml" {
				assertAIGatewayPrunePlan(t, path, rs)
			}
			for _, kind := range []resources.ResourceType{
				resources.ResourceTypeAIGatewayProvider, resources.ResourceTypeAIGatewayPolicy,
				resources.ResourceTypeAIGatewayModel, resources.ResourceTypeAIGatewayMCPServer,
			} {
				namespaces := rs.SyncScope.OwnedChildNamespaces(kind, rs.AIGateways[0].Ref)
				require.Len(t, namespaces, 1, "%s", kind)
				require.Contains(t, namespaces, "smoke-test", "%s", kind)
			}
			if filepath.Base(path) != "pruned.yaml" {
				require.Len(t, rs.AIGatewayPolicies, 1)
				require.Len(t, rs.AIGatewayProviders, 1)
				require.Len(t, rs.AIGatewayModels, 1)
				require.Len(t, rs.AIGatewayMCPServers, 1)
			}
			v := validator.NewNamespaceValidator()
			require.NoError(t, v.ValidateNamespaceRequirement(rs, validator.NamespaceRequirement{
				Mode: validator.NamespaceRequirementSpecific, AllowedNamespaces: []string{"smoke-test"},
			}))
			require.Error(t, v.ValidateNamespaceRequirement(rs, validator.NamespaceRequirement{
				Mode: validator.NamespaceRequirementSpecific, AllowedNamespaces: []string{"another-team"},
			}))
		})
	}
}

func TestGeneratedIdentityFixturesEnforceNamespace(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "kongctl")
	script, err := os.ReadFile("fake_kongctl.py")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(fake, script, 0o600))
	require.NoError(t, os.Chmod(fake, 0o700))
	t.Setenv("FAKE_KONGCTL_LOG", filepath.Join(dir, "commands.log"))
	t.Setenv("FAKE_KONGCTL_STATE", filepath.Join(dir, "state.json"))
	for _, key := range []string{
		"FAKE_FAIL_ON", "FAKE_FAIL_DELETE", "FAKE_BAD_SCAFFOLD", "FAKE_FAULT", "KONGCTL_SMOKE_YES",
	} {
		t.Setenv(key, "")
	}
	cmd := exec.CommandContext(t.Context(), "bash", "../../scripts/smoke-test.sh",
		"--quick", "--resources", "identity_principal,identity_auth_server", "--binary", fake, "--artifacts-dir", dir)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	t.Setenv("KONGCTL_SMOKE_NAMESPACE_IDENTITY_PRINCIPAL", "smoke-test")
	t.Setenv("KONGCTL_SMOKE_NAMESPACE_IDENTITY_AUTH_SERVER", "smoke-test")
	paths, err := filepath.Glob(filepath.Join(dir, "kongctl-smoke.*", "fixtures", "identity_*", "*.yaml"))
	require.NoError(t, err)
	require.Len(t, paths, 4)
	for _, path := range paths {
		t.Run(filepath.Base(filepath.Dir(path))+"/"+filepath.Base(path), func(t *testing.T) {
			rs, err := loader.New().LoadFile(path)
			require.NoError(t, err)
			require.NotNil(t, rs.Identity)
			if len(rs.Identity.Directories) != 0 {
				require.Len(t, rs.Identity.Directories, 1)
				require.NotNil(t, rs.Identity.Directories[0].External,
					"smoke must use the existing default directory without managing it")
				require.Len(t, rs.Identity.Principals, 1)
				require.Len(t, rs.Identity.Identities, 1)
				assertIdentityPrincipalCreatePlan(t, rs)
			} else {
				require.Len(t, rs.Identity.AuthServers, 1)
				require.Len(t, rs.Identity.Scopes, 1)
				require.Len(t, rs.Identity.Claims, 1)
			}
			v := validator.NewNamespaceValidator()
			require.NoError(t, v.ValidateNamespaceRequirement(rs, validator.NamespaceRequirement{
				Mode: validator.NamespaceRequirementSpecific, AllowedNamespaces: []string{"smoke-test"},
			}))
			require.Error(t, v.ValidateNamespaceRequirement(rs, validator.NamespaceRequirement{
				Mode: validator.NamespaceRequirementSpecific, AllowedNamespaces: []string{"another-team"},
			}))
		})
	}
}

func TestIdentityBootstrapFixtureCreatesOnlyProtectedDefaultDirectory(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "kongctl")
	script, err := os.ReadFile("fake_kongctl.py")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(fake, script, 0o600))
	require.NoError(t, os.Chmod(fake, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "state.directories.json"), []byte("[]"), 0o600))
	t.Setenv("FAKE_KONGCTL_LOG", filepath.Join(dir, "commands.log"))
	t.Setenv("FAKE_KONGCTL_STATE", filepath.Join(dir, "state.json"))
	for _, key := range []string{"FAKE_FAIL_ON", "FAKE_FAIL_DELETE", "FAKE_BAD_SCAFFOLD", "FAKE_FAULT"} {
		t.Setenv(key, "")
	}
	cmd := exec.CommandContext(t.Context(), "bash", "../../scripts/smoke-test.sh",
		"--quick", "--resources", "identity_principal", "--binary", fake, "--artifacts-dir", dir)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	paths, err := filepath.Glob(filepath.Join(dir, "kongctl-smoke.*", "fixtures", "identity-directory-bootstrap.yaml"))
	require.NoError(t, err)
	require.Len(t, paths, 1)
	desired, err := loader.New().LoadFile(paths[0])
	require.NoError(t, err)
	p := planner.NewPlanner(state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{},
	}), slog.New(slog.NewTextHandler(io.Discard, nil)))
	plan, err := p.GeneratePlan(t.Context(), desired, planner.Options{Mode: planner.PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, planner.ActionCreate, change.Action)
	require.Equal(t, planner.ResourceTypeIdentityDirectory, change.ResourceType)
	require.Equal(t, "default", change.ResourceRef)
	require.Equal(t, "kctl-smoke-identity-bootstrap", change.Namespace)
	require.Equal(t, true, change.Protection)
	require.Equal(t, true, change.Fields[planner.FieldAllowAllControlPlanes])
}

func assertIdentityPrincipalCreatePlan(t *testing.T, desired *resources.ResourceSet) {
	t.Helper()
	directory := fixture.Directory("default", "", false)
	p := planner.NewPlanner(state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{
			Directories: []kkComps.KongDirectory{directory}, Current: directory,
		},
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{
			directory.ID: {
				{
					ID: "11111111-1111-4111-8111-111111111111", DisplayName: new("Unrelated managed principal"),
					ManagedBy: resources.BuildPrincipalManagedBy("other-team", "other-person", false, "dev"),
				},
				{ID: "22222222-2222-4222-8222-222222222222", DisplayName: new("Unmanaged principal")},
			},
		}},
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
		IdentityBasicAuthAPI:           &fixture.BasicAuthAPI{},
		IdentityPrincipalKeysReader:    &fixture.BasicAuthAPI{},
		IdentityGroupsAPI:              &fixture.GroupAPI{},
	}), slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, mode := range []planner.PlanMode{planner.PlanModeApply, planner.PlanModeSync} {
		plan, err := p.GeneratePlan(t.Context(), desired, planner.Options{Mode: mode})
		require.NoError(t, err)
		require.Len(t, plan.Changes, 2, "only the smoke principal and its custom identity may be created")
		for _, change := range plan.Changes {
			require.Equal(t, planner.ActionCreate, change.Action)
			require.Contains(t, []string{
				planner.ResourceTypeIdentityPrincipal, planner.ResourceTypeIdentityPrincipalIdentity,
			}, change.ResourceType)
			require.Equal(t, "smoke-test", change.Namespace)
		}
	}
}

type smokeReadClient func(*http.Request) (*http.Response, error)

func (f smokeReadClient) Do(req *http.Request) (*http.Response, error) { return f(req) }

func assertAIGatewayPrunePlan(t *testing.T, path string, desired *resources.ResourceSet) {
	t.Helper()
	observed, err := loader.New().LoadFile(filepath.Join(filepath.Dir(path), "updated.yaml"))
	require.NoError(t, err)
	responseObject := func(value any, id string) map[string]any {
		data, err := json.Marshal(value)
		require.NoError(t, err)
		var object map[string]any
		require.NoError(t, json.Unmarshal(data, &object))
		object[planner.FieldID] = id
		object["created_at"], object["updated_at"] = "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"
		return object
	}
	gateway := responseObject(observed.AIGateways[0].CreateAIGatewayRequest, "gateway-id")
	gateway[planner.FieldLabels] = map[string]string{labels.NamespaceKey: "smoke-test"}
	model := responseObject(observed.AIGatewayModels[0], "model-id")
	model[planner.FieldManagedBy] = resources.BuildModelManagedBy("smoke-test", false, "dev")
	store := responseObject(observed.AIGatewayDatastores[0], "store-id")
	list := func(items ...any) map[string]any {
		return map[string]any{"data": items, "meta": map[string]any{"page": map[string]any{"size": 100}}}
	}
	prefix := "/v1/ai-gateways"
	responses := map[string]any{
		prefix: list(gateway), prefix + "/gateway-id": gateway,
		prefix + "/gateway-id/models":              list(model),
		prefix + "/gateway-id/models/model-id":     model,
		prefix + "/gateway-id/datastores":          list(store),
		prefix + "/gateway-id/datastores/store-id": store,
		prefix + "/gateway-id/datastores/store-id/usage": list(map[string]any{
			"type": "model", "id": "model-id", "name": "smoke-model", "display_name": "Smoke model",
		}),
		prefix + "/gateway-id/model-providers": list(),
		prefix + "/gateway-id/policies":        list(),
		prefix + "/gateway-id/mcp-servers":     list(),
	}
	usageCalls := 0
	client := smokeReadClient(func(req *http.Request) (*http.Response, error) {
		value, ok := responses[req.URL.Path]
		if !ok || req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected smoke planning request: %s %s", req.Method, req.URL.Path)
		}
		if req.URL.Path == prefix+"/gateway-id/datastores/store-id/usage" {
			usageCalls++
		}
		data, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(bytes.NewReader(data)), Request: req,
		}, nil
	})
	sdk := &helpers.KonnectSDK{SDK: kkSDK.New(
		kkSDK.WithServerURL("https://konnect.test"), kkSDK.WithClient(client),
	)}
	p := planner.NewPlanner(state.NewClient(state.ClientConfig{
		AIGatewayAPI: sdk.GetAIGatewayAPI(), AIGatewayModelAPI: sdk.GetAIGatewayModelAPI(),
		AIGatewayDatastoresAPI: sdk.GetAIGatewayDatastoresAPI(),
		AIGatewayProvidersAPI:  sdk.GetAIGatewayProvidersAPI(), AIGatewayPoliciesAPI: sdk.GetAIGatewayPoliciesAPI(),
		AIGatewayMCPServersAPI: sdk.GetAIGatewayMCPServersAPI(),
	}), slog.New(slog.NewTextHandler(io.Discard, nil)))
	plan, err := p.GeneratePlan(t.Context(), desired, planner.Options{Mode: planner.PlanModeSync})
	require.NoError(t, err)
	require.Equal(t, 1, usageCalls)
	deletes := make(map[string]planner.PlannedChange)
	for _, change := range plan.Changes {
		if change.Action == planner.ActionDelete {
			require.Equal(t, "smoke-test", change.Namespace)
			deletes[change.ResourceType] = change
		}
	}
	require.Len(t, deletes, 2)
	require.Contains(t, deletes, planner.ResourceTypeAIGatewayModel)
	require.Contains(t, deletes, planner.ResourceTypeAIGatewayDatastore)
	require.Contains(t, deletes[planner.ResourceTypeAIGatewayDatastore].DependsOn,
		deletes[planner.ResourceTypeAIGatewayModel].ID)
}

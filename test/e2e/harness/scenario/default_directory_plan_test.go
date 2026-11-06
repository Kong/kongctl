//go:build e2e

package scenario

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestChildScenarioPlansAgainstExistingDefaultDirectory(t *testing.T) {
	for _, tc := range []struct {
		name, namespace string
		changes         int
	}{
		{"declarative-groups", "identity-group-directory-e2e", 3},
		{"declarative-memberships", "identity-membership-directory-e2e", 6},
		{"metadata-integers", "", 1},
		{"declarative-basic-auth", "identity-basic-directories-e2e", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := fixture.Directory("default", tc.namespace, false)
			directory.Description = ""
			directory.Labels = map[string]string{}
			directory.TTLSecs, directory.NegativeTTLSecs = new(int64(600)), new(int64(600))
			api := &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}, Current: directory}
			client := state.NewClient(state.ClientConfig{
				IdentityDirectoriesAPI: api, IdentityPrincipalsAPI: &fixture.PrincipalAPI{},
				IdentityGroupsAPI: &fixture.GroupAPI{}, IdentityBasicAuthAPI: &fixture.BasicAuthAPI{},
			})
			work := t.TempDir()
			script, err := filepath.Abs(filepath.Join("../../scenarios/identity", tc.name, "fixtures.py"))
			require.NoError(t, err)
			if tc.name == "declarative-basic-auth" {
				require.NoError(t, os.WriteFile(filepath.Join(work, "initial.yaml"), []byte(defaultBasicAuthManifest), 0o600))
			} else {
				output, err := exec.CommandContext(t.Context(), "python3", script, "prepare", work, "default").CombinedOutput()
				require.NoError(t, err, string(output))
			}
			rs, err := loader.New().LoadFromSources([]loader.Source{{
				Path: filepath.Join(work, "initial.yaml"), Type: loader.SourceTypeFile,
			}}, false)
			require.NoError(t, err)
			plan, err := planner.NewPlanner(client, slog.New(slog.DiscardHandler)).GeneratePlan(t.Context(), rs,
				planner.Options{Mode: planner.PlanModeApply})
			require.NoError(t, err)
			require.Len(t, plan.Changes, tc.changes)
			for _, change := range plan.Changes {
				if change.ResourceType == string(resources.ResourceTypeIdentityDirectory) {
					require.Equal(t, planner.ActionUpdate, change.Action)
					require.Equal(t, directory.ID, change.ResourceID)
				}
				if change.ResourceType == string(resources.ResourceTypeIdentityBasicAuth) {
					require.Len(t, change.DependsOn, 1, "the directory already exists; the principal is new")
				}
			}
			require.Zero(t, api.Creates)
			require.Zero(t, api.Deletes)
		})
	}
}

const defaultBasicAuthManifest = `
_defaults:
  kongctl:
    namespace: identity-basic-e2e
identity:
  directories:
    - ref: people
      name: default
      labels:
        e2e-fixture: default
      allow_all_control_planes: true
      allowed_control_planes: []
      vault:
        enabled: false
      kongctl:
        namespace: identity-basic-directories-e2e
        protected: true
      principals:
        - ref: worker
          display_name: Basic-auth worker
          type: agent
          kongctl:
            namespace: identity-basic-principals-e2e
            protected: true
          basic_auths:
            - ref: login
              username: fixture-login
`

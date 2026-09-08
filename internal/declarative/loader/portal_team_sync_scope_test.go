package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderPortalTeamRoleSyncScope(t *testing.T) {
	tests := []struct {
		name               string
		declaration        string
		wantChildTypes     []resources.ResourceType
		wantRootChildTypes []resources.ResourceType
	}{
		{name: "omitted teams"},
		{
			name:        "nested empty teams scope teams and roles",
			declaration: "    teams: []\n",
			wantChildTypes: []resources.ResourceType{
				resources.ResourceTypePortalTeam,
				resources.ResourceTypePortalTeamRole,
			},
		},
		{
			name: "root teams scope teams only",
			declaration: `portal_teams:
  - ref: docs-team
    name: Docs team
    portal: docs-portal
`,
			wantChildTypes: []resources.ResourceType{resources.ResourceTypePortalTeam},
		},
		{
			name:               "root empty teams retain the unscoped collection marker",
			declaration:        "portal_teams: []\n",
			wantRootChildTypes: []resources.ResourceType{resources.ResourceTypePortalTeam},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `portals:
  - ref: docs-portal
    name: Docs portal
` + tt.declaration
			rs, err := New().parseYAML(strings.NewReader(input), "test.yaml", ".")
			require.NoError(t, err)
			require.NotNil(t, rs.SyncScope)

			var want []resources.ChildSyncScope
			for _, kind := range tt.wantChildTypes {
				want = append(want, resources.ChildSyncScope{
					ParentType:   resources.ResourceTypePortal,
					ParentRef:    "docs-portal",
					ResourceType: kind,
				})
			}
			assert.ElementsMatch(t, want, rs.SyncScope.ChildScopes())
			assert.ElementsMatch(t, tt.wantRootChildTypes, rs.SyncScope.RootChildCollectionTypes())
		})
	}
}

package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureSyncScopeTracksExplicitRootAndNestedChildCollections(t *testing.T) {
	input := strings.NewReader(`
apis:
  - ref: orders-api
    name: Orders API
    documents: []
portals:
  - ref: docs-portal
    name: docs-portal
    pages: []
`)

	rs, err := New().parseYAML(input, "test.yaml", ".")
	require.NoError(t, err)
	require.NotNil(t, rs.SyncScope)

	assert.True(t, rs.SyncScope.RootInScope(resources.ResourceTypeAPI))
	assert.True(t, rs.SyncScope.RootInScope(resources.ResourceTypePortal))
	assert.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAPI,
		"orders-api",
		resources.ResourceTypeAPIDocument,
	))
	assert.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypePortal,
		"docs-portal",
		resources.ResourceTypePortalPage,
	))
}

func TestCaptureSyncScopeTracksRootLevelEmptyChildCollections(t *testing.T) {
	input := strings.NewReader(`api_documents: []`)

	rs, err := New().parseYAML(input, "test.yaml", ".")
	require.NoError(t, err)
	require.NotNil(t, rs.SyncScope)

	assert.Equal(
		t,
		[]resources.ResourceType{resources.ResourceTypeAPIDocument},
		rs.SyncScope.RootChildCollectionTypes(),
	)
}

func TestCaptureSyncScopeRejectsNullPortalSingletonChildren(t *testing.T) {
	tests := []struct {
		name      string
		childYAML string
		want      string
	}{
		{
			name:      "customization",
			childYAML: "    customization: null\n",
			want:      `child singleton "customization" cannot be null`,
		},
		{
			name:      "auth settings",
			childYAML: "    auth_settings: null\n",
			want:      `child singleton "auth_settings" cannot be null`,
		},
		{
			name:      "ip allow list",
			childYAML: "    ip_allow_list: null\n",
			want:      `child singleton "ip_allow_list" cannot be null`,
		},
		{
			name:      "integrations",
			childYAML: "    integrations: null\n",
			want:      `child singleton "integrations" cannot be null`,
		},
		{
			name:      "custom domain",
			childYAML: "    custom_domain: null\n",
			want:      `child singleton "custom_domain" cannot be null`,
		},
		{
			name:      "email config",
			childYAML: "    email_config: null\n",
			want:      `child singleton "email_config" cannot be null`,
		},
		{
			name:      "audit log webhook",
			childYAML: "    audit_log_webhook: null\n",
			want:      `child singleton "audit_log_webhook" cannot be null`,
		},
		{
			name:      "assets",
			childYAML: "    assets: null\n",
			want:      `child singleton "assets" cannot be null`,
		},
		{
			name:      "asset logo",
			childYAML: "    assets:\n      logo: null\n",
			want:      `child singleton "assets.logo" cannot be null`,
		},
		{
			name:      "asset favicon",
			childYAML: "    assets:\n      favicon: null\n",
			want:      `child singleton "assets.favicon" cannot be null`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(`
portals:
  - ref: docs-portal
    name: docs-portal
` + tt.childYAML)

			_, err := New().parseYAML(input, "test.yaml", ".")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

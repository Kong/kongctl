package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExplainResourcePathsAllResolve(t *testing.T) {
	paths := ExplainResourcePaths()
	require.NotEmpty(t, paths)
	assert.IsIncreasing(t, paths)

	for _, path := range paths {
		_, err := ResolveExplainSubject(path)
		assert.NoErrorf(t, err, "listed path %q must resolve", path)
	}
}

func TestExplainResourcePathsPrefersNestedPaths(t *testing.T) {
	paths := ExplainResourcePaths()
	explainTypeCount := 0
	for _, resourceType := range RegisteredTypes() {
		doc, ok := explainDocByType(resourceType)
		if !ok {
			continue
		}
		explainTypeCount++
		if len(doc.ParentRelations) > 0 {
			assert.NotContains(t, paths, doc.CanonicalAlias)
		}
	}

	assert.Len(t, paths, explainTypeCount)
	assert.NotContains(t, paths, "organization")
	assert.NotContains(t, paths, "analytics")
	assert.NotContains(t, paths, "api_version")
	assert.NotContains(t, paths, "portal_page")
	assert.NotContains(t, paths, "portal_snippet")
	assert.NotContains(t, paths, "organization_user_team_membership")
	assert.NotContains(t, paths, "organization_user_role")
	assert.NotContains(t, paths, "organization_system_account_team_membership")
	assert.NotContains(t, paths, "organization_system_account_role")
	assert.Contains(t, paths, "api")
	assert.Contains(t, paths, "api.versions")
	assert.Contains(t, paths, "portal")
	assert.Contains(t, paths, "portal.pages")
	assert.Contains(t, paths, "portal.snippets")
	assert.Contains(t, paths, "organization.teams")
	assert.Contains(t, paths, "organization.users.teams")
	assert.Contains(t, paths, "organization.users.roles")
	assert.Contains(t, paths, "organization.system-accounts.teams")
	assert.Contains(t, paths, "organization.system-accounts.roles")
	assert.Contains(t, paths, "analytics.dashboards")
}

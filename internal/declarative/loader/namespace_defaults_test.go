package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamespaceDefaults(t *testing.T) {
	t.Run("namespace defaults from _defaults section", func(t *testing.T) {
		yaml := `
_defaults:
  kongctl:
    namespace: team-alpha

portals:
  - ref: portal1
    name: "Portal 1"
    
apis:
  - ref: api1
    name: "API 1"
`
		dir := t.TempDir()
		file := filepath.Join(dir, "test.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yaml), 0o600))

		l := New()
		rs, err := l.LoadFile(file)
		require.NoError(t, err)

		// Check portal inherited namespace
		require.Len(t, rs.Portals, 1)
		assert.NotNil(t, rs.Portals[0].Kongctl)
		assert.NotNil(t, rs.Portals[0].Kongctl.Namespace)
		assert.Equal(t, "team-alpha", *rs.Portals[0].Kongctl.Namespace)
		assert.Equal(t, resources.NamespaceOriginFileDefault, rs.Portals[0].Kongctl.NamespaceOrigin)
		assert.NotNil(t, rs.Portals[0].Kongctl.Protected)
		assert.False(t, *rs.Portals[0].Kongctl.Protected)

		// Check API inherited namespace
		require.Len(t, rs.APIs, 1)
		assert.NotNil(t, rs.APIs[0].Kongctl)
		assert.NotNil(t, rs.APIs[0].Kongctl.Namespace)
		assert.Equal(t, "team-alpha", *rs.APIs[0].Kongctl.Namespace)
		assert.Equal(t, resources.NamespaceOriginFileDefault, rs.APIs[0].Kongctl.NamespaceOrigin)
		assert.NotNil(t, rs.APIs[0].Kongctl.Protected)
		assert.False(t, *rs.APIs[0].Kongctl.Protected)
	})

	t.Run("protected defaults from _defaults section", func(t *testing.T) {
		yaml := `
_defaults:
  kongctl:
    namespace: production
    protected: true

portals:
  - ref: portal1
    name: "Production Portal"
    
application_auth_strategies:
  - ref: auth1
    name: "Auth Strategy"
    display_name: "Production Auth"
    strategy_type: key_auth
    configs:
      key_auth:
        key_names: ["x-api-key"]
`
		dir := t.TempDir()
		file := filepath.Join(dir, "test.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yaml), 0o600))

		l := New()
		rs, err := l.LoadFile(file)
		require.NoError(t, err)

		// Check portal inherited both namespace and protected
		require.Len(t, rs.Portals, 1)
		assert.NotNil(t, rs.Portals[0].Kongctl)
		assert.NotNil(t, rs.Portals[0].Kongctl.Namespace)
		assert.Equal(t, "production", *rs.Portals[0].Kongctl.Namespace)
		assert.Equal(t, resources.NamespaceOriginFileDefault, rs.Portals[0].Kongctl.NamespaceOrigin)
		assert.NotNil(t, rs.Portals[0].Kongctl.Protected)
		assert.True(t, *rs.Portals[0].Kongctl.Protected)

		// Check auth strategy inherited both namespace and protected
		require.Len(t, rs.ApplicationAuthStrategies, 1)
		assert.NotNil(t, rs.ApplicationAuthStrategies[0].Kongctl)
		assert.NotNil(t, rs.ApplicationAuthStrategies[0].Kongctl.Namespace)
		assert.Equal(t, "production", *rs.ApplicationAuthStrategies[0].Kongctl.Namespace)
		assert.Equal(t, resources.NamespaceOriginFileDefault, rs.ApplicationAuthStrategies[0].Kongctl.NamespaceOrigin)
		assert.NotNil(t, rs.ApplicationAuthStrategies[0].Kongctl.Protected)
		assert.True(t, *rs.ApplicationAuthStrategies[0].Kongctl.Protected)
	})

	t.Run("explicit values override defaults", func(t *testing.T) {
		yaml := `
_defaults:
  kongctl:
    namespace: team-alpha
    protected: true

portals:
  - ref: portal1
    name: "Portal 1"
    kongctl:
      namespace: team-beta
      protected: false
      
apis:
  - ref: api1
    name: "API 1"
    kongctl:
      namespace: team-gamma
      # protected not specified, should inherit from defaults
`
		dir := t.TempDir()
		file := filepath.Join(dir, "test.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yaml), 0o600))

		l := New()
		rs, err := l.LoadFile(file)
		require.NoError(t, err)

		// Check portal has explicit values
		require.Len(t, rs.Portals, 1)
		assert.NotNil(t, rs.Portals[0].Kongctl)
		assert.NotNil(t, rs.Portals[0].Kongctl.Namespace)
		assert.Equal(t, "team-beta", *rs.Portals[0].Kongctl.Namespace)
		assert.Equal(t, resources.NamespaceOriginExplicit, rs.Portals[0].Kongctl.NamespaceOrigin)
		assert.NotNil(t, rs.Portals[0].Kongctl.Protected)
		// Now with pointer types, explicit false is preserved
		assert.False(t, *rs.Portals[0].Kongctl.Protected)

		// Check API has explicit namespace but inherited protected
		require.Len(t, rs.APIs, 1)
		assert.NotNil(t, rs.APIs[0].Kongctl)
		assert.NotNil(t, rs.APIs[0].Kongctl.Namespace)
		assert.Equal(t, "team-gamma", *rs.APIs[0].Kongctl.Namespace)
		assert.Equal(t, resources.NamespaceOriginExplicit, rs.APIs[0].Kongctl.NamespaceOrigin)
		assert.NotNil(t, rs.APIs[0].Kongctl.Protected)
		assert.True(t, *rs.APIs[0].Kongctl.Protected)
	})

	t.Run("default namespace when no _defaults", func(t *testing.T) {
		yaml := `
portals:
  - ref: portal1
    name: "Portal 1"
    
control_planes:
  - ref: cp1
    name: "Control Plane 1"
`
		dir := t.TempDir()
		file := filepath.Join(dir, "test.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yaml), 0o600))

		l := New()
		rs, err := l.LoadFile(file)
		require.NoError(t, err)

		// Check portal gets default namespace
		require.Len(t, rs.Portals, 1)
		assert.NotNil(t, rs.Portals[0].Kongctl)
		assert.NotNil(t, rs.Portals[0].Kongctl.Namespace)
		assert.Equal(t, "default", *rs.Portals[0].Kongctl.Namespace)
		assert.Equal(t, resources.NamespaceOriginImplicitDefault, rs.Portals[0].Kongctl.NamespaceOrigin)
		assert.NotNil(t, rs.Portals[0].Kongctl.Protected)
		assert.False(t, *rs.Portals[0].Kongctl.Protected)

		// Check control plane gets default namespace
		require.Len(t, rs.ControlPlanes, 1)
		assert.NotNil(t, rs.ControlPlanes[0].Kongctl)
		assert.NotNil(t, rs.ControlPlanes[0].Kongctl.Namespace)
		assert.Equal(t, "default", *rs.ControlPlanes[0].Kongctl.Namespace)
		assert.Equal(t, resources.NamespaceOriginImplicitDefault, rs.ControlPlanes[0].Kongctl.NamespaceOrigin)
		assert.NotNil(t, rs.ControlPlanes[0].Kongctl.Protected)
		assert.False(t, *rs.ControlPlanes[0].Kongctl.Protected)
	})

	t.Run("child resources do not get kongctl metadata", func(t *testing.T) {
		yaml := `
_defaults:
  kongctl:
    namespace: team-alpha
    protected: true

apis:
  - ref: api1
    name: "API 1"
    versions:
      - ref: v1
        name: "v1.0.0"
        version: "1.0.0"
        spec:
          openapi: 3.0.0
          info:
            title: Test API
            version: 1.0.0
          paths: {}
`
		dir := t.TempDir()
		file := filepath.Join(dir, "test.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yaml), 0o600))

		l := New()
		rs, err := l.LoadFile(file)
		require.NoError(t, err)

		// Check API has kongctl metadata
		require.Len(t, rs.APIs, 1)
		assert.NotNil(t, rs.APIs[0].Kongctl)
		assert.NotNil(t, rs.APIs[0].Kongctl.Namespace)
		assert.Equal(t, "team-alpha", *rs.APIs[0].Kongctl.Namespace)
		assert.NotNil(t, rs.APIs[0].Kongctl.Protected)
		assert.True(t, *rs.APIs[0].Kongctl.Protected)

		// Check API version (child resource) extracted but no kongctl metadata
		require.Len(t, rs.APIVersions, 1)
		// APIVersion should not have Kongctl field at all (removed in Step 2)
	})

	t.Run("multiple sources with different defaults", func(t *testing.T) {
		yaml1 := `
_defaults:
  kongctl:
    namespace: team-alpha

portals:
  - ref: portal1
    name: "Team Alpha Portal"
`
		yaml2 := `
_defaults:
  kongctl:
    namespace: team-beta
    protected: true

portals:
  - ref: portal2
    name: "Team Beta Portal"
`
		yaml3 := `
portals:
  - ref: portal3
    name: "Default Portal"
`
		dir := t.TempDir()
		file1 := filepath.Join(dir, "team-alpha.yaml")
		file2 := filepath.Join(dir, "team-beta.yaml")
		file3 := filepath.Join(dir, "default.yaml")
		require.NoError(t, os.WriteFile(file1, []byte(yaml1), 0o600))
		require.NoError(t, os.WriteFile(file2, []byte(yaml2), 0o600))
		require.NoError(t, os.WriteFile(file3, []byte(yaml3), 0o600))

		l := New()
		sources := []Source{
			{Type: SourceTypeFile, Path: file1},
			{Type: SourceTypeFile, Path: file2},
			{Type: SourceTypeFile, Path: file3},
		}
		rs, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)

		// Check all portals
		require.Len(t, rs.Portals, 3)

		// Portal 1 from team-alpha
		assert.Equal(t, "portal1", rs.Portals[0].Ref)
		assert.NotNil(t, rs.Portals[0].Kongctl.Namespace)
		assert.Equal(t, "team-alpha", *rs.Portals[0].Kongctl.Namespace)
		assert.NotNil(t, rs.Portals[0].Kongctl.Protected)
		assert.False(t, *rs.Portals[0].Kongctl.Protected)

		// Portal 2 from team-beta
		assert.Equal(t, "portal2", rs.Portals[1].Ref)
		assert.NotNil(t, rs.Portals[1].Kongctl.Namespace)
		assert.Equal(t, "team-beta", *rs.Portals[1].Kongctl.Namespace)
		assert.NotNil(t, rs.Portals[1].Kongctl.Protected)
		assert.True(t, *rs.Portals[1].Kongctl.Protected)

		// Portal 3 with default namespace
		assert.Equal(t, "portal3", rs.Portals[2].Ref)
		assert.NotNil(t, rs.Portals[2].Kongctl.Namespace)
		assert.Equal(t, "default", *rs.Portals[2].Kongctl.Namespace)
		assert.NotNil(t, rs.Portals[2].Kongctl.Protected)
		assert.False(t, *rs.Portals[2].Kongctl.Protected)
	})

	t.Run("protected false in defaults does not override explicit true", func(t *testing.T) {
		yaml := `
_defaults:
  kongctl:
    namespace: staging
    protected: false

portals:
  - ref: portal1
    name: "Critical Portal"
    kongctl:
      protected: true
`
		dir := t.TempDir()
		file := filepath.Join(dir, "test.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yaml), 0o600))

		l := New()
		rs, err := l.LoadFile(file)
		require.NoError(t, err)

		// Check portal keeps its explicit protected=true
		require.Len(t, rs.Portals, 1)
		assert.NotNil(t, rs.Portals[0].Kongctl)
		assert.NotNil(t, rs.Portals[0].Kongctl.Namespace)
		assert.Equal(t, "staging", *rs.Portals[0].Kongctl.Namespace)
		assert.NotNil(t, rs.Portals[0].Kongctl.Protected)
		assert.True(t, *rs.Portals[0].Kongctl.Protected) // Should remain true
	})

	t.Run("empty namespace in defaults is rejected", func(t *testing.T) {
		yaml := `
_defaults:
  kongctl:
    namespace: ""

portals:
  - ref: portal1
    name: "Portal 1"
`
		dir := t.TempDir()
		file := filepath.Join(dir, "test.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yaml), 0o600))

		l := New()
		_, err := l.LoadFile(file)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "namespace in _defaults.kongctl cannot be empty")
	})

	t.Run("empty namespace on resource is rejected", func(t *testing.T) {
		yaml := `
portals:
  - ref: portal1
    name: "Portal 1"
    kongctl:
      namespace: ""
`
		dir := t.TempDir()
		file := filepath.Join(dir, "test.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yaml), 0o600))

		l := New()
		_, err := l.LoadFile(file)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "portal 'portal1' cannot have an empty namespace")
	})

	t.Run("default namespace retained when only defaults provided", func(t *testing.T) {
		yaml := `
_defaults:
  kongctl:
    namespace: team-alpha

portals: []
apis: []
control_planes: []
application_auth_strategies: []
`
		dir := t.TempDir()
		file := filepath.Join(dir, "defaults-only.yaml")
		require.NoError(t, os.WriteFile(file, []byte(yaml), 0o600))

		l := New()
		sources := []Source{
			{Type: SourceTypeFile, Path: file},
		}
		rs, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)

		assert.Equal(t, "team-alpha", rs.DefaultNamespace)
		assert.Len(t, rs.Portals, 0)
		assert.Len(t, rs.APIs, 0)
		assert.Len(t, rs.ControlPlanes, 0)
		assert.Len(t, rs.ApplicationAuthStrategies, 0)
	})

	t.Run("conflicting defaults without resources returns error", func(t *testing.T) {
		yaml1 := `
_defaults:
  kongctl:
    namespace: team-alpha
portals: []
`
		yaml2 := `
_defaults:
  kongctl:
    namespace: team-beta
portals: []
`
		dir := t.TempDir()
		file1 := filepath.Join(dir, "alpha.yaml")
		file2 := filepath.Join(dir, "beta.yaml")
		require.NoError(t, os.WriteFile(file1, []byte(yaml1), 0o600))
		require.NoError(t, os.WriteFile(file2, []byte(yaml2), 0o600))

		l := New()
		sources := []Source{
			{Type: SourceTypeFile, Path: file1},
			{Type: SourceTypeFile, Path: file2},
		}
		_, err := l.LoadFromSources(sources, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting _defaults.kongctl.namespace values")
	})
}

func TestApplyNamespaceDefaultsExternalWithKongctlFails(t *testing.T) {
	ns := "team-alpha"
	l := New()
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				Ref:     "external-portal",
				Kongctl: &resources.KongctlMeta{Namespace: &ns},
				External: &resources.ExternalBlock{
					Selector: &resources.ExternalSelector{MatchFields: map[string]string{"name": "portal"}}},
			},
		},
	}

	err := l.applyNamespaceDefaults(rs, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "portal 'external-portal' is marked as external")
}

package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type explainInlineUnionResource struct {
	ExplainInlineUnion `yaml:",inline" json:",inline"`
	Ref                string `yaml:"ref"     json:"ref"`
}

type ExplainInlineUnion struct {
	ServiceReference *ExplainServiceReference `queryParam:"inline" union:"member"`
}

type ExplainServiceReference struct {
	Service *ExplainService `json:"service,omitempty"`
}

type ExplainService struct {
	ID string `json:"id"`
}

func TestResolveExplainSubject_Resource(t *testing.T) {
	subject, err := ResolveExplainSubject("api")
	require.NoError(t, err)

	assert.Equal(t, "api", subject.DisplayPath)
	assert.Equal(t, ResourceTypeAPI, subject.Doc.ResourceType)
	assert.True(t, subject.Doc.SupportsRoot)
	assert.False(t, subject.Doc.SupportsNestedDeclaration)
	assert.Equal(t, explainResourceClassTopLevel, subject.Doc.ResourceClass)
	assert.True(t, subject.ResourceTarget)
	assert.Empty(t, subject.FieldPath)
	assert.Equal(t, "object", subject.Node.Kind)
}

func TestResolveExplainSubject_HyphenatedAlias(t *testing.T) {
	subject, err := ResolveExplainSubject("event-gateway")
	require.NoError(t, err)

	assert.Equal(t, ResourceTypeEventGatewayControlPlane, subject.Doc.ResourceType)
	assert.Contains(t, subject.Doc.Aliases, "event-gateway")
	assert.Contains(t, subject.Doc.Aliases, "event-gateways")
	assert.Contains(t, subject.Doc.Aliases, "egw")
}

func TestResolveExplainSubject_NestedChildResource(t *testing.T) {
	subject, err := ResolveExplainSubject("api.versions")
	require.NoError(t, err)

	assert.Equal(t, "api.versions", subject.DisplayPath)
	assert.Equal(t, ResourceTypeAPIVersion, subject.Doc.ResourceType)
	assert.Equal(t, explainResourceClassChild, subject.Doc.ResourceClass)
	assert.True(t, subject.Doc.SupportsNestedDeclaration)
	assert.True(t, subject.ResourceTarget)
	assert.Equal(t, []string{"versions"}, subject.FieldPath)
	assert.Equal(t, []ResourceType{ResourceTypeAPI}, subject.AncestorTypes)
}

func TestResolveExplainSubject_AIGatewayModelsNestedChildResource(t *testing.T) {
	subject, err := ResolveExplainSubject("ai_gateway.models")
	require.NoError(t, err)

	assert.Equal(t, "ai_gateway.models", subject.DisplayPath)
	assert.Equal(t, ResourceTypeAIGatewayModel, subject.Doc.ResourceType)
	assert.Equal(t, explainResourceClassChild, subject.Doc.ResourceClass)
	assert.True(t, subject.Doc.SupportsRoot)
	assert.True(t, subject.Doc.SupportsNestedDeclaration)
	assert.True(t, subject.ResourceTarget)
	assert.Equal(t, []string{"models"}, subject.FieldPath)
	assert.Equal(t, []ResourceType{ResourceTypeAIGateway}, subject.AncestorTypes)
}

func TestResolveExplainSubject_AIGatewayAgentsNestedChildResource(t *testing.T) {
	subject, err := ResolveExplainSubject("ai_gateway.agents")
	require.NoError(t, err)

	assert.Equal(t, "ai_gateway.agents", subject.DisplayPath)
	assert.Equal(t, ResourceTypeAIGatewayAgent, subject.Doc.ResourceType)
	assert.Equal(t, explainResourceClassChild, subject.Doc.ResourceClass)
	assert.True(t, subject.Doc.SupportsRoot)
	assert.True(t, subject.Doc.SupportsNestedDeclaration)
	assert.True(t, subject.ResourceTarget)
	assert.Equal(t, []string{"agents"}, subject.FieldPath)
	assert.Equal(t, []ResourceType{ResourceTypeAIGateway}, subject.AncestorTypes)
}

func TestResolveExplainSubject_OrganizationTeamsGroupedResource(t *testing.T) {
	subject, err := ResolveExplainSubject("organization.teams")
	require.NoError(t, err)

	assert.Equal(t, "organization.teams", subject.DisplayPath)
	assert.Equal(t, ResourceTypeOrganizationTeam, subject.Doc.ResourceType)
	assert.True(t, subject.ResourceTarget)
	assert.Equal(t, []string{"teams"}, subject.FieldPath)
	assert.Equal(t, []ExplainScaffoldStep{
		{Name: "organization"},
		{Name: "teams", Array: true},
	}, subject.ScaffoldSteps)
}

func TestResolveExplainSubject_OrganizationTeamRolesNestedResource(t *testing.T) {
	subject, err := ResolveExplainSubject("organization.teams.roles")
	require.NoError(t, err)

	assert.Equal(t, "organization.teams.roles", subject.DisplayPath)
	assert.Equal(t, ResourceTypeOrganizationTeamRole, subject.Doc.ResourceType)
	assert.True(t, subject.ResourceTarget)
	assert.Equal(t, []string{"teams", "roles"}, subject.FieldPath)
	assert.Equal(t, []ResourceType{ResourceTypeOrganizationTeam}, subject.AncestorTypes)
	assert.Equal(t, []ExplainScaffoldStep{
		{Name: "organization"},
		{Name: "teams", Array: true},
		{Name: "roles", Array: true},
	}, subject.ScaffoldSteps)
}

func TestResolveExplainSubject_NestedSingletonChildResource(t *testing.T) {
	subject, err := ResolveExplainSubject("portal.auth_settings")
	require.NoError(t, err)

	assert.Equal(t, ResourceTypePortalAuthSettings, subject.Doc.ResourceType)
	assert.True(t, subject.ResourceTarget)
	assert.Equal(t, []ExplainScaffoldStep{
		{Name: "portals", Array: true},
		{Name: "auth_settings"},
	}, subject.ScaffoldSteps)
}

func TestResolveExplainSubject_FieldPath(t *testing.T) {
	subject, err := ResolveExplainSubject("api.publications.portal_id")
	require.NoError(t, err)

	assert.Equal(t, "api.publications.portal_id", subject.DisplayPath)
	assert.Equal(t, ResourceTypeAPIPublication, subject.Doc.ResourceType)
	assert.False(t, subject.ResourceTarget)
	assert.Equal(t, []string{"publications", "portal_id"}, subject.FieldPath)
	assert.Equal(t, []string{"portal_id"}, subject.FieldRelativePath)
	assert.True(t, subject.FieldRequired)
	assert.Equal(t, "string", subject.Node.Kind)
	assert.Equal(t, "portal", subject.Node.RefKind)
	assert.Equal(t, "!ref", subject.Node.PreferredTag)
}

func TestResolveExplainSubject_RecursivePortalResource(t *testing.T) {
	subject, err := ResolveExplainSubject("portal")
	require.NoError(t, err)

	pagesField, ok := subject.Node.property("pages")
	require.True(t, ok)
	require.NotNil(t, pagesField.Node)
	require.NotNil(t, pagesField.Node.Items)

	childrenField, ok := pagesField.Node.Items.property("children")
	require.True(t, ok)
	require.NotNil(t, childrenField.Node)
	require.NotNil(t, childrenField.Node.Items)

	assert.Equal(t, "object", childrenField.Node.Items.Kind)
	assert.Contains(t, childrenField.Node.Items.Description, "Recursive")
	assert.Contains(t, childrenField.Node.Items.Notes, "schema recursion truncated after the first expansion")
}

func TestRenderExplainSchema_Metadata(t *testing.T) {
	subject, err := ResolveExplainSubject("api")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)

	assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema.Schema)
	assert.Equal(t, "kongctl://declarative/api", schema.ID)
	assert.Equal(t, "api", schema.XResource)
	assert.Equal(t, "api", schema.XPath)
	assert.Equal(t, "apis", schema.XRootKey)
	assert.Equal(t, explainResourceClassTopLevel, schema.XClass)
	require.NotNil(t, schema.XNestedDecl)
	assert.False(t, *schema.XNestedDecl)
}

func TestRenderExplainSchema_APINameDefault(t *testing.T) {
	subject, err := ResolveExplainSubject("api.name")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)

	assert.Equal(t, "ref", schema.XDefault)
	require.NotNil(t, schema.XSubject)
	assert.False(t, *schema.XSubject.Required)
	assert.True(t, *schema.XSubject.Recommended)
}

func TestRenderExplainSchema_ApplicationAuthStrategyUnion(t *testing.T) {
	subject, err := ResolveExplainSubject("application_auth_strategies")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)
	require.Len(t, schema.OneOf, 2)
	assert.Nil(t, schema.Properties)
	assert.Nil(t, schema.Additional)

	keyAuth := schema.OneOf[0]
	require.NotNil(t, keyAuth.Properties["strategy_type"])
	assert.Equal(t, "key_auth", keyAuth.Properties["strategy_type"].Const)
	require.NotNil(t, keyAuth.Properties["configs"].Properties["key-auth"])
	assert.Contains(t, keyAuth.Properties["configs"].Properties["key-auth"].Required, "key_names")

	oidc := schema.OneOf[1]
	require.NotNil(t, oidc.Properties["strategy_type"])
	assert.Equal(t, "openid_connect", oidc.Properties["strategy_type"].Const)
	require.NotNil(t, oidc.Properties["configs"].Properties["openid-connect"])

	assert.Nil(t, keyAuth.Properties["app_auth_strategy_key_auth_request"])
	assert.Nil(t, oidc.Properties["app_auth_strategy_open_i_d_connect_request"])
}

func TestRenderExplainSchema_ApplicationAuthStrategyUnionField(t *testing.T) {
	subject, err := ResolveExplainSubject("application_auth_strategies.strategy_type")
	require.NoError(t, err)

	assert.True(t, subject.FieldRequired)
	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)
	require.Len(t, schema.OneOf, 2)

	assert.Equal(t, "key_auth", schema.OneOf[0].Const)
	assert.Equal(t, "openid_connect", schema.OneOf[1].Const)
	require.NotNil(t, schema.XSubject)
	assert.True(t, *schema.XSubject.Required)
}

func TestRenderExplainSchema_ApplicationAuthStrategyConfigsUnionField(t *testing.T) {
	subject, err := ResolveExplainSubject("application_auth_strategies.configs")
	require.NoError(t, err)

	assert.True(t, subject.FieldRequired)
	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)
	require.Len(t, schema.OneOf, 2)

	assert.Contains(t, schema.OneOf[0].Properties, "key-auth")
	assert.Contains(t, schema.OneOf[1].Properties, "openid-connect")
	require.NotNil(t, schema.XSubject)
	assert.True(t, *schema.XSubject.Required)
}

func TestRenderExplainSchema_PortalAuthSettingsOmitsDeprecatedFields(t *testing.T) {
	subject, err := ResolveExplainSubject("portal.auth_settings")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)

	assert.Contains(t, schema.Properties, "basic_auth_enabled")
	assert.Contains(t, schema.Properties, "konnect_mapping_enabled")
	assert.Contains(t, schema.Properties, "idp_mapping_enabled")
	assert.NotContains(t, schema.Properties, "oidc_auth_enabled")
	assert.NotContains(t, schema.Properties, "saml_auth_enabled")
	assert.NotContains(t, schema.Properties, "oidc_client_id")
	assert.NotContains(t, schema.Properties, "oidc_claim_mappings")
}

func TestRenderExplainSchema_AnalyticsDashboardDiscriminators(t *testing.T) {
	subject, err := ResolveExplainSubject("analytics.dashboards")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)

	tile := schema.Properties["definition"].Properties["tiles"].Items
	require.NotNil(t, tile.Properties["type"])
	assert.Equal(t, "chart", tile.Properties["type"].Const)

	query := tile.Properties["definition"].Properties["query"]
	require.Len(t, query.OneOf, 4)
	assert.Equal(t, "api_usage", query.OneOf[0].Properties["datasource"].Const)
	assert.Equal(t, "llm_usage", query.OneOf[1].Properties["datasource"].Const)
	assert.Equal(t, "agentic_usage", query.OneOf[2].Properties["datasource"].Const)
	assert.Equal(t, "platform_usage", query.OneOf[3].Properties["datasource"].Const)

	chart := tile.Properties["definition"].Properties["chart"]
	require.Len(t, chart.OneOf, 8)
	assert.Equal(t, "timeseries_line", chart.OneOf[0].Properties["type"].Const)
	assert.Equal(t, "horizontal_bar", chart.OneOf[2].Properties["type"].Const)
	assert.Equal(t, "top_n", chart.OneOf[7].Properties["type"].Const)
}

func TestRenderExplainSchema_APIImplementationServiceShape(t *testing.T) {
	subject, err := ResolveExplainSubject("api.implementations")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)

	assert.Contains(t, schema.Properties, "service")
	assert.Contains(t, schema.Properties, "type")
	assert.NotContains(t, schema.Properties, "service_reference")
	assert.NotContains(t, schema.Properties, "control_plane_reference")

	service := schema.Properties["service"]
	require.NotNil(t, service)
	require.NotNil(t, service.Properties["id"])
	require.NotNil(t, service.Properties["control_plane_id"])
	assert.Equal(t, "gateway_service", service.Properties["id"].XRefKind)
	assert.Equal(t, "control_plane", service.Properties["control_plane_id"].XRefKind)
}

func TestAutoExplainInlineSDKUnionUsesPayloadFields(t *testing.T) {
	node, err := autoExplainConcreteNode[explainInlineUnionResource](nil)
	require.NoError(t, err)

	assert.True(t, node.propertyExists("service"))
	assert.True(t, node.propertyExists("ref"))
	assert.False(t, node.propertyExists("service_reference"))
	require.Len(t, node.OneOf, 1)
	assert.True(t, node.OneOf[0].propertyExists("service"))
	assert.True(t, node.OneOf[0].propertyExists("ref"))
	assert.False(t, node.OneOf[0].propertyExists("service_reference"))
}

func TestRenderExplainText_AnalyticsDashboardAllowedValues(t *testing.T) {
	subject, err := ResolveExplainSubject("analytics.dashboards.definition.tiles.definition.query.datasource")
	require.NoError(t, err)

	text := RenderExplainText(subject, false)

	assert.Contains(t, text, "ALLOWED: api_usage|llm_usage|agentic_usage|platform_usage")
}

func TestRenderExplainText_ResourceSubject(t *testing.T) {
	subject, err := ResolveExplainSubject("portal")
	require.NoError(t, err)

	text := RenderExplainText(subject, false)

	assert.Contains(t, text, "RESOURCE\nMATURITY: ga\nRESOURCE CLASS: top-level")
	assert.Contains(t, text, "ROOT KEY: portals[]")
	assert.Contains(t, text, "SUPPORTS ROOT: true")
	assert.Contains(t, text, "SUPPORTS NESTED DECLARATION: false")
	assert.Contains(t, text, "ACCEPTS kongctl metadata: yes")
	assert.Contains(
		t,
		text,
		"CHILD RESOURCES: audit_log_webhook, auth_settings, custom_domain, customization, email_config, identity_providers, integrations, ip_allow_list, pages, snippets, teams", //nolint:lll
	)
	assert.Contains(t, text, "\nFIELD DETAILS: use --extended")
	assert.NotContains(t, text, "RESOURCE: portal")
	assert.NotContains(t, text, "PATH: portal")
	assert.NotContains(t, text, "NESTED PATHS:")
	assert.NotContains(t, text, "\nFIELDS\n")
}

func TestRenderExplainText_ResourceSubjectExtended(t *testing.T) {
	subject, err := ResolveExplainSubject("portal")
	require.NoError(t, err)

	text := RenderExplainText(subject, true)

	assert.Contains(t, text, "\nFIELDS\n- ref: string required")
	assert.NotContains(t, text, "FIELD DETAILS: use --extended")
}

func TestRenderExplainText_FieldSubject(t *testing.T) {
	subject, err := ResolveExplainSubject("portal.name")
	require.NoError(t, err)

	text := RenderExplainText(subject, false)

	assert.Contains(t, text, "FIELD\nPATH: portal.name")
	assert.Contains(t, text, "TYPE: string")
	assert.Contains(t, text, "OPTIONAL: true")
	assert.Contains(t, text, "RECOMMENDED: yes")
	assert.Contains(t, text, "DEFAULT FROM: ref")
	assert.Contains(t, text, "\nRESOURCE\nMATURITY: ga\nRESOURCE CLASS: top-level")
	assert.Contains(t, text, "ROOT KEY: portals[]")
	assert.Contains(t, text, "SUPPORTS NESTED DECLARATION: false")
	assert.Contains(
		t,
		text,
		"CHILD RESOURCES: audit_log_webhook, auth_settings, custom_domain, customization, email_config, identity_providers, integrations, ip_allow_list, pages, snippets, teams", //nolint:lll
	)
	assert.NotContains(t, text, "PLACEMENT")
	assert.NotContains(t, text, "YAML PATH:")
}

func TestRenderExplainText_NestedFieldSubjectPlacement(t *testing.T) {
	subject, err := ResolveExplainSubject("api.publications.portal_id")
	require.NoError(t, err)

	text := RenderExplainText(subject, false)

	assert.Contains(t, text, "FIELD\nPATH: api.publications.portal_id")
	assert.Contains(t, text, "OPTIONAL: false")
	assert.Contains(t, text, "NESTED YAML PATH: apis[].publications[].portal_id")
	assert.Contains(t, text, "ROOT YAML PATH: api_publications[].portal_id")
	assert.Contains(t, text, "\nRESOURCE\nMATURITY: ga\nRESOURCE CLASS: child")
	assert.Contains(t, text, "ROOT KEY: api_publications[]")
	assert.Contains(t, text, "SUPPORTS NESTED DECLARATION: true")
	assert.Contains(t, text, "ACCEPTS kongctl metadata: no")
	assert.NotContains(t, text, "FIELD DETAILS: use --extended")
}

func TestRenderExplainText_NestedChildResourceSummary(t *testing.T) {
	subject, err := ResolveExplainSubject("portal.pages")
	require.NoError(t, err)

	text := RenderExplainText(subject, false)

	assert.Contains(t, text, "RESOURCE\nMATURITY: ga\nRESOURCE CLASS: child")
	assert.Contains(t, text, "ROOT KEY: portal_pages[]")
	assert.Contains(t, text, "SUPPORTS ROOT: true")
	assert.Contains(t, text, "SUPPORTS NESTED DECLARATION: true")
	assert.Contains(t, text, "ACCEPTS kongctl metadata: no")
	assert.Contains(t, text, "CHILD RESOURCES: children")
}

func TestRenderExplainSchema_FieldSubjectMetadata(t *testing.T) {
	subject, err := ResolveExplainSubject("portal.name")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)

	require.NotNil(t, schema.XSubject)
	assert.Equal(t, "field", schema.XSubject.Kind)
	assert.Equal(t, "portal.name", schema.XSubject.Path)
	require.NotNil(t, schema.XSubject.Required)
	assert.False(t, *schema.XSubject.Required)
	require.NotNil(t, schema.XSubject.Recommended)
	assert.True(t, *schema.XSubject.Recommended)
	require.NotNil(t, schema.XPlacement)
	assert.Equal(t, "portals[].name", schema.XPlacement.YAMLPath)
	resource, ok := schema.XResource.(*ExplainSchemaResource)
	require.True(t, ok)
	assert.Equal(t, "portal", resource.Name)
	assert.Equal(t, explainResourceClassTopLevel, resource.ResourceClass)
	assert.Equal(t, "ref", schema.XDefault)
	assert.Empty(t, schema.XPath)
	assert.Empty(t, schema.XClass)
	assert.Nil(t, schema.XRoot)
	assert.Nil(t, schema.XNestedDecl)
}

func TestRenderExplainSchema_NestedFieldSubjectPlacement(t *testing.T) {
	subject, err := ResolveExplainSubject("api.publications.portal_id")
	require.NoError(t, err)

	schema := RenderExplainSchema(subject)
	require.NotNil(t, schema)
	require.NotNil(t, schema.XPlacement)

	assert.Equal(t, "apis[].publications[].portal_id", schema.XPlacement.NestedYAMLPath)
	assert.Equal(t, "api_publications[].portal_id", schema.XPlacement.RootYAMLPath)
}

func TestRenderScaffoldYAML_RootResource(t *testing.T) {
	subject, err := ResolveExplainSubject("api")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "apis:")
	assert.Contains(t, scaffold, "- ref: my-resource")
	assert.Contains(t, scaffold, "name: my-resource")
	assert.Contains(t, scaffold, "# versions:")
	assert.Contains(t, scaffold, "# type: service")
	assert.Contains(t, scaffold, "# service:")
	assert.NotContains(t, scaffold, "service_reference")
	assert.NotContains(t, scaffold, "control_plane_reference")
	assert.NotContains(t, scaffold, "spec_content")
}

func TestResolveExplainSubject_APIVersionSpecPrefersFileScalar(t *testing.T) {
	subject, err := ResolveExplainSubject("api_version.spec")
	require.NoError(t, err)

	assert.Equal(t, "!file", subject.Node.PreferredTag)
	assert.Equal(t, "!file ./specs/api.yaml", subject.Node.Literal)
}

func TestResolveExplainSubject_APIVersionSpecContentHasNoFileTag(t *testing.T) {
	subject, err := ResolveExplainSubject("api_version.spec.content")
	require.NoError(t, err)

	// The nested content field must not inherit the generic "content" !file
	// guidance; that steers users toward the double-wrapping spec.content shape.
	assert.Empty(t, subject.Node.PreferredTag)
	assert.NotContains(t, subject.Node.Literal, "!file")
}

func TestRenderScaffoldYAML_APIVersionSpecUsesFileScalar(t *testing.T) {
	for _, path := range []string{"api_version", "api.versions"} {
		t.Run(path, func(t *testing.T) {
			subject, err := ResolveExplainSubject(path)
			require.NoError(t, err)

			scaffold, err := RenderScaffoldYAML(subject)
			require.NoError(t, err)

			assert.Contains(t, scaffold, "spec: !file ./specs/api.yaml")
			assert.NotContains(t, scaffold, "content: !file")
		})
	}
}

func TestRenderScaffoldYAML_ApplicationAuthStrategyUnion(t *testing.T) {
	subject, err := ResolveExplainSubject("application_auth_strategies")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "strategy_type: key_auth")
	assert.Contains(t, scaffold, "configs:")
	assert.Contains(t, scaffold, "key-auth:")
	assert.Contains(t, scaffold, "key_names:")
	assert.Contains(t, scaffold, "# oneOf option: strategy_type=key_auth")
	assert.Contains(t, scaffold, "# oneOf option: strategy_type=openid_connect")
	assert.Contains(t, scaffold, "# strategy_type: openid_connect")
	assert.Contains(t, scaffold, "# openid-connect:")
	assert.NotContains(t, scaffold, "# ref: my-resource")
	assert.NotContains(t, scaffold, "# name: my-resource")
	assert.NotContains(t, scaffold, "app_auth_strategy_key_auth_request")
	assert.NotContains(t, scaffold, "app_auth_strategy_open_i_d_connect_request")
}

func TestRenderScaffoldYAML_AnalyticsDashboardStarterTile(t *testing.T) {
	subject, err := ResolveExplainSubject("analytics.dashboards")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "analytics:")
	assert.Contains(t, scaffold, "  dashboards:")
	assert.Contains(t, scaffold, "        tiles:")
	assert.Contains(t, scaffold, "          - type: chart")
	assert.Contains(t, scaffold, "                datasource: api_usage")
	assert.Contains(t, scaffold, "                  - request_count")
	assert.Contains(t, scaffold, "                  - time")
	assert.Contains(t, scaffold, "                type: timeseries_line")
	assert.NotContains(t, scaffold, "datasource: value")
	assert.NotContains(t, scaffold, "# oneOf option: type\n")
}

func TestRenderScaffoldYAML_EventGatewayListenerPolicyUnion(t *testing.T) {
	subject, err := ResolveExplainSubject("event_gateway_listener_policy")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "type: tls_server")
	assert.Contains(t, scaffold, "# oneOf option: type=tls_server")
	assert.Contains(t, scaffold, "# oneOf option: type=forward_to_virtual_cluster")
	assert.Contains(t, scaffold, "# oneOf option: type=port_mapping")
	assert.Contains(t, scaffold, "# type: forward_to_virtual_cluster")
	assert.Contains(t, scaffold, "# type: port_mapping")
	assert.Contains(t, scaffold, "# oneOf option: id")
	assert.Contains(t, scaffold, "# oneOf option: name")
	assert.NotContains(t, scaffold, "event_gateway_t_l_s_listener_policy")
	assert.NotContains(t, scaffold, "forward_to_virtual_cluster_policy")
	assert.NotContains(t, scaffold, "virtual_cluster_reference_by_i_d")
}

func TestRenderScaffoldYAML_NestedChildResource(t *testing.T) {
	subject, err := ResolveExplainSubject("api.versions")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "apis:")
	assert.Contains(t, scaffold, "- ref: my-resource")
	assert.Contains(t, scaffold, "versions:")
	assert.Contains(t, scaffold, "- ref: my-resource")
	assert.NotContains(t, scaffold, "api: value")
	assert.NotContains(t, scaffold, "kongctl:")
}

func TestRenderScaffoldYAML_AIGatewayModelsNestedChildResource(t *testing.T) {
	subject, err := ResolveExplainSubject("ai_gateway.models")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "ai_gateways:")
	assert.Contains(t, scaffold, "models:")
	assert.Contains(t, scaffold, "config:")
	assert.Contains(t, scaffold, "route: {}")
	assert.Contains(t, scaffold, "model: {}")
	assert.Contains(t, scaffold, "formats:")
	assert.Contains(t, scaffold, "- type: openai")
	assert.Contains(t, scaffold, "targets:")
	assert.Contains(t, scaffold, "provider: existing-provider-name")
	assert.Contains(t, scaffold, "type: model")
	assert.Contains(t, scaffold, "# type: api")
	assert.NotContains(t, scaffold, "provider: openai")
	assert.NotContains(t, scaffold, "ai_gateway: value")
	assert.NotContains(t, scaffold, "kongctl:")
}

func TestRenderScaffoldYAML_AIGatewayIdentityProviderConsumerClaims(t *testing.T) {
	subject, err := ResolveExplainSubject("ai_gateway_identity_provider")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "consumer_claims:")
	assert.NotContains(t, scaffold, "consumer_claim:")
}

func TestRenderScaffoldYAML_NestedSingletonChildResource(t *testing.T) {
	subject, err := ResolveExplainSubject("portal.auth_settings")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "portals:")
	assert.Contains(t, scaffold, "auth_settings:")
	assert.NotContains(t, scaffold, "auth_settings:\n      - ref:")
	assert.NotContains(t, scaffold, "oidc_auth_enabled")
	assert.NotContains(t, scaffold, "saml_auth_enabled")
	assert.NotContains(t, scaffold, "oidc_client_id")
}

func TestRenderScaffoldYAML_OrganizationTeamResource(t *testing.T) {
	subject, err := ResolveExplainSubject("organization_team")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "organization:")
	assert.Contains(t, scaffold, "  teams:")
	assert.Contains(t, scaffold, "    - ref: my-resource")
	assert.Contains(t, scaffold, "      name: my-resource")
}

func TestRenderScaffoldYAML_OrganizationTeamRoleNestedResource(t *testing.T) {
	subject, err := ResolveExplainSubject("organization.teams.roles")
	require.NoError(t, err)

	scaffold, err := RenderScaffoldYAML(subject)
	require.NoError(t, err)

	assert.Contains(t, scaffold, "organization:")
	assert.Contains(t, scaffold, "  teams:")
	assert.Contains(t, scaffold, "    - ref: my-resource")
	assert.Contains(t, scaffold, "      roles:")
	assert.Contains(t, scaffold, "        - ref: my-resource")
	assert.Contains(t, scaffold, "          role_name: viewer")
	assert.NotContains(t, scaffold, "team: value")
}

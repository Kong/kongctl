package resources

import (
	"fmt"
	"reflect"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func autoExplainConcreteNode[T any](hints map[string]ExplainFieldHint) (*ExplainNode, error) {
	if hints == nil {
		hints = defaultExplainHints("")
	}
	return autoExplainNode(reflect.TypeFor[T](), nil, hints, nil)
}

func autoExplainSDKUnionNode(
	typ reflect.Type,
	path []string,
	hints map[string]ExplainFieldHint,
	stack []reflect.Type,
) (*ExplainNode, bool, error) {
	typ = derefExplainType(typ)
	if typ.Kind() != reflect.Struct {
		return nil, false, nil
	}

	var branches []*ExplainNode
	for field := range typ.Fields() {
		if field.Tag.Get("union") != "member" {
			continue
		}
		memberType := derefExplainType(field.Type)
		branch, err := autoExplainNode(memberType, path, hints, stack)
		if err != nil {
			return nil, true, err
		}
		if name, value, ok := explainConstDiscriminator(memberType); ok {
			explainSetConstStringField(branch, name, value)
		}
		branches = append(branches, branch)
	}

	if len(branches) == 0 {
		return nil, false, nil
	}
	return explainUnionNode(branches...), true, nil
}

func explainConstDiscriminator(typ reflect.Type) (string, string, bool) {
	typ = derefExplainType(typ)
	if typ.Kind() != reflect.Struct {
		return "", "", false
	}
	for field := range typ.Fields() {
		value := field.Tag.Get("const")
		if value == "" {
			continue
		}
		name, _, _, skip := explainFieldName(field, "json")
		if skip || name == "" {
			name, _, _, skip = explainFieldName(field, "yaml")
		}
		if skip || name == "" {
			continue
		}
		return name, value, true
	}
	return "", "", false
}

func explainObject(fields ...*ExplainField) *ExplainNode {
	node := &ExplainNode{
		Kind:       explainKindObject,
		Properties: []*ExplainField{},
		propIndex:  make(map[string]*ExplainField),
	}
	for _, field := range fields {
		node.addField(field)
	}
	return node
}

func explainField(name string, node *ExplainNode, required, recommended bool) *ExplainField {
	return &ExplainField{Name: name, Node: node, Required: required, Recommended: recommended}
}

func explainStringNode(literal string) *ExplainNode {
	return &ExplainNode{Kind: explainKindString, Literal: literal}
}

func explainBoolNode(literal string) *ExplainNode {
	return &ExplainNode{Kind: "boolean", Literal: literal}
}

func explainConstStringNode(value string) *ExplainNode {
	return &ExplainNode{Kind: explainKindString, Const: value, Literal: value, Enum: []any{value}}
}

func explainRefField(name string, kind ResourceType, required bool) *ExplainField {
	node := explainStringNode(fmt.Sprintf("!ref my-%s", stringsForResourceRef(kind)))
	node.RefKind = string(kind)
	node.PreferredTag = "!ref"
	return explainField(name, node, required, required)
}

func explainReferenceUnion(kind ResourceType) *ExplainNode {
	return explainUnionNode(
		explainObject(explainRefField("id", kind, true)),
		explainObject(explainField("name", explainStringNode("my-resource"), true, true)),
	)
}

func explainArrayOf(item *ExplainNode) *ExplainNode {
	return &ExplainNode{Kind: explainKindArray, Items: item}
}

func stringsForResourceRef(kind ResourceType) string {
	return strings.ReplaceAll(string(kind), "_", "-")
}

func explainKongctlField() *ExplainField {
	return explainField("kongctl", explainObject(
		explainField("protected", &ExplainNode{Kind: "boolean", Nullable: true, Literal: "false"}, false, false),
		explainField(
			"namespace",
			&ExplainNode{Kind: explainKindString, Nullable: true, Literal: "default"},
			false,
			false,
		),
	), false, false)
}

func explainResourceRefField() *ExplainField {
	return explainField(SchemaFieldRef, explainStringNode("my-resource"), true, true)
}

func explainUnionNode(branches ...*ExplainNode) *ExplainNode {
	node := explainAggregateUnionNode(branches...)
	node.OneOf = append(node.OneOf, branches...)
	return node
}

func explainAggregateUnionNode(branches ...*ExplainNode) *ExplainNode {
	if len(branches) == 0 {
		return explainObject()
	}

	first := branches[0]
	node := &ExplainNode{
		Kind:         first.Kind,
		Description:  first.Description,
		Default:      first.Default,
		DefaultFrom:  first.DefaultFrom,
		Enum:         append([]any(nil), first.Enum...),
		Nullable:     first.Nullable,
		Recommended:  first.Recommended,
		PreferredTag: first.PreferredTag,
		RefKind:      first.RefKind,
		Notes:        append([]string(nil), first.Notes...),
		Literal:      first.Literal,
		Additional:   first.Additional.clone(),
	}

	sameKind := true
	for _, branch := range branches[1:] {
		if branch.Kind != first.Kind {
			sameKind = false
			break
		}
	}
	if !sameKind {
		node.Kind = "any"
	}

	if sameKind && first.Kind == explainKindObject {
		explainAddAggregateUnionFields(node, branches)
	}

	return node
}

func explainAddAggregateUnionFields(node *ExplainNode, branches []*ExplainNode) {
	fieldNames := make([]string, 0)
	fieldsByName := make(map[string][]*ExplainField)

	for _, branch := range branches {
		for _, field := range branch.Properties {
			if _, ok := fieldsByName[field.Name]; !ok {
				fieldNames = append(fieldNames, field.Name)
			}
			fieldsByName[field.Name] = append(fieldsByName[field.Name], field)
		}
	}

	for _, name := range fieldNames {
		fields := fieldsByName[name]
		node.addField(&ExplainField{
			Name:        name,
			Node:        explainAggregateUnionFieldNode(fields),
			Required:    explainUnionFieldRequired(name, fields, branches),
			Recommended: explainUnionFieldRecommended(fields),
		})
	}
}

func explainAggregateUnionFieldNode(fields []*ExplainField) *ExplainNode {
	if len(fields) == 0 {
		return nil
	}
	if len(fields) == 1 || explainUnionFieldsShareNode(fields) {
		return fields[0].Node.clone()
	}

	branches := make([]*ExplainNode, 0, len(fields))
	for _, field := range fields {
		branches = append(branches, field.Node.clone())
	}
	return explainUnionNode(branches...)
}

func explainUnionFieldRequired(name string, fields []*ExplainField, branches []*ExplainNode) bool {
	if len(fields) != len(branches) {
		return false
	}
	for _, branch := range branches {
		field, ok := branch.property(name)
		if !ok || !field.Required {
			return false
		}
	}
	return true
}

func explainUnionFieldRecommended(fields []*ExplainField) bool {
	for _, field := range fields {
		if field.Recommended {
			return true
		}
	}
	return false
}

func explainUnionFieldsShareNode(fields []*ExplainField) bool {
	first := fields[0].Node.toJSONSchema()
	for _, field := range fields[1:] {
		if !reflect.DeepEqual(first, field.Node.toJSONSchema()) {
			return false
		}
	}
	return true
}

func explainWithCommonFields(branch *ExplainNode, fields ...*ExplainField) *ExplainNode {
	node := explainObject()
	for _, field := range fields {
		node.addField(&ExplainField{
			Name:        field.Name,
			Node:        field.Node.clone(),
			Required:    field.Required,
			Recommended: field.Recommended,
		})
	}
	for _, field := range branch.Properties {
		if !node.propertyExists(field.Name) {
			node.addField(field)
		}
	}
	return node
}

func explainSetConstStringField(node *ExplainNode, name, value string) {
	field := explainField(name, explainConstStringNode(value), true, true)
	explainReplaceField(node, field)
}

func explainReplaceField(node *ExplainNode, field *ExplainField) {
	if node == nil || field == nil {
		return
	}
	for i, existing := range node.Properties {
		if existing.Name == field.Name {
			node.Properties[i] = field
			if node.propIndex != nil {
				node.propIndex[field.Name] = field
			}
			return
		}
	}
	node.addField(field)
}

func explainRemoveField(node *ExplainNode, name string) {
	if node == nil {
		return
	}
	filtered := node.Properties[:0]
	for _, field := range node.Properties {
		if field.Name != name {
			filtered = append(filtered, field)
		}
	}
	node.Properties = filtered
	if node.propIndex != nil {
		delete(node.propIndex, name)
	}
	for _, branch := range node.OneOf {
		explainRemoveField(branch, name)
	}
}

func explainReplacePath(node *ExplainNode, path []string, replacement *ExplainNode) bool {
	if len(path) == 0 {
		return false
	}
	if len(path) == 1 {
		if field, ok := node.property(path[0]); ok {
			field.Node = replacement
			return true
		}
		return false
	}
	field, ok := node.property(path[0])
	if !ok {
		return false
	}
	child := field.Node
	if child.Kind == explainKindArray && child.Items != nil {
		child = child.Items
	}
	return explainReplacePath(child, path[1:], replacement)
}

func explainSetPathRequired(node *ExplainNode, path []string) bool {
	if len(path) == 0 {
		return false
	}
	node = scaffoldActiveNode(node)
	if len(path) == 1 {
		if field, ok := node.property(path[0]); ok {
			field.Required = true
			field.Recommended = true
			return true
		}
		return false
	}
	field, ok := node.property(path[0])
	if !ok {
		return false
	}
	child := field.Node
	if child.Kind == explainKindArray && child.Items != nil {
		child = child.Items
	}
	return explainSetPathRequired(child, path[1:])
}

func apiExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	node, err := autoExplainConcreteNode[APIResource](defaultExplainHints(ResourceTypeAPI))
	if err != nil {
		return nil, err
	}
	explainRemoveField(node, "spec_content")
	return node, nil
}

func apiImplementationExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	return explainObject(
		explainField(SchemaFieldRef, explainStringNode("my-resource"), true, true),
		explainField("api", explainStringNode("my-api"), false, false),
		explainField("type", explainConstStringNode("service"), false, true),
		explainField("service", explainObject(
			explainRefField("id", ResourceTypeGatewayService, true),
			explainRefField("control_plane_id", ResourceTypeControlPlane, true),
		), true, true),
	), nil
}

func dashboardExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	hints := defaultExplainHints(ResourceTypeDashboard)
	hints["name"] = ExplainFieldHint{DefaultFrom: SchemaFieldRef, Literal: "my-resource", Recommended: new(true)}

	node, err := autoExplainConcreteNode[DashboardResource](hints)
	if err != nil {
		return nil, err
	}
	explainReplacePath(node, []string{"definition"}, dashboardDefinitionExplainNode())
	return node, nil
}

func dashboardDefinitionExplainNode() *ExplainNode {
	return explainObject(
		explainField("tiles", explainArrayOf(dashboardTileExplainNode()), true, true),
		explainField("preset_filters", explainArrayOf(dashboardFilterExplainNode()), false, false),
	)
}

func dashboardTileExplainNode() *ExplainNode {
	return explainObject(
		explainField("type", explainConstStringNode("chart"), true, true),
		explainField("layout", dashboardLayoutExplainNode(), true, true),
		explainField("definition", dashboardTileDefinitionExplainNode(), true, true),
	)
}

func dashboardLayoutExplainNode() *ExplainNode {
	return explainObject(
		explainField("position", explainObject(
			explainField("col", &ExplainNode{Kind: "integer", Literal: "0"}, true, true),
			explainField("row", &ExplainNode{Kind: "integer", Literal: "0"}, true, true),
		), true, true),
		explainField("size", explainObject(
			explainField("cols", &ExplainNode{Kind: "integer", Literal: "6"}, true, true),
			explainField("rows", &ExplainNode{Kind: "integer", Literal: "2"}, true, true),
		), true, true),
	)
}

func dashboardTileDefinitionExplainNode() *ExplainNode {
	return explainObject(
		explainField("query", dashboardQueryExplainNode(), true, true),
		explainField("chart", dashboardChartExplainNode(), true, true),
	)
}

func dashboardQueryExplainNode() *ExplainNode {
	return explainUnionNode(
		dashboardQueryBranch("api_usage", "request_count"),
		dashboardQueryBranch("llm_usage", "total_tokens"),
		dashboardQueryBranch("agentic_usage", "request_count"),
		dashboardQueryBranch("platform_usage", "request_count"),
	)
}

func dashboardQueryBranch(datasource string, metric string) *ExplainNode {
	return explainObject(
		explainField("datasource", explainConstStringNode(datasource), true, true),
		explainField("metrics", explainArrayOf(explainStringNode(metric)), false, true),
		explainField("dimensions", explainArrayOf(explainStringNode("time")), false, true),
		explainField("filters", explainArrayOf(dashboardFilterExplainNode()), false, false),
		explainField(
			"granularity",
			&ExplainNode{Kind: explainKindString, Nullable: true, Literal: "hourly"},
			false,
			false,
		),
		explainField("time_range", dashboardTimeRangeExplainNode(), false, false),
	)
}

func dashboardChartExplainNode() *ExplainNode {
	return explainUnionNode(
		dashboardChartBranch("timeseries_line", false, false),
		dashboardChartBranch("timeseries_bar", true, false),
		dashboardChartBranch("horizontal_bar", true, false),
		dashboardChartBranch("vertical_bar", true, false),
		dashboardChartBranch("single_value", false, true),
		dashboardChartBranch("donut", false, false),
		dashboardChartBranch("choropleth_map", false, false),
		dashboardChartBranch("top_n", false, false),
	)
}

func dashboardChartBranch(chartType string, stacked bool, decimalPoints bool) *ExplainNode {
	fields := []*ExplainField{
		explainField(
			"chart_title",
			&ExplainNode{Kind: explainKindString, Nullable: true, Literal: "Request count"},
			false,
			true,
		),
		explainField("type", explainConstStringNode(chartType), true, true),
	}
	if stacked {
		fields = append(fields, explainField(
			"stacked",
			&ExplainNode{Kind: "boolean", Nullable: true, Literal: "false"},
			false,
			false,
		))
	}
	if decimalPoints {
		fields = append(fields, explainField(
			"decimal_points",
			&ExplainNode{Kind: "number", Nullable: true, Literal: "1"},
			false,
			false,
		))
	}
	return explainObject(fields...)
}

func dashboardTimeRangeExplainNode() *ExplainNode {
	return explainUnionNode(
		explainObject(
			explainField("tz", &ExplainNode{Kind: explainKindString, Nullable: true, Literal: "Etc/UTC"}, false, false),
			explainField("type", explainConstStringNode("relative"), true, true),
			explainField(
				"time_range",
				&ExplainNode{Kind: explainKindString, Nullable: true, Literal: "1h"},
				false,
				false,
			),
		),
		explainObject(
			explainField("tz", &ExplainNode{Kind: explainKindString, Nullable: true, Literal: "Etc/UTC"}, false, false),
			explainField("type", explainConstStringNode("absolute"), true, true),
			explainField(
				"start",
				&ExplainNode{Kind: explainKindString, Nullable: true, Literal: "2024-01-01T00:00:00Z"},
				false,
				false,
			),
			explainField(
				"end",
				&ExplainNode{Kind: explainKindString, Nullable: true, Literal: "2024-01-01T01:00:00Z"},
				false,
				false,
			),
		),
	)
}

func dashboardFilterExplainNode() *ExplainNode {
	return explainObject(
		explainField("field", explainStringNode("control_plane"), true, true),
		explainField("operator", explainStringNode("in"), true, true),
		explainField("value", &ExplainNode{Kind: "any", Literal: "value"}, false, false),
	)
}

func applicationAuthStrategyExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	hints := defaultExplainHints(ResourceTypeApplicationAuthStrategy)
	hints["name"] = ExplainFieldHint{DefaultFrom: SchemaFieldRef, Literal: "my-resource", Recommended: new(true)}
	hints["dcr_provider_id"] = ExplainFieldHint{
		PreferredTag: "!ref",
		RefKind:      string(ResourceTypeDCRProvider),
		Literal:      "!ref my-dcr-provider",
	}

	keyAuth, err := autoExplainConcreteNode[kkComps.AppAuthStrategyKeyAuthRequest](hints)
	if err != nil {
		return nil, err
	}
	explainSetConstStringField(keyAuth, "strategy_type", "key_auth")
	explainSetPathRequired(keyAuth, []string{"configs", "key-auth", "key_names"})

	oidc, err := autoExplainConcreteNode[kkComps.AppAuthStrategyOpenIDConnectRequest](hints)
	if err != nil {
		return nil, err
	}
	explainSetConstStringField(oidc, "strategy_type", "openid_connect")
	explainSetPathRequired(oidc, []string{"configs", "openid-connect", "credential_claim"})
	explainSetPathRequired(oidc, []string{"configs", "openid-connect", "scopes"})
	explainSetPathRequired(oidc, []string{"configs", "openid-connect", "auth_methods"})

	common := []*ExplainField{explainResourceRefField(), explainKongctlField()}
	return explainUnionNode(
		explainWithCommonFields(keyAuth, common...),
		explainWithCommonFields(oidc, common...),
	), nil
}

func eventGatewayVirtualClusterExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	node, err := autoExplainConcreteNode[EventGatewayVirtualClusterResource](nil)
	if err != nil {
		return nil, err
	}
	explainReplacePath(node, []string{"destination"}, explainReferenceUnion(ResourceTypeEventGatewayBackendCluster))
	return node, nil
}

func eventGatewayBackendClusterExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	node, err := autoExplainConcreteNode[EventGatewayBackendClusterResource](nil)
	if err != nil {
		return nil, err
	}
	auth := explainDiscriminatedUnion(
		"type",
		explainVariant[kkComps.BackendClusterAuthenticationAnonymous]("type", "anonymous"),
		explainVariant[kkComps.BackendClusterAuthenticationSaslPlain]("type", "sasl_plain"),
		explainVariant[kkComps.BackendClusterAuthenticationSaslScram]("type", "sasl_scram"),
	)
	explainReplacePath(node, []string{"authentication"}, auth)
	return node, nil
}

func eventGatewaySchemaRegistryExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	confluent, err := autoExplainConcreteNode[kkComps.SchemaRegistryConfluent](nil)
	if err != nil {
		return nil, err
	}
	explainSetConstStringField(confluent, "type", "confluent")
	return explainUnionNode(explainWithCommonFields(
		confluent,
		explainResourceRefField(),
		explainRefField("event_gateway", ResourceTypeEventGatewayControlPlane, false),
	)), nil
}

func eventGatewayClusterPolicyExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	acls, err := autoExplainConcreteNode[kkComps.EventGatewayACLsPolicy](nil)
	if err != nil {
		return nil, err
	}
	explainSetConstStringField(acls, "type", "acls")
	return explainUnionNode(explainWithCommonFields(
		acls,
		explainResourceRefField(),
		explainRefField("virtual_cluster", ResourceTypeEventGatewayVirtualCluster, false),
		explainRefField("event_gateway", ResourceTypeEventGatewayControlPlane, false),
	)), nil
}

func eventGatewayProducePolicyExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	modifyHeaders, err := explainVariantNode[kkComps.EventGatewayModifyHeadersPolicyCreate]("type", "modify_headers")
	if err != nil {
		return nil, err
	}
	schemaValidation, err := explainVariantNode[kkComps.EventGatewayProduceSchemaValidationPolicy](
		"type",
		"schema_validation",
	)
	if err != nil {
		return nil, err
	}
	encrypt, err := explainVariantNode[kkComps.EventGatewayEncryptPolicy]("type", "encrypt")
	if err != nil {
		return nil, err
	}
	encryptFields, err := explainVariantNode[kkComps.EventGatewayParsedRecordEncryptFieldsPolicyCreate](
		"type",
		"encrypt_fields",
	)
	if err != nil {
		return nil, err
	}
	explainReplacePath(
		encryptFields,
		[]string{"parent_policy_id"},
		explainRefField("parent_policy_id", ResourceTypeEventGatewayProducePolicy, true).Node,
	)
	return eventGatewayVirtualClusterPolicyUnion(modifyHeaders, schemaValidation, encrypt, encryptFields), nil
}

func eventGatewayConsumePolicyExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	modifyHeaders, err := explainVariantNode[kkComps.EventGatewayModifyHeadersPolicyCreate]("type", "modify_headers")
	if err != nil {
		return nil, err
	}
	schemaValidation, err := explainVariantNode[kkComps.EventGatewayConsumeSchemaValidationPolicy](
		"type",
		"schema_validation",
	)
	if err != nil {
		return nil, err
	}
	decrypt, err := explainVariantNode[kkComps.EventGatewayDecryptPolicy]("type", "decrypt")
	if err != nil {
		return nil, err
	}
	skipRecord, err := explainVariantNode[kkComps.EventGatewaySkipRecordPolicyCreate]("type", "skip_record")
	if err != nil {
		return nil, err
	}
	decryptFields, err := explainVariantNode[kkComps.EventGatewayParsedRecordDecryptFieldsPolicyCreate](
		"type",
		"decrypt_fields",
	)
	if err != nil {
		return nil, err
	}
	explainReplacePath(
		decryptFields,
		[]string{"parent_policy_id"},
		explainRefField("parent_policy_id", ResourceTypeEventGatewayConsumePolicy, true).Node,
	)
	return eventGatewayVirtualClusterPolicyUnion(
		modifyHeaders,
		schemaValidation,
		decrypt,
		skipRecord,
		decryptFields,
	), nil
}

func eventGatewayVirtualClusterPolicyUnion(branches ...*ExplainNode) *ExplainNode {
	common := []*ExplainField{
		explainResourceRefField(),
		explainRefField("virtual_cluster", ResourceTypeEventGatewayVirtualCluster, false),
		explainRefField("event_gateway", ResourceTypeEventGatewayControlPlane, false),
	}
	withCommon := make([]*ExplainNode, 0, len(branches))
	for _, branch := range branches {
		withCommon = append(withCommon, explainWithCommonFields(branch, common...))
	}
	return explainUnionNode(withCommon...)
}

func eventGatewayListenerPolicyExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	tlsServer, err := explainVariantNode[kkComps.EventGatewayTLSListenerPolicy]("type", "tls_server")
	if err != nil {
		return nil, err
	}
	forward, err := explainVariantNode[kkComps.ForwardToVirtualClusterPolicy]("type", "forward_to_virtual_cluster")
	if err != nil {
		return nil, err
	}
	forwardConfig := explainDiscriminatedUnion(
		"type",
		explainVariant[kkComps.ForwardToClusterByPortMappingConfig]("type", "port_mapping"),
		explainVariant[kkComps.ForwardToClusterBySNIConfig]("type", "sni"),
	)
	for _, branch := range forwardConfig.OneOf {
		explainReplacePath(
			branch,
			[]string{"destination"},
			explainReferenceUnion(ResourceTypeEventGatewayVirtualCluster),
		)
	}
	explainReplacePath(forward, []string{"config"}, forwardConfig)
	explainReplacePath(
		tlsServer,
		[]string{"config", "client_authentication", "tls_trust_bundles"},
		explainArrayOf(explainReferenceUnion(ResourceTypeEventGatewayTLSTrustBundle)),
	)

	common := []*ExplainField{
		explainResourceRefField(),
		explainRefField("listener", ResourceTypeEventGatewayListener, false),
		explainRefField("event_gateway", ResourceTypeEventGatewayControlPlane, false),
	}
	return explainUnionNode(
		explainWithCommonFields(tlsServer, common...),
		explainWithCommonFields(forward, common...),
	), nil
}

func portalIdentityProviderExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	oidc, err := autoExplainConcreteNode[kkComps.OIDCIdentityProviderConfig](nil)
	if err != nil {
		return nil, err
	}
	saml, err := autoExplainConcreteNode[kkComps.SAMLIdentityProviderConfigInput](nil)
	if err != nil {
		return nil, err
	}
	config := explainUnionNode(oidc, saml)
	common := []*ExplainField{
		explainResourceRefField(),
		explainRefField(SchemaFieldPortal, ResourceTypePortal, false),
		explainField("enabled", explainBoolNode("false"), false, false),
	}
	oidcBranch := explainObject(append(
		common,
		explainField("type", explainConstStringNode("oidc"), true, true),
		explainField("config", config.OneOf[0], true, true),
	)...)
	samlBranch := explainObject(append(
		common,
		explainField("type", explainConstStringNode("saml"), true, true),
		explainField("config", config.OneOf[1], true, true),
	)...)
	return explainUnionNode(oidcBranch, samlBranch), nil
}

func portalCustomDomainExplainNode(_ ExplainBuildContext) (*ExplainNode, error) {
	node, err := autoExplainConcreteNode[PortalCustomDomainResource](nil)
	if err != nil {
		return nil, err
	}
	ssl := explainDiscriminatedUnion(
		"domain_verification_method",
		explainVariant[kkComps.HTTP]("domain_verification_method", "http"),
		explainVariant[kkComps.CustomCertificate]("domain_verification_method", "custom_certificate"),
	)
	explainReplacePath(node, []string{"ssl"}, ssl)
	return node, nil
}

func explainDiscriminatedUnion(discriminator string, variants ...explainVariantSpec) *ExplainNode {
	branches := make([]*ExplainNode, 0, len(variants))
	for _, variant := range variants {
		if variant.Node == nil {
			continue
		}
		explainSetConstStringField(variant.Node, discriminator, variant.Value)
		branches = append(branches, variant.Node)
	}
	return explainUnionNode(branches...)
}

type explainVariantSpec struct {
	Value string
	Node  *ExplainNode
}

func explainVariant[T any](discriminator, value string) explainVariantSpec {
	node, err := explainVariantNode[T](discriminator, value)
	if err != nil {
		return explainVariantSpec{}
	}
	return explainVariantSpec{Value: value, Node: node}
}

func explainVariantNode[T any](discriminator, value string) (*ExplainNode, error) {
	node, err := autoExplainConcreteNode[T](nil)
	if err != nil {
		return nil, err
	}
	explainSetConstStringField(node, discriminator, value)
	return node, nil
}

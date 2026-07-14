package resources

func aiGatewayRouteExplainNode() *ExplainNode {
	return explainObject(
		explainField("headers", &ExplainNode{Kind: explainKindObject, Additional: &ExplainNode{}}, false, false),
		explainField("hosts", explainArrayOf(explainStringNode("api.example.com")), false, false),
		explainField("https_redirect_status_code", &ExplainNode{Kind: explainKindInteger, Literal: "426"}, false, false),
		explainField("methods", explainArrayOf(explainStringNode("POST")), false, false),
		explainField("paths", explainArrayOf(explainStringNode("/v1")), false, false),
		explainField("preserve_host", explainBoolNode("false"), false, false),
		explainField("protocols", explainArrayOf(explainStringNode("https")), false, false),
		explainField("regex_priority", &ExplainNode{Kind: explainKindInteger, Literal: "0"}, false, false),
		explainField("request_buffering", explainBoolNode("true"), false, false),
		explainField("response_buffering", explainBoolNode("true"), false, false),
		explainField("strip_path", explainBoolNode("true"), false, false),
		explainField("tags", explainArrayOf(explainStringNode("ai-gateway")), false, false),
	)
}

func aiGatewayACLsExplainNode() *ExplainNode {
	return explainUnionNode(
		explainObject(explainField("allow", explainArrayOf(explainStringNode("consumer-group")), true, true)),
		explainObject(explainField("deny", explainArrayOf(explainStringNode("consumer-group")), true, true)),
	)
}

func aiGatewayAccessExplainNode(includeIdentityProviders bool) *ExplainNode {
	fields := []*ExplainField{
		explainField("acls", aiGatewayACLsExplainNode(), false, false),
	}
	if includeIdentityProviders {
		fields = append(fields, explainField(
			"identity_providers",
			explainArrayOf(explainStringNode("identity-provider-name")),
			false,
			false,
		))
	}
	return explainObject(fields...)
}

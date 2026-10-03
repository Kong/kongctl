package resources

func aiGatewayTokenExchangeExplainNode() *ExplainNode {
	conditions := explainObject(
		explainField("has_audience", explainArrayOf(explainStringNode("api.example.com")), false, false),
		explainField("missing_audience", explainArrayOf(explainStringNode("api.example.com")), false, false),
		explainField("has_scopes", explainArrayOf(explainStringNode("openid")), false, false),
		explainField("missing_scopes", explainArrayOf(explainStringNode("openid")), false, false),
	)
	conditions.Description = "Conditions on the subject token that determine whether to exchange it. " +
		"Required and non-empty when exchanging a token from the same issuer."
	issuer := explainStringNode("https://issuer.example.com")
	issuer.Description = "Issuer of the subject token eligible for exchange."
	issuers := explainArrayOf(explainObject(
		explainField("issuer", issuer, true, true),
		explainField("conditions", conditions, false, false),
	))
	issuers.Description = "Subject token issuers eligible for token exchange."
	request := explainObject(
		explainField("scopes", explainArrayOf(explainStringNode("openid")), false, false),
		explainField("audience", explainArrayOf(explainStringNode("api.example.com")), false, false),
		explainField("empty_scopes", explainBoolNode("false"), false, false),
	)
	request.Description = "Scope and audience options for the token exchange request."
	cache := explainObject(
		explainField("enabled", explainBoolNode("true"), false, false),
		explainField("ttl", &ExplainNode{Kind: explainKindInteger, Literal: "60"}, false, false),
	)
	cache.Description = "Caching options for exchanged tokens."
	node := explainObject(
		explainField("subject_token_issuers", issuers, true, true),
		explainField("request", request, false, false),
		explainField("cache", cache, false, false),
	)
	node.Description = "Token exchange configuration for an OpenID Connect AI Gateway auth strategy."
	return node
}

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

func aiGatewayAccessExplainNode(includeAuthStrategies bool) *ExplainNode {
	fields := []*ExplainField{
		explainField("acls", aiGatewayACLsExplainNode(), false, false),
	}
	if includeAuthStrategies {
		fields = append(fields, explainField(
			"auth_strategies",
			explainArrayOf(explainStringNode("auth-strategy-name")),
			false,
			false,
		))
	}
	return explainObject(fields...)
}

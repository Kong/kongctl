package resources

func aiGatewayStringField(value any, key string) string {
	payload, err := marshalObjectToMap(value, "AI Gateway")
	if err != nil {
		return ""
	}
	field, _ := payload[key].(string)
	return field
}

func stripAIGatewayServerFields(payload map[string]any) {
	delete(payload, SchemaFieldID)
	delete(payload, SchemaFieldCreatedAt)
	delete(payload, SchemaFieldUpdatedAt)
}

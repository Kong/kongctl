package planner

func preserveAIGatewayOpenProperties(
	currentAdditional map[string]any,
	currentPayload map[string]any,
	currentCompare map[string]any,
	desiredPayload map[string]any,
) map[string]any {
	updateFields := clonePayloadMap(desiredPayload)
	for key, value := range currentAdditional {
		if _, remainsMutable := currentPayload[key]; !remainsMutable {
			continue
		}
		if _, declared := desiredPayload[key]; declared {
			continue
		}
		delete(currentCompare, key)
		updateFields[key] = value
	}
	return updateFields
}

package planner

import (
	"encoding/json"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
)

func extractEventGatewayPolicyLabels(policy any) map[string]string {
	v := reflect.ValueOf(policy)
	for _, field := range v.Fields() {
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		labelsField := field.Elem().FieldByName("Labels")
		if labelsField.IsValid() && !labelsField.IsNil() {
			if labels, ok := labelsField.Interface().(map[string]string); ok {
				return labels
			}
		}
	}
	return nil
}

func extractEventGatewayPolicyDescription(policy any) string {
	v := reflect.ValueOf(policy)
	for _, field := range v.Fields() {
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		descField := field.Elem().FieldByName("Description")
		if descField.IsValid() && descField.Kind() == reflect.Pointer && !descField.IsNil() {
			return descField.Elem().String()
		}
	}
	return ""
}

func extractEventGatewayPolicyEnabled(policy any) bool {
	v := reflect.ValueOf(policy)
	for _, field := range v.Fields() {
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		enabledField := field.Elem().FieldByName("Enabled")
		if enabledField.IsValid() && enabledField.Kind() == reflect.Pointer && !enabledField.IsNil() {
			return enabledField.Elem().Bool()
		}
	}
	return true // default is enabled
}

func extractEventGatewayPolicyConfig(policy any) map[string]any {
	v := reflect.ValueOf(policy)
	for _, field := range v.Fields() {
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		configField := field.Elem().FieldByName("Config")
		if !configField.IsValid() {
			continue
		}

		data, err := json.Marshal(configField.Interface())
		if err != nil {
			continue
		}

		var config map[string]any
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}

		if len(config) > 0 {
			return config
		}
	}
	return nil
}

func eventGatewayPolicyRequiresRecreate(changedFields map[string]FieldChange) bool {
	if _, typeChanged := changedFields[FieldType]; typeChanged {
		return true
	}
	if _, parentPolicyChanged := changedFields[FieldParentPolicyID]; parentPolicyChanged {
		return true
	}
	return false
}

func (p *Planner) addEventGatewayParentPolicyReference(change *PlannedChange, resourceType string) {
	if change == nil || change.Fields == nil {
		return
	}

	parentPolicyID, ok := change.Fields[FieldParentPolicyID].(string)
	if !ok || !tags.IsRefPlaceholder(parentPolicyID) {
		return
	}

	refInfo := ReferenceInfo{
		Ref: parentPolicyID,
		ID:  resources.UnknownReferenceID,
	}
	targetRef := referenceTargetRef(parentPolicyID)
	if p.resolver != nil {
		if resource, exists := p.resolver.getResourceByTypeAndRef(resourceType, targetRef); exists {
			if moniker := resource.GetMoniker(); moniker != "" {
				refInfo.LookupFields = map[string]string{FieldName: moniker}
			}
		}
	}

	if change.References == nil {
		change.References = make(map[string]ReferenceInfo)
	}
	change.References[FieldParentPolicyID] = refInfo
}

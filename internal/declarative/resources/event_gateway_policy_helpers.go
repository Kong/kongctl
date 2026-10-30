package resources

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func matchEventGatewayPolicy(konnectID *string, moniker string, konnectResource any) bool {
	v := reflect.ValueOf(konnectResource)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}

	nameField := v.FieldByName("Name")
	idField := v.FieldByName("ID")

	if nameField.IsValid() && idField.IsValid() {
		var konnectName string
		if nameField.Kind() == reflect.Pointer && !nameField.IsNil() {
			konnectName = nameField.Elem().String()
		} else if nameField.Kind() == reflect.String {
			konnectName = nameField.String()
		}

		if idField.Kind() == reflect.String && konnectName != "" && konnectName == moniker {
			*konnectID = idField.String()
			return true
		}
	}

	return false
}

type eventGatewayPolicyMetadata struct {
	Ref            string `json:"ref"`
	VirtualCluster string `json:"virtual_cluster,omitempty"`
	EventGateway   string `json:"event_gateway,omitempty"`
	Kongctl        any    `json:"kongctl,omitempty"`
}

func unmarshalEventGatewayPolicyMetadata(data []byte) (eventGatewayPolicyMetadata, error) {
	var meta eventGatewayPolicyMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return meta, err
	}
	if meta.Kongctl != nil {
		return meta, fmt.Errorf("kongctl metadata not supported on child resources")
	}
	return meta, nil
}

func marshalEventGatewayPolicy(policy any, kind, ref, virtualCluster, eventGateway string) ([]byte, error) {
	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s policy: %w", kind, err)
	}
	var result map[string]any
	if err := json.Unmarshal(policyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s policy body: %w", kind, err)
	}
	result[SchemaFieldRef] = ref
	if virtualCluster != "" {
		result["virtual_cluster"] = virtualCluster
	}
	if eventGateway != "" {
		result[SchemaFieldEventGateway] = eventGateway
	}
	return json.Marshal(result)
}

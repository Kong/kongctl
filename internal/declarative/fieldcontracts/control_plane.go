package fieldcontracts

import (
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// Planner identifiers retain compatibility aliases for these field contracts.
const (
	FieldAuthType     = "auth_type"
	FieldClusterType  = "cluster_type"
	FieldCloudGateway = "cloud_gateway"
	FieldProxyURLs    = "proxy_urls"
)

// ControlPlaneFieldChange retains the observed and declared diagnostic values.
type ControlPlaneFieldChange struct{ Old, New any }

type controlPlaneTextField struct {
	name         string
	declaration  func(*kkComps.CreateControlPlaneRequest) *string
	observation  func(kkComps.ControlPlane) string
	create       func(*kkComps.CreateControlPlaneRequest, string)
	update       func(*kkComps.UpdateControlPlaneRequest, string)
	compareEmpty bool
	exportable   func(string) bool
}

func controlPlaneTextFieldFor[T ~string](
	name string,
	declaration func(*kkComps.CreateControlPlaneRequest) **T,
	observation func(kkComps.ControlPlane) string,
	update func(*kkComps.UpdateControlPlaneRequest, string),
	compareEmpty bool,
	exportable func(string) bool,
) controlPlaneTextField {
	return controlPlaneTextField{
		name: name,
		declaration: func(p *kkComps.CreateControlPlaneRequest) *string {
			if value := *declaration(p); value != nil {
				text := string(*value)
				return &text
			}
			return nil
		},
		observation: observation,
		create:      func(p *kkComps.CreateControlPlaneRequest, text string) { value := T(text); *declaration(p) = &value },
		update:      update, compareEmpty: compareEmpty, exportable: exportable,
	}
}

// A nil update accessor means create/export only, not an immutable-field error.
// Empty auth declarations are extracted and mapped, but ignored in comparison.
var controlPlaneTextFields = [...]controlPlaneTextField{
	controlPlaneTextFieldFor(
		FieldDescription,
		func(p *kkComps.CreateControlPlaneRequest) **string { return &p.Description },
		func(p kkComps.ControlPlane) string { return p.Description },
		func(p *kkComps.UpdateControlPlaneRequest, value string) { p.Description = &value },
		true, func(value string) bool { return strings.TrimSpace(value) != "" },
	),
	controlPlaneTextFieldFor(
		FieldClusterType,
		func(p *kkComps.CreateControlPlaneRequest) **kkComps.CreateControlPlaneRequestClusterType {
			return &p.ClusterType
		},
		func(p kkComps.ControlPlane) string { return string(p.Config.ClusterType) },
		nil, false, func(value string) bool { return value != "" },
	),
	controlPlaneTextFieldFor(
		FieldAuthType,
		func(p *kkComps.CreateControlPlaneRequest) **kkComps.AuthType { return &p.AuthType },
		func(p kkComps.ControlPlane) string { return string(p.Config.AuthType) },
		func(p *kkComps.UpdateControlPlaneRequest, value string) {
			typed := kkComps.UpdateControlPlaneRequestAuthType(value)
			p.AuthType = &typed
		},
		false, func(value string) bool { return value != "" },
	),
}

// ExtractControlPlaneFields leaves identity, labels and membership to callers.
func ExtractControlPlaneFields(desired kkComps.CreateControlPlaneRequest, fields map[string]any) {
	for _, field := range controlPlaneTextFields {
		if value := field.declaration(&desired); value != nil {
			fields[field.name] = *value
		}
	}
	if desired.CloudGateway != nil {
		fields[FieldCloudGateway] = *desired.CloudGateway
	}
	if desired.ProxyUrls != nil {
		fields[FieldProxyURLs] = desired.ProxyUrls
	}
}

// CompareControlPlaneFields compares only mutable declared fields. URL order
// matters; nil and empty observed lists compare equally to a declared empty list.
func CompareControlPlaneFields(
	desired kkComps.CreateControlPlaneRequest, current kkComps.ControlPlane,
) map[string]ControlPlaneFieldChange {
	changes := make(map[string]ControlPlaneFieldChange)
	for _, field := range controlPlaneTextFields {
		value := field.declaration(&desired)
		if field.update == nil || value == nil || (!field.compareEmpty && *value == "") {
			continue
		}
		if old := field.observation(current); old != *value {
			changes[field.name] = ControlPlaneFieldChange{Old: old, New: *value}
		}
	}
	if desired.ProxyUrls != nil && !proxyURLsEqual(current.Config.ProxyUrls, desired.ProxyUrls) {
		changes[FieldProxyURLs] = ControlPlaneFieldChange{Old: current.Config.ProxyUrls, New: desired.ProxyUrls}
	}
	return changes
}

// MapControlPlaneCreateFields preserves conversion order and leaves absent or
// nil proxy URLs untouched. The caller maps labels only after this succeeds.
func MapControlPlaneCreateFields(fields map[string]any, target *kkComps.CreateControlPlaneRequest) error {
	for _, field := range controlPlaneTextFields {
		if value, ok := fields[field.name].(string); ok {
			field.create(target, value)
		}
	}
	if value, ok := extractBool(fields[FieldCloudGateway]); ok {
		target.CloudGateway = &value
	}
	if value, ok := fields[FieldProxyURLs]; ok && value != nil {
		urls, err := convertProxyURLs(value)
		if err != nil {
			return fmt.Errorf("invalid proxy_urls value: %w", err)
		}
		target.ProxyUrls = urls
	}
	return nil
}

// MapControlPlaneUpdateField keeps the caller's iteration and error timing.
// Unknown and create-only fields are ignored; explicit nil URLs clear the slice.
func MapControlPlaneUpdateField(name string, value any, target *kkComps.UpdateControlPlaneRequest) error {
	for _, field := range controlPlaneTextFields {
		if field.name == name && field.update != nil {
			if text, ok := value.(string); ok {
				field.update(target, text)
			}
			return nil
		}
	}
	if name == FieldProxyURLs {
		urls, err := convertProxyURLs(value)
		if err != nil {
			return fmt.Errorf("invalid proxy_urls value: %w", err)
		}
		target.ProxyUrls = urls
	}
	return nil
}

// ExportControlPlaneFields omits blank descriptions, empty enums/URLs and false
// cloud_gateway. Nonblank descriptions retain their original whitespace.
func ExportControlPlaneFields(current kkComps.ControlPlane, target *kkComps.CreateControlPlaneRequest) {
	for _, field := range controlPlaneTextFields {
		if value := field.observation(current); field.exportable(value) {
			field.create(target, value)
		}
	}
	if current.Config.CloudGateway {
		value := current.Config.CloudGateway
		target.CloudGateway = &value
	}
	if len(current.Config.ProxyUrls) > 0 {
		target.ProxyUrls = current.Config.ProxyUrls
	}
}

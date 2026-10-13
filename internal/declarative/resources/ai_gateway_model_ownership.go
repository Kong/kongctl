package resources

import (
	"fmt"
	"maps"
	"strconv"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
)

// AI Gateway models use lifecycle metadata rather than restricted public labels.
const (
	ModelNamespaceKey = "kongctl-namespace"
	ModelProtectedKey = "kongctl-protected"
)

func AIGatewayModelManagedBy(model kkComps.AIGatewayModel) map[string]string {
	if model.AIGatewayModelAIGatewayModelAPI != nil {
		return model.AIGatewayModelAIGatewayModelAPI.ManagedBy
	}
	if model.AIGatewayModelAIGatewayModelModel != nil {
		return model.AIGatewayModelAIGatewayModelModel.ManagedBy
	}
	return nil
}

func (a AIGatewayModelResource) ManagedBy() map[string]string {
	if a.AIGatewayModelAPI != nil {
		return a.AIGatewayModelAPI.ManagedBy
	}
	if a.AIGatewayModelModel != nil {
		return a.AIGatewayModelModel.ManagedBy
	}
	return nil
}

func (a *AIGatewayModelResource) SetManagedBy(metadata map[string]string) {
	if a.AIGatewayModelAPI != nil {
		a.AIGatewayModelAPI.ManagedBy = metadata
	}
	if a.AIGatewayModelModel != nil {
		a.AIGatewayModelModel.ManagedBy = metadata
	}
}

// BuildModelManagedBy preserves metadata from other systems. Reserved values
// always come from kongctl configuration, never the raw SDK payload.
func BuildModelManagedBy(current, desired map[string]string, namespace string, protected bool) map[string]string {
	metadata := maps.Clone(current)
	if metadata == nil {
		metadata = make(map[string]string)
	}
	maps.Copy(metadata, desired)
	metadata[ModelNamespaceKey] = namespace
	metadata[ModelProtectedKey] = strconv.FormatBool(protected)
	return metadata
}

func (a AIGatewayModelResource) Protected() bool {
	return a.Kongctl != nil && a.Kongctl.Protected != nil && *a.Kongctl.Protected
}

func ValidateModelOwnership(metadata map[string]string, namespace string) error {
	owner := metadata[ModelNamespaceKey]
	if owner == "" {
		return fmt.Errorf(
			"AI Gateway model is unowned; explicitly adopt it into namespace %q before managing it",
			namespace,
		)
	}
	if namespace == "" || owner != namespace {
		return fmt.Errorf(
			"AI Gateway model belongs to namespace %q, not %q; explicit ownership transfer is required",
			owner,
			namespace,
		)
	}
	if value, ok := metadata[ModelProtectedKey]; ok && value != "true" && value != "false" {
		return fmt.Errorf("AI Gateway model has invalid %s metadata", ModelProtectedKey)
	}
	return nil
}

func NormalizedModelOwnership(model kkComps.AIGatewayModel) map[string]string {
	metadata := AIGatewayModelManagedBy(model)
	if metadata[ModelNamespaceKey] == "" {
		return nil
	}
	return map[string]string{
		labels.NamespaceKey: metadata[ModelNamespaceKey],
		labels.ProtectedKey: metadata[ModelProtectedKey],
	}
}

func isMetadataAlphanumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

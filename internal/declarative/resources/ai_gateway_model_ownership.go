package resources

import (
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// AI Gateway models use lifecycle metadata rather than restricted public labels.
const (
	ModelNamespaceKey = ChildNamespaceKey
	ModelProtectedKey = ChildProtectedKey
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

// BuildModelManagedBy generates ownership metadata from kongctl configuration.
func BuildModelManagedBy(
	namespace string,
	protected bool,
	version string,
) map[string]string {
	return BuildChildManagedBy(namespace, protected, version)
}

func (a AIGatewayModelResource) Protected() bool {
	return a.Kongctl.IsProtected()
}

func ValidateModelOwnership(metadata map[string]string, namespace string) error {
	return ValidateChildOwnership("AI Gateway model", metadata, namespace)
}

func NormalizedModelOwnership(model kkComps.AIGatewayModel) map[string]string {
	return NormalizedChildOwnership(AIGatewayModelManagedBy(model))
}

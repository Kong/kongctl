package helpers

import (
	"log/slog"

	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect

	"github.com/kong/kongctl/internal/config"
)

const (
	// EnvTrue is the string value "true" used for environment variable checks
	EnvTrue = "true"
)

// Provides an interface for the Konnect Go SDK
// "github.com/Kong/sdk-konnect-go" SDK struct
// allowing for easier testing and mocking
type SDKAPI interface {
	GetControlPlaneAPI() ControlPlaneAPI
	GetPortalAPI() PortalAPI
	GetAPIAPI() APIFullAPI // TODO: Change to APIAPI once refactoring is complete
	GetAPIDocumentAPI() APIDocumentAPI
	GetAPIVersionAPI() APIVersionAPI
	GetAPIPublicationAPI() APIPublicationAPI
	GetAPIImplementationAPI() APIImplementationAPI
	GetAppAuthStrategiesAPI() AppAuthStrategiesAPI
	// Portal child resource APIs
	GetPortalPageAPI() PortalPageAPI
	GetPortalCustomizationAPI() PortalCustomizationAPI
	GetPortalCustomDomainAPI() PortalCustomDomainAPI
	GetPortalSnippetAPI() PortalSnippetAPI
}

// This is the real implementation of the SDKAPI
// which wraps the actual SDK implmentation
type KonnectSDK struct {
	SDK          *kkSDK.SDK
	portalImpl *PortalAPIImpl
}

// Returns the real implementation of the GetControlPlaneAPI
// from the Konnect SDK
func (k *KonnectSDK) GetControlPlaneAPI() ControlPlaneAPI {
	return k.SDK.ControlPlanes
}

// Returns the implementation of the PortalAPI interface
func (k *KonnectSDK) GetPortalAPI() PortalAPI {
	if k.portalImpl == nil && k.SDK != nil {
		k.portalImpl = &PortalAPIImpl{
			SDK: k.SDK,
		}
	}
	return k.portalImpl
}

// Returns the implementation of the APIAPI interface
func (k *KonnectSDK) GetAPIAPI() APIFullAPI {
	if k.SDK == nil {
		return nil
	}

	return &APIAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the APIDocumentAPI interface
func (k *KonnectSDK) GetAPIDocumentAPI() APIDocumentAPI {
	if k.SDK == nil {
		return nil
	}

	return &APIDocumentAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the APIVersionAPI interface
func (k *KonnectSDK) GetAPIVersionAPI() APIVersionAPI {
	if k.SDK == nil {
		return nil
	}

	return &APIVersionAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the APIPublicationAPI interface
func (k *KonnectSDK) GetAPIPublicationAPI() APIPublicationAPI {
	if k.SDK == nil {
		return nil
	}

	return &APIPublicationAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the APIImplementationAPI interface
func (k *KonnectSDK) GetAPIImplementationAPI() APIImplementationAPI {
	if k.SDK == nil {
		return nil
	}

	return &APIImplementationAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the AppAuthStrategiesAPI interface
func (k *KonnectSDK) GetAppAuthStrategiesAPI() AppAuthStrategiesAPI {
	if k.SDK == nil {
		return nil
	}

	return &AppAuthStrategiesAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalPageAPI interface
func (k *KonnectSDK) GetPortalPageAPI() PortalPageAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalPageAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalCustomizationAPI interface
func (k *KonnectSDK) GetPortalCustomizationAPI() PortalCustomizationAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalCustomizationAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalCustomDomainAPI interface
func (k *KonnectSDK) GetPortalCustomDomainAPI() PortalCustomDomainAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalCustomDomainAPIImpl{SDK: k.SDK}
}

// Returns the implementation of the PortalSnippetAPI interface
func (k *KonnectSDK) GetPortalSnippetAPI() PortalSnippetAPI {
	if k.SDK == nil {
		return nil
	}

	return &PortalSnippetAPIImpl{SDK: k.SDK}
}

// A function that can build an SDKAPI with a given configuration
type SDKAPIFactory func(cfg config.Hook, logger *slog.Logger) (SDKAPI, error)

// DefaultSDKFactory can be overridden for testing purposes
var DefaultSDKFactory SDKAPIFactory

type Key struct{}

// A Key used to store the SDKFactory in a Context
var SDKAPIFactoryKey = Key{}

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
	GetAPIAPI() APIAPI
	GetAPIDocumentAPI() APIDocumentAPI
	GetAPIVersionAPI() APIVersionAPI
	GetAPIPublicationAPI() APIPublicationAPI
	GetAPIImplementationAPI() APIImplementationAPI
	GetAppAuthStrategiesAPI() AppAuthStrategiesAPI
}

// This is the real implementation of the SDKAPI
// which wraps the actual SDK implmentation
type KonnectSDK struct {
	SDK          *kkSDK.SDK
	publicPortal *PublicPortalAPI
}

// Returns the real implementation of the GetControlPlaneAPI
// from the Konnect SDK
func (k *KonnectSDK) GetControlPlaneAPI() ControlPlaneAPI {
	return k.SDK.ControlPlanes
}

// Returns the implementation of the PortalAPI interface
// for accessing the Developer Portal APIs using the public SDK
func (k *KonnectSDK) GetPortalAPI() PortalAPI {
	if k.publicPortal == nil && k.SDK != nil {
		k.publicPortal = &PublicPortalAPI{
			SDK: k.SDK,
		}
	}
	return k.publicPortal
}

// Returns the implementation of the APIAPI interface
// for accessing the API APIs using the public SDK
func (k *KonnectSDK) GetAPIAPI() APIAPI {
	if k.SDK == nil {
		return nil
	}

	return &PublicAPIAPI{SDK: k.SDK}
}

// Returns the implementation of the APIDocumentAPI interface
// for accessing the API Document APIs using the public SDK
func (k *KonnectSDK) GetAPIDocumentAPI() APIDocumentAPI {
	if k.SDK == nil {
		return nil
	}

	return &PublicAPIDocumentAPI{SDK: k.SDK}
}

// Returns the implementation of the APIVersionAPI interface
// for accessing the API Version APIs using the public SDK
func (k *KonnectSDK) GetAPIVersionAPI() APIVersionAPI {
	if k.SDK == nil {
		return nil
	}

	return &PublicAPIVersionAPI{SDK: k.SDK}
}

// Returns the implementation of the APIPublicationAPI interface
// for accessing the API Publication APIs using the public SDK
func (k *KonnectSDK) GetAPIPublicationAPI() APIPublicationAPI {
	if k.SDK == nil {
		return nil
	}

	return &PublicAPIPublicationAPI{SDK: k.SDK}
}

// Returns the implementation of the APIImplementationAPI interface
// for accessing the API Implementation APIs using the public SDK
func (k *KonnectSDK) GetAPIImplementationAPI() APIImplementationAPI {
	if k.SDK == nil {
		return nil
	}

	return &PublicAPIImplementationAPI{SDK: k.SDK}
}

// Returns the implementation of the AppAuthStrategiesAPI interface
// for accessing the App Auth Strategies APIs using the public SDK
func (k *KonnectSDK) GetAppAuthStrategiesAPI() AppAuthStrategiesAPI {
	if k.SDK == nil {
		return nil
	}

	return &PublicAppAuthStrategiesAPI{SDK: k.SDK}
}

// A function that can build an SDKAPI with a given configuration
type SDKAPIFactory func(cfg config.Hook, logger *slog.Logger) (SDKAPI, error)

type Key struct{}

// A Key used to store the SDKFactory in a Context
var SDKAPIFactoryKey = Key{}

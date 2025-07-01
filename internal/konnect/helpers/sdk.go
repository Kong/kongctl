package helpers

import (
	"fmt"
	"log/slog"
	"os"

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

// debugLogger creates a debug logging function that checks KONGCTL_DEBUG env var
func debugLogger() func(string, ...interface{}) {
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == EnvTrue
	return func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}
}

// Returns the implementation of the APIAPI interface
// for accessing the API APIs using the public SDK
func (k *KonnectSDK) GetAPIAPI() APIAPI {
	debugLog := debugLogger()
	debugLog("GetAPIAPI called")

	if k.SDK == nil {
		debugLog("KonnectSDK.SDK is nil")
		return nil
	}

	debugLog("Successfully returning API API")
	return &PublicAPIAPI{SDK: k.SDK}
}

// Returns the implementation of the APIDocumentAPI interface
// for accessing the API Document APIs using the public SDK
func (k *KonnectSDK) GetAPIDocumentAPI() APIDocumentAPI {
	debugLog := debugLogger()
	debugLog("GetAPIDocumentAPI called")

	if k.SDK == nil {
		debugLog("KonnectSDK.SDK is nil")
		return nil
	}

	debugLog("Successfully returning APIDocument API")
	return &PublicAPIDocumentAPI{SDK: k.SDK}
}

// Returns the implementation of the APIVersionAPI interface
// for accessing the API Version APIs using the public SDK
func (k *KonnectSDK) GetAPIVersionAPI() APIVersionAPI {
	debugLog := debugLogger()
	debugLog("GetAPIVersionAPI called")

	if k.SDK == nil {
		debugLog("KonnectSDK.SDK is nil")
		return nil
	}

	debugLog("Successfully returning APIVersion API")
	return &PublicAPIVersionAPI{SDK: k.SDK}
}

// Returns the implementation of the APIPublicationAPI interface
// for accessing the API Publication APIs using the public SDK
func (k *KonnectSDK) GetAPIPublicationAPI() APIPublicationAPI {
	debugLog := debugLogger()
	debugLog("GetAPIPublicationAPI called")

	if k.SDK == nil {
		debugLog("KonnectSDK.SDK is nil")
		return nil
	}

	debugLog("Successfully returning APIPublication API")
	return &PublicAPIPublicationAPI{SDK: k.SDK}
}

// Returns the implementation of the APIImplementationAPI interface
// for accessing the API Implementation APIs using the public SDK
func (k *KonnectSDK) GetAPIImplementationAPI() APIImplementationAPI {
	debugLog := debugLogger()
	debugLog("GetAPIImplementationAPI called")

	if k.SDK == nil {
		debugLog("KonnectSDK.SDK is nil")
		return nil
	}

	debugLog("Successfully returning APIImplementation API")
	return &PublicAPIImplementationAPI{SDK: k.SDK}
}

// Returns the implementation of the AppAuthStrategiesAPI interface
// for accessing the App Auth Strategies APIs using the public SDK
func (k *KonnectSDK) GetAppAuthStrategiesAPI() AppAuthStrategiesAPI {
	debugLog := debugLogger()

	debugLog("GetAppAuthStrategiesAPI called")

	if k.SDK == nil {
		debugLog("KonnectSDK.SDK is nil")
		return nil
	}

	debugLog("Successfully returning AppAuthStrategies API")
	return &PublicAppAuthStrategiesAPI{SDK: k.SDK}
}

// A function that can build an SDKAPI with a given configuration
type SDKAPIFactory func(cfg config.Hook, logger *slog.Logger) (SDKAPI, error)

type Key struct{}

// A Key used to store the SDKFactory in a Context
var SDKAPIFactoryKey = Key{}

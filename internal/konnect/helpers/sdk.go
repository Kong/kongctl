package helpers

import (
	"fmt"
	"log/slog"
	"os"

	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	kkInternal "github.com/Kong/sdk-konnect-go-internal"

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
	SDK                       *kkSDK.SDK
	InternalSDK               *kkInternal.SDK
	internalPortal            *InternalPortalAPI
	internalAPI               *InternalAPIAPI
	internalAPIDocument       *InternalAPIDocumentAPI
	internalAPIVersion        *InternalAPIVersionAPI
	internalAPIPublication    *InternalAPIPublicationAPI
	internalAPIImplementation *InternalAPIImplementationAPI
}

// Returns the real implementation of the GetControlPlaneAPI
// from the Konnect SDK
func (k *KonnectSDK) GetControlPlaneAPI() ControlPlaneAPI {
	return k.SDK.ControlPlanes
}

// Returns the implementation of the PortalAPI interface
// for accessing the Developer Portal APIs using the internal SDK
func (k *KonnectSDK) GetPortalAPI() PortalAPI {
	if k.internalPortal == nil && k.InternalSDK != nil {
		k.internalPortal = &InternalPortalAPI{
			SDK: k.InternalSDK,
		}
	}
	return k.internalPortal
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
// for accessing the API APIs using the internal SDK
func (k *KonnectSDK) GetAPIAPI() APIAPI {
	// Check if debug flag is set in environment
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == EnvTrue

	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}

	debugLog("GetAPIAPI called")

	if k.InternalSDK == nil {
		debugLog("KonnectSDK.InternalSDK is nil in GetAPIAPI")
		return nil
	}

	if k.internalAPI == nil && k.InternalSDK != nil {
		k.internalAPI = &InternalAPIAPI{
			SDK: k.InternalSDK,
		}
	}

	// Check if we have an SDK and it has a valid API field
	if k.internalAPI == nil {
		debugLog("k.internalAPI is nil")
	} else if k.internalAPI.SDK == nil {
		debugLog("k.internalAPI.SDK is nil")
	} else if k.internalAPI.SDK.API == nil {
		debugLog("k.internalAPI.SDK.API is nil")
	} else {
		debugLog("Successfully created APIAPI implementation")
	}

	return k.internalAPI
}

// Returns the implementation of the APIDocumentAPI interface
// for accessing the API Document APIs using the internal SDK
func (k *KonnectSDK) GetAPIDocumentAPI() APIDocumentAPI {
	debugLog := debugLogger()

	debugLog("GetAPIDocumentAPI called")

	if k.InternalSDK == nil {
		debugLog("KonnectSDK.InternalSDK is nil")
		return nil
	}

	if k.InternalSDK.APIDocumentation == nil {
		debugLog("KonnectSDK.InternalSDK.APIDocumentation is nil")
	} else {
		debugLog("KonnectSDK.InternalSDK.APIDocumentation is NOT nil")
	}

	if k.internalAPIDocument == nil && k.InternalSDK != nil {
		debugLog("Creating new InternalAPIDocumentAPI")
		k.internalAPIDocument = &InternalAPIDocumentAPI{
			SDK: k.InternalSDK,
		}
	}
	return k.internalAPIDocument
}

// Returns the implementation of the APIVersionAPI interface
// for accessing the API Version APIs using the internal SDK
func (k *KonnectSDK) GetAPIVersionAPI() APIVersionAPI {
	debugLog := debugLogger()

	debugLog("GetAPIVersionAPI called")

	if k.InternalSDK == nil {
		debugLog("KonnectSDK.InternalSDK is nil")
		return nil
	}

	if k.InternalSDK.APIVersion == nil {
		debugLog("KonnectSDK.InternalSDK.APIVersion is nil")
	} else {
		debugLog("KonnectSDK.InternalSDK.APIVersion is NOT nil")
	}

	if k.internalAPIVersion == nil && k.InternalSDK != nil {
		debugLog("Creating new InternalAPIVersionAPI")
		k.internalAPIVersion = &InternalAPIVersionAPI{
			SDK: k.InternalSDK,
		}
	}
	return k.internalAPIVersion
}

// Returns the implementation of the APIPublicationAPI interface
// for accessing the API Publication APIs using the internal SDK
func (k *KonnectSDK) GetAPIPublicationAPI() APIPublicationAPI {
	// Check if debug flag is set in environment
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"

	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}

	debugLog("GetAPIPublicationAPI called")

	if k.InternalSDK == nil {
		debugLog("KonnectSDK.InternalSDK is nil")
		return nil
	}

	if k.InternalSDK.APIPublication == nil {
		debugLog("KonnectSDK.InternalSDK.APIPublication is nil")
	} else {
		debugLog("KonnectSDK.InternalSDK.APIPublication is NOT nil")
	}

	if k.internalAPIPublication == nil && k.InternalSDK != nil {
		debugLog("Creating new InternalAPIPublicationAPI")
		k.internalAPIPublication = &InternalAPIPublicationAPI{
			SDK: k.InternalSDK,
		}
	}
	return k.internalAPIPublication
}

// Returns the implementation of the APIImplementationAPI interface
// for accessing the API Implementation APIs using the internal SDK
func (k *KonnectSDK) GetAPIImplementationAPI() APIImplementationAPI {
	// Check if debug flag is set in environment
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == EnvTrue

	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}

	debugLog("GetAPIImplementationAPI called")

	if k.InternalSDK == nil {
		debugLog("KonnectSDK.InternalSDK is nil")
		return nil
	}

	if k.InternalSDK.APIImplementation == nil {
		debugLog("KonnectSDK.InternalSDK.APIImplementation is nil")
	} else {
		debugLog("KonnectSDK.InternalSDK.APIImplementation is NOT nil")
	}

	if k.internalAPIImplementation == nil && k.InternalSDK != nil {
		debugLog("Creating new InternalAPIImplementationAPI")
		k.internalAPIImplementation = &InternalAPIImplementationAPI{
			SDK: k.InternalSDK,
		}
	}
	return k.internalAPIImplementation
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

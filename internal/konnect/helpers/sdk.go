package helpers

import (
	"fmt"
	"log/slog"
	"os"

	kkInternal "github.com/Kong/sdk-konnect-go-internal"
	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	"github.com/kong/kongctl/internal/config"
)

// Provides an interface for the Konnect Go SDK
// "github.com/Kong/sdk-konnect-go" SDK struct
// allowing for easier testing and mocking
type SDKAPI interface {
	GetControlPlaneAPI() ControlPlaneAPI
	GetPortalAPI() PortalAPI
	GetAPIAPI() APIAPI
	GetAPIDocumentAPI() APIDocumentAPI
}

// This is the real implementation of the SDKAPI
// which wraps the actual SDK implmentation
type KonnectSDK struct {
	SDK                *kkSDK.SDK
	InternalSDK        *kkInternal.SDK
	internalPortal     *InternalPortalAPI
	internalAPI        *InternalAPIAPI
	internalAPIDocument *InternalAPIDocumentAPI
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

// Returns the implementation of the APIAPI interface
// for accessing the API APIs using the internal SDK
func (k *KonnectSDK) GetAPIAPI() APIAPI {
	// Check if debug flag is set in environment
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"
	
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
	// Check if debug flag is set in environment
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"
	
	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}
	
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

// A function that can build an SDKAPI with a given configuration
type SDKAPIFactory func(cfg config.Hook, logger *slog.Logger) (SDKAPI, error)

type Key struct{}

// A Key used to store the SDKFactory in a Context
var SDKAPIFactoryKey = Key{}
